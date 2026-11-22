package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gunnaringe/dutyplan/internal/store"
	"golang.org/x/oauth2"
)

// Signing in is OpenID Connect against one provider (authorization code with PKCE, a confidential client). The session is
// a random token in a cookie of this host only (__Host-, so another host of the same domain cannot set or read it); the
// database keeps its SHA-256. Who may do what in an organisation is in the database too (membership): see access.

// Login configures signing in. Without one (New with nil) there is no login and everyone may do everything.
type Login struct {
	Issuer       string // the provider, e.g. https://id.example.org
	ClientID     string
	ClientSecret string
	PublicURL    string // this app's address, e.g. https://dutyplan.example.org/ (the callback is <PublicURL>auth/callback)
	SignupURL    string // optional: where someone without an account makes one (Pocket ID: <issuer>/signup); the start page offers it
}

const (
	sessionFor = 30 * 24 * time.Hour
	loginFor   = 10 * time.Minute // from starting the sign-in to coming back
)

type auth struct {
	opt    Login
	base   *url.URL
	secure bool
	client *http.Client
	st     *store.Store
	log    *slog.Logger

	mu   sync.Mutex
	prov *oidc.Provider
}

func newAuth(opt Login, st *store.Store, log *slog.Logger) (*auth, error) {
	base, err := url.Parse(opt.PublicURL)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" {
		return nil, fmt.Errorf("login: the public URL %q must be an http(s) address", opt.PublicURL)
	}
	if u, err := url.Parse(opt.SignupURL); opt.SignupURL != "" && (err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "") {
		return nil, fmt.Errorf("login: the signup URL %q must be a whole address, like https://id.example.org/signup", opt.SignupURL)
	}
	if opt.Issuer == "" || opt.ClientID == "" {
		return nil, errors.New("login: the issuer and the client id are needed")
	}
	base.Path, base.RawQuery, base.Fragment = "/", "", ""
	return &auth{opt: opt, base: base, secure: base.Scheme == "https", client: &http.Client{Timeout: 10 * time.Second}, st: st, log: log}, nil
}

// provider is the provider's configuration, fetched the first time it is needed (and again after a failure), so the app
// starts while the provider is down.
func (au *auth) provider(ctx context.Context) (*oidc.Provider, error) {
	au.mu.Lock()
	defer au.mu.Unlock()
	if au.prov != nil {
		return au.prov, nil
	}
	p, err := oidc.NewProvider(oidc.ClientContext(ctx, au.client), au.opt.Issuer)
	if err != nil {
		return nil, err
	}
	au.prov = p
	return p, nil
}

func (au *auth) oauth(p *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{ClientID: au.opt.ClientID, ClientSecret: au.opt.ClientSecret, Endpoint: p.Endpoint(),
		RedirectURL: au.url("auth/callback"), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}
}

func (au *auth) url(path string) string {
	return au.base.ResolveReference(&url.URL{Path: path}).String()
}

// cookieName: __Host- needs Secure, so a local http server goes without it.
func (au *auth) cookieName(name string) string {
	if au.secure {
		return "__Host-" + name
	}
	return name
}

func (au *auth) cookie(name, value string, maxAge time.Duration) *http.Cookie {
	c := &http.Cookie{Name: au.cookieName(name), Value: value, Path: "/", HttpOnly: true, Secure: au.secure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(maxAge.Seconds())}
	if maxAge < 0 {
		c.MaxAge = -1
	}
	return c
}

