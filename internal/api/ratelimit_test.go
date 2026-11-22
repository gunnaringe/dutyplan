package api

import (
	"fmt"
	"testing"
	"time"
)

func lowerLimits(t *testing.T, burst, costly float64) {
	wb, we, cb, ce := writeBurst, writeEvery, costlyBurst, costlyEvery
	writeBurst, costlyBurst = burst, costly
	t.Cleanup(func() { writeBurst, writeEvery, costlyBurst, costlyEvery = wb, we, cb, ce })
}

func TestMakingOrganisationsIsLimitedPerPerson(t *testing.T) {
	lowerLimits(t, 100, 2)
	e := newLoginEnv(t)
	ann, bob := e.signIn("ann", "/"), e.signIn("bob", "/")
	for i := range 2 {
		if r := e.as(ann, "POST", "/api/orgs", map[string]string{"name": "Club"}); r.code != 201 {
			t.Fatalf("organisation %d: %d %s", i+1, r.code, r.body)
		}
	}
	r := e.as(ann, "POST", "/api/orgs", map[string]string{"name": "Club"})
	if r.code != 429 || r.errCode() != "rate_limited" || r.hdr.Get("Retry-After") == "" {
		t.Fatalf("a third organisation at once: %d %s %v", r.code, r.body, r.hdr)
	}
	if r := e.as(ann, "GET", "/api/orgs", nil); r.code != 200 {
		t.Errorf("reading is not limited: %d", r.code)
	}
	if r := e.as(ann, "PATCH", "/api/orgs/club", map[string]string{"name": "Club A"}); r.code != 200 {
		t.Errorf("an ordinary change has its own bucket: %d %s", r.code, r.body)
	}
	if r := e.as(bob, "POST", "/api/orgs", map[string]string{"name": "Club"}); r.code != 201 {
		t.Errorf("another person is not limited by the first: %d %s", r.code, r.body)
	}
}

func TestChangesAreLimitedPerPerson(t *testing.T) {
	lowerLimits(t, 3, 100)
	e := newLoginEnv(t)
	ann := e.signIn("ann", "/")
	e.as(ann, "POST", "/api/orgs", map[string]string{"name": "Club"}) // the first of the 3
	codes := []int{}
	for i := range 3 {
		codes = append(codes, e.as(ann, "PATCH", "/api/orgs/club", map[string]string{"name": fmt.Sprint("Club ", i)}).code)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != 429 {
		t.Fatalf("with a burst of 3: %v", codes)
	}
	if r := e.as(ann, "GET", "/api/orgs/club", nil); r.code != 200 {
		t.Errorf("reading after the changes ran out: %d", r.code)
	}
}

func TestABucketFillsAgainWithTime(t *testing.T) {
	l, now := newLimiter(2, time.Second), time.Now()
	l.allow("a", now)
	l.allow("a", now)
	if ok, wait := l.allow("a", now); ok || wait <= 0 || wait > time.Second {
		t.Fatalf("empty: %v %v", ok, wait)
	}
	if ok, _ := l.allow("a", now.Add(1100*time.Millisecond)); !ok {
		t.Fatal("a token back after a second")
	}
	if ok, _ := l.allow("b", now); !ok {
		t.Fatal("each key has its own bucket")
	}
}

func TestWithoutALoginNothingIsLimited(t *testing.T) {
	lowerLimits(t, 1, 1)
	e := newEnv(t)
	for i := range 3 {
		e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": fmt.Sprint("Club ", i)})
	}
}

func TestOnlyTheStartPageWithALoginMayBeIndexed(t *testing.T) {
	e := newLoginEnv(t)
	if r := e.as(nil, "GET", "/", nil); r.hdr.Get("X-Robots-Tag") != "" {
		t.Errorf("/ with a login: X-Robots-Tag %q", r.hdr.Get("X-Robots-Tag"))
	}
	for _, p := range []string{"/o/x", "/app.js", "/api/me"} {
		if r := e.as(nil, "GET", p, nil); r.hdr.Get("X-Robots-Tag") == "" {
			t.Errorf("%s may be indexed", p)
		}
	}
	if r := newEnv(t).do("GET", "/", nil); r.hdr.Get("X-Robots-Tag") == "" {
		t.Error("/ without a login may be indexed")
	}
}
