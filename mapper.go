package revisor

import (
	"net/http"

	"github.com/gorilla/mux"
)

func newSimpleMapper(templateMap map[string][]string) *simpleMapper {

	router := mux.NewRouter().StrictSlash(true)
	mapper := &simpleMapper{router: router}
	for k, v := range templateMap {
		for _, tmpl := range v {
			router.Methods(k).Path(tmpl)

		}
	}
	return mapper
}

type simpleMapper struct {
	router *mux.Router
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
		return tmpl, match.Vars, true
	}
	return "", nil, false
}
