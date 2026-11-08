package planner

import (
	"fmt"
	"sort"
	"strings"
)

// Explain turns the solver's «why is this place empty» (who was ruled out, and for what) into one Norwegian sentence.
func Explain(why map[string]any, person func(string) string, task func(string) string) string {
	list := func(v any) []string {
		arr, _ := v.([]any)
		out := make([]string, 0, len(arr))
		for _, x := range arr {
			if s, ok := x.(string); ok {
				out = append(out, person(s))
			}
		}
		return out
	}
	var parts []string
	if v := list(why["dato_ikke"]); len(v) > 0 {
		parts = append(parts, "passer ikke: "+strings.Join(v, ", "))
	}
	if v := list(why["oppgave_ikke"]); len(v) > 0 {
		parts = append(parts, "har sagt «ikke» til oppgaven: "+strings.Join(v, ", "))
	}
	if v := list(why["ikke_foreslaas"]); len(v) > 0 {
		parts = append(parts, "kan ikke foreslås: "+strings.Join(v, ", "))
	}
	if busy, _ := why["opptatt"].(map[string]any); len(busy) > 0 {
		keys := make([]string, 0, len(busy))
		for k := range busy {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var items []string
		for _, k := range keys {
			var ts []string
			arr, _ := busy[k].([]any)
			for _, t := range arr {
				if s, ok := t.(string); ok {
					name := task(s)
					if name == "" { // not a plain task id (the solver also says «t3 (g1)» for a group's task): show it as it is
						name = s
					}
					ts = append(ts, name)
				}
			}
			items = append(items, fmt.Sprintf("%s (%s)", person(k), strings.Join(ts, ", ")))
		}
		parts = append(parts, "opptatt med noe annet på hendelsen: "+strings.Join(items, ", "))
	}
	if n, _ := why["ikke_med"].(float64); n > 0 {
		parts = append(parts, fmt.Sprintf("%d har ikke valgt oppgaven", int(n)))
	}
	head := "Ingen kan ta denne"
	if n, _ := why["kandidater"].(float64); n > 0 {
		head = fmt.Sprintf("%d kunne stilt, men trengs andre steder eller kan ikke kombineres med noe de allerede har", int(n))
	}
	if len(parts) == 0 {
		return head + "."
	}
	return head + ": " + strings.Join(parts, "; ") + "."
}
