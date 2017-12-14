package revisor

import (
	"bytes"
	"encoding/json"
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
	err = verifier(httptest.NewRequest("PUT", "/user/testuser", nil))
	assert.Regexp(t, "body is empty", err)
}

func TestVerifier(t *testing.T) {
	verifier, err := NewVerifier(testdata + sampleV2YAML)
	assert.NoError(t, err)
	err = verifier(nil, httptest.NewRequest("GET", "/", nil))
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

	// t.Run("success loading yaml by URL", func(t *testing.T) {
	// 	a, err := newAPIVerifier(fmt.Sprintf("http://%s/%s", listener.Addr(), sampleV2YAML))
	// 	assert.NoError(t, err)
	// 	assert.NotNil(t, a)
	// })
	//
	// t.Run("success loading json by URL", func(t *testing.T) {
	// 	a, err := newAPIVerifier(fmt.Sprintf("http://%s/%s", listener.Addr(), sampleV2JSON))
	// 	assert.NoError(t, err)
	// 	assert.NotNil(t, a)
	// })
}

type TestUser struct {
	ID              int64  `json:"id,omitempty"`
	Username        string `json:"username,omitempty"`
	FirstName       string `json:"firstname,omitempty"`
	LastName        string `json:"lastname,omitempty"`
	Email           string `json:"email,omitempty"`
	Password        string `json:"password,omitempty"`
	Phone           string `json:"phone,omitempty"`
	UserStatus      int32  `json:"user_status,omitempty"`
	AdditionalField string `json:"additional_field,omitempty"`
}

