package revisor

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/pkg/errors"
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
	a := newAPIVerifier(definitionPath)
	a.setOptions(options...)
	return a.verifyRequest, nil
}

// NewResponseVerifier returns a function that can be used to verify if response
// satisfies OpenAPI definition constraints. Two perform such alidation we need
// to specify context in which current response was received, and that is represented
// by method and url paramters
func NewResponseVerifier(definitionPath, method, url string, options ...option) (func(*http.Response) error, error) {
	a := newAPIVerifier(definitionPath)
	a.setOptions(options...)
	return func(res *http.Response) error {
		return a.verifyResponse(method, url, res)
	}, nil
}

// NewVerifier returns a function that can be used to verify both - a request
// and the response made in the context of the request
func NewVerifier(definitionPath string, options ...option) (func(*http.Request, *http.Response) error, error) {
	a := newAPIVerifier(definitionPath)
	a.setOptions(options...)
	return a.verify, nil
}

func newAPIVerifier(definitionPath string) *apiVerifier {
	mapper := newSimpleMapper(map[string][]string{})
	a := &apiVerifier{
		name:   definitionPath,
		mapper: mapper,
	}
	return a
}

// apiVerifier implements various verification functions and encloses various
// verification options as well as an OpenAPI Document
type apiVerifier struct {
	name   string
	opt    string
	mapper *simpleMapper
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

// loadDefinition loads API definition located by definitionPath
// which can be either a path to a local file or a URL
func loadDefinition(definitionPath string) ([]byte, error) {
	if isValidUrl(definitionPath) {
		return loadByURL(definitionPath)
	} else {
		if _, err := os.Stat(definitionPath); !os.IsNotExist(err) {
			return ioutil.ReadFile(definitionPath)
		}
	}
	return nil, errors.New("api definition failed to load")
}

// loadByURL berforms GET request to fetch definition located by URL
func loadByURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to perform request")
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

// isValidUrl checks if specified path is a valid URL
func isValidUrl(path string) bool {
	_, err := url.ParseRequestURI(path)
	if err != nil {
		return false
	}
	return true
}

func buildDocument(rawDef []byte) {}
