package revisor

import (
	"net/http/httptest"
	"testing"
)

func TestRequestVerifier(t *testing.T) {
	verifier := NewRequestVerifier("RequestVerifier")
	verifier(nil)
}
func TestResponseVerifier(t *testing.T) {
	verifier := NewResponseVerifier("GET", "/path", "ResponseVerifier")
	verifier(nil)
}
func TestVerifier(t *testing.T) {
	verifier := NewVerifier("RequestResponseVerifier")
	verifier(httptest.NewRequest("GET", "/", nil), nil)
}
