package revisor

import (
	"encoding/json"
	"net/http"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/swag"
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

// NewResponseVerifier returns a function that can be used to verify if response
// satisfies OpenAPI definition constraints. Two perform such alidation we need
// to specify context in which current response was received, and that is represented
// by method and url paramters
func NewResponseVerifier(definitionPath, method, url string, options ...option) (func(*http.Response) error, error) {
	a, err := newAPIVerifier(definitionPath)
	a.setOptions(options...)
	return func(res *http.Response) error {
		return a.verifyResponse(method, url, res)
	}, err
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
func (a *apiVerifier) verifyResponse(method, url string, res *http.Response) error {
	return nil
}

func (a *apiVerifier) verify(req *http.Request, res *http.Response) error {
	err := a.verifyRequest(req)
	if err != nil {
		return nil
	}
	return a.verifyResponse(req.Method, req.URL.Path, res)
}

func (a *apiVerifier) setOptions(options ...option) {
	for _, opt := range options {
		opt(a)
	}
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
			continue
		}
		if pathItem.Put != nil {
			requestsMap[http.MethodPut] = append(requestsMap[http.MethodPut], path)
			continue
		}
		if pathItem.Post != nil {
			requestsMap[http.MethodPost] = append(requestsMap[http.MethodPost], path)
			continue
		}
		if pathItem.Delete != nil {
			requestsMap[http.MethodDelete] = append(requestsMap[http.MethodDelete], path)
			continue
		}
		if pathItem.Options != nil {
			requestsMap[http.MethodOptions] = append(requestsMap[http.MethodOptions], path)
			continue
		}
		if pathItem.Head != nil {
			requestsMap[http.MethodHead] = append(requestsMap[http.MethodHead], path)
			continue
		}
		if pathItem.Patch != nil {
			requestsMap[http.MethodPatch] = append(requestsMap[http.MethodPatch], path)
			continue
		}
	}
	a.mapper = newSimpleMapper(requestsMap)
	return nil
}
