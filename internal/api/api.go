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
	"net/url"
	"strconv"

	"github.com/gunnaringe/dutyplan/internal/planner"
	"github.com/gunnaringe/dutyplan/internal/store"
)

type server struct {
	st      *store.Store
	planner *planner.Service
	static  fs.FS
	log     *slog.Logger
}

// New builds the handler. static must contain index.html, i18n.js, app.js and style.css. pl may be nil: planning then answers
// 503 «unavailable» and everything else works.
func New(st *store.Store, pl *planner.Service, static fs.FS, log *slog.Logger) http.Handler {
	if pl == nil {
		pl = planner.New(st, nil, 1, log)
	}
	s := &server{st: st, planner: pl, static: static, log: log}
	mux := http.NewServeMux()

	// planner: can this server plan at all? The page hides «Plan» when not.
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]bool{"ok": true, "planner": s.planner.Available()})
	})

	mux.HandleFunc("GET /api/orgs", s.listOrgs)
	mux.HandleFunc("POST /api/orgs", s.createOrg)
	mux.HandleFunc("GET /api/orgs/{org}", s.getOrg)
	mux.HandleFunc("PATCH /api/orgs/{org}", s.renameOrg)
	mux.HandleFunc("DELETE /api/orgs/{org}", s.deleteOrg)

	mux.HandleFunc("GET /api/orgs/{org}/plans", s.listPlans)
	mux.HandleFunc("POST /api/orgs/{org}/plans", s.createPlan)
	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}", s.getPlan)
	mux.HandleFunc("PATCH /api/orgs/{org}/plans/{plan}", s.renamePlan)
	mux.HandleFunc("DELETE /api/orgs/{org}/plans/{plan}", s.deletePlan)

	mux.HandleFunc("GET /api/orgs/{org}/people", s.listPeople)
	mux.HandleFunc("POST /api/orgs/{org}/people", s.createPerson)
	mux.HandleFunc("PATCH /api/orgs/{org}/people/{id}", s.updatePerson)

	mux.HandleFunc("GET /api/orgs/{org}/log", s.listLog)

	s.setupRoutes(mux)
	s.groupsRoutes(mux)
	s.cellsRoutes(mux)
	s.weightsRoutes(mux)
	s.runsRoutes(mux)
	s.tableRoutes(mux)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeErr(w, s.log, store.ErrNotFound) })

	index := func(w http.ResponseWriter, r *http.Request) { s.serveFile(w, "index.html") }
	mux.HandleFunc("GET /{$}", index)
	mux.HandleFunc("GET /o/{rest...}", index) // the page itself reads /o/<org>/p/<plan> from the address
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /i18n.js", func(w http.ResponseWriter, r *http.Request) { s.serveFile(w, "i18n.js") })
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
	case name == "app.js" || name == "i18n.js":
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

// apiError is the body of every refused request. Error is the kind (not_found, invalid, conflict, ...), Code and
// Params say precisely what was wrong so the page can write it in the user's language, and Message is the English text.
type apiError struct {
	Error   string         `json:"error"`
	Code    string         `json:"code,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
	Message string         `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// coded builds the error body, with the code and parameters when the error carries them.
func coded(kind string, err error) apiError {
	e := apiError{Error: kind, Message: err.Error()}
	var c *store.Coded
	if errors.As(err, &c) {
		e.Code, e.Params = c.Code, c.Params
	}
	return e
}

func writeErr(w http.ResponseWriter, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, 404, apiError{Error: "not_found", Message: "not found"})
	case errors.Is(err, store.ErrConflict):
		writeJSON(w, 409, coded("conflict", err))
	case errors.Is(err, store.ErrActor):
		writeJSON(w, 400, apiError{Error: "actor_required", Message: "send who you are in the X-Actor header (percent-encoded)"})
	case errors.Is(err, store.ErrInvalid):
		writeJSON(w, 400, coded("invalid", err))
	default:
		log.Error("request failed", "err", err)
		writeJSON(w, 500, apiError{Error: "internal", Message: "internal error"})
	}
}

// decode reads a small JSON body; unknown fields are an error so typos do not pass silently.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if ct := r.Header.Get("Content-Type"); len(ct) < 16 || ct[:16] != "application/json" {
		writeJSON(w, 415, apiError{Error: "unsupported_media_type", Message: "send Content-Type: application/json"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, 400, apiError{Error: "invalid", Message: "bad JSON body: " + err.Error()})
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

// renameOrg changes the name only; the address stays.
func (s *server) renameOrg(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	o, err := s.st.RenameOrg(r.Context(), actor(r), r.PathValue("org"), b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, o)
}

// deleteOrg removes an organisation without plans. The caller confirms with the organisation's address name in
// ?confirm=, so a stray DELETE cannot do it. The organisation's change log goes with it, so the deletion is written to
// the server log instead.
func (s *server) deleteOrg(w http.ResponseWriter, r *http.Request) {
	o, err := s.st.DeleteOrg(r.Context(), actor(r), r.PathValue("org"), r.URL.Query().Get("confirm"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	who, _ := url.PathUnescape(actor(r))
	s.log.Warn("organisation deleted", "org", o.Slug, "name", o.Name, "by", who)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) renamePlan(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	p, err := s.st.RenamePlan(r.Context(), r.PathValue("org"), r.PathValue("plan"), actor(r), b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *server) deletePlan(w http.ResponseWriter, r *http.Request) {
	noContent(w, s.st.DeletePlan(r.Context(), r.PathValue("org"), r.PathValue("plan"), actor(r)), s)
}
