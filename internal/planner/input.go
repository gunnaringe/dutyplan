// Package planner connects the stored plan data to the Python CP-SAT solver (a subprocess speaking JSON) and turns
// its answer into something a page can show. The solver is told only what belongs to one organisation's plan.
package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/gunnaringe/dutyplan/internal/store"
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
func gid(id int64) string { return "g" + strconv.FormatInt(id, 10) }

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
	Groups     []solverGroup     `json:"groups,omitempty"`
	GroupTasks []solverGroupTask `json:"group_tasks,omitempty"`
	Locked     []solverLocked    `json:"locked,omitempty"`
	Pools      []solverPool      `json:"pools,omitempty"`
	Params     map[string]any    `json:"params,omitempty"` // the plan's weights, and (not hashed) time limit, seed and workers
}

// solverLocked is a manual cell: a person, a group or a text; Fills only for the last two.
type solverLocked struct {
	Date   string `json:"date"`
	Task   string `json:"task"`
	Person string `json:"person,omitempty"`
	Group  string `json:"group,omitempty"`
	Label  string `json:"label,omitempty"`
	Fills  int    `json:"fills,omitempty"`
}

// solverPool is a selection (utvalg): the only people and groups that can be chosen. The lists are never null.
type solverPool struct {
	Date   string   `json:"date"`
	Task   string   `json:"task"`
	People []string `json:"people"`
	Groups []string `json:"groups"`
}

type solverGroup struct {
	ID         string   `json:"id"`
	Members    []string `json:"members"`
	Proposable bool     `json:"proposable"`
}

type solverGroupTask struct {
	Group string `json:"group"`
	Task  string `json:"task"`
	Fills int    `json:"fills"`
	Level string `json:"level"`
}

// notNull makes nil slices empty ones: the solver iterates over what it is given, and JSON null is not a list.
func notNull(slices ...any) {
	for _, p := range slices {
		if v := reflect.ValueOf(p).Elem(); v.IsNil() {
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		}
	}
}

// weightParams turns the plan's weights into the planner's params. «w_task.godt» is nested: {"w_task": {"godt": ...}}.
// No weights give no params at all, so a plan that never changed them has the same input (and hash) as before they existed.
func weightParams(weights map[string]any) map[string]any {
	if len(weights) == 0 {
		return nil
	}
	params := map[string]any{}
	for key, v := range weights {
		if group, name, nested := strings.Cut(key, "."); nested {
			sub, _ := params[group].(map[string]any)
			if sub == nil {
				sub = map[string]any{}
				params[group] = sub
			}
			sub[name] = v
		} else {
			params[key] = v
		}
	}
	return params
}

// Snapshot keeps the names as they were when the run started, so the result reads correctly even if something is
// renamed or removed later.
type Snapshot struct {
	Events []SnapEvent             `json:"events"`
	Tasks  []SnapTask              `json:"tasks"` // every task of the organisation (names for the view)
	People []SnapPerson            `json:"people"`
	Slots  map[int64]map[int64]int `json:"slots"` // day type -> task -> places
	Groups []SnapGroup             `json:"groups,omitempty"`
	// Warnings are things the planner was not told about, such as a manual cell for a person who is no longer active.
	Warnings []string `json:"warnings,omitempty"`
}

// SnapGroup is a group as the run saw it: the active members only.
type SnapGroup struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Members []int64 `json:"members"`
}

