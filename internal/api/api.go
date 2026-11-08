// Package api is the JSON HTTP API plus the embedded single-page frontend.
//
// There is no login. The organisation is part of every path (/api/orgs/{org}/...), so the server keeps no "current
// organisation" and switching is just navigating. Writes carry the self-declared actor in X-Actor (percent-encoded
// UTF-8) and must be application/json; the custom header also makes browsers refuse cross-site writes (no CORS is
// offered). The organisation list is readable by anyone who can reach the address.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gunnaringe/tjenesteplan/internal/store"
)

type server struct {
	st     *store.Store
	static fs.FS
	log    *slog.Logger
}

// New builds the handler. static must contain index.html, app.js and style.css.
func New(st *store.Store, static fs.FS, log *slog.Logger) http.Handler {
	s := &server{st: st, static: static, log: log}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]bool{"ok": true}) })

	mux.HandleFunc("GET /api/orgs", s.listOrgs)
	mux.HandleFunc("POST /api/orgs", s.createOrg)
	mux.HandleFunc("GET /api/orgs/{org}", s.getOrg)

	mux.HandleFunc("GET /api/orgs/{org}/plans", s.listPlans)
	mux.HandleFunc("POST /api/orgs/{org}/plans", s.createPlan)
	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}", s.getPlan)

	mux.HandleFunc("GET /api/orgs/{org}/people", s.listPeople)
	mux.HandleFunc("POST /api/orgs/{org}/people", s.createPerson)
	mux.HandleFunc("PATCH /api/orgs/{org}/people/{id}", s.updatePerson)

	mux.HandleFunc("GET /api/orgs/{org}/log", s.listLog)

	s.setupRoutes(mux)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeErr(w, s.log, store.ErrNotFound) })

	index := func(w http.ResponseWriter, r *http.Request) { s.serveFile(w, "index.html") }
	mux.HandleFunc("GET /{$}", index)
	mux.HandleFunc("GET /o/{rest...}", index) // the page itself reads /o/<org>/p/<plan> from the address
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /app.js", func(w http.ResponseWriter, r *http.Request) { s.serveFile(w, "app.js") })
	mux.HandleFunc("GET /style.css", func(w http.ResponseWriter, r *http.Request) { s.serveFile(w, "style.css") })

	return secure(recoverer(mux, log))
}

func (s *server) serveFile(w http.ResponseWriter, name string) {
	b, err := fs.ReadFile(s.static, name)
	if err != nil {
		writeErr(w, s.log, store.ErrNotFound)
		return
	}
	switch {
	case name == "app.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case name == "style.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(b)
}

func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Robots-Tag", "noindex, nofollow") // no login: keep it out of search engines
		h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func recoverer(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Error("panic", "path", r.URL.Path, "value", v)
				writeJSON(w, 500, apiError{Error: "internal", Message: "internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---- helpers -------------------------------------------------------------------------------------------------

type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, 404, apiError{"not_found", "not found"})
	case errors.Is(err, store.ErrConflict):
		writeJSON(w, 409, apiError{"conflict", err.Error()})
	case errors.Is(err, store.ErrActor):
		writeJSON(w, 400, apiError{"actor_required", "send who you are in the X-Actor header (percent-encoded)"})
	case errors.Is(err, store.ErrInvalid):
		writeJSON(w, 400, apiError{"invalid", err.Error()})
	default:
		log.Error("request failed", "err", err)
		writeJSON(w, 500, apiError{"internal", "internal error"})
	}
}

// decode reads a small JSON body; unknown fields are an error so typos do not pass silently.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if ct := r.Header.Get("Content-Type"); len(ct) < 16 || ct[:16] != "application/json" {
		writeJSON(w, 415, apiError{"unsupported_media_type", "send Content-Type: application/json"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, 400, apiError{"invalid", "bad JSON body: " + err.Error()})
		return false
	}
	return true
}

func actor(r *http.Request) string { return r.Header.Get("X-Actor") }

// ---- handlers ------------------------------------------------------------------------------------------------

func (s *server) listOrgs(w http.ResponseWriter, r *http.Request) {
	orgs, err := s.st.ListOrgs(r.Context())
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, orgs)
}

type nameBody struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (s *server) createOrg(w http.ResponseWriter, r *http.Request) {
	var b nameBody
	if !decode(w, r, &b) {
		return
	}
	o, err := s.st.CreateOrg(r.Context(), actor(r), b.Name, b.Slug)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, o)
}

func (s *server) getOrg(w http.ResponseWriter, r *http.Request) {
	o, err := s.st.GetOrg(r.Context(), r.PathValue("org"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, o)
}

func (s *server) listPlans(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListPlans(r.Context(), r.PathValue("org"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) createPlan(w http.ResponseWriter, r *http.Request) {
	var b nameBody
	if !decode(w, r, &b) {
		return
	}
	p, err := s.st.CreatePlan(r.Context(), r.PathValue("org"), actor(r), b.Name, b.Slug)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, p)
}

func (s *server) getPlan(w http.ResponseWriter, r *http.Request) {
	p, err := s.st.GetPlan(r.Context(), r.PathValue("org"), r.PathValue("plan"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *server) listPeople(w http.ResponseWriter, r *http.Request) {
	v, err := s.st.ListPeople(r.Context(), r.PathValue("org"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *server) createPerson(w http.ResponseWriter, r *http.Request) {
	var b nameBody
	if !decode(w, r, &b) {
		return
	}
	p, err := s.st.CreatePerson(r.Context(), r.PathValue("org"), actor(r), b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, p)
}

func (s *server) updatePerson(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, s.log, store.ErrNotFound)
		return
	}
	var patch store.PersonPatch
	if !decode(w, r, &patch) {
		return
	}
	p, err := s.st.UpdatePerson(r.Context(), r.PathValue("org"), actor(r), id, patch)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *server) listLog(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	v, err := s.st.ListLog(r.Context(), r.PathValue("org"), r.URL.Query().Get("plan"), limit)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, v)
}
