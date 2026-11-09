package i18n

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var verbs = regexp.MustCompile(`%[sdv]`)

// The two catalogs must have the same keys, and every message the same verbs in the same order, or a translation
// would print the wrong thing (or «%!s(MISSING)») for one language.
func TestCatalogsHaveTheSameKeysAndVerbs(t *testing.T) {
	en, nb := Keys(EN), Keys(NB)
	if strings.Join(en, ",") != strings.Join(nb, ",") {
		t.Errorf("keys differ:\n en only: %v\n nb only: %v", diff(en, nb), diff(nb, en))
	}
	for _, k := range en {
		if a, b := verbs.FindAllString(Format(EN, k), -1), verbs.FindAllString(Format(NB, k), -1); fmt.Sprint(a) != fmt.Sprint(b) {
			t.Errorf("%s: verbs %v in English, %v in Norwegian", k, a, b)
		}
		if Format(EN, k) == "" || Format(NB, k) == "" {
			t.Errorf("%s is empty", k)
		}
	}
}

func diff(a, b []string) []string {
	in := map[string]bool{}
	for _, x := range b {
		in[x] = true
	}
	var out []string
	for _, x := range a {
		if !in[x] {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

func TestParseAndFallback(t *testing.T) {
	for tag, want := range map[string]Lang{"nb": NB, "no": NB, "nn": NB, "nb-NO": NB, "NB": NB, " nb ": NB, "en": EN, "en-GB": EN, "": EN, "de": EN} {
		if got := Parse(tag); got != want {
			t.Errorf("Parse(%q) = %q, want %q", tag, got, want)
		}
	}
	if got := T("xx", "explain.none"); got != "Nobody can take this" {
		t.Errorf("an unknown language falls back to English: %q", got)
	}
	if got := T(NB, "no.such.key"); got != "no.such.key" {
		t.Errorf("an unknown key is shown as the key: %q", got)
	}
	if got := T(NB, "explain.event_never", "Anna"); got != "passer ikke: Anna" {
		t.Errorf("%q", got)
	}
}
