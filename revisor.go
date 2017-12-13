package revisor

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

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

// options is a struct that holds all possible options
type options struct {
	strictContentType bool
}

/// StrictContentType is an example implementation of option setter function
// TODO add proper description
func StrictContentType(opt bool) option {
	return func(a *apiVerifier) {
		a.opts.strictContentType = opt
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
		// TODO setup defaults opts in separate func
		opts: options{strictContentType: true},
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
	opts           options
	mapper         *simpleMapper
	doc            *loads.Document
}

// verifyRequest verifies if request is valid according to OpenAPI definition
// and configured options
func (a *apiVerifier) verifyRequest(req *http.Request) error {
	requestDef, consumes, err := a.getRequestDef(req)
	if err != nil {
		return err
	}
	body, err := readRequestBody(req)
	if err != nil {
		return errors.Wrap(err, "failed to verify request")
	}
	if requestDef != nil {
		if requestDef.Required {
			err = checkIfSchemaOrBodyIsEmpty(requestDef.Schema, len(body))
			if err != nil {
				return errors.Wrap(err, "either defined schema or request body is empty")
			}
		}
		contentType, err := a.matchRequestContentType(req.Header.Get("Content-Type"), consumes)
		if err != nil {
			return err
		}

		decoded, err := decodeBody(contentType, body)
		if err != nil {
			return errors.Wrap(err, "failed to decode request")
		}
		return validate.AgainstSchema(requestDef.Schema, decoded, strfmt.Default)
	}
	return nil
}

// verifyResponse verifies if the response is valid according to OpenAPI definition
// and configured options
func (a *apiVerifier) verifyResponse(res *http.Response, req *http.Request) error {
	response, err := a.getResponseDef(req, res)
	if err != nil {
		return err
	}
	body, err := readResponseBody(res)
	if err != nil {
		return errors.Wrap(err, "response not valid")
	}
	err = checkIfSchemaOrBodyIsEmpty(response.Schema, len(body))
	if err != nil {
		return errors.Wrap(err, "either defined schema or response body is empty")
	}

	decoded, err := decodeBody("application/json", body)
	if err != nil {
		return errors.Wrap(err, "failed to decode response")
	}
	return validate.AgainstSchema(response.Schema, decoded, strfmt.Default)
}

// getRequestDef checks parameters defined on both Path and Operation components
// Second return parameter is a slice of mime types that can be consumed by operation
// returns an error if no body parameters were found
func (a *apiVerifier) getRequestDef(req *http.Request) (*spec.Parameter, []string, error) {
	pathDef, err := a.getPathDef(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get pathItem defintiion")
	}
	operation, err := a.operationByMethod(req.Method, pathDef)

	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get operation defintiion")
	}
	reqBodyParameter := getBodyParameter(operation.Parameters)
	if reqBodyParameter == nil {
		reqBodyParameter = getBodyParameter(pathDef.Parameters)
	}
	consumes := operation.Consumes
	if len(consumes) == 0 {
		consumes = a.doc.Spec().Consumes
	}
	return reqBodyParameter, consumes, nil
}

func (a *apiVerifier) matchRequestContentType(contentType string, consumes []string) (string, error) {
	if len(consumes) == 0 && a.opts.strictContentType {
		return "", errors.New("consumes is empty")
	}
	matched := strings.Trim(contentType, " ")
	for _, typeStr := range consumes {
		target := strings.Trim(typeStr, " ")
		if a.opts.strictContentType {
			if strings.Compare(matched, target) == 0 {
				break
			}
		} else {
			match, err := regexp.MatchString(target, matched)
			if match || err != nil {
				return target, err
			}
		}
	}
	return matched, nil
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
func checkIfSchemaOrBodyIsEmpty(schema *spec.Schema, bodyLen int) error {
	if schema == nil && bodyLen != 0 {
		return errors.New("schema is not defined")
	}
	if schema != nil && bodyLen == 0 {
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

func decodeBody(contentType string, body []byte) (interface{}, error) {
	decoder := getDecoder(contentType)
	decoded, err := decoder(body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode")
	}
	return decoded, nil
}

// readRequestBody reads contents from the request body and returns slice of bytes
// ReadCloser associated with request will be assigned a new buffer value,
// so that upstream calls will be able to read the body again.
func readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return []byte{}, nil
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, errors.Wrap(err, "error reading request body")
	}
	r.Body = ioutil.NopCloser(bytes.NewReader(body))
	return body, nil
}

// readResponseBody reads contents from the response body and returns slice of bytes
// ReadCloser associated with reponse will be assigned a new buffer value,
// so that upstream calls will be able to read the body again.
func readResponseBody(r *http.Response) ([]byte, error) {
	if r.Body == nil {
		return []byte{}, nil
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, errors.Wrap(err, "error reading response body")
	}
	r.Body = ioutil.NopCloser(bytes.NewReader(body))
	return body, nil
}

func getDecoder(contentType string) func([]byte) (interface{}, error) {
	if strings.Contains(contentType, "json") {
		return jsonDecoder
	}
	return func([]byte) (interface{}, error) {
		return nil, errors.New("failed to decode as " + contentType)
	}
}

func nopDecoder([]byte) (interface{}, error) {
	return nil, nil
}

func jsonDecoder(b []byte) (decoded interface{}, err error) {
	err = json.Unmarshal(b, &decoded)
	if err != nil {
		err = errors.Wrap(err, "failed to decode json")
		decoded = nil
	}
	return
}
