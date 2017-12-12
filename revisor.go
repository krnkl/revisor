package revisor

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
	"github.com/pkg/errors"
)

const (
	ver2 = "2.0"
)

type option func(*apiVerifier)

// SetSomeOption is an example implementation of option setter function
func SetSomeOption(opt string) option {
	return func(a *apiVerifier) {
		a.opt = opt
	}
}

// NewRequestVerifier returns a function that can be used to verify if request
// satisfies OpenAPI definition constraints
func NewRequestVerifier(definitionPath string, options ...option) (func(*http.Request) error, error) {
	a, err := newAPIVerifier(definitionPath)
	a.setOptions(options...)
	return a.verifyRequest, err
}

// NewVerifier returns a function that can be used to verify both - a request
// and the response made in the context of the request
func NewVerifier(definitionPath string, options ...option) (func(*http.Response, *http.Request) error, error) {
	a, err := newAPIVerifier(definitionPath)
	a.setOptions(options...)
	return a.verifyRequestAndReponse, err
}

func newAPIVerifier(definitionPath string) (*apiVerifier, error) {

	b, err := swag.LoadFromFileOrHTTP(definitionPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load definition")
	}

	a := &apiVerifier{
		definitionPath: definitionPath,
	}
	err = a.initDocument(b)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build Document")
	}

	err = a.initMapper()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request mapper")
	}
	return a, nil
}

// apiVerifier implements various verification functions and encloses various
// verification options as well as an OpenAPI Document
type apiVerifier struct {
	definitionPath string
	opt            string
	mapper         *simpleMapper
	doc            *loads.Document
}

// verifyRequest verifies if request is valid according to OpenAPI definition
// and configured options
func (a *apiVerifier) verifyRequest(req *http.Request) error {
	requestDef, err := a.getRequestDef(req)
	if err != nil {
		return err
	}
	if requestDef.Required {
		err = checkIfSchemaOrBodyIsEmpty(requestDef.Schema, req.ContentLength)
		if err != nil {
			return errors.Wrap(err, "either defined schema or request body is empty")
		}
	}

	decoded, err := decodeBody(req.Body)
	if err != nil {
		return errors.Wrap(err, "failed to decode request")
	}
	return validate.AgainstSchema(requestDef.Schema, decoded, strfmt.Default)

}

// verifyResponse verifies if the response is valid according to OpenAPI definition
// and configured options
func (a *apiVerifier) verifyResponse(res *http.Response, req *http.Request) error {
	response, err := a.getResponseDef(req, res)
	if err != nil {
		return err
	}

	err = checkIfSchemaOrBodyIsEmpty(response.Schema, res.ContentLength)
	if err != nil {
		return errors.Wrap(err, "either defined schema or response body is empty")
	}

	decoded, err := decodeBody(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to decode response")
	}
	return validate.AgainstSchema(response.Schema, decoded, strfmt.Default)
}

// getRequestDef checks parameters defined on both Path and Operation components
// returns an error if no body parameters were found
func (a *apiVerifier) getRequestDef(req *http.Request) (*spec.Parameter, error) {
	pathDef, err := a.getPathDef(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pathItem defintiion")
	}
	operation, err := a.operationByMethod(req.Method, pathDef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get operation defintiion")
	}
	reqBodyParameter := getBodyParameter(operation.Parameters)
	if reqBodyParameter == nil {
		reqBodyParameter = getBodyParameter(pathDef.Parameters)
	}
	if reqBodyParameter == nil {
		return nil, errors.New("no body parameter definition found")
	}
	return reqBodyParameter, nil
}

// getBodyParameter filters parameter of type body from list of parameters
// it returns first occurence of such parameter
func getBodyParameter(params []spec.Parameter) *spec.Parameter {
	for _, parameter := range params {
		if parameter.In == "body" {
			return &parameter
		}
	}
	return nil
}

func (a *apiVerifier) getPathDef(req *http.Request) (*spec.PathItem, error) {

	pathTmpl, _, ok := a.mapper.mapRequest(req)
	if !ok {
		return nil, errors.New("no path template matches current request")
	}
	pathDef, ok := a.doc.Spec().Paths.Paths[pathTmpl]
	if !ok {
		return nil, errors.New("no path item definition found for path template")
	}
	return &pathDef, nil
}

func (a *apiVerifier) getResponseDef(req *http.Request, res *http.Response) (*spec.Response, error) {

	pathDef, err := a.getPathDef(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get path item defintiion")
	}
	response, err := a.responseByMethodAndStatus(req.Method, res.StatusCode, pathDef)
	if err != nil {
		return nil, errors.Wrap(err, "response not valid")
	}
	return response, nil
}

