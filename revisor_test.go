package revisor

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func Test_LoadDefinition(t *testing.T) {

	http.HandleFunc("/swagger.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./internal/testdata/sample_open_api_v2.yaml")
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { err = http.Serve(listener, nil) }()
	require.NoError(t, err)

	t.Run("local file does not exist", func(t *testing.T) {
		b, err := loadDefinition("./non-existing-file.yaml")
		assert.Regexp(t, "api definition failed to load", err)
		assert.Nil(t, b)
	})

	t.Run("success loading local file", func(t *testing.T) {
		b, err := loadDefinition("./internal/testdata/sample_open_api_v2.yaml")
		assert.NoError(t, err)
		assert.NotNil(t, b)
	})

	t.Run("success loading by URL", func(t *testing.T) {
		b, err := loadDefinition(fmt.Sprintf("http://%s/swagger.yaml", listener.Addr()))
		assert.NoError(t, err)
		assert.NotNil(t, b)
	})

	t.Run("url not found", func(t *testing.T) {
		b, err := loadDefinition(fmt.Sprintf("http://%s/not-existing-path.yaml", listener.Addr()))
		assert.Regexp(t, "request return error: Not Found", err)
		assert.Nil(t, b)
	})

	t.Run("url not valid", func(t *testing.T) {
		b, err := loadDefinition("localhost:8080/swagger.yaml")
		assert.Regexp(t, "failed to perform request: Get localhost:8080/swagger.yaml", err)
		assert.Nil(t, b)
	})
}
