package api

import (
	"net/http"

	"github.com/gunnaringe/tjenesteplan/internal/store"
)

// groupsRoutes: groups are a named set of people per organisation (a team, a choir, a couple). They have no rules of
// their own; the planner may propose a group for the tasks it is set up for, and a manual cell can name one.
func (s *server) groupsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/orgs/{org}/groups", s.listGroups)
	mux.HandleFunc("POST /api/orgs/{org}/groups", s.createGroup)
	mux.HandleFunc("PATCH /api/orgs/{org}/groups/{id}", s.updateGroup)
	mux.HandleFunc("PUT /api/orgs/{org}/groups/{id}/members", s.setGroupMembers)
	mux.HandleFunc("PUT /api/orgs/{org}/groups/{id}/tasks", s.setGroupTasks)
	mux.HandleFunc("DELETE /api/orgs/{org}/groups/{id}", s.deleteGroup)
}

func (s *server) listGroups(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListGroups(r.Context(), r.PathValue("org"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) createGroup(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	g, err := s.st.CreateGroup(r.Context(), r.PathValue("org"), actor(r), b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, g)
}

func (s *server) updateGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var patch store.GroupPatch
	if !decode(w, r, &patch) {
		return
	}
	g, err := s.st.UpdateGroup(r.Context(), r.PathValue("org"), actor(r), id, patch)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, g)
}

func (s *server) setGroupMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b struct {
		People []int64 `json:"people"`
	}
	if !decode(w, r, &b) {
		return
	}
	g, err := s.st.SetGroupMembers(r.Context(), r.PathValue("org"), actor(r), id, b.People)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, g)
}

func (s *server) setGroupTasks(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b struct {
		Tasks []store.GroupTask `json:"tasks"`
	}
	if !decode(w, r, &b) {
		return
	}
	g, err := s.st.SetGroupTasks(r.Context(), r.PathValue("org"), actor(r), id, b.Tasks)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, g)
}

func (s *server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	noContent(w, s.st.DeleteGroup(r.Context(), r.PathValue("org"), actor(r), id), s)
}
