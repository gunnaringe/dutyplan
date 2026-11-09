package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// databaseAtVersion8 builds a database the way the app made it before the codes became English: the first eight migrations,
// then rows with the old Norwegian codes in every table the ninth migration touches.
func databaseAtVersion8(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`)
	entries, _ := fs.ReadDir(migrationFiles, "migrations")
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		v, _ := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if v > 8 {
			break
		}
		body, _ := migrationFiles.ReadFile("migrations/" + name)
		exec(string(body))
		exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, 'x')`, v)
	}
	exec(`INSERT INTO organization (id, slug, name, created_at) VALUES (1, 'org', 'Org', 'x')`)
	exec(`INSERT INTO plan (id, org_id, slug, name, created_at, weights) VALUES
		(1, 1, 'a', 'A', 'x', '{"w_task.godt":3,"w_task.darlig":-2,"w_date.godt":4,"w_date.darlig":-7,"w_combo.bor":8,"w_combo.praktisk":1,"w_combo.upraktisk":-6,"w_fair":12,"default_combination":"praktisk"}'),
		(2, 1, 'b', 'B', 'x', '{"default_combination":"upraktisk"}'),
		(3, 1, 'c', 'C', 'x', '{}')`)
	for i, n := range []string{"Ann", "Bo"} {
		exec(`INSERT INTO person (id, org_id, name, created_at) VALUES (?, 1, ?, 'x')`, i+1, n)
	}
	for i, n := range []string{"Sound", "Camera", "Coffee"} {
		exec(`INSERT INTO task (id, org_id, name, created_at) VALUES (?, 1, ?, 'x')`, i+1, n)
	}
	exec(`INSERT INTO day_type (id, org_id, name, created_at) VALUES (1, 1, 'Service', 'x')`)
	exec(`INSERT INTO event (id, org_id, plan_id, date, day_type_id, created_at) VALUES (1, 1, 1, '2026-08-02', 1, 'x'), (2, 1, 1, '2026-08-09', 1, 'x')`)
	exec(`INSERT INTO person_task (org_id, person_id, task_id, level) VALUES (1,1,1,'ikke'), (1,1,2,'darlig'), (1,2,1,'noytral'), (1,2,3,'godt')`)
	exec(`INSERT INTO person_event (org_id, plan_id, person_id, event_id, level) VALUES (1,1,1,1,'ikke'), (1,1,1,2,'darlig'), (1,1,2,1,'godt')`)
	exec(`INSERT INTO task_combination (org_id, task_a, task_b, level) VALUES (1,1,2,'kan_ikke'), (1,1,3,'upraktisk'), (1,2,3,'praktisk')`)
	exec(`INSERT INTO grp (id, org_id, name, created_at) VALUES (1, 1, 'Choir', 'x')`)
	exec(`INSERT INTO grp_task (org_id, group_id, task_id, fills, level) VALUES (1,1,1,2,'darlig'), (1,1,2,1,'noytral'), (1,1,3,1,'godt')`)
	// a finished run, one that failed already, and one still going
	exec(`INSERT INTO plan_run (id, org_id, plan_id, status, seed, time_limit, actor, input_hash, input_json, snapshot_json, output_json, error, started_at) VALUES
		(1, 1, 1, 'done', 1, 10, 'x', 'h', '{}', '{}', '{"status":"OPTIMAL","unfilled":[{"why":{"dato_ikke":["p1"]}}]}', NULL, 'x'),
		(2, 1, 1, 'failed', 2, 10, 'x', 'h', '{}', '{}', NULL, 'boom', 'x'),
		(3, 1, 1, 'running', 3, 10, 'x', 'h', '{}', '{}', NULL, NULL, 'x')`)
}

