package api

import (
	"net/http"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// sectionsRoutes: sections place people in the table (headings in Availability), and a person can be moved within and
// between them. Only placement: the planner never sees it.
func (s *server) sectionsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/orgs/{org}/sections", s.listSections)
	mux.HandleFunc("POST /api/orgs/{org}/sections", s.createSection)
	mux.HandleFunc("PATCH /api/orgs/{org}/sections/{id}", s.renameSection)
	mux.HandleFunc("PUT /api/orgs/{org}/sections/{id}/position", s.moveSection)
	mux.HandleFunc("DELETE /api/orgs/{org}/sections/{id}", s.deleteSection)
	mux.HandleFunc("PUT /api/orgs/{org}/people/{id}/position", s.movePerson)
}

func (s *server) listSections(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListSections(r.Context(), r.PathValue("org"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) createSection(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	v, err := s.st.CreateSection(r.Context(), r.PathValue("org"), b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, v)
}

func (s *server) renameSection(w http.ResponseWriter, r *http.Request) {
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
	v, err := s.st.RenameSection(r.Context(), r.PathValue("org"), id, b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

// moveBody is where a section or a person goes; section (0 = none) is only for a person.
type moveBody struct {
	Position *int  `json:"position"`
	Section  int64 `json:"section"`
}

// position reads the body; a missing position is position_invalid.
func (s *server) position(w http.ResponseWriter, r *http.Request, b *moveBody) bool {
	if !decode(w, r, b) {
		return false
	}
	if b.Position == nil {
		writeErr(w, s.log, store.Invalid("position_invalid", nil, "send the new position in position"))
		return false
	}
	return true
}

func (s *server) moveSection(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b moveBody
	if !s.position(w, r, &b) {
		return
	}
	noContent(w, s.st.MoveSection(r.Context(), r.PathValue("org"), id, *b.Position), s)
}

func (s *server) deleteSection(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	noContent(w, s.st.DeleteSection(r.Context(), r.PathValue("org"), id), s)
}

// movePerson puts a person in a section (0 = none) at a position among its people.
func (s *server) movePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b moveBody
	if !s.position(w, r, &b) {
		return
	}
	noContent(w, s.st.MovePerson(r.Context(), r.PathValue("org"), id, b.Section, *b.Position), s)
}
