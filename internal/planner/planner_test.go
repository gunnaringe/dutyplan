package planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gunnaringe/dutyplan/internal/i18n"
	"github.com/gunnaringe/dutyplan/internal/store"
)

var ctx = context.Background()

type world struct {
	st                     *store.Store
	org                    store.Org
	plan                   store.Plan
	singing, sound, camera store.Task
	events                 []store.Event
	people                 map[string]store.Person
}

// newWorld: one organisation, three tasks (Singing is manual-only), a plan with four Sundays that have one place for each
// task, and six people who have chosen tasks and marked a few dates.
func newWorld(t *testing.T, orgName string) world {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return populate(t, st, orgName)
}

func populate(t *testing.T, st *store.Store, orgName string) world {
	t.Helper()
	w := world{st: st, people: map[string]store.Person{}}
	var err error
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	w.org, err = st.CreateOrg(ctx, orgName)
	must(err)
	w.plan, err = st.CreatePlan(ctx, w.org.Slug, "Autumn 2026")
	must(err)
	w.singing, err = st.CreateTask(ctx, w.org.Slug, w.plan.Slug, "Singing")
	must(err)
	off := false
	w.singing, err = st.UpdateTask(ctx, w.org.Slug, w.plan.Slug, w.singing.ID, store.TaskPatch{Planned: &off})
	must(err)
	w.sound, err = st.CreateTask(ctx, w.org.Slug, w.plan.Slug, "Sound")
	must(err)
	w.camera, err = st.CreateTask(ctx, w.org.Slug, w.plan.Slug, "Camera")
	must(err)
	must(st.SetCombination(ctx, w.org.Slug, w.plan.Slug, w.sound.ID, w.camera.ID, "cannot"))
	for _, d := range []string{"2026-08-09", "2026-08-16", "2026-08-23", "2026-08-30"} {
		evs, err := addWithPlaces(st, w.org.Slug, w.plan.Slug, store.NewEvents{Date: d}, map[int64]int{w.sound.ID: 1, w.camera.ID: 1, w.singing.ID: 1})
		must(err)
		w.events = append(w.events, evs[0])
	}
	for _, n := range []string{"Aslaug", "Birger", "Cecilie", "Dina", "Embla", "Frode"} {
		p, err := st.CreatePerson(ctx, w.org.Slug, w.plan.Slug, n)
		must(err)
		w.people[n] = p
	}
	choose := func(person string, task store.Task, level string) {
		must(st.SetPersonTask(ctx, w.org.Slug, w.plan.Slug, w.people[person].ID, task.ID, level))
	}
	for _, n := range []string{"Aslaug", "Birger", "Cecilie"} {
		choose(n, w.sound, "good")
	}
	for _, n := range []string{"Cecilie", "Dina", "Embla"} {
		choose(n, w.camera, "good")
	}
	// Frode chose no task: he takes part in nothing
	must(st.SetAvailability(ctx, w.org.Slug, w.plan.Slug, w.people["Aslaug"].ID, map[int64]string{w.events[0].ID: "never"}))
	must(st.SetAvailability(ctx, w.org.Slug, w.plan.Slug, w.people["Dina"].ID, map[int64]string{w.events[1].ID: "never", w.events[2].ID: "good"}))
	return w
}

