package api

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// Who may do what is in procs.go (one line per procedure). This file has the share links: the cookie that remembers
// them in a browser, opening one (/s/<token>), and the members and links of an organisation.

const linksCookie = "dutyplan-links"

// linkTokens are the tokens of the links opened in this browser (newest first).
func (s *server) linkTokens(r *http.Request) []string {
	if s.auth == nil {
		return nil
	}
	c, err := r.Cookie(s.auth.cookieName(linksCookie))
	if err != nil || c.Value == "" {
		return nil
	}
	var out []string
	for _, t := range strings.Split(c.Value, ".") {
		if t != "" && len(t) <= 64 && len(out) < maxLinks {
			out = append(out, t)
		}
	}
	return out
}

const maxLinks = 20

// links are the links opened in this browser that still work (the tokens come with the request, see who()).
func (s *server) links(ctx context.Context) []store.Link {
	tokens, _ := ctx.Value(tokensKey).([]string)
	if len(tokens) == 0 {
		return nil
	}
	ls, err := s.st.LinksByTokens(ctx, tokens)
	if err != nil {
		s.log.Error("links", "err", err)
		return nil
	}
	return slices.DeleteFunc(ls, func(l store.Link) bool { return l.Level == store.LevelJoin })
}

// who puts the person signed in and the tokens of the browser's links into the request's context, for the API's
// interceptor (procs.go) and the stream.
func (s *server) who(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		if u, ok := s.auth.user(r); ok {
			ctx = context.WithValue(ctx, userKey, u)
		}
		if t := s.linkTokens(r); len(t) > 0 {
			ctx = context.WithValue(ctx, tokensKey, t)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// sees: may a stream with this grant hear of a change (a link to one plan hears nothing of the other plans).
func (g grant) sees(c change) bool {
	if g.plan == "" || c.Plan == "" {
		return true
	}
	return c.Plan == g.plan
}

// openLink is the address of a link: GET /s/<token>. A view or edit link is remembered in this browser (a cookie) and
// opens its plan; an invitation makes the person signed in a member (after signing in, when needed).
func (s *server) openLink(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	ls, err := s.st.LinksByTokens(r.Context(), []string{token})
	if err != nil || len(ls) == 0 {
		http.Redirect(w, r, "/?link=invalid", http.StatusSeeOther)
		return
	}
	l := ls[0]
	if l.Level == store.LevelJoin {
		u, ok := s.auth.user(r)
		if !ok { // first a page that says who invites, with the way to sign in (back to this link) or make an account
			http.Redirect(w, r, "/?"+url.Values{"invite": {token}, "org": {l.OrgName}}.Encode(), http.StatusSeeOther)
			return
		}
		o, err := s.st.Join(r.Context(), token, u.ID)
		if err != nil {
			http.Redirect(w, r, "/?link=invalid", http.StatusSeeOther)
			return
		}
		s.hub.publish(o.Slug, change{Proc: "Joined"})
		http.Redirect(w, r, "/o/"+url.PathEscape(o.Slug)+"?joined", http.StatusSeeOther) // the page says welcome
		return
	}
	tokens := []string{token}
	for _, t := range s.linkTokens(r) {
		if t != token && len(tokens) < maxLinks {
			tokens = append(tokens, t)
		}
	}
	http.SetCookie(w, s.auth.cookie(linksCookie, strings.Join(tokens, "."), 400*24*time.Hour))
	to := "/o/" + url.PathEscape(l.Org)
	if l.Plan != "" {
		to += "/p/" + url.PathEscape(l.Plan)
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}
