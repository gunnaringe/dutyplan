package api

import (
	"net/http"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// cellsRoutes: manual cells (a person or a text set by hand; locked) and selections (only these can be
// chosen) of one plan. Both write for several events at once, all or nothing.
func (s *server) cellsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/cells", s.listCells)
	mux.HandleFunc("POST /api/orgs/{org}/plans/{plan}/cells", s.addCell)
	mux.HandleFunc("DELETE /api/orgs/{org}/plans/{plan}/cells/{id}", s.deleteCell)
	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}/pools", s.listPools)
	mux.HandleFunc("PUT /api/orgs/{org}/plans/{plan}/pools", s.setPool)
}

func (s *server) listCells(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListCells(r.Context(), r.PathValue("org"), r.PathValue("plan"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) addCell(w http.ResponseWriter, r *http.Request) {
	var in store.NewCell
	if !decode(w, r, &in) {
		return
	}
	c, err := s.st.AddCell(r.Context(), r.PathValue("org"), r.PathValue("plan"), in)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, c)
}

func (s *server) deleteCell(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	noContent(w, s.st.DeleteCell(r.Context(), r.PathValue("org"), r.PathValue("plan"), id), s)
}

func (s *server) listPools(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListPools(r.Context(), r.PathValue("org"), r.PathValue("plan"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) setPool(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Event  int64   `json:"event"`
		Task   int64   `json:"task"`
		People []int64 `json:"people"`
	}
	if !decode(w, r, &b) {
		return
	}
	noContent(w, s.st.SetPool(r.Context(), r.PathValue("org"), r.PathValue("plan"), b.Event, b.Task, b.People), s)
}
