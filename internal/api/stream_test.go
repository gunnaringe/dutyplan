package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// listen opens the organisation's stream on a real server and returns the messages («data:» lines) as they come.
func listen(t *testing.T, srv *httptest.Server, org string) <-chan string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/orgs/"+org+"/stream", nil)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	out := make(chan string, 16)
	go func() {
		defer res.Body.Close()
		defer close(out)
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				out <- data
			}
		}
	}()
	return out
}

// next is the next message, or "" when none comes in a short while.
func next(ch <-chan string, wait time.Duration) string {
	select {
	case m := <-ch:
		return m
	case <-time.After(wait):
		return ""
	}
}

func (e *env) doAs(client, method, path string, body any) reply {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(clientHeader, client)
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return reply{w.Code, w.Body.Bytes(), w.Header()}
}

func TestAWriteIsAnnouncedOnTheStreamOfItsOrganisationOnly(t *testing.T) {
	e := newEnv(t)
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Autumn 2026"})
	a, b := listen(t, srv, "org-a"), listen(t, srv, "org-b")

	if r := e.doAs("tab-1", "POST", "/api/orgs/org-a/plans/"+term+"/people", map[string]string{"name": "Aslaug"}); r.code != 201 {
		t.Fatalf("create person: %d %s", r.code, r.body)
	}
	var c change
	m := next(a, 2*time.Second)
	if json.Unmarshal([]byte(m), &c) != nil || c != (change{Origin: "tab-1", Proc: "CreatePerson", Plan: term}) {
		t.Fatalf("org A's stream got %q, want the change with its origin, method and path", m)
	}
	if m := next(b, 200*time.Millisecond); m != "" {
		t.Fatalf("org B's stream got %q for a change in org A", m)
	}
	// reading and a refused write change nothing, so they are not announced
	e.mustDo(200, "GET", "/api/orgs/org-a/plans/"+term+"/people", nil)
	if r := e.doAs("tab-1", "POST", "/api/orgs/org-a/plans/"+term+"/people", map[string]string{"name": ""}); r.code < 400 {
		t.Fatalf("an empty name was accepted: %d", r.code)
	}
	if m := next(a, 200*time.Millisecond); m != "" {
		t.Fatalf("org A's stream got %q without a change", m)
	}
}

func TestTheStreamOfAnUnknownOrganisationIsNotFound(t *testing.T) {
	e := newEnv(t)
	if r := e.do("GET", "/api/orgs/nope/stream", nil); r.code != 404 {
		t.Fatalf("got %d, want 404", r.code)
	}
}

func TestTheEndOfARunIsAnnounced(t *testing.T) {
	release := make(chan struct{})
	e := newEnvWith(t, fakeSolver(func(context.Context, []byte) ([]byte, error) { <-release; return emptyAnswer, nil }))
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	org, plan := e.plannable("Org A")
	s := listen(t, srv, org)
	e.mustDo(202, "POST", "/api/orgs/"+org+"/plans/"+plan+"/runs", map[string]int{"time_limit": 10})
	if m := next(s, 2*time.Second); m == "" {
		t.Fatal("starting a run was not announced")
	}
	close(release)
	e.wait()
	var c change
	m := next(s, 2*time.Second)
	if json.Unmarshal([]byte(m), &c) != nil || c != (change{Proc: "RunFinished", Plan: plan}) {
		t.Fatalf("the end of the run: got %q, want the plan's runs, without an origin", m)
	}
	var latest struct {
		Run struct {
			Status string `json:"status"`
		} `json:"run"`
	}
	e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/"+plan+"/runs/latest", nil).json(&latest)
	if latest.Run.Status != "done" {
		t.Fatalf("announced before the result was stored: %q", latest.Run.Status)
	}
}

func TestCloseStreamsEndsTheOpenStreams(t *testing.T) {
	e := newEnv(t)
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	s := listen(t, srv, "org-a")
	e.app.CloseStreams()
	select {
	case _, open := <-s:
		if open {
			t.Fatal("a message instead of the end")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the stream stayed open")
	}
}

func TestAPublishNeverBlocksAndMergesOrigins(t *testing.T) {
	h := newHub()
	ch, cancel, ok := h.subscribe("o")
	if !ok {
		t.Fatal("subscribe")
	}
	defer cancel()
	a := change{Origin: "tab-1", Proc: "AddCell", Plan: "p"}
	b := change{Origin: "tab-2", Proc: "AddCell", Plan: "p"}
	h.publish("o", a)
	h.publish("o", b) // the stream has not sent the first yet
	if c := <-ch; c.Origin != "" {
		t.Fatalf("two tabs' changes kept origin %q; the first tab would skip the second's", c.Origin)
	}
	h.publish("o", a)
	h.publish("o", change{Origin: "tab-1", Proc: "SetPool", Plan: "p"})
	if c := <-ch; c != (change{Origin: "tab-1"}) {
		t.Fatalf("two different changes merged into %+v, want one without a procedure (anything may have changed), from tab-1", c)
	}
	h.publish("o", a)
	h.publish("o", a)
	if c := <-ch; c != a {
		t.Fatalf("the same change twice became %+v", c)
	}
}

func TestTheStreamIsPingedSoProxiesKeepItOpen(t *testing.T) {
	old := pingEvery
	pingEvery = 20 * time.Millisecond
	defer func() { pingEvery = old }()
	e := newEnv(t)
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	res, err := srv.Client().Get(srv.URL + "/api/orgs/org-a/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	deadline := time.After(2 * time.Second)
	got := make(chan bool, 1)
	go func() {
		for sc.Scan() {
			if sc.Text() == ": ping" {
				got <- true
				return
			}
		}
	}()
	select {
	case <-got:
	case <-deadline:
		t.Fatal("no ping")
	}
}

// The server's timeouts are for ordinary answers: a stream outlives them.
func TestTheStreamOutlivesTheServerTimeouts(t *testing.T) {
	e := newEnv(t)
	srv := httptest.NewUnstartedServer(e.h)
	srv.Config.ReadTimeout, srv.Config.WriteTimeout = 200*time.Millisecond, 200*time.Millisecond
	srv.Start()
	t.Cleanup(srv.Close)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Autumn 2026"})
	s := listen(t, srv, "org-a")
	time.Sleep(time.Second)
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/"+term+"/people", map[string]string{"name": "Aslaug"})
	if m := next(s, 2*time.Second); m == "" {
		t.Fatal("the stream was cut by the server's timeouts")
	}
}
