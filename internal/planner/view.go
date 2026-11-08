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
		TaskLevel string `json:"task_level"`
		DateLevel string `json:"date_level"`
	} `json:"assignments"`
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
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Weak bool   `json:"weak"` // the person said «dårlig» for the event or the task
}

type Cell struct {
	TaskID      int64        `json:"task_id"`
	Slots       int          `json:"slots"`
	People      []CellPerson `json:"people"`
	Missing     int          `json:"missing"`
	Explanation string       `json:"explanation,omitempty"`
}

type EventRow struct {
	ID      int64  `json:"id"`
	Date    string `json:"date"`
	DayType string `json:"day_type"`
	Note    string `json:"note"`
	Cells   []Cell `json:"cells"` // in the order of RunView.Tasks; a task the event does not have is omitted
}

type LoadRow struct {
	PersonID int64   `json:"person_id"`
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	Target   float64 `json:"target"`
}

type RunView struct {
	Run      RunInfo    `json:"run"`
	Tasks    []SnapTask `json:"tasks"`
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
	v.Warnings = append(v.Warnings, out.Warnings...)

	people := map[string]string{}
	for _, p := range snap.People {
		people["p"+strconv.FormatInt(p.ID, 10)] = p.Name
	}
	tasks := map[string]string{}
	for _, t := range snap.Tasks {
		tasks[tid(t.ID)] = t.Name
	}
	personName := func(id string) string {
		if n, ok := people[id]; ok {
			return n
		}
		return id
	}
	taskName := func(id string) string {
		if n, ok := tasks[id]; ok {
			return n
		}
		return id
	}

	type key struct{ ev, task string }
	assigned := map[key][]CellPerson{}
	for _, a := range out.Assignments {
		pid, _ := parseID(a.Person, "p")
		weak := a.TaskLevel == "darlig" || a.DateLevel == "darlig"
		k := key{a.Date, a.Task}
		assigned[k] = append(assigned[k], CellPerson{ID: pid, Name: personName(a.Person), Weak: weak})
		if weak {
			what := "hendelsen"
			switch {
			case a.TaskLevel == "darlig" && a.DateLevel == "darlig":
				what = "hendelsen og oppgaven"
			case a.TaskLevel == "darlig":
				what = "oppgaven"
			}
			evID, _ := parseID(a.Date, "e")
			date := ""
			for _, e := range snap.Events {
				if e.ID == evID {
					date = e.Date
				}
			}
			v.Weak = append(v.Weak, fmt.Sprintf("%s fikk %s %s, selv om det er «dårlig» for %s.", personName(a.Person), taskName(a.Task), date, what))
		}
	}
	missing := map[key]int{}
	why := map[key]string{}
	for _, u := range out.Unfilled {
		k := key{u.Date, u.Task}
		missing[k] = u.Missing
		why[k] = Explain(u.Why, personName, taskName)
	}

	used := map[int64]bool{}
	for _, byTask := range snap.Slots {
		for t, n := range byTask {
			if n > 0 {
				used[t] = true
			}
		}
	}
	for _, t := range snap.Tasks {
		if used[t.ID] {
			v.Tasks = append(v.Tasks, t)
		}
	}
	for _, e := range snap.Events {
		row := EventRow{ID: e.ID, Date: e.Date, DayType: e.DayType, Note: e.Note, Cells: []Cell{}}
		for _, t := range v.Tasks {
			n := snap.Slots[e.DayTypeID][t.ID]
			if n == 0 {
				continue
			}
			k := key{eid(e.ID), tid(t.ID)}
			ppl := assigned[k]
			sort.Slice(ppl, func(i, j int) bool { return ppl[i].Name < ppl[j].Name })
			if ppl == nil {
				ppl = []CellPerson{}
			}
			row.Cells = append(row.Cells, Cell{TaskID: t.ID, Slots: n, People: ppl, Missing: missing[k], Explanation: why[k]})
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

// List returns the plan's recent runs with the facts a list needs.
func (s *Service) List(ctx context.Context, orgSlug, planSlug string) ([]store.Run, error) {
	return s.st.ListRuns(ctx, orgSlug, planSlug, 0)
}
