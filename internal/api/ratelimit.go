package api

import (
	"sync"
	"time"
)

// Rate limits, with a login only (anyone may sign up): changes per person, or per address for someone with a link. A
// token bucket each: a burst, then a steady rate. Making an organisation, a link or a planning run costs more, so it has
// its own, tighter bucket (procs.go: `costly`; the interceptor applies them). Variables so the tests can lower them.
var (
	writeBurst  = 300.0
	writeEvery  = 200 * time.Millisecond // a change back every 0.2 s: 5 a second
	costlyBurst = 20.0
	costlyEvery = 30 * time.Second
)

type bucket struct {
	tokens float64
	at     time.Time
}

type limiter struct {
	mu      sync.Mutex
	burst   float64
	every   time.Duration
	buckets map[string]*bucket
	pruned  time.Time
}

func newLimiter(burst float64, every time.Duration) *limiter {
	return &limiter{burst: burst, every: every, buckets: map[string]*bucket{}}
}

// allow takes a token from key's bucket; when there is none, it says how long until there is.
func (l *limiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.pruned) > time.Minute { // full buckets are forgotten, so the map does not grow without end
		for k, b := range l.buckets {
			if b.tokens+float64(now.Sub(b.at))/float64(l.every) >= l.burst {
				delete(l.buckets, k)
			}
		}
		l.pruned = now
	}
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, at: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+float64(now.Sub(b.at))/float64(l.every))
	b.at = now
	if b.tokens < 1 {
		return false, time.Duration((1 - b.tokens) * float64(l.every))
	}
	b.tokens--
	return true, 0
}
