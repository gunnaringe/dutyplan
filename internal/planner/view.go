package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gunnaringe/dutyplan/internal/i18n"
	"github.com/gunnaringe/dutyplan/internal/store"
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
	Weak   bool   `json:"weak"`   // the person said «poor» for the event or the task
	Locked bool   `json:"locked"` // a manual cell when the run was made (set by hand or locked): the planner kept it
}

// CellGroup is a group in a place: proposed by the planner, or set by hand (Locked).
type CellGroup struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
	Fills   int      `json:"fills"`
	Weak    bool     `json:"weak"` // a member said «poor» for the event or the task
	Locked  bool     `json:"locked"`
}

// CellText is a manual cell with free text («Outside guest»).
type CellText struct {
	Text  string `json:"text"`
	Fills int    `json:"fills"`
}

type Cell struct {
	TaskID      int64        `json:"task_id"`
	Slots       int          `json:"slots"` // filled + missing: the event's places, or more when manual cells exceed them
	People      []CellPerson `json:"people"`
	Groups      []CellGroup  `json:"groups"`
	Texts       []CellText   `json:"texts"`
	Missing     int          `json:"missing"`
	Explanation string       `json:"explanation,omitempty"`
}

type EventRow struct {
	ID   int64  `json:"id"`
	Date string `json:"date"`
	// The time of day and the description are the event's current ones while it exists (they do not change the plan, so
	// they do not make a run out of date), else the ones from when the run was made. Events of a date are in time order.
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Description string `json:"description"`
	Cells       []Cell `json:"cells"` // in the order of RunView.Tasks; a task with nothing in this event is omitted
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

// langOf is the optional language argument: English unless one is given.
func langOf(lang []i18n.Lang) i18n.Lang {
	if len(lang) > 0 {
		return lang[0]
	}
	return i18n.EN
}

func parseID(s string, prefix string) (int64, bool) {
	if !strings.HasPrefix(s, prefix) {
		return 0, false
	}
	n, err := strconv.ParseInt(s[len(prefix):], 10, 64)
	return n, err == nil
}

// View reads a run and shows it with the names of the time it was made. A run that is still going, or failed, has no grid.
func (s *Service) View(ctx context.Context, orgSlug, planSlug string, runID int64, lang ...i18n.Lang) (RunView, error) {
	l := langOf(lang)
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
		return v, fmt.Errorf("the run's snapshot cannot be read: %w", err)
	}
	var out solverOutput
	if err := json.Unmarshal(r.OutputJSON, &out); err != nil {
		return v, fmt.Errorf("the run's answer cannot be read: %w", err)
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
	names := Namer{Person: personName, Task: taskName, Group: groupName, Event: eventName, L: l}
	memberNames := func(ids []string) []string {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			out = append(out, personName(id))
		}
		sort.Strings(out)
		return out
	}

	for _, note := range snap.Notes {
		args := make([]any, len(note.Args))
		for i, a := range note.Args {
			args[i] = a
		}
		v.Warnings = append(v.Warnings, i18n.T(l, "note."+note.Code, args...))
	}
	for _, w := range out.Warnings {
		if t := TranslateWarning(w, names); t != "" {
			v.Warnings = append(v.Warnings, t)
		}
	}

	// A cell is every (event, task) the solver says something about: someone in it, a group or text in it, or places
	// missing. Its places are what is filled plus what is missing, so a manual cell for a task the planner does not
	// distribute (or beyond the event's places) just works, and a place of such a task shows nothing.
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
	weakGroup := map[[3]string]bool{} // (event, task, group): a member has a «poor»
	for _, c := range out.Concessions {
		if c.Group != "" {
			weakGroup[[3]string{c.Date, c.Task, c.Group}] = true
		}
	}
	for _, a := range out.Assignments {
		id, _ := parseID(a.Person, "p")
		weak := a.TaskLevel == "poor" || a.DateLevel == "poor"
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
		what := "event"
		switch {
		case c.TaskLevel == "poor" && c.DateLevel == "poor":
			what = "both"
		case c.TaskLevel == "poor":
			what = "task"
		}
		who := personName(c.Person)
		if c.Group != "" {
			who += i18n.T(l, "compromise.member_of", groupName(c.Group))
		}
		v.Weak = append(v.Weak, i18n.T(l, "compromise", who, taskName(c.Task), eventName(c.Date), i18n.T(l, "compromise.what."+what)))
	}

	// A place the answer says nothing about is shown as missing, never dropped silently.
	for _, e := range snap.Events {
		for task, n := range snap.SlotsFor(e) {
			if n > 0 && cells[key{eid(e.ID), tid(task)}] == nil {
				c := cell(eid(e.ID), tid(task))
				c.missing, c.why = n, i18n.T(l, "explain.silent")
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
	live := map[int64]store.Event{}
	if evs, err := s.st.ListEvents(ctx, orgSlug, planSlug); err == nil {
		for _, e := range evs {
			live[e.ID] = e
		}
	}
	for _, e := range snap.Events {
		row := EventRow{ID: e.ID, Date: e.Date, StartTime: e.StartTime, EndTime: e.EndTime, Description: e.Description, Cells: []Cell{}}
		if cur, ok := live[e.ID]; ok {
			row.StartTime, row.EndTime, row.Description = cur.StartTime, cur.EndTime, cur.Description
		}
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
	sort.SliceStable(v.Events, func(i, j int) bool { // by date, then those with a start time in time order, the rest as they were
		a, b := v.Events[i], v.Events[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if (a.StartTime == "") != (b.StartTime == "") {
			return a.StartTime != ""
		}
		return a.StartTime < b.StartTime
	})
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
