package revisor

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

func newSimpleMapper(basePath string, templateMap map[string][]string) *simpleMapper {

	router := mux.NewRouter().StrictSlash(true).PathPrefix(basePath).Subrouter()
	mapper := &simpleMapper{router: router, basePath: basePath}
	for k, v := range templateMap {
		for _, tmpl := range v {
			router.Methods(k).Path(tmpl)

		}
	}
	return mapper
}

type simpleMapper struct {
	router   *mux.Router
	basePath string
}

// mapRequest returns configured template that matches HTTP
// method and actual path.
// vars return parameter is a map of all variables set in
// the path according to the matched template
// isSet return parameter indicates if template was configured at all
func (s *simpleMapper) mapRequest(r *http.Request) (tmpl string, vars map[string]string, isSet bool) {
	match := mux.RouteMatch{}
	if s.router.Match(r, &match) {
		tmpl, err := match.Route.GetPathTemplate()
		if err != nil {
			return "", nil, false
		}
		return strings.TrimPrefix(tmpl, s.basePath), match.Vars, true
	}
	return "", nil, false
}