func (w world) data(t *testing.T) store.PlanningData {
	t.Helper()
	d, err := w.st.GetPlanningData(ctx, w.org.Slug, w.plan.Slug)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestBuildInputMapsTheStoredPlanToTheSolversFormat(t *testing.T) {
	w := newWorld(t, "Club")
	off := false
	w.st.UpdatePerson(ctx, w.org.Slug, w.plan.Slug, w.people["Embla"].ID, store.PersonPatch{Proposable: &off})
	input, hash, snap, err := BuildInput(w.data(t), Params{TimeLimit: 7, Seed: 3, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	var in map[string]any
	if err := json.Unmarshal(input, &in); err != nil {
		t.Fatal(err)
	}
	if got := in["params"].(map[string]any); got["time_limit"].(float64) != 7 || got["seed"].(float64) != 3 || got["workers"].(float64) != 2 {
		t.Errorf("params %v", got)
	}
	people := in["people"].([]any)
	if len(people) != 6 {
		t.Fatalf("everyone: %v", people)
	}
	proposable := map[string]bool{}
	for _, p := range people {
		m := p.(map[string]any)
		proposable[m["id"].(string)] = m["proposable"].(bool)
	}
	if proposable[pid(w.people["Embla"].ID)] || !proposable[pid(w.people["Aslaug"].ID)] {
		t.Errorf("proposable flag: %v", proposable)
	}
	pt := in["person_tasks"].(map[string]any)[pid(w.people["Cecilie"].ID)].(map[string]any)
	if pt[tid(w.sound.ID)] != "good" || pt[tid(w.camera.ID)] != "good" {
		t.Errorf("Cecilie's tasks: %v", pt)
	}
	pd := in["person_dates"].(map[string]any)[pid(w.people["Dina"].ID)].(map[string]any)
	if pd[eid(w.events[1].ID)] != "never" || pd[eid(w.events[2].ID)] != "good" {
		t.Errorf("Dina's dates: %v", pd)
	}
	combos := in["combinations"].([]any)
	if len(combos) != 1 || combos[0].(map[string]any)["level"] != "cannot" {
		t.Errorf("combinations: %v", combos)
	}
	tasks := map[string]bool{}
	for _, x := range in["tasks"].([]any) {
		m := x.(map[string]any)
		tasks[m["id"].(string)] = m["planned"].(bool)
	}
	if tasks[tid(w.singing.ID)] || !tasks[tid(w.sound.ID)] {
		t.Errorf("planned flags: %v", tasks)
	}
	if len(in["dates"].([]any)) != 4 || len(in["day_type_tasks"].(map[string]any)[eid(w.events[0].ID)].(map[string]any)) != 3 {
		t.Errorf("events and their places: %v %v", in["dates"], in["day_type_tasks"])
	}
	// the snapshot names every task (with its planned flag) and the people
	if len(snap.Tasks) != 3 || snap.Tasks[0].Planned == snap.Tasks[1].Planned && snap.Tasks[1].Planned == snap.Tasks[2].Planned || len(snap.People) != 6 || snap.Places[w.events[0].ID][w.singing.ID] != 0 || snap.Places[w.events[0].ID][w.sound.ID] != 1 {
		t.Errorf("snapshot: %+v", snap)
	}
	// deterministic, and the hash ignores seed and time limit but not the data
	again, hash2, _, _ := BuildInput(w.data(t), Params{TimeLimit: 7, Seed: 3, Workers: 2})
	_, hash3, _, _ := BuildInput(w.data(t), Params{TimeLimit: 60, Seed: 9})
	if string(again) != string(input) || hash != hash2 || hash != hash3 {
		t.Error("the input must be deterministic and the hash independent of the knobs")
	}
	w.st.SetAvailability(ctx, w.org.Slug, w.plan.Slug, w.people["Birger"].ID, map[int64]string{w.events[3].ID: "poor"})
	if _, hash4, _, _ := BuildInput(w.data(t), Params{}); hash4 == hash {
		t.Error("a change in the data must change the hash")
	}
}

func TestBuildInputOnlyContainsTheOrganisationsOwnData(t *testing.T) {
	w := newWorld(t, "Org A")
	b := populate(t, w.st, "Org B")
	for _, c := range []struct {
		name  string
		w     world
		other world
	}{{"A", w, b}, {"B", b, w}} {
		input, _, _, err := BuildInput(c.w.data(t), Params{})
		if err != nil {
			t.Fatal(err)
		}
		own := map[string]bool{}
		for _, p := range c.w.people {
			own[pid(p.ID)] = true
		}
		var in struct {
			People []struct {
				ID string `json:"id"`
			} `json:"people"`
			Dates []struct {
				ID string `json:"id"`
			} `json:"dates"`
		}
		json.Unmarshal(input, &in)
		for _, p := range in.People {
			if !own[p.ID] {
				t.Errorf("%s: input has a person that is not theirs: %s", c.name, p.ID)
			}
		}
		for _, p := range c.other.people {
			if strings.Contains(string(input), `"`+pid(p.ID)+`"`) {
				t.Errorf("%s: the other organisation's person %s is in the input", c.name, pid(p.ID))
			}
		}
		for _, e := range c.other.events {
			if strings.Contains(string(input), `"`+eid(e.ID)+`"`) {
				t.Errorf("%s: the other organisation's event %s is in the input", c.name, eid(e.ID))
			}
		}
	}
}

func testNames() Namer {
	look := func(m map[string]string) func(string) string {
		return func(id string) string {
			if n, ok := m[id]; ok {
				return n
			}
			return id
		}
	}
	return Namer{
		Person: look(map[string]string{"p1": "Aslaug", "p2": "Birger", "p3": "Cecilie"}),
		Task:   look(map[string]string{"t1": "Sound", "t2": "Singing"}),
		Event:  look(map[string]string{"e5": "2026-08-09"}),
	}
}

// both holds one sentence in each language.
type both struct{ en, nb string }

func TestExplain(t *testing.T) {
	someEN := "1 could have taken it, but are needed elsewhere or cannot be combined with something they already have"
	someNB := "1 kunne stilt, men trengs andre steder eller kan ikke kombineres med noe de allerede har"
	cases := []struct {
		why  map[string]any
		want both
	}{
		{map[string]any{"candidates": 0.0}, both{"Nobody can take this.", "Ingen kan ta denne."}},
		{map[string]any{"candidates": 0.0, "not_in_task": 3.0, "event_never": []any{"p1", "p2"}},
			both{"Nobody can take this: cannot attend: Aslaug, Birger; 3 have not chosen the task.", "Ingen kan ta denne: passer ikke: Aslaug, Birger; 3 har ikke valgt oppgaven."}},
		{map[string]any{"candidates": 0.0, "task_never": []any{"p3"}, "not_proposable": []any{"p1"}},
			both{"Nobody can take this: said «never» to the task: Cecilie; cannot be proposed: Aslaug.", "Ingen kan ta denne: har sagt «ikke» til oppgaven: Cecilie; kan ikke foreslås: Aslaug."}},
		{map[string]any{"candidates": 2.0, "busy": map[string]any{"p2": []any{"t1"}, "p1": []any{"t1", "t2"}}},
			both{"2 could have taken it, but are needed elsewhere or cannot be combined with something they already have: busy with something else at the event: Aslaug (Sound, Singing), Birger (Sound).",
				"2 kunne stilt, men trengs andre steder eller kan ikke kombineres med noe de allerede har: opptatt med noe annet på hendelsen: Aslaug (Sound, Singing), Birger (Sound)."}},
		{map[string]any{"candidates": 1.0, "busy": map[string]any{"p1": []any{"t2"}}},
			both{someEN + ": busy with something else at the event: Aslaug (Singing).", someNB + ": opptatt med noe annet på hendelsen: Aslaug (Singing)."}},
		// the selection and who is outside it
		{map[string]any{"candidates": 0.0, "selection": []any{"p1", "p2"}, "outside_selection": 4.0},
			both{"Nobody can take this: the selection is Aslaug, Birger (4 others are outside it).", "Ingen kan ta denne: utvalget er Aslaug, Birger (4 andre står utenfor)."}},
		{map[string]any{"candidates": 0.0, "selection": []any{"p1"}}, both{"Nobody can take this: the selection is Aslaug.", "Ingen kan ta denne: utvalget er Aslaug."}},
		{map[string]any{"candidates": 0.0, "outside_selection": 3.0},
			both{"Nobody can take this: the selection is empty (3 others are outside it).", "Ingen kan ta denne: utvalget er tomt (3 andre står utenfor)."}},
	}
	for _, c := range cases {
		for l, want := range map[i18n.Lang]string{i18n.EN: c.want.en, i18n.NB: c.want.nb} {
			n := testNames()
			n.L = l
			if got := Explain(c.why, n); got != want {
				t.Errorf("%s:\n got %q\nwant %q", l, got, want)
			}
		}
	}
}

func TestSolverWarningsAreReadableAndTheDesignedUseIsSilent(t *testing.T) {
	cases := map[string]both{
		"locked: p1 on t1 e5 breaks «event_never» (manual choice kept)":       {"Aslaug was set on Sound 2026-08-09 by hand, even though it does not suit them.", "Aslaug er satt på Sound 2026-08-09 for hånd, selv om det ikke passer."},
		"locked: p1 on t1 e5 breaks «not_in_task» (manual choice kept)":       {"Aslaug was set on Sound 2026-08-09 by hand, even though they have not chosen the task.", "Aslaug er satt på Sound 2026-08-09 for hånd, selv om hen ikke har valgt oppgaven."},
		"locked: p1 on t1 e5 breaks «task_never» (manual choice kept)":        {"Aslaug was set on Sound 2026-08-09 by hand, even though they said «never» to the task.", "Aslaug er satt på Sound 2026-08-09 for hånd, selv om hen har sagt «ikke» til oppgaven."},
		"locked: p1 on t1 e5 breaks «outside_selection» (manual choice kept)": {"Aslaug was set on Sound 2026-08-09 by hand, even though they are outside the selection.", "Aslaug er satt på Sound 2026-08-09 for hånd, selv om hen står utenfor utvalget."},
		"locked: p1 on t1 e5 breaks «busy:t2» (manual choice kept)":           {"Aslaug was set on Sound 2026-08-09 by hand, even though they are busy with Singing.", "Aslaug er satt på Sound 2026-08-09 for hånd, selv om hen er opptatt med Singing."},
		"something else from the solver":                                      {"something else from the solver", "something else from the solver"},
	}
	for in, want := range cases {
		for l, w := range map[i18n.Lang]string{i18n.EN: want.en, i18n.NB: want.nb} {
			n := testNames()
			n.L = l
			if got := TranslateWarning(in, n); got != w {
				t.Errorf("%s:\n got %q\nwant %q", l, got, w)
			}
		}
	}
	// a manual cell for someone who «cannot be proposed» is how that setting is meant to be used: no warning
	for _, l := range []i18n.Lang{i18n.EN, i18n.NB} {
		n := testNames()
		n.L = l
		if got := TranslateWarning("locked: p1 on t1 e5 breaks «not_proposable» (manual choice kept)", n); got != "" {
			t.Errorf("must be silent, got %q", got)
		}
	}
}

// ---- the service, with a fake solver ----------------------------------------------------------------------------

type fakeSolver struct {
	reply func(ctx context.Context, input []byte) ([]byte, error)
}

func (f fakeSolver) Solve(c context.Context, input []byte) ([]byte, error) { return f.reply(c, input) }

func nopLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newService(w world, f func(ctx context.Context, input []byte) ([]byte, error)) *Service {
	return New(w.st, fakeSolver{f}, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// cannedAnswer: Aslaug takes Sound on event 1 (weak: «poor»), Cecilie takes Camera on event 1; Sound on event 2 is empty.
func cannedAnswer(w world) []byte {
	out := map[string]any{
		"status": "FEASIBLE", "wall_time": 1.5,
		"assignments": []map[string]any{
			{"date": eid(w.events[0].ID), "task": tid(w.sound.ID), "person": pid(w.people["Aslaug"].ID), "task_level": "neutral", "date_level": "poor"},
			{"date": eid(w.events[0].ID), "task": tid(w.camera.ID), "person": pid(w.people["Cecilie"].ID), "task_level": "neutral", "date_level": "neutral"},
		},
		"unfilled": []map[string]any{{"date": eid(w.events[1].ID), "task": tid(w.sound.ID), "missing": 1,
			"why": map[string]any{"candidates": 0, "event_never": []string{pid(w.people["Aslaug"].ID)}}}},
		"concessions": []map[string]any{{"date": eid(w.events[0].ID), "task": tid(w.sound.ID), "person": pid(w.people["Aslaug"].ID), "task_level": "neutral", "date_level": "poor"}},
		"load":        map[string]any{pid(w.people["Aslaug"].ID): map[string]any{"count": 1, "target": 1.26, "capacity": 4, "history": 0}},
		"violations":  []string{},
	}
	b, _ := json.Marshal(out)
	return b
}

func waitDone(t *testing.T, s *Service) {
	t.Helper()
	done := make(chan struct{})
	go func() { s.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not finish")
	}
}

func TestRunProducesAViewWithNamesExplanationsAndCompromises(t *testing.T) {
	w := newWorld(t, "Club")
	var sent []byte
	s := newService(w, func(_ context.Context, input []byte) ([]byte, error) { sent = input; return cannedAnswer(w), nil })
	id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, s)
	var in struct {
		Params struct {
			Seed      int     `json:"seed"`
			TimeLimit float64 `json:"time_limit"`
			Workers   int     `json:"workers"`
		} `json:"params"`
	}
	json.Unmarshal(sent, &in)
	if in.Params.Seed != 1 || in.Params.TimeLimit != 5 || in.Params.Workers != 1 {
		t.Errorf("params the solver got: %+v", in.Params)
	}
	v, err := s.View(ctx, w.org.Slug, w.plan.Slug, id)
	if err != nil {
		t.Fatal(err)
	}
	if v.Run.Status != "done" || v.Run.SolverStatus != "FEASIBLE" || !v.Run.Checked || len(v.Run.Violations) != 0 || v.Run.Stale {
		t.Errorf("run: %+v", v.Run)
	}
	if len(v.Tasks) != 2 || v.Tasks[0].Name != "Camera" || v.Tasks[1].Name != "Sound" {
		t.Errorf("columns are the planner's tasks in name order: %+v", v.Tasks)
	}
	if len(v.Events) != 4 || len(v.Events[0].Cells) != 2 {
		t.Fatalf("events: %+v", v.Events)
	}
	lyd0 := v.Events[0].Cells[1]
	if lyd0.Slots != 1 || len(lyd0.People) != 1 || lyd0.People[0].Name != "Aslaug" || !lyd0.People[0].Weak || lyd0.Missing != 0 {
		t.Errorf("first Sound cell: %+v", lyd0)
	}
	lyd1 := v.Events[1].Cells[1]
	if lyd1.Missing != 1 || len(lyd1.People) != 0 || !strings.Contains(lyd1.Explanation, "cannot attend: Aslaug") {
		t.Errorf("empty cell with its reason: %+v", lyd1)
	}
	if len(v.Weak) != 1 || !strings.Contains(v.Weak[0], "Aslaug got Sound 2026-08-09") || !strings.Contains(v.Weak[0], "the event") {
		t.Errorf("compromises: %v", v.Weak)
	}
	if len(v.Load) != 1 || v.Load[0].Name != "Aslaug" || v.Load[0].Target != 1.3 {
		t.Errorf("load: %+v", v.Load)
	}

	// the names are the ones from the start of the run; the run is stale once the data changes
	w.st.SetAvailability(ctx, w.org.Slug, w.plan.Slug, w.people["Aslaug"].ID, map[int64]string{w.events[3].ID: "never"})
	if v, _ = s.View(ctx, w.org.Slug, w.plan.Slug, id); !v.Run.Stale {
		t.Error("a change after the run makes it stale")
	}
	// a second run counts the seed up
	var seed2 []byte
	s2 := newService(w, func(_ context.Context, input []byte) ([]byte, error) { seed2 = input; return cannedAnswer(w), nil })
	s2.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5})
	waitDone(t, s2)
	json.Unmarshal(seed2, &in)
	if in.Params.Seed != 2 {
		t.Errorf("try again must change the seed: %d", in.Params.Seed)
	}
}

func TestTheIndependentCheckIsReportedWhetherItFoundBreachesOrNeverRan(t *testing.T) {
	w := newWorld(t, "Club")
	for name, c := range map[string]struct {
		answer     string
		checked    bool
		violations []string
	}{
		"breaches":  {`{"status":"FEASIBLE","assignments":[],"unfilled":[],"load":{},"violations":["Aslaug has two tasks"]}`, true, []string{"Aslaug has two tasks"}},
		"clean":     {`{"status":"OPTIMAL","assignments":[],"unfilled":[],"load":{},"violations":[]}`, true, nil},
		"unchecked": {`{"status":"OPTIMAL","assignments":[],"unfilled":[],"load":{}}`, false, nil},
	} {
		s := newService(w, func(context.Context, []byte) ([]byte, error) { return []byte(c.answer), nil })
		id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5})
		if err != nil {
			t.Fatal(err)
		}
		waitDone(t, s)
		v, err := s.View(ctx, w.org.Slug, w.plan.Slug, id)
		if err != nil {
			t.Fatal(err)
		}
		if v.Run.Checked != c.checked || strings.Join(v.Run.Violations, "|") != strings.Join(c.violations, "|") {
			t.Errorf("%s: checked %v violations %v", name, v.Run.Checked, v.Run.Violations)
		}
	}
}

