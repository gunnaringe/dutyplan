package api

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gunnaringe/dutyplan/internal/planner"
	"github.com/gunnaringe/dutyplan/internal/store"
)

// fakeProvider is an OpenID Connect provider for the tests: discovery, keys, and a token endpoint that hands out an ID
// token for whoever the test says is signing in.
type fakeProvider struct {
	srv      *httptest.Server
	key      *rsa.PrivateKey
	sub      string
	nonce    string // from the authorize address
	verifier string // the PKCE challenge the token request must match
	change   func(map[string]any)
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakeProvider{key: key, sub: "user-1"}
	mux := http.NewServeMux()
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	b64 := base64.RawURLEncoding.EncodeToString
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"issuer": p.srv.URL, "authorization_endpoint": p.srv.URL + "/authorize",
			"token_endpoint": p.srv.URL + "/token", "jwks_uri": p.srv.URL + "/jwks", "end_session_endpoint": p.srv.URL + "/end-session",
			"id_token_signing_alg_values_supported": []string{"RS256"}})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "k1", "alg": "RS256", "use": "sig",
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes())}}})
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		r.ParseForm()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Form.Get("code") != "good-code" || b64(sum[:]) != p.verifier {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		claims := map[string]any{"iss": p.srv.URL, "sub": p.sub, "aud": "dutyplan", "exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(), "nonce": p.nonce, "name": "Kari " + p.sub, "email": p.sub + "@example.org"}
		if p.change != nil {
			p.change(claims)
		}
		head, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
		body, _ := json.Marshal(claims)
		msg := b64(head) + "." + b64(body)
		sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, func() []byte { s := sha256.Sum256([]byte(msg)); return s[:] }())
		json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": msg + "." + b64(sig)})
	})
	return p
}

type loginEnv struct {
	*env
	prov *fakeProvider
}

