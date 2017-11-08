package revisor

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestVerifier(t *testing.T) {
	verifier, err := NewRequestVerifier("RequestVerifier")
	assert.NoError(t, err)
	err = verifier(nil)
	assert.NoError(t, err)
}
func TestResponseVerifier(t *testing.T) {
	verifier, err := NewResponseVerifier("GET", "/path", "ResponseVerifier")
	assert.NoError(t, err)
	err = verifier(nil)
	assert.NoError(t, err)
}
func TestVerifier(t *testing.T) {
	verifier, err := NewVerifier("RequestResponseVerifier")
	assert.NoError(t, err)
	err = verifier(httptest.NewRequest("GET", "/", nil), nil)
	assert.NoError(t, err)
}
