package planner

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/gunnaringe/dutyplan/internal/i18n"
)

// Namer turns the solver's ids (p3, t2, g1, e5) into the names the run was made with, and says in which language the
// sentences are written.
type Namer struct {
	Person, Task, Group, Event func(string) string
	L                          i18n.Lang
}

func (n Namer) t(key string, args ...any) string { return i18n.T(n.L, key, args...) }

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
			return n.t("explain.group.not_proposable_by", name, n.Person(who))
		}
		return n.t("explain.group.not_proposable", name)
	case "too_many_places":
		return n.t("explain.group.too_many_places", name)
	case "outside_selection":
		return n.t("explain.group.outside_selection", name)
	case "task_never", "event_never", "busy":
		return n.t("explain.group."+kind, name, n.Person(who))
	}
	return ""
}

// Explain turns the solver's «why is this place empty» (who was ruled out, and for what) into one sentence.
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
		var names []string
		names = append(names, list(why["selection"])...)
		if arr, _ := why["selection_groups"].([]any); len(arr) > 0 {
			for _, g := range arr {
				if s, ok := g.(string); ok {
					names = append(names, n.Group(s))
				}
			}
		}
		s := n.t("explain.selection_empty")
		if len(names) > 0 {
			s = n.t("explain.selection", strings.Join(names, ", "))
		}
		if c, _ := why["outside_selection"].(float64); c > 0 {
			s += n.t("explain.selection_outside", int(c))
		}
		parts = append(parts, s)
	}
	for _, key := range []string{"event_never", "task_never", "not_proposable"} {
		if v := list(why[key]); len(v) > 0 {
			parts = append(parts, n.t("explain."+key, strings.Join(v, ", ")))
		}
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
		parts = append(parts, n.t("explain.busy", strings.Join(items, ", ")))
	}
	if c, _ := why["not_in_task"].(float64); c > 0 {
		parts = append(parts, n.t("explain.not_in_task", int(c)))
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
	head := n.t("explain.none")
	if c, _ := why["candidates"].(float64); c > 0 {
		head = n.t("explain.some", int(c))
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

// TranslateWarning writes one of the solver's warnings with names instead of ids, in the language of the Namer. An
// empty result means the warning is not worth showing: a manual cell for someone who «cannot be proposed» is how that
// setting is meant to be used.
func TranslateWarning(w string, n Namer) string {
	if m := warnLocked.FindStringSubmatch(w); m != nil {
		person, task, ev := n.Person(m[1]), n.Task(m[2]), n.Event(m[3])
		kind, rest, _ := strings.Cut(m[4], ":")
		switch kind {
		case "not_proposable":
			return ""
		case "not_in_task", "task_never", "event_never", "outside_selection":
			return n.t("warn.manual."+kind, person, task, ev)
		case "busy":
			return n.t("warn.manual.busy", person, task, ev, n.busy(rest))
		}
		return n.t("warn.manual.other", person, task, ev, m[4])
	}
	if m := warnGroupMember.FindStringSubmatch(w); m != nil {
		return n.t("warn.manual.group_member", n.Group(m[1]), n.Task(m[2]), n.Event(m[3]), n.Person(m[4]))
	}
	if m := warnLockedGroup.FindStringSubmatch(w); m != nil {
		return n.t("warn.manual.locked_with_group", n.Person(m[1]), n.Task(m[2]), n.Event(m[3]), n.Group(m[4]), n.Task(m[5]))
	}
	return w
}
