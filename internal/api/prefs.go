package api

import (
	"net/http"
	"strconv"

	"github.com/gunnaringe/tjenesteplan/internal/store"
)

// prefsRoutes: what a person answers about themselves. No login: the page is reached by the person's id, and the
// answers are logged with the self-declared actor like everything else.
func (s *server) prefsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/people/{id}/me", s.getMe)
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

func (s *server) getMe(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	me, err := s.st.GetMe(r.Context(), r.PathValue("org"), r.PathValue("plan"), id)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, me)
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
	noContent(w, s.st.SetPersonTask(r.Context(), r.PathValue("org"), actor(r), pid, tid, b.Level), s)
}

func (s *server) removePersonTask(w http.ResponseWriter, r *http.Request) {
	pid, tid, ok := s.personAndTask(w, r)
	if !ok {
		return
	}
	noContent(w, s.st.RemovePersonTask(r.Context(), r.PathValue("org"), actor(r), pid, tid), s)
}

func (s *server) setAvailability(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b struct {
		Levels map[string]string `json:"levels"` // event id -> ikke | darlig | godt | "" (neutral)
	}
	if !decode(w, r, &b) {
		return
	}
	levels := make(map[int64]string, len(b.Levels))
	for k, v := range b.Levels {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			writeJSON(w, 400, apiError{"invalid", "event ids must be numbers"})
			return
		}
		levels[id] = v
	}
	noContent(w, s.st.SetAvailability(r.Context(), r.PathValue("org"), r.PathValue("plan"), actor(r), pid, levels), s)
}
