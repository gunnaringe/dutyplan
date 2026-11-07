package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gunnaringe/tjenesteplan/internal/store"
)

type env struct {
	t *testing.T
	h http.Handler
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	static := fstest.MapFS{"index.html": {Data: []byte("<!doctype html>INDEX")}, "app.js": {Data: []byte("//js")}, "style.css": {Data: []byte("/*css*/")}}
	return &env{t, New(st, static, slog.New(slog.NewTextHandler(io.Discard, nil)))}
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

// do sends a request; actor is the percent-encoded X-Actor value (empty = header not sent).
func (e *env) do(method, path, actor string, body any) reply {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set("X-Actor", actor)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return reply{w.Code, w.Body.Bytes(), w.Header()}
}

func (e *env) mustDo(want int, method, path string, body any) reply {
	e.t.Helper()
	r := e.do(method, path, "Test", body)
	if r.code != want {
		e.t.Fatalf("%s %s = %d (%s), want %d", method, path, r.code, r.body, want)
	}
	return r
}

func TestFlowAndSwitchingOrganisation(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Høst 2026"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Vår 2027"})
	e.mustDo(201, "POST", "/api/orgs/org-b/plans", map[string]string{"name": "Høst 2026"})

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
	e.mustDo(200, "GET", "/api/orgs/org-a/plans/host-2026", nil).json(&p)
	if p.Name != "Høst 2026" {
		t.Fatalf("plan = %+v", p)
	}
}

func TestWritesNeedActorJSONAndKnownFields(t *testing.T) {
	e := newEnv(t)
	if r := e.do("POST", "/api/orgs", "", map[string]string{"name": "X Org"}); r.code != 400 || r.errCode() != "actor_required" {
		t.Errorf("no actor: %d %s", r.code, r.body)
	}
	req := httptest.NewRequest("POST", "/api/orgs", strings.NewReader(`name=X`))
	req.Header.Set("X-Actor", "T")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != 415 {
		t.Errorf("form post must be refused (cross-site writes), got %d", w.Code)
	}
	if r := e.do("POST", "/api/orgs", "T", map[string]string{"name": "X Org", "navn": "oops"}); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("unknown field next to a valid one must be refused: %d %s", r.code, r.body)
	}
	// A valid body padded past the size limit: only the limit can reject it (the content itself is fine).
	padded := `{"name":"Padded Org"` + strings.Repeat(" ", 70<<10) + `}`
	req = httptest.NewRequest("POST", "/api/orgs", strings.NewReader(padded))
	req.Header.Set("X-Actor", "T")
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	e.h.ServeHTTP(w2, req)
	if w2.Code != 400 {
		t.Errorf("oversized body must be refused, got %d", w2.Code)
	}
	if r := e.do("POST", "/api/orgs", "T", map[string]string{"name": "   "}); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("blank name: %d %s", r.code, r.body)
	}
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	if r := e.do("POST", "/api/orgs", "T", map[string]string{"name": "Org A"}); r.code != 409 {
		t.Errorf("duplicate: %d", r.code)
	}
	var log []store.LogEntry
	e.mustDo(200, "GET", "/api/orgs/org-a/log", nil).json(&log)
	if len(log) != 1 {
		t.Errorf("only the one successful write is logged: %v", log)
	}
}

// Cross-organisation requests look exactly like requests for things that do not exist.
func TestCrossOrganisationRequestsAre404(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Høst 2026"})
	var person store.Person
	e.mustDo(201, "POST", "/api/orgs/org-a/people", map[string]string{"name": "Kari Test"}).json(&person)

	for _, c := range []struct{ method, path string }{
		{"GET", "/api/orgs/org-b/plans/host-2026"},
		{"PATCH", "/api/orgs/org-b/people/" + itoa(person.ID)},
		{"GET", "/api/orgs/ingen/plans"},
		{"POST", "/api/orgs/ingen/people"},
		{"GET", "/api/orgs/ingen/log"},
		{"PATCH", "/api/orgs/org-a/people/abc"},
		{"PATCH", "/api/orgs/org-a/people/99999"},
		{"GET", "/api/ingen/ting"},
	} {
		var body any
		if c.method != "GET" {
			body = map[string]any{"proposable": false, "name": "Kari Test"}
			if strings.HasSuffix(c.path, "/people") {
				body = map[string]string{"name": "Ny"}
			}
		}
		if r := e.do(c.method, c.path, "T", body); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("%s %s = %d %s, want 404 not_found", c.method, c.path, r.code, r.body)
		}
	}
	var people []store.Person
	e.mustDo(200, "GET", "/api/orgs/org-a/people", nil).json(&people)
	if len(people) != 1 || !people[0].Proposable {
		t.Errorf("A's person must be untouched: %v", people)
	}
	var logB []store.LogEntry
	e.mustDo(200, "GET", "/api/orgs/org-b/log", nil).json(&logB)
	if len(logB) != 1 || logB[0].Action != "org.create" {
		t.Errorf("B's log shows only B's own creation: %v", logB)
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
	if r := e.do("PATCH", "/api/orgs/org-a/people/"+itoa(p.ID), "T", map[string]any{}); r.code != 400 {
		t.Errorf("empty patch: %d", r.code)
	}
}

func TestFrontendIsServedWithSafeHeaders(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/", "/o/org-a", "/o/org-a/p/host-2026"} {
		r := e.do("GET", path, "", nil)
		if r.code != 200 || !strings.Contains(string(r.body), "INDEX") {
			t.Errorf("%s = %d %q", path, r.code, r.body)
		}
	}
	r := e.do("GET", "/app.js", "", nil)
	if r.code != 200 || !strings.HasPrefix(r.hdr.Get("Content-Type"), "text/javascript") {
		t.Errorf("app.js = %d %v", r.code, r.hdr)
	}
	for _, h := range []string{"X-Content-Type-Options", "Content-Security-Policy", "X-Robots-Tag", "Referrer-Policy"} {
		if r.hdr.Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
	if r := e.do("GET", "/favicon.ico", "", nil); r.code != 204 {
		t.Errorf("favicon = %d, want 204 (keeps the browser console clean)", r.code)
	}
	if r := e.do("GET", "/hemmelig.txt", "", nil); r.code != 404 {
		t.Errorf("unknown path = %d, want 404 (no file listing)", r.code)
	}
	if r := e.do("GET", "/api/health", "", nil); r.code != 200 {
		t.Errorf("health = %d", r.code)
	}
}
