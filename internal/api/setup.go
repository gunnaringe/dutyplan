package api

import (
	"net/http"
	"strconv"

	"github.com/gunnaringe/tjenesteplan/internal/store"
)

// setupRoutes: tasks, day types and combinations (per organisation), dates (per plan).
func (s *server) setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/orgs/{org}/tasks", s.listTasks)
	mux.HandleFunc("POST /api/orgs/{org}/tasks", s.createTask)
	mux.HandleFunc("POST /api/orgs/{org}/tasks/bulk", s.createTasks)
	mux.HandleFunc("PATCH /api/orgs/{org}/tasks/{id}", s.updateTask)
	mux.HandleFunc("DELETE /api/orgs/{org}/tasks/{id}", s.deleteTask)

	mux.HandleFunc("GET /api/orgs/{org}/day-types", s.listDayTypes)
	mux.HandleFunc("POST /api/orgs/{org}/day-types", s.createDayType)
	mux.HandleFunc("PATCH /api/orgs/{org}/day-types/{id}", s.renameDayType)
	mux.HandleFunc("PUT /api/orgs/{org}/day-types/{id}/tasks", s.setDayTypeTasks)
	mux.HandleFunc("DELETE /api/orgs/{org}/day-types/{id}", s.deleteDayType)

	mux.HandleFunc("GET /api/orgs/{org}/combinations", s.listCombinations)
	mux.HandleFunc("PUT /api/orgs/{org}/combinations", s.setCombination)

	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/events", s.listEvents)
	mux.HandleFunc("POST /api/orgs/{org}/plans/{plan}/events", s.addEvent)
	mux.HandleFunc("POST /api/orgs/{org}/plans/{plan}/events/bulk", s.addEventsBulk)
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
	t, err := s.st.CreateTask(r.Context(), r.PathValue("org"), actor(r), b.Name, b.Planned == nil || *b.Planned)
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
	created, skipped, err := s.st.CreateTasks(r.Context(), r.PathValue("org"), actor(r), b.Names, b.Planned == nil || *b.Planned)
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
	t, err := s.st.UpdateTask(r.Context(), r.PathValue("org"), actor(r), id, p)
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
	noContent(w, s.st.DeleteTask(r.Context(), r.PathValue("org"), actor(r), id), s)
}

// ---- day types -----------------------------------------------------------------------------------------------

func (s *server) listDayTypes(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListDayTypes(r.Context(), r.PathValue("org"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) createDayType(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	d, err := s.st.CreateDayType(r.Context(), r.PathValue("org"), actor(r), b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, d)
}

func (s *server) renameDayType(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	d, err := s.st.RenameDayType(r.Context(), r.PathValue("org"), actor(r), id, b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, d)
}

func (s *server) setDayTypeTasks(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b struct {
		Tasks []store.DayTypeTask `json:"tasks"`
	}
	if !decode(w, r, &b) {
		return
	}
	d, err := s.st.SetDayTypeTasks(r.Context(), r.PathValue("org"), actor(r), id, b.Tasks)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, d)
}

func (s *server) deleteDayType(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	noContent(w, s.st.DeleteDayType(r.Context(), r.PathValue("org"), actor(r), id), s)
}

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
	noContent(w, s.st.SetCombination(r.Context(), r.PathValue("org"), actor(r), b.A, b.B, b.Level), s)
}

// ---- dates ---------------------------------------------------------------------------------------------------

func (s *server) listEvents(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListEvents(r.Context(), r.PathValue("org"), r.PathValue("plan"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) addEvent(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Date      string `json:"date"`
		DayTypeID int64  `json:"day_type_id"`
		Note      string `json:"note"`
	}
	if !decode(w, r, &b) {
		return
	}
	d, err := s.st.AddEvent(r.Context(), r.PathValue("org"), r.PathValue("plan"), actor(r), b.Date, b.DayTypeID, b.Note)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, d)
}

func (s *server) addEventsBulk(w http.ResponseWriter, r *http.Request) {
	var b struct {
		From      string `json:"from"`
		To        string `json:"to"`
		Weekdays  []int  `json:"weekdays"` // ISO: 1 = Monday ... 7 = Sunday
		DayTypeID int64  `json:"day_type_id"`
	}
	if !decode(w, r, &b) {
		return
	}
	created, skipped, err := s.st.AddEventsBulk(r.Context(), r.PathValue("org"), r.PathValue("plan"), actor(r), b.From, b.To, b.Weekdays, b.DayTypeID)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	if created == nil {
		created = []store.Event{}
	}
	if skipped == nil {
		skipped = []string{}
	}
	writeJSON(w, 201, map[string]any{"created": created, "skipped": skipped})
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
	d, err := s.st.UpdateEvent(r.Context(), r.PathValue("org"), r.PathValue("plan"), actor(r), id, p)
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
	noContent(w, s.st.DeleteEvent(r.Context(), r.PathValue("org"), r.PathValue("plan"), actor(r), id), s)
}
