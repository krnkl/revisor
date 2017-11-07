package revisor

import (
	"net/http"
)

type option func(*apiVerifier)

func SetSomeOption(opt string) option {
	return func(a *apiVerifier) {
		a.opt = opt
	}
}

// NewRequestVerifier returns a function that can be used to verify if request
// satisfies OpenAPI definition constraints
func NewRequestVerifier(definition string, options ...option) func(*http.Request) error {
	a := apiVerifier{name: definition}
	a.setOptions(options...)
	return a.verifyRequest
}

// NewResponseVerifier returns a function that can be used to verify if response
// satisfies OpenAPI definition constraints. Two perform such alidation we need
// to specify context in which current response was received, and that is represented
// by method and url paramters
func NewResponseVerifier(definition, method, url string, options ...option) func(*http.Response) error {
	a := apiVerifier{name: definition}
	a.setOptions(options...)
	return func(res *http.Response) error {
		return a.verifyResponse(method, url, res)
	}
}

// NewVerifier returns a function that can be used to verify both - a request
// and the response made in the context of the request
func NewVerifier(definition string, options ...option) func(*http.Request, *http.Response) error {
	a := apiVerifier{name: definition}
	a.setOptions(options...)
	return a.verify
}

// apiVerifier implements various verification functions and encloses various
// verification options as well as an OpenAPI Document
type apiVerifier struct {
	name string
	opt  string
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
