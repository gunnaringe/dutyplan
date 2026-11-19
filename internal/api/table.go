package api

import (
	"net/http"
	"strconv"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// tableRoutes: what the table on the plan page needs beyond events and people. Places per event (instead of the day
// type's), the plan's info rows and their values, everyone's answers in one call, and one task against all the others.
func (s *server) tableRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/places", s.listPlaces)
	mux.HandleFunc("PUT /api/orgs/{org}/plans/{plan}/events/{id}/places/{task}", s.setPlaces)
	mux.HandleFunc("DELETE /api/orgs/{org}/plans/{plan}/events/{id}/places/{task}", s.resetPlaces)

	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/fields", s.listFields)
	mux.HandleFunc("POST /api/orgs/{org}/plans/{plan}/fields", s.addField)
	mux.HandleFunc("PATCH /api/orgs/{org}/plans/{plan}/fields/{id}", s.renameField)
	mux.HandleFunc("DELETE /api/orgs/{org}/plans/{plan}/fields/{id}", s.deleteField)
	mux.HandleFunc("PUT /api/orgs/{org}/plans/{plan}/events/{id}/fields/{field}", s.setFieldValue)

	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/answers", s.listAnswers)
	mux.HandleFunc("PUT /api/orgs/{org}/tasks/{id}/combinations", s.setTaskCombinations)
}

// idAnd reads {id} and a second id from the path; a malformed one is 404, like an unknown one.
func (s *server) idAnd(w http.ResponseWriter, r *http.Request, second string) (int64, int64, bool) {
	id, ok := s.idParam(w, r)
	if !ok {
		return 0, 0, false
	}
	other, err := strconv.ParseInt(r.PathValue(second), 10, 64)
	if err != nil {
		writeErr(w, s.log, store.ErrNotFound)
		return 0, 0, false
	}
	return id, other, true
}

func (s *server) listPlaces(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListEventPlaces(r.Context(), r.PathValue("org"), r.PathValue("plan"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) setPlaces(w http.ResponseWriter, r *http.Request) {
	ev, task, ok := s.idAnd(w, r, "task")
	if !ok {
		return
	}
	var b struct {
		Slots *int `json:"slots"`
	}
	if !decode(w, r, &b) {
		return
	}
	if b.Slots == nil {
		writeErr(w, s.log, store.Invalid("places_invalid", map[string]any{"max": 50}, "send the number of places in slots"))
		return
	}
	noContent(w, s.st.SetEventPlaces(r.Context(), r.PathValue("org"), r.PathValue("plan"), ev, task, b.Slots), s)
}

func (s *server) resetPlaces(w http.ResponseWriter, r *http.Request) {
	ev, task, ok := s.idAnd(w, r, "task")
	if !ok {
		return
	}
	noContent(w, s.st.SetEventPlaces(r.Context(), r.PathValue("org"), r.PathValue("plan"), ev, task, nil), s)
}

func (s *server) listFields(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListFields(r.Context(), r.PathValue("org"), r.PathValue("plan"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) addField(w http.ResponseWriter, r *http.Request) {
	var b nameBody
	if !decode(w, r, &b) {
		return
	}
	f, err := s.st.AddField(r.Context(), r.PathValue("org"), r.PathValue("plan"), b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, f)
}

func (s *server) renameField(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b nameBody
	if !decode(w, r, &b) {
		return
	}
	f, err := s.st.RenameField(r.Context(), r.PathValue("org"), r.PathValue("plan"), id, b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, f)
}

func (s *server) deleteField(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	noContent(w, s.st.DeleteField(r.Context(), r.PathValue("org"), r.PathValue("plan"), id), s)
}

func (s *server) setFieldValue(w http.ResponseWriter, r *http.Request) {
	ev, field, ok := s.idAnd(w, r, "field")
	if !ok {
		return
	}
	var b struct {
		Value string `json:"value"`
	}
	if !decode(w, r, &b) {
		return
	}
	noContent(w, s.st.SetFieldValue(r.Context(), r.PathValue("org"), r.PathValue("plan"), ev, field, b.Value), s)
}

func (s *server) listAnswers(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListAnswers(r.Context(), r.PathValue("org"), r.PathValue("plan"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) setTaskCombinations(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b struct {
		Level string `json:"level"`
	}
	if !decode(w, r, &b) {
		return
	}
	noContent(w, s.st.SetTaskCombinations(r.Context(), r.PathValue("org"), id, b.Level), s)
}
