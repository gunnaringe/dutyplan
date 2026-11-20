package api

import (
	"net/http"
	"strconv"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// prefsRoutes: the tasks a person takes part in, and their answers per event (set in the plan's table).
func (s *server) prefsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PUT /api/orgs/{org}/people/{id}/tasks/{task}", s.setPersonTask)
	mux.HandleFunc("DELETE /api/orgs/{org}/people/{id}/tasks/{task}", s.removePersonTask)
	mux.HandleFunc("PUT /api/orgs/{org}/plans/{plan}/people/{id}/availability", s.setAvailability)
}

func (s *server) personAndTask(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	pid, ok := s.idParam(w, r)
	if !ok {
		return 0, 0, false
	}
	tid, err := strconv.ParseInt(r.PathValue("task"), 10, 64)
	if err != nil {
		writeErr(w, s.log, store.ErrNotFound)
		return 0, 0, false
	}
	return pid, tid, true
}

func (s *server) setPersonTask(w http.ResponseWriter, r *http.Request) {
	pid, tid, ok := s.personAndTask(w, r)
	if !ok {
		return
	}
	var b struct {
		Level string `json:"level"`
	}
	if !decode(w, r, &b) {
		return
	}
	noContent(w, s.st.SetPersonTask(r.Context(), r.PathValue("org"), pid, tid, b.Level), s)
}

func (s *server) removePersonTask(w http.ResponseWriter, r *http.Request) {
	pid, tid, ok := s.personAndTask(w, r)
	if !ok {
		return
	}
	noContent(w, s.st.RemovePersonTask(r.Context(), r.PathValue("org"), pid, tid), s)
}

func (s *server) setAvailability(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b struct {
		Levels map[string]string `json:"levels"` // event id -> never | poor | good | "" (neutral)
	}
	if !decode(w, r, &b) {
		return
	}
	levels := make(map[int64]string, len(b.Levels))
	for k, v := range b.Levels {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			writeJSON(w, 400, apiError{Error: "invalid", Message: "event ids must be numbers"})
			return
		}
		levels[id] = v
	}
	noContent(w, s.st.SetAvailability(r.Context(), r.PathValue("org"), r.PathValue("plan"), pid, levels), s)
}