func TestRunFailuresAreStoredWithAReason(t *testing.T) {
	w := newWorld(t, "Club")
	for name, c := range map[string]struct {
		reply func(context.Context, []byte) ([]byte, error)
		want  string
	}{
		"crash":    {func(context.Context, []byte) ([]byte, error) { return nil, errors.New("exit status 1: boom") }, "boom"},
		"not json": {func(context.Context, []byte) ([]byte, error) { return []byte("hei"), nil }, "could not be read"},
		"no plan":  {func(context.Context, []byte) ([]byte, error) { return []byte(`{"status":"INFEASIBLE"}`), nil }, "INFEASIBLE"},
		"panic":    {func(context.Context, []byte) ([]byte, error) { panic("uff") }, "internal error"},
		"timeout":  {func(c context.Context, _ []byte) ([]byte, error) { <-c.Done(); return nil, ErrTimeout }, "took too long"},
	} {
		s := newService(w, c.reply)
		s.timeoutOverride = 100 * time.Millisecond
		id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5})
		if err != nil {
			t.Fatal(name, err)
		}
		waitDone(t, s)
		v, _ := s.View(ctx, w.org.Slug, w.plan.Slug, id)
		if v.Run.Status != "failed" || !strings.Contains(v.Run.Error, c.want) || len(v.Events) != 0 {
			t.Errorf("%s: %+v", name, v.Run)
		}
	}
}

