package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

var ctx = context.Background()

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustOrg(t *testing.T, s *Store, name string) Org {
	t.Helper()
	o, err := s.CreateOrg(ctx, "Test", name, "")
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestMigrationsRunOnceAndAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	for i := 0; i < 2; i++ {
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var n int
		if err := s.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != 1 {
			t.Fatalf("run %d: %d migrations recorded, err %v", i, n, err)
		}
		s.Close()
	}
}

func TestForeignKeysAreEnforced(t *testing.T) {
	s := open(t)
	_, err := s.DB().Exec(`INSERT INTO plan (org_id, slug, name, created_at) VALUES (999, 'x', 'x', 'now')`)
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Fatalf("orphan plan insert should fail with a foreign key error, got %v", err)
	}
}

func TestActorIsRequiredAndValidated(t *testing.T) {
	s := open(t)
	for name, actor := range map[string]string{
		"empty": "", "blank": "%20%20", "bad encoding": "%zz", "too long": strings.Repeat("a", 81), "control char": "a%00b",
	} {
		if _, err := s.CreateOrg(ctx, actor, "Org "+name, ""); !errors.Is(err, ErrActor) {
			t.Errorf("%s: want ErrActor, got %v", name, err)
		}
	}
	if orgs, _ := s.ListOrgs(ctx); len(orgs) != 0 {
		t.Fatalf("rejected writes must leave nothing behind, got %v", orgs)
	}
}

func TestActorIsPercentDecodedAndLogged(t *testing.T) {
	s := open(t)
	if _, err := s.CreateOrg(ctx, "%C3%85se%20%C3%98deg%C3%A5rd", "Menighet", ""); err != nil {
		t.Fatal(err)
	}
	log, err := s.ListLog(ctx, "menighet", "", 10)
	if err != nil || len(log) != 1 || log[0].Actor != "Åse Ødegård" || log[0].Action != "org.create" {
		t.Fatalf("log = %+v, err %v", log, err)
	}
}

func TestFailedWriteLeavesNoLogLine(t *testing.T) {
	s := open(t)
	mustOrg(t, s, "Menighet")
	if _, err := s.CreateOrg(ctx, "Test", "Menighet", ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	log, _ := s.ListLog(ctx, "menighet", "", 10)
	if len(log) != 1 {
		t.Fatalf("one successful write, one log line; got %d", len(log))
	}
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Høst 2026": "host-2026", "  Vår  2027!! ": "var-2027", "Åse & Ødegård": "ase-odegard", "---": "", "A": "a",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	s := open(t)
	if _, err := s.CreateOrg(ctx, "Test", "???", ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("a name with no usable slug must be invalid, got %v", err)
	}
	if _, err := s.CreateOrg(ctx, "Test", "Fin", "Bad Slug"); !errors.Is(err, ErrInvalid) {
		t.Errorf("explicit slug must already be a slug, got %v", err)
	}
}

// The main correctness constraint of the tenancy design: nothing leaks between organisations.
func TestTenantIsolation(t *testing.T) {
	s := open(t)
	a, b := mustOrg(t, s, "Org A"), mustOrg(t, s, "Org B")

	planA, err := s.CreatePlan(ctx, a.Slug, "Ola", "Høst 2026", "")
	if err != nil {
		t.Fatal(err)
	}
	personA, err := s.CreatePerson(ctx, a.Slug, "Ola", "Kari Test")
	if err != nil {
		t.Fatal(err)
	}

	// Reading another organisation's plan or people through your own organisation fails.
	if _, err := s.GetPlan(ctx, b.Slug, planA.Slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("B reads A's plan: want ErrNotFound, got %v", err)
	}
	if people, _ := s.ListPeople(ctx, b.Slug); len(people) != 0 {
		t.Errorf("B sees A's people: %v", people)
	}
	if plans, _ := s.ListPlans(ctx, b.Slug); len(plans) != 0 {
		t.Errorf("B sees A's plans: %v", plans)
	}

	// Writing to A's person through B fails and changes nothing.
	off := false
	if _, err := s.UpdatePerson(ctx, b.Slug, "Eva", personA.ID, PersonPatch{Proposable: &off}); !errors.Is(err, ErrNotFound) {
		t.Errorf("B updates A's person: want ErrNotFound, got %v", err)
	}
	if people, _ := s.ListPeople(ctx, a.Slug); len(people) != 1 || !people[0].Proposable {
		t.Errorf("A's person changed: %v", people)
	}

	// The change log is per organisation.
	for _, e := range mustLog(t, s, b.Slug, "") {
		if e.Plan != "" || strings.Contains(e.Detail, "Høst") || strings.Contains(e.Detail, "Kari") {
			t.Errorf("B's log shows A's activity: %+v", e)
		}
	}
	if got := mustLog(t, s, b.Slug, planA.Slug); len(got) != 0 {
		t.Errorf("B filtering by A's plan slug must show nothing, got %v", got)
	}

	// The same plan slug and person name may exist in both, as separate things.
	planB, err := s.CreatePlan(ctx, b.Slug, "Eva", "Høst 2026", "")
	if err != nil || planB.Slug != planA.Slug {
		t.Fatalf("same plan name in B: %+v, %v", planB, err)
	}
	if _, err := s.CreatePerson(ctx, b.Slug, "Eva", "Kari Test"); err != nil {
		t.Fatalf("same person name in B: %v", err)
	}
	// ... but not twice in one organisation.
	if _, err := s.CreatePlan(ctx, a.Slug, "Ola", "Høst 2026", ""); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate plan in A: want ErrConflict, got %v", err)
	}
	if _, err := s.CreatePerson(ctx, a.Slug, "Ola", "Kari Test"); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate person in A: want ErrConflict, got %v", err)
	}
}

