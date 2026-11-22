package api

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// web/llms.txt is the guide for agents that use a server (served at /llms.txt). It must name every API route the server
// has, and no route it does not have, so it never drifts from the code.
func TestTheAgentGuideNamesEveryRouteAndNoOther(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	inCode := map[string]bool{}
	route := regexp.MustCompile(`HandleFunc\("((?:GET|POST|PUT|PATCH|DELETE) /api/[^"]+)"`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range route.FindAllStringSubmatch(string(b), -1) {
			inCode[m[1]] = true
		}
	}
	doc, err := os.ReadFile(filepath.Join("..", "..", "web", "llms.txt"))
	if err != nil {
		t.Fatal(err)
	}
	inDoc := map[string]bool{}
	for _, m := range regexp.MustCompile("`((?:GET|POST|PUT|PATCH|DELETE) /api/[^`? ]+)").FindAllStringSubmatch(string(doc), -1) {
		inDoc[m[1]] = true
	}
	var missing, extra []string
	for r := range inCode {
		if !inDoc[r] {
			missing = append(missing, r)
		}
	}
	for r := range inDoc {
		if !inCode[r] {
			extra = append(extra, r)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(inCode) < 50 || len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("web/llms.txt and the routes differ (%d in the code)\nnot in the guide: %v\nnot in the code: %v", len(inCode), missing, extra)
	}
}

func TestTheAgentGuideIsServed(t *testing.T) {
	e := newEnv(t)
	e.h = New(e.st, e.pl, fstest.MapFS{"index.html": {Data: []byte("x")}, "llms.txt": {Data: []byte("# Tjenesteplan")}}, slog.New(slog.DiscardHandler))
	r := e.do("GET", "/llms.txt", nil)
	if r.code != 200 || string(r.body) != "# Tjenesteplan" || !strings.HasPrefix(r.hdr.Get("Content-Type"), "text/plain") {
		t.Fatalf("GET /llms.txt: %d %q %q", r.code, r.hdr.Get("Content-Type"), r.body)
	}
}