type SnapEvent struct {
	ID        int64  `json:"id"`
	Date      string `json:"date"`
	DayTypeID int64  `json:"day_type_id"`
	DayType   string `json:"day_type"`
	Note      string `json:"note"`
}
type SnapTask struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Planned bool   `json:"planned"` // the planner distributes it (otherwise it only has manual cells)
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
		snap.Tasks = append(snap.Tasks, SnapTask{t.ID, t.Name, t.Planned})
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
	// Groups: only active members count. A group left with none is not sent (nor anything that names it).
	active := map[int64]bool{}
	for _, person := range d.People {
		if person.Active {
			active[person.ID] = true
		}
	}
	groups := map[int64]bool{}
	for _, g := range d.Groups {
		sg := solverGroup{ID: gid(g.ID), Members: []string{}, Proposable: g.Proposable}
		snapG := SnapGroup{ID: g.ID, Name: g.Name, Members: []int64{}}
		for _, m := range g.Members {
			if active[m] {
				sg.Members = append(sg.Members, pid(m))
				snapG.Members = append(snapG.Members, m)
			}
		}
		if len(sg.Members) == 0 {
			if len(g.Tasks) > 0 {
				snap.Warnings = append(snap.Warnings, "Gruppen «"+g.Name+"» har ingen aktive medlemmer og er ikke med i planleggingen.")
			}
			continue
		}
		groups[g.ID] = true
		in.Groups = append(in.Groups, sg)
		snap.Groups = append(snap.Groups, snapG)
		for _, t := range g.Tasks {
			if planned[t.TaskID] { // the task may have been taken out of the planner's hands after the group was set up
				in.GroupTasks = append(in.GroupTasks, solverGroupTask{gid(g.ID), tid(t.TaskID), t.Fills, t.Level})
			}
		}
	}
	// Manual cells and selections. Whatever names someone who is not active, or a group left without active members,
	// cannot be sent (the solver would count the place without knowing who): it is left out and the run says so.
	evDate := map[int64]string{}
	for _, e := range d.Events {
		evDate[e.ID] = e.Date
	}
	taskName := map[int64]string{}
	for _, t := range d.Tasks {
		taskName[t.ID] = t.Name
	}
	personName := map[int64]string{}
	for _, person := range d.People {
		personName[person.ID] = person.Name
	}
	groupName := map[int64]string{}
	for _, g := range d.Groups {
		groupName[g.ID] = g.Name
	}
	where := func(ev, task int64) string { return taskName[task] + " " + evDate[ev] }
	for _, c := range d.Cells {
		l := solverLocked{Date: eid(c.EventID), Task: tid(c.TaskID)}
		switch {
		case c.PersonID != nil:
			if !active[*c.PersonID] {
				snap.Warnings = append(snap.Warnings, "Ruten for "+personName[*c.PersonID]+" på "+where(c.EventID, c.TaskID)+" er ikke med fordi personen ikke er aktiv.")
				continue
			}
			l.Person = pid(*c.PersonID)
		case c.GroupID != nil:
			if !groups[*c.GroupID] {
				snap.Warnings = append(snap.Warnings, "Ruten for gruppen «"+groupName[*c.GroupID]+"» på "+where(c.EventID, c.TaskID)+" er ikke med fordi gruppen ikke har aktive medlemmer.")
				continue
			}
			l.Group, l.Fills = gid(*c.GroupID), c.Fills
		default:
			l.Label, l.Fills = c.Label, c.Fills
		}
		in.Locked = append(in.Locked, l)
	}
	for _, pl := range d.Pools {
		sp := solverPool{Date: eid(pl.EventID), Task: tid(pl.TaskID), People: []string{}, Groups: []string{}}
		for _, person := range pl.People {
			if active[person] {
				sp.People = append(sp.People, pid(person))
			}
		}
		for _, g := range pl.Groups {
			if groups[g] {
				sp.Groups = append(sp.Groups, gid(g))
			}
		}
		if len(sp.People)+len(sp.Groups) == 0 { // the restriction stays: nobody can be chosen, and the place says why
			snap.Warnings = append(snap.Warnings, "Utvalget for "+where(pl.EventID, pl.TaskID)+" har ingen aktive personer eller grupper, så ingen kan velges.")
		}
		in.Pools = append(in.Pools, sp)
	}
	for _, c := range d.Combinations {
		in.Combinations = append(in.Combinations, struct {
			A     string `json:"a"`
			B     string `json:"b"`
			Level string `json:"level"`
		}{tid(c.A), tid(c.B), c.Level})
	}
	notNull(&in.Tasks, &in.Dates, &in.People, &in.Combinations)
	in.Params = weightParams(d.Weights) // the weights change the plan, so they are part of the hash
	data, err := json.Marshal(in)
	if err != nil {
		return nil, "", snap, err
	}
	sum := sha256.Sum256(data)
	hash = hex.EncodeToString(sum[:])
	if in.Params == nil {
		in.Params = map[string]any{}
	}
	in.Params["time_limit"], in.Params["seed"], in.Params["workers"] = float64(p.TimeLimit), p.Seed, p.Workers
	input, err = json.Marshal(in)
	return input, hash, snap, err
}