func TestOnlyOneRunAtATimeAndSensibleRefusals(t *testing.T) {
	w := newWorld(t, "Club")
	release := make(chan struct{})
	s := newService(w, func(_ context.Context, _ []byte) ([]byte, error) { <-release; return cannedAnswer(w), nil })
	if _, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5}); !errors.Is(err, ErrBusy) {
		t.Errorf("a second run while one is going: %v", err)
	}
	close(release)
	waitDone(t, s)
	if _, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5}); err != nil {
		t.Errorf("after the first finished: %v", err)
	}
	waitDone(t, s)

	for name, p := range map[string]Params{"too short": {TimeLimit: 1}, "too long": {TimeLimit: 1000}} {
		if _, err := s.Start(ctx, w.org.Slug, w.plan.Slug, p); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := s.Start(ctx, w.org.Slug, "nope", Params{TimeLimit: 5}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown plan: %v", err)
	}
	if _, err := New(w.st, nil, 1, nil).Start(ctx, w.org.Slug, w.plan.Slug, Params{}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("no solver: %v", err)
	}
	empty, _ := w.st.CreatePlan(ctx, w.org.Slug, "Tom plan")
	if _, err := s.Start(ctx, w.org.Slug, empty.Slug, Params{TimeLimit: 5}); !errors.Is(err, ErrNothingToPlan) || !errors.Is(err, store.ErrInvalid) {
		t.Errorf("a plan without events: %v", err)
	}
}