func TestMigrationToEnglishCodesRewritesEveryTableItTouches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	databaseAtVersion8(t, path)
	s, err := Open(path) // applies migration 9 to the old database
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows := func(q string) string {
		t.Helper()
		r, err := s.DB().Query(q)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		var out []string
		for r.Next() {
			var v string
			if err := r.Scan(&v); err != nil {
				t.Fatal(err)
			}
			out = append(out, v)
		}
		return strings.Join(out, ",")
	}
	for q, want := range map[string]string{
		`SELECT level FROM person_task ORDER BY person_id, task_id`:     "never,poor,neutral,good",
		`SELECT level FROM person_event ORDER BY person_id, event_id`:   "never,poor,good",
		`SELECT level FROM task_combination ORDER BY task_a, task_b`:    "cannot,impractical,practical",
		`SELECT level FROM grp_task ORDER BY task_id`:                   "poor,neutral,good",
		`SELECT status FROM plan_run ORDER BY id`:                       "failed,failed,running",
		`SELECT COALESCE(error, '') FROM plan_run ORDER BY id`:          "made by an older version; run again,boom,",
		`SELECT COALESCE(output_json, 'none') FROM plan_run WHERE id=1`: "none",
		`SELECT weights FROM plan WHERE id = 3`:                         "{}",
		`SELECT weights FROM plan WHERE id = 2`:                         `{"default_combination":"impractical"}`,
	} {
		if got := rows(q); got != want {
			t.Errorf("%s\n got %q\nwant %q", q, got, want)
		}
	}
	w, err := s.GetWeights(ctx, "org", "a")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(w) != "map[default_combination:practical w_combo.impractical:-6 w_combo.practical:1 w_combo.preferred:8 w_date.good:4 w_date.poor:-7 w_fair:12 w_task.good:3 w_task.poor:-2]" {
		t.Errorf("the plan's weights are readable by the new code, with the new keys: %v", w)
	}
	// the old codes are refused from now on, the new ones are accepted, and nothing points at a missing row
	for _, bad := range []string{
		`INSERT INTO person_task (org_id, person_id, task_id, level) VALUES (1,2,2,'godt')`,
		`INSERT INTO person_event (org_id, plan_id, person_id, event_id, level) VALUES (1,1,2,2,'noytral')`,
		`INSERT INTO task_combination (org_id, task_a, task_b, level) VALUES (1,2,3,'bor')`,
		`INSERT INTO grp_task (org_id, group_id, task_id, fills, level) VALUES (1,1,3,1,'ikke')`,
	} {
		if _, err := s.DB().Exec(bad); err == nil {
			t.Errorf("an old code must be refused: %s", bad)
		}
	}
	if _, err := s.DB().Exec(`INSERT INTO person_task (org_id, person_id, task_id, level) VALUES (1,2,2,'good')`); err != nil {
		t.Errorf("a new code is accepted: %v", err)
	}
	if got := rows(`SELECT COUNT(*) FROM pragma_foreign_key_check`); got != "0" {
		t.Errorf("foreign key check found %s problems", got)
	}
	// a run from before the migration opens without trouble; it is simply failed
	if got := rows(`SELECT COUNT(*) FROM schema_migrations WHERE version = 9`); got != "1" {
		t.Errorf("migration 9 recorded: %s", got)
	}
}

func TestAFreshDatabaseHasTheEnglishCodesAndNoOldRows(t *testing.T) {
	s := open(t)
	if _, err := s.DB().Exec(`INSERT INTO organization (id, slug, name, created_at) VALUES (1, 'o', 'O', 'x')`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE sql LIKE '%noytral%' OR sql LIKE '%kan_ikke%' OR sql LIKE '%upraktisk%'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("no table definition may mention an old code: %d %v", n, err)
	}
}

func TestMigrationAddsPinningToExistingRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	databaseAtVersion8(t, path)
	s, err := Open(path) // applies migrations 9 and 10 to the old database
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n := count(t, s, `SELECT COUNT(*) FROM plan_run WHERE pinned = 0`); n != 3 {
		t.Fatalf("old runs start unpinned: %d of 3", n)
	}
	// migration 9 turned the old finished run into a failed one ("made by an older version"), so none of them can be pinned
	if err := s.PinRun(ctx, "org", "a", "Ola", 1, true); !codeIs(err, "run_not_done") {
		t.Errorf("an old run cannot be pinned: %v", err)
	}
	id, err := s.CreateRun(ctx, "org", "a", "Ola", 4, 5, "h", []byte("{}"), []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRun(ctx, id, []byte(`{"status":"OPTIMAL"}`), ""); err != nil {
		t.Fatal(err)
	}
	if err := s.PinRun(ctx, "org", "a", "Ola", id, true); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM plan_run WHERE pinned = 1`); n != 1 {
		t.Errorf("pinned runs: %d", n)
	}
}
