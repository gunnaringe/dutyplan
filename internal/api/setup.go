package api

import (
	"net/http"
	"strconv"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// setupRoutes: tasks and combinations (per organisation), events (per plan).
func (s *server) setupRoutes(mux *http.ServeMux) {
	s.prefsRoutes(mux)
	mux.HandleFunc("GET /api/orgs/{org}/tasks", s.listTasks)
	mux.HandleFunc("POST /api/orgs/{org}/tasks", s.createTask)
	mux.HandleFunc("POST /api/orgs/{org}/tasks/bulk", s.createTasks)
	mux.HandleFunc("PATCH /api/orgs/{org}/tasks/{id}", s.updateTask)
	mux.HandleFunc("DELETE /api/orgs/{org}/tasks/{id}", s.deleteTask)

	mux.HandleFunc("GET /api/orgs/{org}/combinations", s.listCombinations)
	mux.HandleFunc("PUT /api/orgs/{org}/combinations", s.setCombination)

	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/events", s.listEvents)
	mux.HandleFunc("POST /api/orgs/{org}/plans/{plan}/events", s.addEvent)
	mux.HandleFunc("POST /api/orgs/{org}/plans/{plan}/events/{id}/copy", s.copyEvent)
	mux.HandleFunc("PATCH /api/orgs/{org}/plans/{plan}/events/{id}", s.updateEvent)
	mux.HandleFunc("DELETE /api/orgs/{org}/plans/{plan}/events/{id}", s.deleteEvent)
}

// idParam reads a numeric path value; anything else is simply "not found".
func (s *server) idParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, s.log, store.ErrNotFound)
		return 0, false
	}
	return id, true
}

func noContent(w http.ResponseWriter, err error, s *server) {
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- tasks ---------------------------------------------------------------------------------------------------

func (s *server) listTasks(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListTasks(r.Context(), r.PathValue("org"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) createTask(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name    string `json:"name"`
		Planned *bool  `json:"planned"`
	}
	if !decode(w, r, &b) {
		return
	}
	t, err := s.st.CreateTask(r.Context(), r.PathValue("org"), b.Name, b.Planned == nil || *b.Planned)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, t)
}

func (s *server) createTasks(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Names   string `json:"names"` // one per line
		Planned *bool  `json:"planned"`
	}
	if !decode(w, r, &b) {
		return
	}
	created, skipped, err := s.st.CreateTasks(r.Context(), r.PathValue("org"), b.Names, b.Planned == nil || *b.Planned)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	if created == nil {
		created = []store.Task{}
	}
	if skipped == nil {
		skipped = []string{}
	}
	writeJSON(w, 201, map[string]any{"created": created, "skipped": skipped})
}

func (s *server) updateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var p store.TaskPatch
	if !decode(w, r, &p) {
		return
	}
	t, err := s.st.UpdateTask(r.Context(), r.PathValue("org"), id, p)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, t)
}

func (s *server) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	noContent(w, s.st.DeleteTask(r.Context(), r.PathValue("org"), id), s)
}

// ---- day types -----------------------------------------------------------------------------------------------

// ---- combinations --------------------------------------------------------------------------------------------

func (s *server) listCombinations(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListCombinations(r.Context(), r.PathValue("org"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) setCombination(w http.ResponseWriter, r *http.Request) {
	var b store.Combination
	if !decode(w, r, &b) {
		return
	}
	noContent(w, s.st.SetCombination(r.Context(), r.PathValue("org"), b.A, b.B, b.Level), s)
}

// ---- events --------------------------------------------------------------------------------------------------

func (s *server) listEvents(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListEvents(r.Context(), r.PathValue("org"), r.PathValue("plan"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

// addEvent adds an event, or with "until" one every week up to and including that date; the answer is the list.
func (s *server) addEvent(w http.ResponseWriter, r *http.Request) {
	var b store.NewEvents
	if !decode(w, r, &b) {
		return
	}
	evs, err := s.st.AddEvents(r.Context(), r.PathValue("org"), r.PathValue("plan"), b)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, evs)
}

// copyEvent copies an event to other dates, with what the body says comes along (store.CopyOf).
func (s *server) copyEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b store.CopyOf
	if !decode(w, r, &b) {
		return
	}
	evs, err := s.st.CopyEvent(r.Context(), r.PathValue("org"), r.PathValue("plan"), id, b)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, evs)
}

func (s *server) updateEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var p store.EventPatch
	if !decode(w, r, &p) {
		return
	}
	d, err := s.st.UpdateEvent(r.Context(), r.PathValue("org"), r.PathValue("plan"), id, p)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, d)
}

func (s *server) deleteEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	noContent(w, s.st.DeleteEvent(r.Context(), r.PathValue("org"), r.PathValue("plan"), id), s)
}
