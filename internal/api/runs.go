package api

import (
	"errors"
	"net/http"

	"github.com/gunnaringe/dutyplan/internal/i18n"
	"github.com/gunnaringe/dutyplan/internal/planner"
	"github.com/gunnaringe/dutyplan/internal/store"
)

// runsRoutes: «Planlegg». Starting a run answers at once (202) and the page polls the run until it is done or failed.
// Every run is scoped by organisation and plan; a run of another plan or organisation is simply not found.
func (s *server) runsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/orgs/{org}/plans/{plan}/runs", s.startRun)
	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/runs", s.listRuns)
	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/runs/{id}", s.getRun)
	mux.HandleFunc("PUT /api/orgs/{org}/plans/{plan}/runs/{id}/pin", s.pinRun(true))
	mux.HandleFunc("DELETE /api/orgs/{org}/plans/{plan}/runs/{id}/pin", s.pinRun(false))
}

func (s *server) writeRunErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, planner.ErrBusy):
		writeJSON(w, 409, apiError{Error: "busy", Message: err.Error()})
	case errors.Is(err, planner.ErrUnavailable):
		writeJSON(w, 503, apiError{Error: "unavailable", Message: err.Error()})
	default:
		writeErr(w, s.log, err)
	}
}

func (s *server) startRun(w http.ResponseWriter, r *http.Request) {
	var b struct {
		TimeLimit int `json:"time_limit"` // seconds; 0 = the default
	}
	if !decode(w, r, &b) {
		return
	}
	id, err := s.planner.Start(r.Context(), r.PathValue("org"), r.PathValue("plan"), planner.Params{TimeLimit: b.TimeLimit})
	if err != nil {
		s.writeRunErr(w, err)
		return
	}
	writeJSON(w, 202, map[string]int64{"id": id})
}

func (s *server) listRuns(w http.ResponseWriter, r *http.Request) {
	v, err := s.planner.List(r.Context(), r.PathValue("org"), r.PathValue("plan"))
	if err != nil {
		s.writeRunErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

// getRun: the id is a number, or «latest» for the newest run of the plan.
func (s *server) getRun(w http.ResponseWriter, r *http.Request) {
	org, plan := r.PathValue("org"), r.PathValue("plan")
	var id int64
	if r.PathValue("id") == "latest" {
		runs, err := s.planner.List(r.Context(), org, plan)
		if err != nil {
			s.writeRunErr(w, err)
			return
		}
		if len(runs) == 0 {
			writeErr(w, s.log, store.ErrNotFound)
			return
		}
		id = runs[0].ID
	} else {
		var ok bool
		if id, ok = s.idParam(w, r); !ok {
			return
		}
	}
	v, err := s.planner.View(r.Context(), org, plan, id, i18n.Parse(r.URL.Query().Get("lang")))
	if err != nil {
		s.writeRunErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

// pinRun pins (PUT) or unpins (DELETE) a finished run as the plan's current plan.
func (s *server) pinRun(pin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.idParam(w, r)
		if !ok {
			return
		}
		noContent(w, s.st.PinRun(r.Context(), r.PathValue("org"), r.PathValue("plan"), id, pin), s)
	}
}
