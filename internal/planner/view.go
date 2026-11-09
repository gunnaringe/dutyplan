package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gunnaringe/tjenesteplan/internal/store"
)

type solverOutput struct {
	Status      string  `json:"status"`
	WallTime    float64 `json:"wall_time"`
	Assignments []struct {
		Date      string `json:"date"`
		Task      string `json:"task"`
		Person    string `json:"person"`
		Locked    bool   `json:"locked"`
		TaskLevel string `json:"task_level"`
		DateLevel string `json:"date_level"`
	} `json:"assignments"`
	GroupAssignments []struct {
		Date    string   `json:"date"`
		Task    string   `json:"task"`
		Group   string   `json:"group"`
		Members []string `json:"members"`
		Fills   int      `json:"fills"`
	} `json:"group_assignments"`
	Manual []struct { // manual cells that name a group or a text (a person is in Assignments, with locked)
		Date  string `json:"date"`
		Task  string `json:"task"`
		Group string `json:"group"`
		Label string `json:"label"`
		Fills int    `json:"fills"`
	} `json:"manual"`
	Concessions []struct {
		Date      string `json:"date"`
		Task      string `json:"task"`
		Person    string `json:"person"`
		Group     string `json:"group"`
		TaskLevel string `json:"task_level"`
		DateLevel string `json:"date_level"`
	} `json:"concessions"`
	Unfilled []struct {
		Date    string         `json:"date"`
		Task    string         `json:"task"`
		Missing int            `json:"missing"`
		Why     map[string]any `json:"why"`
	} `json:"unfilled"`
	Warnings   []string `json:"warnings"`
	Violations []string `json:"violations"`
	Load       map[string]struct {
		Count  int     `json:"count"`
		Target float64 `json:"target"`
	} `json:"load"`
}

type RunInfo struct {
	store.Run
	Stale        bool     `json:"stale"`         // the plan's data has changed since this run
	SolverStatus string   `json:"solver_status"` // OPTIMAL | FEASIBLE
	WallTime     float64  `json:"wall_time"`
	Checked      bool     `json:"checked"` // the independent checker has looked at the plan
	Violations   []string `json:"violations"`
}

type CellPerson struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Weak   bool   `json:"weak"`   // the person said «dårlig» for the event or the task
	Locked bool   `json:"locked"` // set by hand: the planner keeps it
}

// CellGroup is a group in a place: proposed by the planner, or set by hand (Locked).
type CellGroup struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
	Fills   int      `json:"fills"`
	Weak    bool     `json:"weak"` // a member said «dårlig» for the event or the task
	Locked  bool     `json:"locked"`
}

// CellText is a manual cell with free text («Gjest utenfra»).
type CellText struct {
	Text  string `json:"text"`
	Fills int    `json:"fills"`
}

type Cell struct {
	TaskID      int64        `json:"task_id"`
	Slots       int          `json:"slots"` // filled + missing: the day type's places, or more when manual cells exceed them
	People      []CellPerson `json:"people"`
	Groups      []CellGroup  `json:"groups"`
	Texts       []CellText   `json:"texts"`
	Missing     int          `json:"missing"`
	Explanation string       `json:"explanation,omitempty"`
}

type EventRow struct {
	ID      int64  `json:"id"`
	Date    string `json:"date"`
	DayType string `json:"day_type"`
	Note    string `json:"note"`
	Cells   []Cell `json:"cells"` // in the order of RunView.Tasks; a task with nothing in this event is omitted
}

type LoadRow struct {
	PersonID int64   `json:"person_id"`
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	Target   float64 `json:"target"`
}

type RunView struct {
	Run      RunInfo    `json:"run"`
	Tasks    []SnapTask `json:"tasks"` // the columns: every task that has a place in some event, planned or not
	Events   []EventRow `json:"events"`
	Weak     []string   `json:"weak"` // plain-language list of the compromises
	Load     []LoadRow  `json:"load"`
	Warnings []string   `json:"warnings"`
}

func parseID(s string, prefix string) (int64, bool) {
	if !strings.HasPrefix(s, prefix) {
		return 0, false
	}
	n, err := strconv.ParseInt(s[len(prefix):], 10, 64)
	return n, err == nil
}

