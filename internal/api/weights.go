package api

import (
	"net/http"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// weightsRoutes: the planner's weighting is a setting of one plan. The reply carries the list of settings (with their
// defaults and limits) and the values the plan has changed; everything not in «values» is the default.
func (s *server) weightsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/weights", s.getWeights)
	mux.HandleFunc("PUT /api/orgs/{org}/plans/{plan}/weights", s.setWeights)
}

type weightsReply struct {
	Specs  []store.WeightSpec `json:"specs"`
	Values map[string]any     `json:"values"`
}

func (s *server) getWeights(w http.ResponseWriter, r *http.Request) {
	values, err := s.st.GetWeights(r.Context(), r.PathValue("org"), r.PathValue("plan"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, weightsReply{store.WeightSpecs, values})
}

// setWeights replaces what the plan has changed: leave a key out to send it back to the default, send {} to reset all.
func (s *server) setWeights(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Values map[string]any `json:"values"`
	}
	if !decode(w, r, &b) {
		return
	}
	if b.Values == nil {
		b.Values = map[string]any{}
	}
	values, err := s.st.SetWeights(r.Context(), r.PathValue("org"), r.PathValue("plan"), b.Values)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, weightsReply{store.WeightSpecs, values})
}