func TestRunsAreInvisibleToOtherOrganisations(t *testing.T) {
	w := newWorld(t, "Org A")
	b := populate(t, w.st, "Org B")
	s := newService(w, func(_ context.Context, _ []byte) ([]byte, error) { return cannedAnswer(w), nil })
	id, _ := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5})
	waitDone(t, s)
	if _, err := s.View(ctx, b.org.Slug, b.plan.Slug, id); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("B views A's run: %v", err)
	}
	if l, _ := s.List(ctx, b.org.Slug, b.plan.Slug); len(l) != 0 {
		t.Errorf("B lists A's runs: %v", l)
	}
}

// ---- the real solver ---------------------------------------------------------------------------------------------

func realSolverCommand() string { return os.Getenv("DUTYPLAN_SOLVER") }

func realSolver(t *testing.T) Solver {
	t.Helper()
	cmd := realSolverCommand()
	if cmd == "" {
		t.Skip("DUTYPLAN_SOLVER is not set (run via mise run test:app)")
	}
	return ExecSolver{Argv: strings.Fields(cmd)}
}

func TestTheRealSolverPlansTheStoredDataWithoutBreakingARule(t *testing.T) {
	solver := realSolver(t)
	w := newWorld(t, "Club")
	off := false
	w.st.UpdatePerson(ctx, w.org.Slug, w.plan.Slug, w.people["Embla"].ID, store.PersonPatch{Proposable: &off})  // Embla may not be proposed
	w.st.UpdatePerson(ctx, w.org.Slug, w.plan.Slug, w.people["Birger"].ID, store.PersonPatch{Proposable: &off}) // nor Birger
	s := New(w.st, solver, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))

	run := func() RunView {
		id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5, Seed: 1})
		if err != nil {
			t.Fatal(err)
		}
		waitDone(t, s)
		v, err := s.View(ctx, w.org.Slug, w.plan.Slug, id)
		if err != nil {
			t.Fatal(err)
		}
		if v.Run.Status != "done" {
			t.Fatalf("run failed: %s", v.Run.Error)
		}
		return v
	}
	v := run()
	if !v.Run.Checked || len(v.Run.Violations) != 0 {
		t.Fatalf("the independent checker found: %v (checked %v)", v.Run.Violations, v.Run.Checked)
	}
	who := map[string]map[string][]string{} // event date -> task -> people
	for _, e := range v.Events {
		who[e.Date] = map[string][]string{}
		for _, c := range e.Cells {
			var names []string
			for _, p := range c.People {
				names = append(names, p.Name)
			}
			name := ""
			for _, tk := range v.Tasks {
				if tk.ID == c.TaskID {
					name = tk.Name
				}
			}
			who[e.Date][name] = names
		}
	}
	// Sound can be Aslaug or Cecilie (Birger may not be proposed, Frode chose nothing); Aslaug cannot on 9 Aug. Camera: Cecilie or Dina (Embla may not be proposed).
	for date, tasks := range who {
		for _, n := range tasks["Sound"] {
			if n != "Aslaug" && n != "Cecilie" || (date == "2026-08-09" && n == "Aslaug") {
				t.Errorf("%s Sound: %s is not allowed", date, n)
			}
		}
		for _, n := range tasks["Camera"] {
			if n != "Cecilie" && n != "Dina" || (date == "2026-08-16" && n == "Dina") {
				t.Errorf("%s Camera: %s is not allowed", date, n)
			}
		}
		// Sound and Camera cannot be combined, so Cecilie never has both on one event
		if len(tasks["Sound"]) == 1 && len(tasks["Camera"]) == 1 && tasks["Sound"][0] == tasks["Camera"][0] {
			t.Errorf("%s: %s has both Sound and Camera", date, tasks["Sound"][0])
		}
	}
	for _, e := range v.Events {
		for _, c := range e.Cells {
			if c.Missing > 0 {
				t.Errorf("%s: a place is empty although people could take it: %s", e.Date, c.Explanation)
			}
		}
	}

	// the same data and seed give the same plan (one worker, small problem: solved to the end)
	assign := func(v RunView) string {
		var parts []string
		for _, e := range v.Events {
			for _, c := range e.Cells {
				for _, p := range c.People {
					parts = append(parts, fmt.Sprintf("%s/%d/%s", e.Date, c.TaskID, p.Name))
				}
			}
		}
		sort.Strings(parts)
		return strings.Join(parts, ",")
	}
	if a, b := assign(v), assign(run()); a != b {
		t.Errorf("the same input must give the same plan:\n%s\n%s", a, b)
	}
}

