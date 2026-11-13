package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The pictures in docs/img are made by `mise run app:shots` and shown in README.md: none may be missing, and none may be left
// over from an older set. The mutation runs copy only the Go sources, so without the README next to them the test skips.
func TestReadmePicturesExistAndAreUsed(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Skip("no README.md next to the Go sources (a copy made for the mutations)")
	}
	used := map[string]bool{}
	for _, m := range regexp.MustCompile(`\]\((docs/img/[^)\s]+)\)`).FindAllStringSubmatch(string(readme), -1) {
		used[m[1]] = true
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(m[1]))); err != nil {
			t.Errorf("README.md shows %s, which does not exist: run mise run app:shots", m[1])
		}
	}
	if len(used) == 0 {
		t.Error("README.md shows no pictures from docs/img")
	}
	files, err := os.ReadDir("../docs/img")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".png") && !used["docs/img/"+f.Name()] {
			t.Errorf("docs/img/%s is not used in README.md: remove it, or show it", f.Name())
		}
	}
}
