// Package api is the JSON HTTP API plus the embedded single-page frontend.
//
// People sign in with OpenID Connect (auth.go), and see and change only the organisations they are members of; without a
// login (New, or --no-login) everyone may do everything. The organisation is part of every path (/api/orgs/{org}/...), so
// the server keeps no "current organisation" and switching is just navigating. Writes with a body must be
// application/json, and cross-site writes are refused (http.CrossOriginProtection: Sec-Fetch-Site and Origin; no CORS is
// offered).
package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gunnaringe/dutyplan/internal/planner"
	"github.com/gunnaringe/dutyplan/internal/store"
)

type server struct {
	st      *store.Store
	planner *planner.Service
	static  fs.FS
	// version holds a short hash of each asset's content. The page links them as /app.js?v=<hash>, so a deploy gives them new
	// addresses and no browser or proxy cache can hand out an old script with a new page (seen 2026-11-20: an old app.js asked
	// for a removed API and showed «Page not found»).
	version map[string]string
	log     *slog.Logger
	hub     *hub  // the open streams (stream.go)
	auth    *auth // nil: no login
}

// New builds the handler without a login. static must contain index.html, i18n.js, app.js and style.css. pl may be nil:
// planning then answers 503 «unavailable» and everything else works.
func New(st *store.Store, pl *planner.Service, static fs.FS, log *slog.Logger) *Handler {
	return build(st, pl, static, log, nil)
}

// NewWithLogin builds the handler with signing in (auth.go).
func NewWithLogin(st *store.Store, pl *planner.Service, static fs.FS, log *slog.Logger, login Login) (*Handler, error) {
	au, err := newAuth(login, st, log)
	if err != nil {
		return nil, err
	}
	return build(st, pl, static, log, au), nil
}

func build(st *store.Store, pl *planner.Service, static fs.FS, log *slog.Logger, au *auth) *Handler {
	if pl == nil {
		pl = planner.New(st, nil, 1, log)
	}
	s := &server{st: st, planner: pl, static: static, log: log, version: map[string]string{}, hub: newHub(), auth: au}
	pl.OnFinish(func(org, plan string) {
		s.hub.publish(org, change{Method: "RUN", Path: "/orgs/" + org + "/plans/" + plan + "/runs"})
	})
	for _, name := range assets {
		if static == nil {
			break
		}
		if b, err := fs.ReadFile(static, name); err == nil {
			sum := sha256.Sum256(b)
			s.version[name] = hex.EncodeToString(sum[:6])
		}
	}
	mux := http.NewServeMux()

	// planner: can this server plan at all? The page hides «Plan» when not.
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]bool{"ok": true, "planner": s.planner.Available()})
	})

	mux.HandleFunc("GET /api/me", s.me)
	if au != nil {
		mux.HandleFunc("GET /auth/login", au.login)
		mux.HandleFunc("GET /auth/callback", au.callback)
		mux.HandleFunc("POST /auth/logout", au.logout)
		mux.HandleFunc("GET /s/{token}", s.openLink)
	}

	mux.HandleFunc("GET /api/orgs", s.listOrgs)
	mux.HandleFunc("POST /api/orgs", s.createOrg)
	mux.HandleFunc("GET /api/orgs/{org}", s.getOrg)
	mux.HandleFunc("PATCH /api/orgs/{org}", s.renameOrg)
	mux.HandleFunc("GET /api/orgs/{org}/stream", s.stream)
	mux.HandleFunc("DELETE /api/orgs/{org}", s.deleteOrg)

	mux.HandleFunc("GET /api/orgs/{org}/plans", s.listPlans)
	mux.HandleFunc("POST /api/orgs/{org}/plans", s.createPlan)
	mux.HandleFunc("GET /api/orgs/{org}/plans/{plan}", s.getPlan)
	mux.HandleFunc("PATCH /api/orgs/{org}/plans/{plan}", s.renamePlan)
	mux.HandleFunc("DELETE /api/orgs/{org}/plans/{plan}", s.deletePlan)

	mux.HandleFunc("GET /api/orgs/{org}/people", s.listPeople)
	mux.HandleFunc("POST /api/orgs/{org}/people", s.createPerson)
	mux.HandleFunc("PATCH /api/orgs/{org}/people/{id}", s.updatePerson)

	s.setupRoutes(mux)
	s.cellsRoutes(mux)
	s.weightsRoutes(mux)
	s.runsRoutes(mux)
	s.tableRoutes(mux)
	s.sectionsRoutes(mux)
	s.linkRoutes(mux)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeErr(w, s.log, store.ErrNotFound) })

	index := func(w http.ResponseWriter, r *http.Request) { s.serveFile(w, r, "index.html") }
	mux.HandleFunc("GET /{$}", index)
	mux.HandleFunc("GET /o/{rest...}", index) // the page itself reads /o/<org>/p/<plan> from the address
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, name := range assets {
		mux.HandleFunc("GET /"+name, func(w http.ResponseWriter, r *http.Request) { s.serveFile(w, r, name) })
	}

	return &Handler{Handler: secure(s.indexable, http.NewCrossOriginProtection().Handler(recoverer(s.access(s.limit(s.notifying(mux))), log))), hub: s.hub}
}