func mustLog(t *testing.T, s *Store, org, plan string) []LogEntry {
	t.Helper()
	l, err := s.ListLog(ctx, org, plan, 100)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestUnknownOrganisationIsNotFoundEverywhere(t *testing.T) {
	s := open(t)
	if _, err := s.GetOrg(ctx, "ingen"); !errors.Is(err, ErrNotFound) {
		t.Error("GetOrg")
	}
	if _, err := s.ListPlans(ctx, "ingen"); !errors.Is(err, ErrNotFound) {
		t.Error("ListPlans")
	}
	if _, err := s.CreatePlan(ctx, "ingen", "T", "Plan", ""); !errors.Is(err, ErrNotFound) {
		t.Error("CreatePlan")
	}
	if _, err := s.CreatePerson(ctx, "ingen", "T", "Kari"); !errors.Is(err, ErrNotFound) {
		t.Error("CreatePerson")
	}
	if _, err := s.ListLog(ctx, "ingen", "", 10); !errors.Is(err, ErrNotFound) {
		t.Error("ListLog")
	}
}

func TestUpdatePersonTogglesAndLogs(t *testing.T) {
	s := open(t)
	o := mustOrg(t, s, "Menighet")
	p, _ := s.CreatePerson(ctx, o.Slug, "Ola", "Kari Test")
	off := false
	got, err := s.UpdatePerson(ctx, o.Slug, "Ola", p.ID, PersonPatch{Proposable: &off})
	if err != nil || got.Proposable || !got.Active {
		t.Fatalf("got %+v, err %v", got, err)
	}
	if _, err := s.UpdatePerson(ctx, o.Slug, "Ola", p.ID, PersonPatch{Proposable: &off}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no-op update: want ErrInvalid, got %v", err)
	}
	log := mustLog(t, s, o.Slug, "")
	if len(log) != 3 || log[0].Action != "person.update" || !strings.Contains(log[0].Detail, "proposable true -> false") {
		t.Errorf("log = %+v", log)
	}
}

func TestLogFilterByPlanAndLimit(t *testing.T) {
	s := open(t)
	o := mustOrg(t, s, "Menighet")
	p1, _ := s.CreatePlan(ctx, o.Slug, "Ola", "Høst 2026", "")
	s.CreatePlan(ctx, o.Slug, "Ola", "Vår 2027", "")
	if got := mustLog(t, s, o.Slug, p1.Slug); len(got) != 1 || got[0].Plan != p1.Slug {
		t.Errorf("plan filter: %+v", got)
	}
	if got, _ := s.ListLog(ctx, o.Slug, "", 2); len(got) != 2 || got[0].Action != "plan.create" {
		t.Errorf("limit and order: %+v", got)
	}
}
