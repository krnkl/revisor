package revisor

import (
	"encoding/json"
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
func NewVerifier(definitionPath string, options ...option) (func(*http.Request, *http.Response) error, error) {
	a, err := newAPIVerifier(definitionPath)
	a.setOptions(options...)
	return a.verify, err
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
func (a *apiVerifier) verifyRequest(*http.Request) error {
	return nil
}

// verifyResponse verifies if the response is valid according to OpenAPI definition
// and configured options
func (a *apiVerifier) verifyResponse(req *http.Request, res *http.Response) (errorParam error) {
	response, err := a.getResponseDef(req, res)
	if err != nil {
		errorParam = err
		return
	}

	err = checkIfSchemaOrResponseEmpty(response.Schema, res)
	if err != nil {
		return errors.Wrap(err, "either defined schema or response body is empty")
	}

	var decoded map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&decoded)
	if err != nil {
		return errors.Wrap(err, "failed to read and decode response body")
	}
	defer func() {
		err = res.Body.Close()
		if err != nil {
			errorParam = err
		}
	}()
	errorParam = validate.AgainstSchema(response.Schema, decoded, strfmt.Default)
	return
}

func (a *apiVerifier) getResponseDef(req *http.Request, res *http.Response) (*spec.Response, error) {
	pathTmpl, _, ok := a.mapper.mapRequest(req)
	if !ok {
		return nil, errors.New("no path template matches current request")
	}
	pathDef, ok := a.doc.Spec().Paths.Paths[pathTmpl]
	if !ok {
		return nil, errors.New("no matching path template found in the definition")
	}
	response, err := a.responseByMethodAndStatus(req.Method, res.StatusCode, &pathDef)
	if err != nil {
		return nil, errors.Wrap(err, "response not valid")
	}
	return response, nil
}

func checkIfSchemaOrResponseEmpty(schema *spec.Schema, res *http.Response) error {
	if schema == nil && (res.ContentLength != -1 && res.ContentLength != 0) {
		return errors.New("schema is not defined")
	}

	if schema != nil && (res.ContentLength == -1 || res.ContentLength == 0) {
		return errors.New("response body is empty")
	}
	return nil
}

func (a *apiVerifier) verify(req *http.Request, res *http.Response) error {
	err := a.verifyRequest(req)
	if err != nil {
		return nil
	}
	return a.verifyResponse(req, res)
}

func (a *apiVerifier) setOptions(options ...option) {
	for _, opt := range options {
		opt(a)
	}
}

func (a *apiVerifier) responseByMethodAndStatus(method string, status int, pathDef *spec.PathItem) (*spec.Response, error) {
	var operation *spec.Operation = nil
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
	response := operation.OperationProps.Responses.Default
	if def, ok := operation.OperationProps.Responses.StatusCodeResponses[status]; ok {
		response = &def
	}
	if response == nil {
		return nil, errors.New("neither default nor response schema for current status code is defined")
	}
	return response, nil
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
	a.doc, err = doc.Expanded(nil)
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
