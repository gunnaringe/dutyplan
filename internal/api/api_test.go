package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gunnaringe/dutyplan/internal/planner"
	"github.com/gunnaringe/dutyplan/internal/store"
)

type env struct {
	t  *testing.T
	h  http.Handler
	st *store.Store
	pl *planner.Service
}

func newEnv(t *testing.T) *env { return newEnvWith(t, nil) }

// newEnvWith: a server whose planner runs the given solver (nil = no planner configured).
func newEnvWith(t *testing.T, solver planner.Solver) *env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	static := fstest.MapFS{"index.html": {Data: []byte("<!doctype html>INDEX")}, "i18n.js": {Data: []byte("//i18n")}, "app.js": {Data: []byte("//js")}, "style.css": {Data: []byte("/*css*/")}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pl := planner.New(st, solver, 1, log)
	return &env{t: t, h: New(st, pl, static, log), st: st, pl: pl}
}

type reply struct {
	code int
	body []byte
	hdr  http.Header
}

func (r reply) json(v any) {
	if err := json.Unmarshal(r.body, v); err != nil {
		panic(err)
	}
}
func (r reply) errCode() string {
	var e apiError
	json.Unmarshal(r.body, &e)
	return e.Error
}

// do sends a request, with a JSON body when body is not nil.
func (e *env) do(method, path string, body any) reply {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return reply{w.Code, w.Body.Bytes(), w.Header()}
}

func (e *env) mustDo(want int, method, path string, body any) reply {
	e.t.Helper()
	r := e.do(method, path, body)
	if r.code != want {
		e.t.Fatalf("%s %s = %d (%s), want %d", method, path, r.code, r.body, want)
	}
	return r
}

func TestFlowAndSwitchingOrganisation(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Autumn 2026"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Spring 2027"})
	e.mustDo(201, "POST", "/api/orgs/org-b/plans", map[string]string{"name": "Autumn 2026"})

	var orgs []store.Org
	e.mustDo(200, "GET", "/api/orgs", nil).json(&orgs)
	if len(orgs) != 2 {
		t.Fatalf("orgs = %v", orgs)
	}
	var plansA, plansB []store.Plan
	e.mustDo(200, "GET", "/api/orgs/org-a/plans", nil).json(&plansA)
	e.mustDo(200, "GET", "/api/orgs/org-b/plans", nil).json(&plansB)
	if len(plansA) != 2 || len(plansB) != 1 {
		t.Fatalf("each organisation has its own plans: %v / %v", plansA, plansB)
	}
	var p store.Plan
	e.mustDo(200, "GET", "/api/orgs/org-a/plans/autumn-2026", nil).json(&p)
	if p.Name != "Autumn 2026" {
		t.Fatalf("plan = %+v", p)
	}
}

func TestWritesNeedJSONAndKnownFields(t *testing.T) {
	e := newEnv(t)
	req := httptest.NewRequest("POST", "/api/orgs", strings.NewReader(`name=X`))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != 415 {
		t.Errorf("form post must be refused (cross-site writes), got %d", w.Code)
	}
	if r := e.do("POST", "/api/orgs", map[string]string{"name": "X Org", "title": "oops"}); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("unknown field next to a valid one must be refused: %d %s", r.code, r.body)
	}
	// A valid body padded past the size limit: only the limit can reject it (the content itself is fine).
	padded := `{"name":"Padded Org"` + strings.Repeat(" ", 70<<10) + `}`
	req = httptest.NewRequest("POST", "/api/orgs", strings.NewReader(padded))
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	e.h.ServeHTTP(w2, req)
	if w2.Code != 400 {
		t.Errorf("oversized body must be refused, got %d", w2.Code)
	}
	if r := e.do("POST", "/api/orgs", map[string]string{"name": "   "}); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("blank name: %d %s", r.code, r.body)
	}
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	if r := e.do("POST", "/api/orgs", map[string]string{"name": "Org A"}); r.code != 409 {
		t.Errorf("duplicate: %d", r.code)
	}
}

// A browser refuses nothing by itself for a cross-site DELETE or a bodiless PUT, so the server does: the request a
// page on another site sends carries Sec-Fetch-Site: cross-site (or an Origin that is not this host).
func TestCrossSiteWritesAreRefused(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	for _, h := range []map[string]string{{"Sec-Fetch-Site": "cross-site"}, {"Origin": "https://elsewhere.example"}} {
		req := httptest.NewRequest("DELETE", "/api/orgs/org-a?confirm=org-a", nil)
		for k, v := range h {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Errorf("%v: a cross-site delete must be refused, got %d", h, w.Code)
		}
	}
	same := httptest.NewRequest("GET", "/api/orgs", nil)
	same.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, same)
	if w.Code != 200 {
		t.Errorf("reads stay open: %d", w.Code)
	}
	e.mustDo(204, "DELETE", "/api/orgs/org-a?confirm=org-a", nil)
}

