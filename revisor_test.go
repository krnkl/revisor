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

var (
	testdata     = "internal/testdata/"
	sampleV2YAML = "sample_open_api_v2.yaml"
	sampleV2JSON = "sample_open_api_v2.json"
)

func TestRequestVerifier(t *testing.T) {
	verifier, err := NewRequestVerifier(testdata + sampleV2YAML)
	assert.NoError(t, err)
	err = verifier(nil)
	assert.NoError(t, err)
}

func TestVerifier(t *testing.T) {
	verifier, err := NewVerifier(testdata + sampleV2YAML)
	assert.NoError(t, err)
	err = verifier(httptest.NewRequest("GET", "/", nil), nil)
	assert.Regexp(t, "no path template matches current request", err)
}

func TestAPIVerifier_New(t *testing.T) {

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { err = http.Serve(listener, http.FileServer(http.Dir("internal/testdata"))) }()
	require.NoError(t, err)

	t.Run("local file does not exist", func(t *testing.T) {
		a, err := newAPIVerifier("./non-existing-file.yaml")
		assert.Regexp(t, "failed to load definition", err)
		assert.Nil(t, a)
	})

	t.Run("success loading local yaml file", func(t *testing.T) {
		a, err := newAPIVerifier(testdata + sampleV2YAML)
		assert.NoError(t, err)
		assert.NotNil(t, a)
	})

	t.Run("success loading local json file", func(t *testing.T) {
		a, err := newAPIVerifier(testdata + sampleV2JSON)
		assert.NoError(t, err)
		assert.NotNil(t, a)
	})

	t.Run("fail to load yaml", func(t *testing.T) {
		a, err := newAPIVerifier(testdata + "invalid.yaml")
		assert.Regexp(t, "cannot unmarshal", err)
		assert.Nil(t, a)
	})

	t.Run("fail to load json", func(t *testing.T) {
		a, err := newAPIVerifier(testdata + "invalid.json")
		assert.Regexp(t, "cannot unmarshal", err)
		assert.Nil(t, a)
	})

	t.Run("success loading yaml by URL", func(t *testing.T) {
		a, err := newAPIVerifier(fmt.Sprintf("http://%s/%s", listener.Addr(), sampleV2YAML))
		assert.NoError(t, err)
		assert.NotNil(t, a)
	})

	t.Run("success loading json by URL", func(t *testing.T) {
		a, err := newAPIVerifier(fmt.Sprintf("http://%s/%s", listener.Addr(), sampleV2JSON))
		assert.NoError(t, err)
		assert.NotNil(t, a)
	})
}
