package revisor

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleMapper_New(t *testing.T) {
	mapper := newSimpleMapper(map[string][]string{
		"GET": []string{"/path", "/path/{id}"},
	})
	assert.NotNil(t, mapper)
}

func TestSimpleMapper_MapRequest(t *testing.T) {

	mapper := newSimpleMapper(map[string][]string{
		"GET": []string{"/path", "/path/{id}"},
	})
	mapper.router.Methods("OPTIONS")

	tests := []struct {
		name    string
		request *http.Request
		tmpl    string
		isSet   bool
	}{
		{"tmpl found", httptest.NewRequest("GET", "/path", nil), "/path", true},
		{"tmpl root not configured", httptest.NewRequest("GET", "/", nil), "", false},
		{"tmpl with parameter found", httptest.NewRequest("GET", "/path/resource-id", nil), "/path/{id}", true},
		{"tmpl is not configured", httptest.NewRequest("OPTIONS", "/", nil), "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tmpl, _, ok := mapper.mapRequest(test.request)
			assert.Equal(t, test.isSet, ok)
			assert.Equal(t, test.tmpl, tmpl)
		})
	}
}
