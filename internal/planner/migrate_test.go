package planner

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// TestEveryPlanGetsItsOwnPeopleAndTasks: migration 012 on a database with two plans in one organisation (and one without
// plans). Each plan's table, answers, notes, cells, selections, combinations, sections and its runs look the same by name
// afterwards, the second plan has ids of its own, and a run that was current is still current.
func TestEveryPlanGetsItsOwnPeopleAndTasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	st, err := store.OpenUpTo(path, 11)
	if err != nil {
		t.Fatal(err)
	}
	db := st.DB()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	const at = "2026-11-23T10:00:00Z"
	// The organisation «club» with the plans autumn (1) and spring (2); «empty» (2) has no plan.
	exec(`INSERT INTO organization (id, slug, name, created_at) VALUES (1, 'club', 'Club', ?), (2, 'empty', 'Empty', ?)`, at, at)
	exec(`INSERT INTO plan (id, org_id, slug, name, created_at) VALUES (1, 1, 'autumn', 'Autumn', ?), (2, 1, 'spring', 'Spring', ?)`, at, at)
	exec(`INSERT INTO person_section (id, org_id, name, position, created_at) VALUES (1, 1, 'Resting', 0, ?)`, at)
	exec(`INSERT INTO person (id, org_id, name, proposable, section_id, position, created_at) VALUES
		(1, 1, 'Kari Test', 1, NULL, 0, ?), (2, 1, 'Ola Sample', 1, NULL, 1, ?), (3, 1, 'Mona Example', 0, 1, 0, ?),
		(4, 2, 'Pia Demo', 1, NULL, 0, ?)`, at, at, at, at)
	exec(`INSERT INTO task_section (id, org_id, name, position, created_at) VALUES (1, 1, 'Tech', 0, ?)`, at)
	exec(`INSERT INTO task (id, org_id, name, planned, position, created_at, section_id) VALUES
		(1, 1, 'Sound', 1, 0, ?, 1), (2, 1, 'Video', 1, 1, ?, 1), (3, 1, 'Coffee', 0, 0, ?, NULL), (4, 2, 'Chairs', 1, 0, ?, NULL)`, at, at, at, at)
	exec(`INSERT INTO task_combination (org_id, task_a, task_b, level) VALUES (1, 1, 2, 'cannot')`)
	exec(`INSERT INTO person_task (org_id, person_id, task_id, level) VALUES (1, 1, 1, 'good'), (1, 2, 1, 'poor'), (1, 2, 2, 'good'), (2, 4, 4, 'good')`)
	exec(`INSERT INTO event (id, org_id, plan_id, date, created_at) VALUES (1, 1, 1, '2026-09-06', ?), (2, 1, 2, '2027-01-10', ?), (3, 1, 2, '2027-01-17', ?)`, at, at, at)
	for _, e := range []struct{ id, plan int }{{1, 1}, {2, 2}, {3, 2}} {
		exec(`INSERT INTO event_task (org_id, plan_id, event_id, task_id, slots) VALUES (1, ?, ?, 1, 1), (1, ?, ?, 2, 1), (1, ?, ?, 3, 2)`,
			e.plan, e.id, e.plan, e.id, e.plan, e.id)
	}
	exec(`INSERT INTO person_event (org_id, plan_id, person_id, event_id, level) VALUES
		(1, 1, 1, 1, 'good'), (1, 2, 1, 2, 'never'), (1, 2, 2, 2, 'good'), (1, 2, 2, 3, 'poor')`)
	exec(`INSERT INTO answer_note (org_id, plan_id, person_id, event_id, note) VALUES (1, 1, 2, 1, 'Late'), (1, 2, 1, 2, 'Away')`)
	exec(`INSERT INTO manual_cell (org_id, plan_id, event_id, task_id, person_id, label, fills, locked, created_at) VALUES
		(1, 1, 1, 1, 1, NULL, 1, 0, ?), (1, 2, 2, 2, 2, NULL, 1, 1, ?), (1, 2, 3, 3, NULL, 'Guests', 2, 0, ?)`, at, at, at)
	exec(`INSERT INTO pool_member (org_id, plan_id, event_id, task_id, person_id) VALUES (1, 2, 3, 1, 1), (1, 2, 3, 1, 2)`)

	// A finished run of spring, made from spring's data with the old ids: current when the migration runs.
	two := int64(2)
	spring := store.PlanningData{
		Tasks:        []store.Task{{ID: 1, Name: "Sound", Planned: true, Section: 1}, {ID: 2, Name: "Video", Planned: true, Section: 1}, {ID: 3, Name: "Coffee"}},
		Events:       []store.Event{{ID: 2, Date: "2027-01-10"}, {ID: 3, Date: "2027-01-17"}},
		People:       []store.PlanPerson{{ID: 1, Name: "Kari Test", Proposable: true}, {ID: 2, Name: "Ola Sample", Proposable: true}, {ID: 3, Name: "Mona Example"}},
		PersonTasks:  map[int64]map[int64]string{1: {1: "good"}, 2: {1: "poor", 2: "good"}},
		PersonEvents: map[int64]map[int64]string{1: {2: "never"}, 2: {2: "good", 3: "poor"}},
		Combinations: []store.Combination{{A: 1, B: 2, Level: "cannot"}},
		Cells:        []store.ManualCell{{EventID: 2, TaskID: 2, PersonID: &two, Fills: 1, Locked: true}, {EventID: 3, TaskID: 3, Label: "Guests", Fills: 2}},
		Pools:        []store.Pool{{EventID: 3, TaskID: 1, People: []int64{1, 2}}},
		EventPlaces:  map[int64]map[int64]int{2: {1: 1, 2: 1, 3: 2}, 3: {1: 1, 2: 1, 3: 2}},
	}
	input, hash, snap, err := BuildInput(spring, Params{TimeLimit: 5, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	snapJSON, _ := json.Marshal(snap)
	output := `{"status":"OPTIMAL","wall_time":0.1,"assignments":[
		{"date":"e2","task":"t1","person":"p2","locked":false,"task_level":"poor","date_level":"good"},
		{"date":"e2","task":"t2","person":"p2","locked":true,"task_level":"good","date_level":"good"},
		{"date":"e3","task":"t1","person":"p1","locked":false,"task_level":"good","date_level":""}],
		"manual":[{"date":"e3","task":"t3","label":"Guests","fills":2}],"concessions":[],
		"unfilled":[{"date":"e3","task":"t2","missing":1,"why":{"task_never":[],"event_never":[],"not_proposable":["p3"],"not_in_task":1,"busy":{"p1":["t1"]},"candidates":1}}],
		"warnings":["locked: p2 on t2 e2 breaks «x» (manual choice kept)"],"violations":[],
		"load":{"p1":{"count":1,"target":1.5},"p2":{"count":2,"target":1.5}}}`
	exec(`INSERT INTO plan_run (org_id, plan_id, status, seed, time_limit, input_hash, input_json, snapshot_json, output_json, started_at, finished_at)
		VALUES (1, 2, 'done', 1, 5, ?, ?, ?, ?, ?, ?)`, hash, string(input), string(snapJSON), output, at, at)
	// an older run of spring that was out of date already: it stays so
	exec(`INSERT INTO plan_run (org_id, plan_id, status, seed, time_limit, input_hash, input_json, snapshot_json, output_json, started_at, finished_at)
		VALUES (1, 2, 'done', 1, 5, 'out-of-date', ?, ?, ?, ?, ?)`, string(input), string(snapJSON), output, at, at)
	// and one of autumn, which keeps its ids
	exec(`INSERT INTO plan_run (org_id, plan_id, status, seed, time_limit, input_hash, input_json, snapshot_json, output_json, started_at, finished_at)
		VALUES (1, 1, 'done', 1, 5, 'old', '{"people":[{"id":"p1"}]}', '{"people":[{"id":1,"name":"Kari Test"}]}', '{"load":{"p1":{}}}', ?, ?)`, at, at)

	before := map[string][]string{"autumn": shapeOf(t, db, 1), "spring": shapeOf(t, db, 2)}
	st.Close()

	if st, err = store.Open(path); err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	db = st.DB()
	for plan, id := range map[string]int64{"autumn": 1, "spring": 2} {
		if got := shapeOf(t, db, id); !reflect.DeepEqual(got, before[plan]) {
			t.Errorf("%s changed:\nbefore %q\nafter  %q", plan, before[plan], got)
		}
	}
	ids := func(q string, args ...any) []int64 {
		t.Helper()
		rows, err := db.Query(q, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []int64
		for rows.Next() {
			var n int64
			rows.Scan(&n)
			out = append(out, n)
		}
		return out
	}
	if got := ids(`SELECT id FROM person WHERE plan_id = 1 ORDER BY id`); !reflect.DeepEqual(got, []int64{1, 2, 3}) {
		t.Errorf("the first plan keeps its people and ids: %v", got)
	}
	springPeople := ids(`SELECT id FROM person WHERE plan_id = 2 ORDER BY id`)
	if len(springPeople) != 3 || springPeople[0] <= 4 {
		t.Errorf("the second plan has copies with new ids: %v", springPeople)
	}
	if got := ids(`SELECT id FROM person WHERE org_id = 2 AND plan_id IS NULL`); !reflect.DeepEqual(got, []int64{4}) {
		t.Errorf("an organisation without plans keeps its people for its first plan: %v", got)
	}
	if got := ids(`SELECT COUNT(*) FROM person_task WHERE org_id = 2 AND plan_id IS NULL`); !reflect.DeepEqual(got, []int64{1}) {
		t.Errorf("and the tasks they take: %v", got)
	}

	// The runs: the ids are spring's own, and nothing else changed.
	var sn, out, h string
	if err := db.QueryRow(`SELECT snapshot_json, output_json, input_hash FROM plan_run WHERE plan_id = 2 ORDER BY id LIMIT 1`).Scan(&sn, &out, &h); err != nil {
		t.Fatal(err)
	}
	newID := map[string]string{}
	for _, q := range []string{`SELECT 'p' || old.id, 'p' || new.id FROM person old JOIN person new ON new.name = old.name AND new.plan_id = 2 WHERE old.plan_id = 1`,
		`SELECT 't' || old.id, 't' || new.id FROM task old JOIN task new ON new.name = old.name AND new.plan_id = 2 WHERE old.plan_id = 1`} {
		rows, _ := db.Query(q)
		for rows.Next() {
			var a, b string
			rows.Scan(&a, &b)
			newID[a] = b
		}
		rows.Close()
	}
	wantOut := idToken.ReplaceAllStringFunc(output, func(s string) string { return newID[s] })
	if out != wantOut || strings.Contains(out, `"p1"`) {
		t.Errorf("the answer of spring's run:\n%s\nwant\n%s", out, wantOut)
	}
	var s2 Snapshot
	json.Unmarshal([]byte(sn), &s2)
	if len(s2.People) != 3 || "p"+itoa(s2.People[0].ID) != newID["p1"] || len(s2.Kept) != 1 || s2.Kept[0] != "e2/"+newID["t2"]+"/"+newID["p2"] {
		t.Errorf("the snapshot of spring's run: %s", sn)
	}
	var stale int
	db.QueryRow(`SELECT COUNT(*) FROM plan_run WHERE plan_id = 2 AND input_hash = 'out-of-date'`).Scan(&stale)
	if stale != 1 {
		t.Errorf("a run that was out of date became current")
	}
	var autumnOut string
	db.QueryRow(`SELECT output_json FROM plan_run WHERE plan_id = 1`).Scan(&autumnOut)
	if autumnOut != `{"load":{"p1":{}}}` {
		t.Errorf("autumn's run keeps its ids: %s", autumnOut)
	}

	// Spring's run is still the current one, and reads the same.
	svc := New(st, nil, 1, nil)
	var runID int64
	db.QueryRow(`SELECT id FROM plan_run WHERE plan_id = 2 ORDER BY id LIMIT 1`).Scan(&runID)
	v, err := svc.View(ctx, "club", "spring", runID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Run.Stale || h == hash {
		t.Errorf("a run that was current stays current, with a hash of the new ids (stale %v, same hash %v)", v.Run.Stale, h == hash)
	}
	var names []string
	for _, e := range v.Events {
		for _, c := range e.Cells {
			for _, p := range c.People {
				names = append(names, e.Date+" "+p.Name)
			}
		}
	}
	sort.Strings(names)
	if want := []string{"2027-01-10 Ola Sample", "2027-01-10 Ola Sample", "2027-01-17 Kari Test"}; !reflect.DeepEqual(names, want) {
		t.Errorf("the run's names: %q", names)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

// shapeOf is a plan by names: what the table, the answers and the choices say, without ids.
func shapeOf(t *testing.T, db *sql.DB, plan int64) []string {
	t.Helper()
	var perPlan bool
	db.QueryRow(`SELECT COUNT(*) > 0 FROM pragma_table_info('person') WHERE name = 'plan_id'`).Scan(&perPlan)
	var out []string
	for _, q := range []string{
		`SELECT 'answer ' || e.date || ' ' || p.name || ' ' || a.level FROM person_event a JOIN event e ON e.id = a.event_id JOIN person p ON p.id = a.person_id WHERE a.plan_id = ?1`,
		`SELECT 'note ' || e.date || ' ' || p.name || ' ' || n.note FROM answer_note n JOIN event e ON e.id = n.event_id JOIN person p ON p.id = n.person_id WHERE n.plan_id = ?1`,
		`SELECT 'places ' || e.date || ' ' || t.name || ' ' || x.slots FROM event_task x JOIN event e ON e.id = x.event_id JOIN task t ON t.id = x.task_id WHERE x.plan_id = ?1`,
		`SELECT 'cell ' || e.date || ' ' || t.name || ' ' || COALESCE(p.name, c.label) || ' ' || c.locked FROM manual_cell c JOIN event e ON e.id = c.event_id
			JOIN task t ON t.id = c.task_id LEFT JOIN person p ON p.id = c.person_id WHERE c.plan_id = ?1`,
		`SELECT 'pool ' || e.date || ' ' || t.name || ' ' || p.name FROM pool_member m JOIN event e ON e.id = m.event_id JOIN task t ON t.id = m.task_id
			JOIN person p ON p.id = m.person_id WHERE m.plan_id = ?1`,
		// per organisation before 012, per plan after
		`SELECT 'takes ' || p.name || ' ' || t.name || ' ' || x.level FROM person_task x JOIN person p ON p.id = x.person_id JOIN task t ON t.id = x.task_id
			WHERE p.org_id = 1 AND {x}`,
		`SELECT 'combination ' || a.name || ' ' || b.name || ' ' || c.level FROM task_combination c JOIN task a ON a.id = c.task_a JOIN task b ON b.id = c.task_b
			WHERE c.org_id = 1 AND {c}`,
		`SELECT 'person ' || p.name || ' ' || p.proposable || ' ' || p.position || ' ' || COALESCE(s.name, '-') FROM person p LEFT JOIN person_section s ON s.id = p.section_id
			WHERE p.org_id = 1 AND {p}`,
		`SELECT 'task ' || x.name || ' ' || x.planned || ' ' || x.position || ' ' || COALESCE(s.name, '-') FROM task x LEFT JOIN task_section s ON s.id = x.section_id
			WHERE x.org_id = 1 AND {x}`,
	} {
		for _, x := range []string{"x", "c", "p"} {
			cond := "?1 > 0"
			if perPlan {
				cond = x + ".plan_id = ?1"
			}
			q = strings.ReplaceAll(q, "{"+x+"}", cond)
		}
		rows, err := db.Query(q, plan)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		for rows.Next() {
			var s string
			rows.Scan(&s)
			out = append(out, s)
		}
		rows.Close()
	}
	sort.Strings(out)
	return out
}