func hashToken(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// safeNext is where to go after signing in: a path of this app, never another site («//x», «/\x»).
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, `/\`) || strings.ContainsAny(next, "\r\n") {
		return "/"
	}
	return next
}

type loginState struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Next     string `json:"next"`
}

// login starts signing in: GET /auth/login?next=<path>.
func (au *auth) login(w http.ResponseWriter, r *http.Request) {
	p, err := au.provider(r.Context())
	if err != nil {
		au.fail(w, r, fmt.Errorf("the sign-in service cannot be reached: %w", err))
		return
	}
	st := loginState{State: randomToken(), Nonce: randomToken(), Verifier: oauth2.GenerateVerifier(), Next: safeNext(r.URL.Query().Get("next"))}
	b, _ := json.Marshal(st)
	http.SetCookie(w, au.cookie("dutyplan-login", base64.RawURLEncoding.EncodeToString(b), loginFor))
	http.Redirect(w, r, au.oauth(p).AuthCodeURL(st.State, oidc.Nonce(st.Nonce), oauth2.S256ChallengeOption(st.Verifier)), http.StatusFound)
}

// callback is where the provider sends the browser back: GET /auth/callback?code=&state=.
func (au *auth) callback(w http.ResponseWriter, r *http.Request) {
	var st loginState
	c, err := r.Cookie(au.cookieName("dutyplan-login"))
	if err == nil {
		var b []byte
		if b, err = base64.RawURLEncoding.DecodeString(c.Value); err == nil {
			err = json.Unmarshal(b, &st)
		}
	}
	http.SetCookie(w, au.cookie("dutyplan-login", "", -1))
	q := r.URL.Query()
	switch {
	case err != nil || st.State == "":
		au.fail(w, r, errors.New("the sign-in was not started here, or took too long"))
		return
	case q.Get("error") != "":
		au.fail(w, r, fmt.Errorf("the sign-in service said: %s %s", q.Get("error"), q.Get("error_description")))
		return
	case q.Get("state") != st.State:
		au.fail(w, r, errors.New("the sign-in's state does not match"))
		return
	}
	p, err := au.provider(r.Context())
	if err != nil {
		au.fail(w, r, fmt.Errorf("the sign-in service cannot be reached: %w", err))
		return
	}
	ctx := oidc.ClientContext(r.Context(), au.client)
	tok, err := au.oauth(p).Exchange(ctx, q.Get("code"), oauth2.VerifierOption(st.Verifier))
	if err != nil {
		au.fail(w, r, fmt.Errorf("the code was not accepted: %w", err))
		return
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		au.fail(w, r, errors.New("the sign-in service sent no ID token"))
		return
	}
	idt, err := p.Verifier(&oidc.Config{ClientID: au.opt.ClientID}).Verify(ctx, raw)
	if err != nil {
		au.fail(w, r, fmt.Errorf("the ID token is not valid: %w", err))
		return
	}
	if idt.Nonce != st.Nonce {
		au.fail(w, r, errors.New("the ID token's nonce does not match"))
		return
	}
	var claims struct {
		Name     string `json:"name"`
		Username string `json:"preferred_username"`
		Email    string `json:"email"`
	}
	idt.Claims(&claims)
	name := claims.Name
	if name == "" {
		name = claims.Username
	}
	token := randomToken()
	if _, err := au.st.SignedIn(r.Context(), idt.Issuer, idt.Subject, name, claims.Email, hashToken(token), raw, time.Now().Add(sessionFor)); err != nil {
		au.fail(w, r, err)
		return
	}
	http.SetCookie(w, au.cookie("dutyplan", token, sessionFor))
	http.Redirect(w, r, safeNext(st.Next), http.StatusFound)
}

// logout ends the session: POST /auth/logout. The answer is {"to": address}: the provider's page that ends the sign-in
// there too (OpenID Connect RP-initiated logout, back to the start afterwards), or the start when it has none.
func (au *auth) logout(w http.ResponseWriter, r *http.Request) {
	var idToken string
	if c, err := r.Cookie(au.cookieName("dutyplan")); err == nil {
		idToken, _ = au.st.EndSession(r.Context(), hashToken(c.Value))
	}
	http.SetCookie(w, au.cookie("dutyplan", "", -1))
	writeJSON(w, 200, map[string]string{"to": au.endSession(r.Context(), idToken)})
}

func (au *auth) endSession(ctx context.Context, idToken string) string {
	if idToken == "" {
		return "/"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	p, err := au.provider(ctx)
	if err != nil {
		return "/"
	}
	var meta struct {
		EndSession string `json:"end_session_endpoint"`
	}
	if err := p.Claims(&meta); err != nil || meta.EndSession == "" {
		return "/"
	}
	u, err := url.Parse(meta.EndSession)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "/"
	}
	q := u.Query()
	q.Set("id_token_hint", idToken)
	q.Set("client_id", au.opt.ClientID)
	q.Set("post_logout_redirect_uri", au.url(""))
	u.RawQuery = q.Encode()
	return u.String()
}

// fail goes to the start, which says that signing in did not work; why is in the server's log.
func (au *auth) fail(w http.ResponseWriter, r *http.Request, err error) {
	au.log.Warn("sign-in failed", "err", err)
	http.Redirect(w, r, "/?signin=failed", http.StatusSeeOther)
}

// user is the person signed in, if anyone.
func (au *auth) user(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(au.cookieName("dutyplan"))
	if err != nil || c.Value == "" {
		return store.User{}, false
	}
	u, err := au.st.SessionUser(r.Context(), hashToken(c.Value))
	return u, err == nil
}
