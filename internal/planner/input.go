// Package planner connects the stored plan data to the Python CP-SAT solver (a subprocess speaking JSON) and turns
// its answer into something a page can show. The solver is told only what belongs to one organisation's plan.
package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/gunnaringe/tjenesteplan/internal/store"
)

// Params are the knobs of one run. The seed makes «try again» give a different plan.
type Params struct {
	TimeLimit int // seconds
	Seed      int
	Workers   int
}

func tid(id int64) string { return "t" + strconv.FormatInt(id, 10) }
func eid(id int64) string { return "e" + strconv.FormatInt(id, 10) }
func did(id int64) string { return "d" + strconv.FormatInt(id, 10) }
func pid(id int64) string { return "p" + strconv.FormatInt(id, 10) }

type solverInput struct {
	Tasks []struct {
		ID      string `json:"id"`
		Planned bool   `json:"planned"`
	} `json:"tasks"`
	Dates []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"dates"`
	DayTypeTasks map[string]map[string]int `json:"day_type_tasks"`
	People       []struct {
		ID         string `json:"id"`
		Proposable bool   `json:"proposable"`
	} `json:"people"`
	PersonTasks  map[string]map[string]string `json:"person_tasks"`
	PersonDates  map[string]map[string]string `json:"person_dates"`
	Combinations []struct {
		A     string `json:"a"`
		B     string `json:"b"`
		Level string `json:"level"`
	} `json:"combinations"`
	Params *solverParams `json:"params,omitempty"`
}

type solverParams struct {
	TimeLimit float64 `json:"time_limit"`
	Seed      int     `json:"seed"`
	Workers   int     `json:"workers"`
}

// Snapshot keeps the names as they were when the run started, so the result reads correctly even if something is
// renamed or removed later.
type Snapshot struct {
	Events []SnapEvent             `json:"events"`
	Tasks  []SnapTask              `json:"tasks"` // the planner's tasks only
	People []SnapPerson            `json:"people"`
	Slots  map[int64]map[int64]int `json:"slots"` // day type -> task -> places
}

type SnapEvent struct {
	ID        int64  `json:"id"`
	Date      string `json:"date"`
	DayTypeID int64  `json:"day_type_id"`
	DayType   string `json:"day_type"`
	Note      string `json:"note"`
}
type SnapTask struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type SnapPerson struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// HasWork reports whether there is anything to plan: an event whose day type has a task the planner assigns.
func HasWork(d store.PlanningData) bool {
	planned := map[int64]bool{}
	for _, t := range d.Tasks {
		if t.Planned {
			planned[t.ID] = true
		}
	}
	slots := map[int64]bool{}
	for _, dt := range d.DayTypes {
		for _, t := range dt.Tasks {
			if planned[t.TaskID] && t.Slots > 0 {
				slots[dt.ID] = true
			}
		}
	}
	for _, e := range d.Events {
		if slots[e.DayTypeID] {
			return true
		}
	}
	return false
}

// BuildInput maps the plan's data to the solver's input. The hash covers the data only (not seed or time limit):
// when the data changes afterwards, a stored run is out of date. Output is deterministic.
func BuildInput(d store.PlanningData, p Params) (input []byte, hash string, snap Snapshot, err error) {
	var in solverInput
	snap = Snapshot{Events: []SnapEvent{}, Tasks: []SnapTask{}, People: []SnapPerson{}, Slots: map[int64]map[int64]int{}}
	taskIDs := map[int64]bool{}
	for _, t := range d.Tasks {
		taskIDs[t.ID] = true
		in.Tasks = append(in.Tasks, struct {
			ID      string `json:"id"`
			Planned bool   `json:"planned"`
		}{tid(t.ID), t.Planned})
		if t.Planned {
			snap.Tasks = append(snap.Tasks, SnapTask{t.ID, t.Name})
		}
	}
	sort.Slice(snap.Tasks, func(i, j int) bool { return snap.Tasks[i].Name < snap.Tasks[j].Name })
	in.DayTypeTasks = map[string]map[string]int{}
	planned := map[int64]bool{}
	for _, t := range d.Tasks {
		planned[t.ID] = t.Planned
	}
	for _, dt := range d.DayTypes {
		in.DayTypeTasks[did(dt.ID)] = map[string]int{}
		snap.Slots[dt.ID] = map[int64]int{}
		for _, t := range dt.Tasks {
			in.DayTypeTasks[did(dt.ID)][tid(t.TaskID)] = t.Slots
			if planned[t.TaskID] {
				snap.Slots[dt.ID][t.TaskID] = t.Slots
			}
		}
	}
	for _, e := range d.Events {
		in.Dates = append(in.Dates, struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		}{eid(e.ID), did(e.DayTypeID)})
		snap.Events = append(snap.Events, SnapEvent{e.ID, e.Date, e.DayTypeID, e.DayType, e.Note})
	}
	in.PersonTasks = map[string]map[string]string{}
	in.PersonDates = map[string]map[string]string{}
	for _, person := range d.People {
		if !person.Active { // an inactive person is not planned
			continue
		}
		in.People = append(in.People, struct {
			ID         string `json:"id"`
			Proposable bool   `json:"proposable"`
		}{pid(person.ID), person.Proposable})
		snap.People = append(snap.People, SnapPerson{person.ID, person.Name})
		if tasks := d.PersonTasks[person.ID]; len(tasks) > 0 {
			m := map[string]string{}
			for task, level := range tasks {
				if taskIDs[task] {
					m[tid(task)] = level
				}
			}
			in.PersonTasks[pid(person.ID)] = m
		}
		if events := d.PersonEvents[person.ID]; len(events) > 0 {
			m := map[string]string{}
			for ev, level := range events {
				m[eid(ev)] = level
			}
			in.PersonDates[pid(person.ID)] = m
		}
	}
	for _, c := range d.Combinations {
		in.Combinations = append(in.Combinations, struct {
			A     string `json:"a"`
			B     string `json:"b"`
			Level string `json:"level"`
		}{tid(c.A), tid(c.B), c.Level})
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil, "", snap, err
	}
	sum := sha256.Sum256(data)
	hash = hex.EncodeToString(sum[:])
	in.Params = &solverParams{TimeLimit: float64(p.TimeLimit), Seed: p.Seed, Workers: p.Workers}
	input, err = json.Marshal(in)
	return input, hash, snap, err
}