// View reads a run and shows it with the names of the time it was made. A run that is still going, or failed, has no grid.
func (s *Service) View(ctx context.Context, orgSlug, planSlug string, runID int64) (RunView, error) {
	var v RunView
	r, err := s.st.GetRun(ctx, orgSlug, planSlug, runID)
	if err != nil {
		return v, err
	}
	v.Run = RunInfo{Run: r.Run, Violations: []string{}}
	v.Tasks, v.Events, v.Weak, v.Load, v.Warnings = []SnapTask{}, []EventRow{}, []string{}, []LoadRow{}, []string{}
	if cur, err := s.st.GetPlanningData(ctx, orgSlug, planSlug); err == nil {
		if _, hash, _, err := BuildInput(cur, Params{}); err == nil {
			v.Run.Stale = hash != r.InputHash
		}
	}
	if r.Status != "done" {
		return v, nil
	}
	var snap Snapshot
	if err := json.Unmarshal(r.SnapshotJSON, &snap); err != nil {
		return v, fmt.Errorf("kjøringens øyeblikksbilde kan ikke leses: %w", err)
	}
	var out solverOutput
	if err := json.Unmarshal(r.OutputJSON, &out); err != nil {
		return v, fmt.Errorf("kjøringens svar kan ikke leses: %w", err)
	}
	v.Run.SolverStatus, v.Run.WallTime = out.Status, out.WallTime
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(r.OutputJSON, &raw)
	_, v.Run.Checked = raw["violations"]
	if out.Violations != nil {
		v.Run.Violations = out.Violations
	}

	people := map[string]string{}
	for _, p := range snap.People {
		people[pid(p.ID)] = p.Name
	}
	tasks := map[string]string{}
	for _, t := range snap.Tasks {
		tasks[tid(t.ID)] = t.Name
	}
	groups := map[string]SnapGroup{}
	for _, g := range snap.Groups {
		groups[gid(g.ID)] = g
	}
	dates := map[string]string{}
	for _, e := range snap.Events {
		dates[eid(e.ID)] = e.Date
	}
	orID := func(m map[string]string) func(string) string {
		return func(id string) string {
			if n, ok := m[id]; ok {
				return n
			}
			return id
		}
	}
	personName, taskName, eventName := orID(people), orID(tasks), orID(dates)
	groupName := func(id string) string {
		if g, ok := groups[id]; ok {
			return g.Name
		}
		return id
	}
	names := Namer{Person: personName, Task: taskName, Group: groupName, Event: eventName}
	memberNames := func(ids []string) []string {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			out = append(out, personName(id))
		}
		sort.Strings(out)
		return out
	}

	v.Warnings = append(v.Warnings, snap.Warnings...)
	for _, w := range out.Warnings {
		if t := TranslateWarning(w, names); t != "" {
			v.Warnings = append(v.Warnings, t)
		}
	}

	// A cell is every (event, task) the solver says something about: someone in it, a group or text in it, or places
	// missing. Its places are what is filled plus what is missing, so a manual cell for a task the planner does not
	// distribute (or beyond the day type's places) just works, and a day-type place of such a task shows nothing.
	type key struct{ ev, task string }
	type acc struct {
		people  []CellPerson
		groups  []CellGroup
		texts   []CellText
		missing int
		why     string
	}
	cells := map[key]*acc{}
	cell := func(date, task string) *acc {
		k := key{date, task}
		if cells[k] == nil {
			cells[k] = &acc{}
		}
		return cells[k]
	}
	weakGroup := map[[3]string]bool{} // (event, task, group): some member has a «dårlig»
	for _, c := range out.Concessions {
		if c.Group != "" {
			weakGroup[[3]string{c.Date, c.Task, c.Group}] = true
		}
	}
	for _, a := range out.Assignments {
		id, _ := parseID(a.Person, "p")
		weak := a.TaskLevel == "darlig" || a.DateLevel == "darlig"
		c := cell(a.Date, a.Task)
		c.people = append(c.people, CellPerson{ID: id, Name: personName(a.Person), Weak: weak, Locked: a.Locked})
	}
	for _, g := range out.GroupAssignments {
		id, _ := parseID(g.Group, "g")
		c := cell(g.Date, g.Task)
		c.groups = append(c.groups, CellGroup{ID: id, Name: groupName(g.Group), Members: memberNames(g.Members), Fills: g.Fills, Weak: weakGroup[[3]string{g.Date, g.Task, g.Group}]})
	}
	for _, m := range out.Manual {
		c := cell(m.Date, m.Task)
		fills := m.Fills
		if fills < 1 {
			fills = 1
		}
		if m.Group != "" {
			id, _ := parseID(m.Group, "g")
			var members []string
			for _, mem := range groups[m.Group].Members {
				members = append(members, personName(pid(mem)))
			}
			sort.Strings(members)
			c.groups = append(c.groups, CellGroup{ID: id, Name: groupName(m.Group), Members: members, Fills: fills, Weak: weakGroup[[3]string{m.Date, m.Task, m.Group}], Locked: true})
		} else {
			c.texts = append(c.texts, CellText{Text: m.Label, Fills: fills})
		}
	}
	for _, u := range out.Unfilled {
		c := cell(u.Date, u.Task)
		c.missing = u.Missing
		c.why = Explain(u.Why, names)
	}

	for _, c := range out.Concessions {
		what := "hendelsen"
		switch {
		case c.TaskLevel == "darlig" && c.DateLevel == "darlig":
			what = "hendelsen og oppgaven"
		case c.TaskLevel == "darlig":
			what = "oppgaven"
		}
		who := personName(c.Person)
		if c.Group != "" {
			who += " (som medlem av " + groupName(c.Group) + ")"
		}
		v.Weak = append(v.Weak, fmt.Sprintf("%s fikk %s %s, selv om det er «dårlig» for %s.", who, taskName(c.Task), eventName(c.Date), what))
	}

	// A day-type place the answer says nothing about is shown as missing, never dropped silently.
	for _, e := range snap.Events {
		for task, n := range snap.Slots[e.DayTypeID] {
			if n > 0 && cells[key{eid(e.ID), tid(task)}] == nil {
				c := cell(eid(e.ID), tid(task))
				c.missing, c.why = n, "Planleggeren sa ingenting om denne plassen."
			}
		}
	}

	inGrid := map[string]bool{}
	for k := range cells {
		inGrid[k.task] = true
	}
	for _, t := range snap.Tasks {
		if inGrid[tid(t.ID)] {
			v.Tasks = append(v.Tasks, t)
		}
	}
	sort.Slice(v.Tasks, func(i, j int) bool { return v.Tasks[i].Name < v.Tasks[j].Name })
	for _, e := range snap.Events {
		row := EventRow{ID: e.ID, Date: e.Date, DayType: e.DayType, Note: e.Note, Cells: []Cell{}}
		for _, t := range v.Tasks {
			a := cells[key{eid(e.ID), tid(t.ID)}]
			if a == nil {
				continue
			}
			sort.Slice(a.people, func(i, j int) bool { return a.people[i].Name < a.people[j].Name })
			sort.Slice(a.groups, func(i, j int) bool { return a.groups[i].Name < a.groups[j].Name })
			sort.Slice(a.texts, func(i, j int) bool { return a.texts[i].Text < a.texts[j].Text })
			c := Cell{TaskID: t.ID, People: nonNil(a.people), Groups: nonNilG(a.groups), Texts: nonNilT(a.texts), Missing: a.missing, Explanation: a.why}
			c.Slots = len(a.people) + a.missing
			for _, g := range a.groups {
				c.Slots += g.Fills
			}
			for _, x := range a.texts {
				c.Slots += x.Fills
			}
			row.Cells = append(row.Cells, c)
		}
		v.Events = append(v.Events, row)
	}
	for p, l := range out.Load {
		id, _ := parseID(p, "p")
		v.Load = append(v.Load, LoadRow{PersonID: id, Name: personName(p), Count: l.Count, Target: float64(int(l.Target*10+0.5)) / 10})
	}
	sort.Slice(v.Load, func(i, j int) bool { return v.Load[i].Name < v.Load[j].Name })
	return v, nil
}

func nonNil(x []CellPerson) []CellPerson {
	if x == nil {
		return []CellPerson{}
	}
	return x
}
func nonNilG(x []CellGroup) []CellGroup {
	if x == nil {
		return []CellGroup{}
	}
	return x
}
func nonNilT(x []CellText) []CellText {
	if x == nil {
		return []CellText{}
	}
	return x
}

// List returns the plan's recent runs with the facts a list needs.
func (s *Service) List(ctx context.Context, orgSlug, planSlug string) ([]store.Run, error) {
	return s.st.ListRuns(ctx, orgSlug, planSlug, 0)
}
