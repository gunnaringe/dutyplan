package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	langOpen  = regexp.MustCompile(`^  (en|nb): \{$`)
	entryLine = regexp.MustCompile(`^    '([^']+)': '((?:[^'\\]|\\.)*)',$`)
	holder    = regexp.MustCompile(`\{(\w+)\}`)
)

// catalogs reads i18n.js: language -> key -> text. The file keeps one entry per line so this can stay a few lines.
func catalogs(t *testing.T) map[string]map[string]string {
	t.Helper()
	b, err := os.ReadFile("i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]string{}
	var cur map[string]string
	for n, line := range strings.Split(string(b), "\n") {
		if m := langOpen.FindStringSubmatch(line); m != nil {
			cur = map[string]string{}
			out[m[1]] = cur
			continue
		}
		if cur == nil {
			continue
		}
		if line == "  }," {
			cur = nil
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := entryLine.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("i18n.js:%d is not «    'key': 'text',»: %s", n+1, line)
		}
		if _, dup := cur[m[1]]; dup {
			t.Errorf("i18n.js:%d: duplicate key %s", n+1, m[1])
		}
		cur[m[1]] = m[2]
	}
	if len(out["en"]) == 0 || len(out["nb"]) == 0 {
		t.Fatalf("found %d English and %d Norwegian texts", len(out["en"]), len(out["nb"]))
	}
	return out
}

func holders(s string) []string {
	var out []string
	for _, m := range holder.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func TestLanguagesHaveTheSameKeysAndPlaceholders(t *testing.T) {
	c := catalogs(t)
	for key, en := range c["en"] {
		nb, ok := c["nb"][key]
		if !ok {
			t.Errorf("%s has no Norwegian text", key)
			continue
		}
		if a, b := strings.Join(holders(en), ","), strings.Join(holders(nb), ","); a != b {
			t.Errorf("%s: English has {%s}, Norwegian {%s}", key, a, b)
		}
		if strings.TrimSpace(en) == "" || strings.TrimSpace(nb) == "" {
			t.Errorf("%s is empty", key)
		}
	}
	for key := range c["nb"] {
		if _, ok := c["en"][key]; !ok {
			t.Errorf("%s has no English text", key)
		}
	}
	for key, nb := range c["nb"] {
		if strings.Contains(nb, "'") || strings.Contains(c["en"][key], "'") {
			t.Errorf("%s: use ’ (not ') inside texts", key)
		}
	}
	// Anything that counts has both forms.
	for key := range c["en"] {
		if base, ok := strings.CutSuffix(key, ".one"); ok {
			if _, has := c["en"][base+".other"]; !has {
				t.Errorf("%s has no .other", base)
			}
		}
	}
}

// Every key the page names in app.js exists. Keys built from several parts ('combo.' + level) are written out in the
// page as whole strings somewhere, or listed here.
func TestKeysUsedByThePageExist(t *testing.T) {
	c := catalogs(t)
	prefixes := map[string]bool{}
	for key := range c["en"] {
		prefixes[strings.SplitN(key, ".", 2)[0]] = true
	}
	b, err := os.ReadFile("app.js")
	if err != nil {
		t.Fatal(err)
	}
	lit := regexp.MustCompile(`'([a-z_0-9]+(?:\.[a-z_0-9]+)+)'`)
	seen := 0
	for _, m := range lit.FindAllStringSubmatch(string(b), -1) {
		key := m[1]
		if !prefixes[strings.SplitN(key, ".", 2)[0]] {
			continue
		}
		seen++
		_, plain := c["en"][key]
		_, counted := c["en"][key+".one"]
		if !plain && !counted {
			t.Errorf("app.js uses %q, which i18n.js does not have", key)
		}
	}
	if seen < 100 {
		t.Errorf("only %d keys found in app.js: the pattern no longer matches the code", seen)
	}
	// Parts that app.js puts together at run time.
	for _, k := range []string{"standard", "cannot", "impractical", "practical", "preferred"} {
		if _, ok := c["en"]["combo."+k]; !ok {
			t.Errorf("combo.%s is missing", k)
		}
	}
	for _, k := range []string{"none", "good", "poor", "never"} { // the answer in a cell of the table
		if _, ok := c["en"]["answer."+k]; !ok {
			t.Errorf("answer.%s is missing", k)
		}
	}
	for d := 1; d <= 7; d++ {
		if _, ok := c["en"]["week."+string(rune('0'+d))]; !ok {
			t.Errorf("week.%d is missing", d)
		}
	}
	for _, l := range []string{"en", "nb"} {
		if _, ok := c["en"]["lang."+l]; !ok {
			t.Errorf("lang.%s is missing", l)
		}
	}
}

// goSources returns the non-test Go files under ../internal.
func goSources(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir("../internal", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		b, err := os.ReadFile(p)
		out[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Every error code the server can send has a text in both languages, and the text only uses parameters the server sends.
func TestEveryErrorCodeHasAText(t *testing.T) {
	c := catalogs(t)
	call := regexp.MustCompile(`\b(?:Invalid|Conflict)\("([a-z_]+)", (nil|map\[string\]any\{[^}]*\})`)
	key := regexp.MustCompile(`"(\w+)":`)
	params := map[string]map[string]bool{}
	for _, src := range goSources(t) {
		for _, m := range call.FindAllStringSubmatch(src, -1) {
			if params[m[1]] == nil {
				params[m[1]] = map[string]bool{}
			}
			for _, k := range key.FindAllStringSubmatch(m[2], -1) {
				params[m[1]][k[1]] = true
			}
		}
	}
	// already_exists and in_use are built by exists() and inUse() with kind and name.
	params["already_exists"] = map[string]bool{"kind": true, "name": true}
	params["in_use"] = map[string]bool{"kind": true, "name": true}
	if len(params) < 30 {
		t.Fatalf("only %d error codes found in the Go code: the pattern no longer matches", len(params))
	}
	for code, ps := range params {
		for _, l := range []string{"en", "nb"} {
			text, ok := c[l]["err."+code]
			if !ok {
				t.Errorf("error code %s has no %s text (err.%s)", code, l, code)
				continue
			}
			for _, h := range holders(text) {
				if !ps[h] {
					t.Errorf("err.%s (%s) uses {%s}, which the server does not send", code, l, h)
				}
			}
		}
	}
}

// The kinds in «already exists» / «in use», the names in «must be 1-80 characters», the kinds of failure that carry no code, and
// the weights each have a text.
func TestKindsAndWeightsHaveTexts(t *testing.T) {
	c := catalogs(t)
	src := goSources(t)
	var all strings.Builder
	for _, s := range src {
		all.WriteString(s)
	}
	need := func(re string, prefix string, fix func(string) string) {
		ms := regexp.MustCompile(re).FindAllStringSubmatch(all.String(), -1)
		if len(ms) == 0 {
			t.Errorf("no matches for %s: the pattern no longer matches the code", re)
		}
		for _, m := range ms {
			for _, l := range []string{"en", "nb"} {
				k := prefix + fix(m[1])
				if _, ok := c[l][k]; !ok {
					t.Errorf("%s has no %s text", k, l)
				}
			}
		}
	}
	us := func(s string) string { return strings.ReplaceAll(s, " ", "_") }
	need(`\b(?:exists|inUse)\("([a-z ]+)"`, "kind.", us)
	need(`cleanName\([^,]+, "(\w+)"\)`, "what.", us)
	need(`apiError\{Error: "(\w+)"`, "err.kind.", us)
	need(`coded\("(\w+)"`, "err.kind.", us)
	// Weights: label and hint in both languages, and a group in app.js.
	app, err := os.ReadFile("app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`(?:num\(|\{Key: )"([\w.]+)"`).FindAllStringSubmatch(all.String(), -1) {
		for _, l := range []string{"en", "nb"} {
			for _, part := range []string{".label", ".hint"} {
				if _, ok := c[l]["weight."+m[1]+part]; !ok {
					t.Errorf("weight.%s%s has no %s text", m[1], part, l)
				}
			}
		}
		if !strings.Contains(string(app), "'"+m[1]+"':") && !strings.Contains(string(app), " "+m[1]+": '") {
			t.Errorf("app.js WEIGHT_GROUP has no group for %s", m[1])
		}
	}
}