// Cross-organisation requests look exactly like requests for things that do not exist.
func TestCrossOrganisationRequestsAre404(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Autumn 2026"})
	var person store.Person
	e.mustDo(201, "POST", "/api/orgs/org-a/people", map[string]string{"name": "Kari Test"}).json(&person)

	for _, c := range []struct{ method, path string }{
		{"GET", "/api/orgs/org-b/plans/autumn-2026"},
		{"PATCH", "/api/orgs/org-b/people/" + itoa(person.ID)},
		{"GET", "/api/orgs/nope/plans"},
		{"POST", "/api/orgs/nope/people"},
		{"GET", "/api/orgs/nope/people"},
		{"PATCH", "/api/orgs/org-a/people/abc"},
		{"PATCH", "/api/orgs/org-a/people/99999"},
		{"GET", "/api/nope/nothing"},
	} {
		var body any
		if c.method != "GET" {
			body = map[string]any{"proposable": false, "name": "Kari Test"}
			if strings.HasSuffix(c.path, "/people") {
				body = map[string]string{"name": "Ny"}
			}
		}
		if r := e.do(c.method, c.path, body); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("%s %s = %d %s, want 404 not_found", c.method, c.path, r.code, r.body)
		}
	}
	var people []store.Person
	e.mustDo(200, "GET", "/api/orgs/org-a/people", nil).json(&people)
	if len(people) != 1 || !people[0].Proposable {
		t.Errorf("A's person must be untouched: %v", people)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestPersonPatch(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	var p store.Person
	e.mustDo(201, "POST", "/api/orgs/org-a/people", map[string]string{"name": "Kari Test"}).json(&p)
	e.mustDo(200, "PATCH", "/api/orgs/org-a/people/"+itoa(p.ID), map[string]any{"proposable": false}).json(&p)
	if p.Proposable || !p.Active {
		t.Errorf("patch = %+v", p)
	}
	if r := e.do("PATCH", "/api/orgs/org-a/people/"+itoa(p.ID), map[string]any{}); r.code != 400 {
		t.Errorf("empty patch: %d", r.code)
	}
}

func TestFrontendIsServedWithSafeHeaders(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/", "/o/org-a", "/o/org-a/p/autumn-2026"} {
		r := e.do("GET", path, nil)
		if r.code != 200 || !strings.Contains(string(r.body), "INDEX") {
			t.Errorf("%s = %d %q", path, r.code, r.body)
		}
	}
	for _, js := range []string{"/app.js", "/i18n.js"} {
		r := e.do("GET", js, nil)
		if r.code != 200 || !strings.HasPrefix(r.hdr.Get("Content-Type"), "text/javascript") {
			t.Errorf("%s = %d %v", js, r.code, r.hdr)
		}
	}
	r := e.do("GET", "/app.js", nil)
	if r.code != 200 || !strings.HasPrefix(r.hdr.Get("Content-Type"), "text/javascript") {
		t.Errorf("app.js = %d %v", r.code, r.hdr)
	}
	for _, h := range []string{"X-Content-Type-Options", "Content-Security-Policy", "X-Robots-Tag", "Referrer-Policy"} {
		if r.hdr.Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
	if r := e.do("GET", "/favicon.ico", nil); r.code != 204 {
		t.Errorf("favicon = %d, want 204 (keeps the browser console clean)", r.code)
	}
	if r := e.do("GET", "/hemmelig.txt", nil); r.code != 404 {
		t.Errorf("unknown path = %d, want 404 (no file listing)", r.code)
	}
	if r := e.do("GET", "/api/health", nil); r.code != 200 {
		t.Errorf("health = %d", r.code)
	}
}

// addWithPlaces adds the events and gives each the places (task -> places), the way the page does it.
func addWithPlaces(st *store.Store, org, plan string, in store.NewEvents, places map[int64]int) ([]store.Event, error) {
	evs, err := st.AddEvents(context.Background(), org, plan, in)
	if err != nil {
		return nil, err
	}
	for _, e := range evs {
		for task, n := range places {
			if err := st.SetEventPlaces(context.Background(), org, plan, e.ID, task, n); err != nil {
				return nil, err
			}
		}
	}
	return evs, nil
}
