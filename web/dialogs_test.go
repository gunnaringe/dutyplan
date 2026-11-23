package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The page asks in its own dialogs (ask, modal), never in the browser's: 2026-11-22, the user: "La alle dialoger og
// bekreftelser være pent rendret."
func TestThePageUsesNoDialogsOfTheBrowser(t *testing.T) {
	native := regexp.MustCompile(`(^|[^\w.])(window\.)?(confirm|prompt|alert)\(`)
	for _, f := range append(pageFiles(t), "i18n.js") {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for n, line := range strings.Split(string(b), "\n") {
			if m := native.FindString(line); m != "" && !strings.HasPrefix(strings.TrimSpace(line), "//") {
				t.Errorf("%s:%d: a dialog of the browser (%s): use ask() or modal()", f, n+1, strings.TrimSpace(m))
			}
		}
	}
}
