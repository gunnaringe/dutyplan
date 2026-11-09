package planner

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Namer turns the solver's ids (p3, t2, g1, e5) into the names the run was made with.
type Namer struct {
	Person, Task, Group, Event func(string) string
}

var busyGroupTask = regexp.MustCompile(`^(t\d+) \((g\d+)\)$`)

// busy names what keeps someone from a place: a task id, or «t3 (g1)» for a group's task.
func (n Namer) busy(s string) string {
	if m := busyGroupTask.FindStringSubmatch(s); m != nil {
		return n.Task(m[1]) + " (" + n.Group(m[2]) + ")"
	}
	return n.Task(s)
}

// groupWhy says in words why the planner could not propose a group for a place. Reasons come as «event_never:p3» etc.
func (n Namer) groupWhy(g, reason string) string {
	kind, who, _ := strings.Cut(reason, ":")
	name := n.Group(g)
	switch kind {
	case "not_proposable":
		if who != "" {
			return fmt.Sprintf("%s kan ikke foreslås fordi %s ikke kan foreslås", name, n.Person(who))
		}
		return name + " kan ikke foreslås"
	case "too_many_places":
		return name + " trenger flere plasser enn det er igjen"
	case "outside_selection":
		return name + " står utenfor utvalget"
	case "task_never":
		return fmt.Sprintf("%s: %s har sagt «ikke» til oppgaven", name, n.Person(who))
	case "event_never":
		return fmt.Sprintf("%s: %s passer ikke", name, n.Person(who))
	case "busy":
		return fmt.Sprintf("%s: %s er opptatt med noe annet", name, n.Person(who))
	}
	return ""
}

// Explain turns the solver's «why is this place empty» (who was ruled out, and for what) into one Norwegian sentence.
func Explain(why map[string]any, n Namer) string {
	list := func(v any) []string {
		arr, _ := v.([]any)
		out := make([]string, 0, len(arr))
		for _, x := range arr {
			if s, ok := x.(string); ok {
				out = append(out, n.Person(s))
			}
		}
		return out
	}
	var parts []string
	if _, has := why["outside_selection"]; has || why["selection"] != nil || why["selection_groups"] != nil {
		v := list(why["selection"])
		var names []string
		names = append(names, v...)
		if arr, _ := why["selection_groups"].([]any); len(arr) > 0 {
			for _, g := range arr {
				if s, ok := g.(string); ok {
					names = append(names, n.Group(s))
				}
			}
		}
		s := "utvalget er tomt"
		if len(names) > 0 {
			s = "utvalget er " + strings.Join(names, ", ")
		}
		if c, _ := why["outside_selection"].(float64); c > 0 {
			s += fmt.Sprintf(" (%d andre står utenfor)", int(c))
		}
		parts = append(parts, s)
	}
	if v := list(why["event_never"]); len(v) > 0 {
		parts = append(parts, "passer ikke: "+strings.Join(v, ", "))
	}
	if v := list(why["task_never"]); len(v) > 0 {
		parts = append(parts, "har sagt «ikke» til oppgaven: "+strings.Join(v, ", "))
	}
	if v := list(why["not_proposable"]); len(v) > 0 {
		parts = append(parts, "kan ikke foreslås: "+strings.Join(v, ", "))
	}
	if busy, _ := why["busy"].(map[string]any); len(busy) > 0 {
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
					name := n.busy(s)
					if name == "" { // not something we can name: show it as it is
						name = s
					}
					ts = append(ts, name)
				}
			}
			items = append(items, fmt.Sprintf("%s (%s)", n.Person(k), strings.Join(ts, ", ")))
		}
		parts = append(parts, "opptatt med noe annet på hendelsen: "+strings.Join(items, ", "))
	}
	if c, _ := why["not_in_task"].(float64); c > 0 {
		parts = append(parts, fmt.Sprintf("%d har ikke valgt oppgaven", int(c)))
	}
	if groups, _ := why["groups"].(map[string]any); len(groups) > 0 {
		keys := make([]string, 0, len(groups))
		for g := range groups {
			keys = append(keys, g)
		}
		sort.Slice(keys, func(i, j int) bool { return n.Group(keys[i]) < n.Group(keys[j]) })
		for _, g := range keys {
			if r, _ := groups[g].(string); r != "" && r != "candidate" {
				if s := n.groupWhy(g, r); s != "" {
					parts = append(parts, s)
				}
			}
		}
	}
	head := "Ingen kan ta denne"
	if c, _ := why["candidates"].(float64); c > 0 {
		head = fmt.Sprintf("%d kunne stilt, men trengs andre steder eller kan ikke kombineres med noe de allerede har", int(c))
	}
	if len(parts) == 0 {
		return head + "."
	}
	return head + ": " + strings.Join(parts, "; ") + "."
}

var (
	warnLocked      = regexp.MustCompile(`^locked: (\S+) on (\S+) (\S+) breaks «([^»]+)» \(manual choice kept\)$`)
	warnGroupMember = regexp.MustCompile(`^group (\S+) is on (\S+) (\S+), but (\S+) has «never» that day \(manual choice kept\)$`)
	warnLockedGroup = regexp.MustCompile(`^(\S+) is locked on (\S+) (\S+) while group (\S+) is on (\S+)$`)
)

// TranslateWarning writes one of the solver's warnings with names instead of ids. An empty result means the warning
// is not worth showing: a manual cell for someone who «kan ikke foreslås» is how that setting is meant to be used.
func TranslateWarning(w string, n Namer) string {
	if m := warnLocked.FindStringSubmatch(w); m != nil {
		person, task, ev := n.Person(m[1]), n.Task(m[2]), n.Event(m[3])
		kind, rest, _ := strings.Cut(m[4], ":")
		switch kind {
		case "not_proposable":
			return ""
		case "not_in_task":
			return fmt.Sprintf("%s er satt på %s %s for hånd, selv om hen ikke har valgt oppgaven.", person, task, ev)
		case "task_never":
			return fmt.Sprintf("%s er satt på %s %s for hånd, selv om hen har sagt «ikke» til oppgaven.", person, task, ev)
		case "event_never":
			return fmt.Sprintf("%s er satt på %s %s for hånd, selv om det ikke passer.", person, task, ev)
		case "outside_selection":
			return fmt.Sprintf("%s er satt på %s %s for hånd, selv om hen står utenfor utvalget.", person, task, ev)
		case "busy":
			return fmt.Sprintf("%s er satt på %s %s for hånd, selv om hen er opptatt med %s.", person, task, ev, n.busy(rest))
		}
		return fmt.Sprintf("%s er satt på %s %s for hånd, selv om «%s».", person, task, ev, m[4])
	}
	if m := warnGroupMember.FindStringSubmatch(w); m != nil {
		return fmt.Sprintf("%s er satt på %s %s for hånd, men %s har sagt at det ikke passer.", n.Group(m[1]), n.Task(m[2]), n.Event(m[3]), n.Person(m[4]))
	}
	if m := warnLockedGroup.FindStringSubmatch(w); m != nil {
		return fmt.Sprintf("%s er satt på %s %s for hånd samtidig som %s står på %s; de to kan ikke kombineres.", n.Person(m[1]), n.Task(m[2]), n.Event(m[3]), n.Group(m[4]), n.Task(m[5]))
	}
	return w
}
