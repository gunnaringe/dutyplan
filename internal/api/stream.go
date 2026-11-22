package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// The page keeps one stream open per organisation (GET /api/orgs/{org}/stream, Server-Sent Events). After every write that
// went through, and when a planning run ends, the stream says what changed: the method and path of the write (a run's end is
// «RUN» on the plan's runs). The page fetches again only what that write can have changed, and redraws without a reload. The
// message also carries the id of the browser tab that made the change (from the X-Dutyplan-Client header), so that tab,
// which has already redrawn, can skip it. No data goes over the stream.

// clientHeader names the tab that sends a write. It is only an echo for the stream, never trusted for anything else.
const clientHeader = "X-Dutyplan-Client"

// pingEvery keeps the connection alive through proxies (Cloudflare closes an idle one after 100 s).
var pingEvery = 25 * time.Second

// maxStreams caps the open streams of the whole server, so open connections cannot use up its file handles.
const maxStreams = 1000

// change is one message. An empty Path means «anything may have changed» (messages that were merged).
type change struct {
	Origin string `json:"origin,omitempty"`
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
}

type hub struct {
	mu     sync.Mutex
	subs   map[string]map[chan change]struct{} // org slug -> the streams open on it
	n      int
	closed bool
	done   chan struct{}
}

func newHub() *hub {
	return &hub{subs: map[string]map[chan change]struct{}{}, done: make(chan struct{})}
}

// subscribe opens a stream on an organisation; ok is false when the server has too many or is shutting down.
func (h *hub) subscribe(org string) (ch chan change, cancel func(), ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || h.n >= maxStreams {
		return nil, nil, false
	}
	ch = make(chan change, 1)
	if h.subs[org] == nil {
		h.subs[org] = map[chan change]struct{}{}
	}
	h.subs[org][ch] = struct{}{}
	h.n++
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, there := h.subs[org][ch]; there {
			delete(h.subs[org], ch)
			h.n--
			if len(h.subs[org]) == 0 {
				delete(h.subs, org)
			}
		}
	}, true
}

// publish tells every stream on the organisation what changed. It never blocks: when a stream has not sent the last message
// yet, the two are merged into one that says «anything may have changed» (no path), and keeps the origin only if both
// came from the same tab.
func (h *hub) publish(org string, c change) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[org] {
		select {
		case ch <- c:
		default:
			m := c
			select {
			case old := <-ch:
				if old != c {
					m.Method, m.Path = "", ""
				}
				if old.Origin != c.Origin {
					m.Origin = ""
				}
			default:
			}
			ch <- m
		}
	}
}

// close ends every stream (the server is shutting down).
func (h *hub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.closed {
		h.closed = true
		close(h.done)
	}
}

// orgOf is the organisation a request is about, or "" (the list of organisations, creating one, the frontend).
func orgOf(path string) string {
	rest, ok := strings.CutPrefix(path, "/api/orgs/")
	if !ok {
		return ""
	}
	org, _, _ := strings.Cut(rest, "/")
	return org
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// notifying publishes a change on the organisation after every write to it that went through.
func (s *server) notifying(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		org := orgOf(r.URL.Path)
		if org == "" || r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status != 0 && sw.status < 400 {
			s.hub.publish(org, change{Origin: r.Header.Get(clientHeader), Method: r.Method, Path: strings.TrimPrefix(r.URL.Path, "/api")})
		}
	})
}

func (s *server) stream(w http.ResponseWriter, r *http.Request) {
	org := r.PathValue("org")
	if _, err := s.st.GetOrg(r.Context(), org); err != nil {
		writeErr(w, s.log, err)
		return
	}
	ch, cancel, ok := s.hub.subscribe(org)
	if !ok {
		writeJSON(w, 503, apiError{Error: "unavailable", Message: "too many open streams"})
		return
	}
	defer cancel()
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // the server's write timeout is for ordinary answers, not for a stream
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	send := func(s string) bool {
		if _, err := w.Write([]byte(s)); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !send("retry: 3000\n: open\n\n") {
		return
	}
	ping := time.NewTicker(pingEvery)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.hub.done:
			return
		case c := <-ch:
			if !grantFrom(r).sees(org, c) {
				continue
			}
			b, _ := json.Marshal(c)
			if !send("data: " + string(b) + "\n\n") {
				return
			}
		case <-ping.C:
			if !send(": ping\n\n") {
				return
			}
		}
	}
}

// Handler is the whole app. CloseStreams ends the open streams, so a shutdown does not wait for them.
type Handler struct {
	http.Handler
	hub *hub
}

func (h *Handler) CloseStreams() { h.hub.close() }