func newLoginEnv(t *testing.T) *loginEnv {
	t.Helper()
	p := newFakeProvider(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	static := fstest.MapFS{"index.html": {Data: []byte("INDEX")}, "i18n.js": {}, "app.js": {}, "style.css": {}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pl := planner.New(st, nil, 1, log)
	h, err := NewWithLogin(st, pl, static, log, Login{Issuer: p.srv.URL, ClientID: "dutyplan", ClientSecret: "s", PublicURL: "https://plan.example/",
		SignupURL: "https://id.example/signup"})
	if err != nil {
		t.Fatal(err)
	}
	return &loginEnv{env: &env{t: t, h: h, st: st, pl: pl}, prov: p}
}

func cookiesOf(h http.Header) []*http.Cookie { return (&http.Response{Header: h}).Cookies() }

// signIn goes through the whole sign-in as sub and returns the session cookie.
func (e *loginEnv) signIn(sub, next string) *http.Cookie {
	e.t.Helper()
	e.prov.sub = sub
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, httptest.NewRequest("GET", "/auth/login?next="+url.QueryEscape(next), nil))
	if w.Code != 302 {
		e.t.Fatalf("login: %d", w.Code)
	}
	to, _ := url.Parse(w.Header().Get("Location"))
	q := to.Query()
	if !strings.HasPrefix(to.String(), e.prov.srv.URL+"/authorize") || q.Get("code_challenge_method") != "S256" || q.Get("redirect_uri") != "https://plan.example/auth/callback" {
		e.t.Fatalf("login sent the browser to %s", to)
	}
	e.prov.nonce, e.prov.verifier = q.Get("nonce"), q.Get("code_challenge")
	req := httptest.NewRequest("GET", "/auth/callback?code=good-code&state="+url.QueryEscape(q.Get("state")), nil)
	for _, c := range cookiesOf(w.Header()) {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != 302 {
		e.t.Fatalf("callback: %d, to %s", w.Code, w.Header().Get("Location"))
	}
	if got := w.Header().Get("Location"); got != next {
		e.t.Fatalf("after signing in: to %q, want %q", got, next)
	}
	for _, c := range cookiesOf(w.Header()) {
		if c.Name == "__Host-dutyplan" && c.Value != "" {
			if !c.Secure || !c.HttpOnly || c.Path != "/" || c.Domain != "" {
				e.t.Fatalf("the session cookie is not a __Host- cookie: %+v", c)
			}
			return c
		}
	}
	e.t.Fatal("no session cookie")
	return nil
}

func (e *loginEnv) as(c *http.Cookie, method, path string, body any) reply {
	req := httptest.NewRequest(method, path, nil)
	if body != nil {
		b, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
	}
	if c != nil {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return reply{w.Code, w.Body.Bytes(), w.Header()}
}

func TestWithoutASessionOnlyThePagesAndHealthAreOpen(t *testing.T) {
	e := newLoginEnv(t)
	for _, path := range []string{"/", "/o/x/p/y", "/app.js", "/api/health", "/api/me"} {
		if r := e.as(nil, "GET", path, nil); r.code != 200 {
			t.Errorf("GET %s: %d, want 200", path, r.code)
		}
	}
	var me map[string]any
	e.as(nil, "GET", "/api/me", nil).json(&me)
	if me["login"] != true || me["user"] != nil || me["signup"] != "https://id.example/signup" {
		t.Errorf("me without a session: %v", me)
	}
	for _, req := range [][2]string{{"GET", "/api/orgs"}, {"POST", "/api/orgs"}, {"GET", "/api/orgs/x/people"}, {"GET", "/api/orgs/x/stream"}} {
		if r := e.as(nil, req[0], req[1], map[string]string{"name": "X"}); r.code != 401 || r.errCode() != "signed_out" {
			t.Errorf("%s %s: %d %s, want 401 signed_out", req[0], req[1], r.code, r.body)
		}
	}
}

func TestSigningInGivesASessionAndSigningOutEndsIt(t *testing.T) {
	e := newLoginEnv(t)
	c := e.signIn("kari", "/o/choir")
	var me struct {
		Login bool
		User  *store.User
	}
	e.as(c, "GET", "/api/me", nil).json(&me)
	if !me.Login || me.User == nil || me.User.Name != "Kari kari" || me.User.Email != "kari@example.org" {
		t.Fatalf("me: %+v", me)
	}
	r := e.as(c, "POST", "/auth/logout", nil)
	var out struct{ To string }
	r.json(&out)
	to, _ := url.Parse(out.To)
	if r.code != 200 || !strings.HasPrefix(out.To, e.prov.srv.URL+"/end-session") || to.Query().Get("id_token_hint") == "" ||
		to.Query().Get("post_logout_redirect_uri") != "https://plan.example/" {
		t.Fatalf("logout: %d %s", r.code, out.To)
	}
	if r := e.as(c, "GET", "/api/orgs", nil); r.code != 401 {
		t.Fatalf("the session still works after signing out: %d", r.code)
	}
}

func TestASignInThatDoesNotCheckOutGivesNoSession(t *testing.T) {
	e := newLoginEnv(t)
	start := func() (*httptest.ResponseRecorder, url.Values) {
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, httptest.NewRequest("GET", "/auth/login", nil))
		to, _ := url.Parse(w.Header().Get("Location"))
		e.prov.nonce, e.prov.verifier = to.Query().Get("nonce"), to.Query().Get("code_challenge")
		return w, to.Query()
	}
	callback := func(w *httptest.ResponseRecorder, query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/auth/callback?"+query, nil)
		if w != nil {
			for _, c := range cookiesOf(w.Header()) {
				req.AddCookie(c)
			}
		}
		out := httptest.NewRecorder()
		e.h.ServeHTTP(out, req)
		return out
	}
	cases := map[string]func() *httptest.ResponseRecorder{
		"no login cookie": func() *httptest.ResponseRecorder {
			_, q := start()
			return callback(nil, "code=good-code&state="+q.Get("state"))
		},
		"wrong state": func() *httptest.ResponseRecorder { w, _ := start(); return callback(w, "code=good-code&state=other") },
		"wrong code": func() *httptest.ResponseRecorder {
			w, q := start()
			return callback(w, "code=bad&state="+q.Get("state"))
		},
		"wrong nonce": func() *httptest.ResponseRecorder {
			w, q := start()
			e.prov.nonce = "other"
			return callback(w, "code=good-code&state="+q.Get("state"))
		},
		"other audience": func() *httptest.ResponseRecorder {
			w, q := start()
			e.prov.change = func(c map[string]any) { c["aud"] = "someone-else" }
			defer func() { e.prov.change = nil }()
			return callback(w, "code=good-code&state="+q.Get("state"))
		},
		"provider error": func() *httptest.ResponseRecorder {
			w, q := start()
			return callback(w, "error=access_denied&state="+q.Get("state"))
		},
	}
	for name, run := range cases {
		w := run()
		if w.Code != 303 || w.Header().Get("Location") != "/?signin=failed" {
			t.Errorf("%s: %d to %q", name, w.Code, w.Header().Get("Location"))
		}
		for _, c := range cookiesOf(w.Header()) {
			if c.Name == "__Host-dutyplan" && c.Value != "" {
				t.Errorf("%s: got a session", name)
			}
		}
	}
}

func TestAfterSigningInOnlyAPathOfThisAppIsFollowed(t *testing.T) {
	for next, want := range map[string]string{"/o/a": "/o/a", "//evil.example": "/", `/\evil.example`: "/", "https://evil.example": "/", "": "/"} {
		if got := safeNext(next); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", next, got, want)
		}
	}
}

func TestMembersSeeAndChangeOnlyTheirOwnOrganisations(t *testing.T) {
	e := newLoginEnv(t)
	kari, ola := e.signIn("kari", "/"), e.signIn("ola", "/")
	var a, b store.Org
	if r := e.as(kari, "POST", "/api/orgs", map[string]string{"name": "Choir"}); r.code != 201 {
		t.Fatalf("create: %d %s", r.code, r.body)
	} else {
		r.json(&a)
	}
	// the same name again: another address, so nobody learns the name is taken
	e.as(ola, "POST", "/api/orgs", map[string]string{"name": "Choir"}).json(&b)
	if a.Slug != "choir" || b.Slug != "choir-2" {
		t.Fatalf("addresses %q and %q", a.Slug, b.Slug)
	}
	var list []store.Org
	e.as(kari, "GET", "/api/orgs", nil).json(&list)
	if len(list) != 1 || list[0].Slug != "choir" {
		t.Fatalf("kari's organisations: %+v", list)
	}
	if r := e.as(kari, "POST", "/api/orgs/choir/people", map[string]string{"name": "Aslaug"}); r.code != 201 {
		t.Fatalf("a member's write: %d", r.code)
	}
	// someone else's organisation does not exist for her, nor does one that does not exist at all
	for _, path := range []string{"/api/orgs/choir-2", "/api/orgs/choir-2/people", "/api/orgs/choir-2/stream", "/api/orgs/nope/people", "/api/orgs/choir%2F..%2Fchoir-2/people"} {
		if r := e.as(kari, "GET", path, nil); r.code != 404 {
			t.Errorf("GET %s by another: %d, want 404", path, r.code)
		}
	}
	for _, req := range [][2]string{{"POST", "/api/orgs/choir-2/people"}, {"PATCH", "/api/orgs/choir-2"}, {"DELETE", "/api/orgs/choir-2?confirm=choir-2"}} {
		if r := e.as(kari, req[0], req[1], map[string]string{"name": "X"}); r.code != 404 {
			t.Errorf("%s %s by another: %d, want 404", req[0], req[1], r.code)
		}
	}
	if r := e.as(ola, "DELETE", "/api/orgs/choir-2?confirm=choir-2", nil); r.code != 204 {
		t.Fatalf("the owner deletes: %d %s", r.code, r.body)
	}
	if r := e.as(ola, "GET", "/api/orgs/choir-2", nil); r.code != 404 {
		t.Fatalf("after deleting: %d", r.code)
	}
}

func TestOnlyTheOwnerDeletesTheOrganisation(t *testing.T) {
	e := newLoginEnv(t)
	kari, ola := e.signIn("kari", "/"), e.signIn("ola", "/")
	e.as(kari, "POST", "/api/orgs", map[string]string{"name": "Choir"})
	var olaID int64
	e.st.DB().QueryRow(`SELECT id FROM app_user WHERE subject = 'ola'`).Scan(&olaID)
	if _, err := e.st.DB().Exec(`INSERT INTO membership (org_id, user_id, role, created_at) SELECT id, ?, 'editor', '' FROM organization WHERE slug = 'choir'`, olaID); err != nil {
		t.Fatal(err)
	}
	if r := e.as(ola, "PATCH", "/api/orgs/choir", map[string]string{"name": "The choir"}); r.code != 200 {
		t.Fatalf("an editor renames: %d", r.code)
	}
	if r := e.as(ola, "DELETE", "/api/orgs/choir?confirm=choir", nil); r.code != 403 {
		t.Fatalf("an editor deletes: %d, want 403", r.code)
	}
	if r := e.as(kari, "DELETE", "/api/orgs/choir?confirm=choir", nil); r.code != 204 {
		t.Fatalf("the owner deletes: %d", r.code)
	}
}

func TestARunOutSessionIsSignedOut(t *testing.T) {
	e := newLoginEnv(t)
	c := e.signIn("kari", "/")
	e.st.DB().Exec(`UPDATE session SET expires_at = '2000-01-01T00:00:00Z'`)
	if r := e.as(c, "GET", "/api/orgs", nil); r.code != 401 {
		t.Fatalf("an expired session: %d", r.code)
	}
}

func TestWithoutALoginMeSaysSo(t *testing.T) {
	e := newEnv(t)
	var me map[string]any
	e.mustDo(200, "GET", "/api/me", nil).json(&me)
	if me["login"] != false {
		t.Fatalf("me: %v", me)
	}
	if r := e.do("GET", "/auth/login", nil); r.code == 302 {
		t.Fatal("a sign-in without a login")
	}
}