func TestTheRealSolverReportsWhyAPlaceCannotBeFilled(t *testing.T) {
	solver := realSolver(t)
	w := newWorld(t, "Club")
	// nobody wants Camera any more: remove the three who chose it
	for _, n := range []string{"Cecilie", "Dina", "Embla"} {
		w.st.RemovePersonTask(ctx, w.org.Slug, w.plan.Slug, w.people[n].ID, w.camera.ID)
	}
	s := New(w.st, solver, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	id, _ := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5})
	waitDone(t, s)
	v, _ := s.View(ctx, w.org.Slug, w.plan.Slug, id)
	if v.Run.Status != "done" || len(v.Run.Violations) != 0 {
		t.Fatalf("%+v", v.Run)
	}
	for _, e := range v.Events {
		var camera *Cell
		for i := range e.Cells {
			if e.Cells[i].TaskID == w.camera.ID {
				camera = &e.Cells[i]
			}
		}
		if camera == nil || camera.Missing != 1 || !strings.Contains(camera.Explanation, "Nobody can take this") || !strings.Contains(camera.Explanation, "have not chosen the task") {
			t.Errorf("%s: %+v", e.Date, camera)
		}
	}
}

// The solver iterates over every list it is given, and JSON null is not a list: a plan with nothing set must still send [].
func TestEmptyListsAreSentAsListsNeverAsNull(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	org, _ := st.CreateOrg(ctx, "Tom")
	plan, _ := st.CreatePlan(ctx, org.Slug, "Plan")
	d, err := st.GetPlanningData(ctx, org.Slug, plan.Slug)
	if err != nil {
		t.Fatal(err)
	}
	raw, _, _, err := BuildInput(d, Params{})
	if err != nil {
		t.Fatal(err)
	}
	var in map[string]any
	json.Unmarshal(raw, &in)
	for _, key := range []string{"tasks", "dates", "people", "combinations"} {
		if list, ok := in[key].([]any); !ok || list == nil {
			t.Errorf("%s must be a list, got %v", key, in[key])
		}
	}
}