func TestAPIVerifierV2_VerifyResponse(t *testing.T) {

	a, err := newAPIVerifier(testdata + sampleV2YAML)
	require.NoError(t, err)
	require.NotNil(t, a)

	validUser := func() *TestUser {
		return &TestUser{
			// Immutable
			ID:       123456,
			Username: "test-user",
			Email:    "test-user@example.com",
			// Mutable
			LastName:   "Bar",
			Password:   "supersecret",
			Phone:      "+12 (34) 5678 910",
			UserStatus: 1,
		}

	}
	tests := []struct {
		name  string
		req   *http.Request
		code  int
		err   string
		alter func(*TestUser) interface{}
	}{
		{
			"valid response",
			httptest.NewRequest("GET", "/user/testuser", nil),
			http.StatusOK,
			"",
			func(u *TestUser) interface{} { return u },
		},
		{
			"validates default",
			httptest.NewRequest("GET", "/user/testuser", nil),
			http.StatusInternalServerError,
			"",
			func(u *TestUser) interface{} {
				return json.RawMessage(`{"error":"internal error","error_description":"error"}`)
			},
		},
		{
			"no schema configured",
			httptest.NewRequest("PUT", "/user/testuser", nil),
			http.StatusPermanentRedirect,
			"neither default nor response schema for current status code is defined",
			func(u *TestUser) interface{} { return u },
		},
		{
			"no path template found",
			httptest.NewRequest("GET", "/not-found", nil),
			http.StatusNotFound,
			"no path template matches current request",
			func(u *TestUser) interface{} { return u },
		},
		{
			"no schema for http method",
			httptest.NewRequest("HEAD", "/user/testuser", nil),
			http.StatusMethodNotAllowed,
			"no path template matches current request",
			func(u *TestUser) interface{} { return u },
		},
		{
			"schema is not defined but response body is not empty",
			httptest.NewRequest("GET", "/user/testuser", nil),
			http.StatusNotFound,
			"schema is not defined",
			func(u *TestUser) interface{} { return u },
		},
		{
			"missing required field",
			httptest.NewRequest("GET", "/user/testuser", nil),
			http.StatusOK,
			".id in body is required",
			func(u *TestUser) interface{} { u.ID = 0; return u },
		},
		{
			"type incorrect",
			httptest.NewRequest("GET", "/user/testuser", nil),
			http.StatusOK,
			"firstname in body must be of type integer",
			func(u *TestUser) interface{} { u.FirstName = "firstname"; return u },
		},
		{
			"format incorrect",
			httptest.NewRequest("GET", "/user/testuser", nil),
			http.StatusOK,
			"email in body must be of type email",
			func(u *TestUser) interface{} { u.Email = "invalid-email"; return u },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			serialized, err := json.Marshal(test.alter(validUser()))
			assert.NoError(t, err)

			rec.Header().Set("Content-Type", "application/json")
			rec.WriteHeader(test.code)
			_, err = rec.Write(serialized)
			assert.NoError(t, err)
			err = a.verifyResponse(rec.Result(), test.req)

			if test.err != "" {
				assert.Regexp(t, test.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
	t.Run("schema defined but response body is empty", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusOK)
		err = a.verifyResponse(rec.Result(), httptest.NewRequest("GET", "/user/testuser", nil))
		assert.Regexp(t, "response body is empty", err)
	})
	t.Run("fails to decode response body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		invalid := []byte("invalid-json")
		rec.Header().Set("Content-Type", "application/json")
		rec.WriteHeader(http.StatusOK)
		_, err = rec.Write(invalid)
		assert.NoError(t, err)

		err = a.verifyResponse(rec.Result(), httptest.NewRequest("GET", "/user/testuser", nil))
		assert.Regexp(t, "failed to decode response", err)
	})
	// TODO: add tests with disabled strict Content-Type when content type matches or doesn't match
}

func TestAPIVerifierV2_VerifyRequest(t *testing.T) {

	a, err := newAPIVerifier(testdata + sampleV2YAML)
	require.NoError(t, err)
	require.NotNil(t, a)

	validUser := func() *TestUser {
		return &TestUser{
			// Immutable
			ID:       123456,
			Username: "test-user",
			Email:    "test-user@example.com",
			// Mutable
			LastName:   "Bar",
			Password:   "supersecret",
			Phone:      "+12 (34) 5678 910",
			UserStatus: 1,
		}

	}
	tests := []struct {
		name   string
		method string
		path   string
		err    string
		alter  func(*TestUser) interface{}
	}{
		{
			"valid request",
			"PUT",
			"/user/testuser",
			"",
			func(u *TestUser) interface{} { return u },
		},
		{
			"failed to find path",
			"PATCH",
			"/user/testuser",
			"no path template matches current request",
			func(u *TestUser) interface{} { return nil },
		},
		{
			"no definition but body not empty",
			"GET",
			"/user/testuser",
			"definition is not defined but body is not empty",
			func(u *TestUser) interface{} { return u },
		},
		{
			"missing required field",
			"PUT",
			"/user/testuser",
			".id in body is required",
			func(u *TestUser) interface{} { u.ID = 0; return u },
		},
		{
			"type incorrect",
			"PUT",
			"/user/testuser",
			"firstname in body must be of type integer",
			func(u *TestUser) interface{} { u.FirstName = "firstname"; return u },
		},
		{
			"format incorrect",
			"PUT",
			"/user/testuser",
			"email in body must be of type email",
			func(u *TestUser) interface{} { u.Email = "invalid-email"; return u },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serialized, err := json.Marshal(test.alter(validUser()))
			assert.NoError(t, err)

			req, err := http.NewRequest(test.method, test.path, bytes.NewReader(serialized))
			req.Header.Add("Content-Type", "application/json")
			assert.NoError(t, err)
			err = a.verifyRequest(req)

			if test.err != "" {
				assert.Regexp(t, test.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
	t.Run("schema defined but request body is empty", func(t *testing.T) {
		req, err := http.NewRequest("PUT", "/user/testuser", nil)
		assert.NoError(t, err)
		err = a.verifyRequest(req)
		assert.Regexp(t, "body is empty", err)
	})
	t.Run("fails to decode request body", func(t *testing.T) {
		invalid := []byte("invalid-json")
		assert.NoError(t, err)

		req, err := http.NewRequest("PUT", "/user/testuser", bytes.NewReader(invalid))
		req.Header.Add("Content-Type", "application/json")
		assert.NoError(t, err)
		err = a.verifyRequest(req)
		assert.Regexp(t, "failed to decode request", err)
	})
	// TODO add tests:
	// Content-Type strict mode disabled
	// problem json
}
