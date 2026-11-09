// Package i18n holds the texts the server composes for the page: explanations of empty places, warnings, compromises.
// They are rendered at view time from a code and parameters, in the language the page asks for (?lang=), so the
// language switch always wins. English is the fallback for anything missing.
package i18n

import (
	"fmt"
	"sort"
	"strings"
)

// Lang is a language the page can show.
type Lang string

const (
	EN Lang = "en"
	NB Lang = "nb" // Norwegian (bokmål)
)

// Parse reads a language tag such as "nb", "no", "nn" or "nb-NO"; anything else is English.
func Parse(tag string) Lang {
	switch strings.ToLower(strings.SplitN(strings.TrimSpace(tag), "-", 2)[0]) {
	case "nb", "no", "nn":
		return NB
	}
	return EN
}

var catalogs = map[Lang]map[string]string{EN: en, NB: nb}

// T renders a message. The arguments fill the verbs of the format; a missing key falls back to English, then to the key.
func T(l Lang, key string, args ...any) string {
	f, ok := catalogs[l][key]
	if !ok {
		if f, ok = en[key]; !ok {
			return key
		}
	}
	return fmt.Sprintf(f, args...)
}

// Keys lists the keys of one language, sorted (for the parity test).
func Keys(l Lang) []string {
	var out []string
	for k := range catalogs[l] {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Format returns the raw format string of a key (for the parity test).
func Format(l Lang, key string) string { return catalogs[l][key] }
