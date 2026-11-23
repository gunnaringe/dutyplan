package web

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The page is written in web/src (TypeScript) and bundled into app.js; the tests that read the page's code read the
// sources, so a message in a comment or a name the bundler renames cannot fool them. The generated API code is left out.
func pageFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir("src", func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "gen" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".ts") && !strings.HasSuffix(p, ".d.ts") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil || len(files) < 5 {
		t.Fatalf("the page's sources in web/src: %v (%d files)", err, len(files))
	}
	sort.Strings(files)
	return files
}

// pageSource is all of the page's sources in one text.
func pageSource(t *testing.T) []byte {
	t.Helper()
	var all strings.Builder
	for _, f := range pageFiles(t) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		all.Write(b)
		all.WriteString("\n")
	}
	return []byte(all.String())
}