// checkIfSchemaOrBodyIsEmpty accepts Schema definition and length
// of either request or response body
func checkIfSchemaOrBodyIsEmpty(schema *spec.Schema, contentLen int64) error {
	if schema == nil && (contentLen != -1 && contentLen != 0) {
		return errors.New("schema is not defined")
	}
	if schema != nil && (contentLen == -1 || contentLen == 0) {
		return errors.New("body is empty")
	}
	return nil
}

func (a *apiVerifier) verifyRequestAndReponse(res *http.Response, req *http.Request) error {
	var report error
	err := a.verifyRequest(req)
	if err != nil {
		report = errors.Wrap(err, "request validation failed")
		if res != nil && res.StatusCode < 400 {
			report = errors.Wrap(err, "request validation failed but response status code is ok")
		}
	}

	err = a.verifyResponse(res, req)
	if err != nil {
		report = errors.Wrap(err, "response validation failed")
	}
	return report
}

func (a *apiVerifier) setOptions(options ...option) {
	for _, opt := range options {
		opt(a)
	}
}

func (a *apiVerifier) responseByMethodAndStatus(method string, status int, pathDef *spec.PathItem) (*spec.Response, error) {
	operation, err := a.operationByMethod(method, pathDef)
	if err != nil {
		return nil, errors.Wrap(err, "response definition not configured")
	}
	response := operation.OperationProps.Responses.Default
	if def, ok := operation.OperationProps.Responses.StatusCodeResponses[status]; ok {
		response = &def
	}
	if response == nil {
		return nil, errors.New("neither default nor response schema for current status code is defined")
	}
	return response, nil
}

func (a *apiVerifier) operationByMethod(method string, pathDef *spec.PathItem) (*spec.Operation, error) {
	var operation *spec.Operation
	switch method {
	case http.MethodGet:
		operation = pathDef.Get
	case http.MethodPut:
		operation = pathDef.Put
	case http.MethodPost:
		operation = pathDef.Post
	case http.MethodDelete:
		operation = pathDef.Delete
	case http.MethodOptions:
		operation = pathDef.Options
	case http.MethodHead:
		operation = pathDef.Head
	case http.MethodPatch:
		operation = pathDef.Patch
	}
	if operation == nil {
		return nil, errors.New("no operation configured for method: " + method)
	}
	return operation, nil
}

func (a *apiVerifier) initDocument(raw []byte) error {
	rawJSON := json.RawMessage(raw)
	if swag.YAMLMatcher(a.definitionPath) {
		yamlDoc, err := swag.BytesToYAMLDoc(raw)
		if err != nil {
			return errors.Wrap(err, "failed to parse yaml document")
		}
		rawJSON, err = swag.YAMLToJSON(yamlDoc)
		if err != nil {
			return errors.Wrap(err, "failed to convert doc to json")
		}
	}
	doc, err := loads.Analyzed(rawJSON, ver2)
	if err != nil {
		return errors.Wrap(err, "failed to load swagger spec")
	}
	a.doc, err = doc.Expanded(&spec.ExpandOptions{RelativeBase: a.definitionPath})
	if err != nil {
		return errors.Wrap(err, "failed to expand document")
	}
	return nil
}

func (a *apiVerifier) initMapper() error {
	requestsMap := make(map[string][]string)
	for path, pathItem := range a.doc.Spec().Paths.Paths {
		if pathItem.Get != nil {
			requestsMap[http.MethodGet] = append(requestsMap[http.MethodGet], path)
		}
		if pathItem.Put != nil {
			requestsMap[http.MethodPut] = append(requestsMap[http.MethodPut], path)
		}
		if pathItem.Post != nil {
			requestsMap[http.MethodPost] = append(requestsMap[http.MethodPost], path)
		}
		if pathItem.Delete != nil {
			requestsMap[http.MethodDelete] = append(requestsMap[http.MethodDelete], path)
		}
		if pathItem.Options != nil {
			requestsMap[http.MethodOptions] = append(requestsMap[http.MethodOptions], path)
		}
		if pathItem.Head != nil {
			requestsMap[http.MethodHead] = append(requestsMap[http.MethodHead], path)
		}
		if pathItem.Patch != nil {
			requestsMap[http.MethodPatch] = append(requestsMap[http.MethodPatch], path)
		}
	}
	a.mapper = newSimpleMapper(requestsMap)
	return nil
}

func decodeBody(r io.ReadCloser) (decoded interface{}, errorParam error) {

	err := json.NewDecoder(r).Decode(&decoded)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read and decode")
	}
	defer func() {
		err = r.Close()
		if err != nil {
			errorParam = err
		}
	}()
	return
}