// assets are the files the page links; each gets ?v=<hash of its content> where index.html links it.
const robotsMeta = `<meta name="robots" content="noindex, nofollow">`

var assets = []string{"i18n.js", "app.js", "style.css", "landing.png", "demo-en.mp4", "demo-nb.mp4", "logo.svg", "llms.txt"}

// serveFile sends one embedded file. index.html gets the assets' versions in its links. An asset asked for with its
// current version never changes, so it may be cached for a year; anything else (the page, an asset without or with an
// old version) must be checked again every time.
func (s *server) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	b, err := fs.ReadFile(s.static, name)
	if err != nil {
		writeErr(w, s.log, store.ErrNotFound)
		return
	}
	if name == "index.html" {
		page := string(b)
		for _, a := range assets {
			page = strings.ReplaceAll(page, `"/`+a+`"`, `"/`+a+`?v=`+s.version[a]+`"`)
		}
		if s.indexable(r) {
			page = strings.Replace(page, robotsMeta, "", 1)
		}
		b = []byte(page)
	}
	switch {
	case name == "app.js" || name == "i18n.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case name == "style.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(name, ".png"):
		w.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(name, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(name, ".mp4"):
		w.Header().Set("Content-Type", "video/mp4")
	case strings.HasSuffix(name, ".txt"):
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	if v := r.URL.Query().Get("v"); v != "" && v == s.version[name] {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	// The version as an ETag: what is checked again costs a 304, not the whole file. ServeContent also answers range
	// requests (a browser asks for a video in parts) and If-None-Match.
	if v := s.version[name]; v != "" {
		w.Header().Set("ETag", `"`+v+`"`)
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(b))
}

// indexable: only the start page with a login (the landing page says what this is); everything else, and all of it
// without a login, is kept out of search engines.
func (s *server) indexable(r *http.Request) bool { return s.auth != nil && r.URL.Path == "/" }

func secure(indexable func(*http.Request) bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		if !indexable(r) {
			h.Set("X-Robots-Tag", "noindex, nofollow")
		}
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
	case errors.Is(err, store.ErrInvalid):
		writeJSON(w, 400, coded("invalid", err))
	default:
		log.Error("request failed", "err", err)
		writeJSON(w, 500, apiError{Error: "internal", Message: "internal error"})
	}
}

// decode reads a small JSON body; unknown fields are an error so typos do not pass silently.
func decode(w http.ResponseWriter, r *http.Request, v any) bool { return decodeMax(w, r, v, 64<<10) }

// decodeMax is decode with another limit on the body (the bulk writes of a selection are bigger).
func decodeMax(w http.ResponseWriter, r *http.Request, v any, max int64) bool {
	if ct := r.Header.Get("Content-Type"); len(ct) < 16 || ct[:16] != "application/json" {
		writeJSON(w, 415, apiError{Error: "unsupported_media_type", Message: "send Content-Type: application/json"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, max)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, 400, apiError{Error: "invalid", Message: "bad JSON body: " + err.Error()})
		return false
	}
	return true
}

// ---- handlers ------------------------------------------------------------------------------------------------

// listOrgs: with a login, the organisations the person is a member of, and those of the links opened in this browser.
func (s *server) listOrgs(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		orgs, err := s.st.ListOrgs(r.Context())
		if err != nil {
			writeErr(w, s.log, err)
			return
		}
		writeJSON(w, 200, orgs)
		return
	}
	orgs := []store.Org{}
	var err error
	if u, ok := userFrom(r); ok {
		orgs, err = s.st.ListOrgsOf(r.Context(), u.ID)
	}
	for _, l := range s.links(r) {
		if !slices.ContainsFunc(orgs, func(o store.Org) bool { return o.Slug == l.Org }) {
			orgs = append(orgs, store.Org{Slug: l.Org, Name: l.OrgName})
		}
	}
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, orgs)
}

