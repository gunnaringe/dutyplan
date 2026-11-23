// Package api is the Connect API (proto/dutyplan/v1, JSON only) plus the embedded single-page frontend.
//
// People sign in with OpenID Connect (auth.go), and see and change only the organisations they are members of; without a
// login (New, or --no-login) everyone may do everything. Every request about an organisation names it (and its plan) in
// the message, so the server keeps no "current organisation"; procs.go decides access from those fields alone. Calls
// must be application/json, and cross-site writes are refused (http.CrossOriginProtection: Sec-Fetch-Site and Origin;
// no CORS is offered).
package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/gunnaringe/dutyplan/gen/dutyplan/v1/dutyplanv1connect"
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
	hub     *hub     // the open streams (stream.go)
	writes  *limiter // the rate limits (ratelimit.go, with a login only)
	dear    *limiter
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
	s := &server{st: st, planner: pl, static: static, log: log, version: map[string]string{}, hub: newHub(), auth: au,
		writes: newLimiter(writeBurst, writeEvery), dear: newLimiter(costlyBurst, costlyEvery)}
	pl.OnFinish(func(org, plan string) {
		s.hub.publish(org, change{Proc: "RunFinished", Plan: plan})
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

	// The API: Connect, JSON only (rpc.go), every call through the interceptor (procs.go).
	opts := []connect.HandlerOption{
		connect.WithCodec(jsonCodec{"json"}), connect.WithCodec(jsonCodec{"json; charset=utf-8"}),
		connect.WithInterceptors(s.interceptor()),
	}
	for _, mount := range []func() (string, http.Handler){
		func() (string, http.Handler) {
			return dutyplanv1connect.NewAccountServiceHandler(accountSvc{s}, opts...)
		},
		func() (string, http.Handler) { return dutyplanv1connect.NewOrgServiceHandler(orgSvc{s}, opts...) },
		func() (string, http.Handler) { return dutyplanv1connect.NewSetupServiceHandler(setupSvc{s}, opts...) },
		func() (string, http.Handler) { return dutyplanv1connect.NewPlanServiceHandler(planSvc{s}, opts...) },
		func() (string, http.Handler) { return dutyplanv1connect.NewDraftServiceHandler(draftSvc{s}, opts...) },
	} {
		path, h := mount()
		mux.Handle("POST "+path, requireJSON(s.gate, h))
	}

	// the stream of changes is Server-Sent Events, not Connect (stream.go)
	mux.HandleFunc("GET /api/orgs/{org}/stream", s.stream)
	if au != nil {
		mux.HandleFunc("GET /auth/login", au.login)
		mux.HandleFunc("GET /auth/callback", au.callback)
		mux.HandleFunc("POST /auth/logout", au.logout)
		mux.HandleFunc("GET /s/{token}", s.openLink)
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, apiError{Error: "not_found", Message: "not found"})
	})

	index := func(w http.ResponseWriter, r *http.Request) { s.serveFile(w, r, "index.html") }
	mux.HandleFunc("GET /{$}", index)
	mux.HandleFunc("GET /o/{rest...}", index) // the page itself reads /o/<org>/p/<plan> from the address
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, name := range assets {
		mux.HandleFunc("GET /"+name, func(w http.ResponseWriter, r *http.Request) { s.serveFile(w, r, name) })
	}

	return &Handler{Handler: secure(s.indexable, http.NewCrossOriginProtection().Handler(recoverer(s.who(mux), log))), hub: s.hub}
}

// assets are the files the page links; each gets ?v=<hash of its content> where index.html links it.
const robotsMeta = `<meta name="robots" content="noindex, nofollow">`

var assets = []string{"i18n.js", "app.js", "style.css", "landing.png", "demo-en.mp4", "demo-nb.mp4", "signature.woff2", "logo.svg", "llms.txt"}

// serveFile sends one embedded file. index.html gets the assets' versions in its links. An asset asked for with its
// current version never changes, so it may be cached for a year; anything else (the page, an asset without or with an
// old version) must be checked again every time.
func (s *server) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	b, err := fs.ReadFile(s.static, name)
	if err != nil {
		writeJSON(w, 404, apiError{Error: "not_found", Message: "not found"})
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
	case strings.HasSuffix(name, ".woff2"):
		w.Header().Set("Content-Type", "font/woff2")
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