func TestTheSameRunIsWrittenInNorwegianWhenAsked(t *testing.T) {
	w := newWorld(t, "Club")
	s := newService(w, func(context.Context, []byte) ([]byte, error) { return cannedAnswer(w), nil })
	id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, s)
	en, _ := s.View(ctx, w.org.Slug, w.plan.Slug, id)
	nb, err := s.View(ctx, w.org.Slug, w.plan.Slug, id, i18n.NB)
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct{ en, nb string }{
		"compromise":  {en.Weak[0], nb.Weak[0]},
		"explanation": {en.Events[1].Cells[1].Explanation, nb.Events[1].Cells[1].Explanation},
	} {
		if c.en == c.nb || c.en == "" {
			t.Errorf("%s must differ between the languages: %q / %q", name, c.en, c.nb)
		}
	}
	if !strings.Contains(nb.Weak[0], "Aslaug fikk Sound 2026-08-09") || !strings.Contains(nb.Weak[0], "hendelsen") {
		t.Errorf("compromise in Norwegian: %v", nb.Weak)
	}
	if !strings.Contains(nb.Events[1].Cells[1].Explanation, "passer ikke: Aslaug") || !strings.HasPrefix(nb.Events[1].Cells[1].Explanation, "Ingen kan ta denne") {
		t.Errorf("explanation in Norwegian: %q", nb.Events[1].Cells[1].Explanation)
	}
	// the stored run does not depend on the language: asking for Norwegian does not change what English shows afterwards
	if again, _ := s.View(ctx, w.org.Slug, w.plan.Slug, id); again.Weak[0] != en.Weak[0] {
		t.Errorf("English changed: %q", again.Weak)
	}
}

// addWithPlaces adds the events and gives each the places (task -> places), the way the page does it.
func addWithPlaces(st *store.Store, org, plan string, in store.NewEvents, places map[int64]int) ([]store.Event, error) {
	evs, err := st.AddEvents(context.Background(), org, plan, in)
	if err != nil {
		return nil, err
	}
	for _, e := range evs {
		for task, n := range places {
			if err := st.SetEventPlaces(context.Background(), org, plan, e.ID, task, n); err != nil {
				return nil, err
			}
		}
	}
	return evs, nil
}

// runWith makes a run whose planner answers with the given output.
func (w world) runWith(t *testing.T, answer []byte) *Service {
	t.Helper()
	s := newService(w, func(context.Context, []byte) ([]byte, error) { return answer, nil })
	if _, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5}); err != nil {
		t.Fatal(err)
	}
	waitDone(t, s)
	return s
}