type nameBody struct {
	Name string `json:"name"`
}

func (s *server) createOrg(w http.ResponseWriter, r *http.Request) {
	var b nameBody
	if !decode(w, r, &b) {
		return
	}
	var o store.Org
	var err error
	if u, ok := userFrom(r); ok {
		o, err = s.st.CreateOrgOwnedBy(r.Context(), b.Name, u.ID) // the one who makes it owns it
	} else {
		o, err = s.st.CreateOrg(r.Context(), b.Name)
	}
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 201, o)
}

// getOrg also says what the asker may do there: role admin or member (without a login everyone is admin), or else
// the level of their link (view or edit) and, for a link to one plan, that plan.
func (s *server) getOrg(w http.ResponseWriter, r *http.Request) {
	o, err := s.st.GetOrg(r.Context(), r.PathValue("org"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	g := grantFrom(r)
	if s.auth == nil {
		g.role = store.RoleAdmin
	}
	writeJSON(w, 200, struct {
		store.Org
		Role  string `json:"role,omitempty"`
		Level string `json:"level,omitempty"`
		Plan  string `json:"plan,omitempty"`
	}{o, g.role, g.level, g.plan})
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
	p, err := s.st.CreatePlan(r.Context(), r.PathValue("org"), b.Name)
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
	p, err := s.st.CreatePerson(r.Context(), r.PathValue("org"), b.Name)
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
	p, err := s.st.UpdatePerson(r.Context(), r.PathValue("org"), id, patch)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, p)
}

// renameOrg changes the name only; the address stays.
func (s *server) renameOrg(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	o, err := s.st.RenameOrg(r.Context(), r.PathValue("org"), b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, o)
}

// deleteOrg removes an organisation without plans. The caller confirms with the organisation's address name in
// ?confirm=, so a stray DELETE cannot do it. The deletion is written to the server log.
func (s *server) deleteOrg(w http.ResponseWriter, r *http.Request) {
	o, err := s.st.DeleteOrg(r.Context(), r.PathValue("org"), r.URL.Query().Get("confirm"))
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	s.log.Warn("organisation deleted", "org", o.Slug, "name", o.Name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) renamePlan(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	p, err := s.st.RenamePlan(r.Context(), r.PathValue("org"), r.PathValue("plan"), b.Name)
	if err != nil {
		writeErr(w, s.log, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *server) deletePlan(w http.ResponseWriter, r *http.Request) {
	noContent(w, s.st.DeletePlan(r.Context(), r.PathValue("org"), r.PathValue("plan")), s)
}
