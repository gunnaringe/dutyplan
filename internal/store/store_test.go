package store

import (
	"context"
	"errors"
	"io/fs"
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

// addWithPlaces adds the events and gives each the places (task -> places), the way the page does it.
func addWithPlaces(ctx context.Context, s *Store, org, plan string, in NewEvents, places map[int64]int) ([]Event, error) {
	evs, err := s.AddEvents(ctx, org, plan, in)
	if err != nil {
		return nil, err
	}
	for _, e := range evs {
		for task, n := range places {
			if err := s.SetEventPlaces(ctx, org, plan, e.ID, task, n); err != nil {
				return nil, err
			}
		}
	}
	return evs, nil
}

// unplannedTask creates a task the planner does not distribute (only names set by hand).
func unplannedTask(ctx context.Context, s *Store, org, name string) (Task, error) {
	t, err := s.CreateTask(ctx, org, name)
	if err != nil {
		return t, err
	}
	off := false
	return s.UpdateTask(ctx, org, t.ID, TaskPatch{Planned: &off})
}

func mustOrg(t *testing.T, s *Store, name string) Org {
	t.Helper()
	o, err := s.CreateOrg(ctx, name)
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
		files, _ := fs.ReadDir(migrationFiles, "migrations")
		var n int
		if err := s.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != len(files) || n < 1 {
			t.Fatalf("run %d: %d migrations recorded for %d files, err %v", i, n, len(files), err)
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

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Autumn 2026": "autumn-2026", "  Vår  2027!! ": "var-2027", "Åse & Ødegård": "ase-odegard", "---": "", "A": "a",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	s := open(t)
	if _, err := s.CreateOrg(ctx, "???"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a name with no usable slug must be invalid, got %v", err)
	}
}

// The main correctness constraint of the tenancy design: nothing leaks between organisations.
func TestTenantIsolation(t *testing.T) {
	s := open(t)
	a, b := mustOrg(t, s, "Org A"), mustOrg(t, s, "Org B")

	planA, err := s.CreatePlan(ctx, a.Slug, "Autumn 2026")
	if err != nil {
		t.Fatal(err)
	}
	personA, err := s.CreatePerson(ctx, a.Slug, "Kari Test")
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
	if _, err := s.UpdatePerson(ctx, b.Slug, personA.ID, PersonPatch{Proposable: &off}); !errors.Is(err, ErrNotFound) {
		t.Errorf("B updates A's person: want ErrNotFound, got %v", err)
	}
	if people, _ := s.ListPeople(ctx, a.Slug); len(people) != 1 || !people[0].Proposable {
		t.Errorf("A's person changed: %v", people)
	}

	// The same plan slug and person name may exist in both, as separate things.
	planB, err := s.CreatePlan(ctx, b.Slug, "Autumn 2026")
	if err != nil || planB.Slug != planA.Slug {
		t.Fatalf("same plan name in B: %+v, %v", planB, err)
	}
	if _, err := s.CreatePerson(ctx, b.Slug, "Kari Test"); err != nil {
		t.Fatalf("same person name in B: %v", err)
	}
	// ... but not twice in one organisation.
	if _, err := s.CreatePlan(ctx, a.Slug, "Autumn 2026"); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate plan in A: want ErrConflict, got %v", err)
	}
	if _, err := s.CreatePerson(ctx, a.Slug, "Kari Test"); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate person in A: want ErrConflict, got %v", err)
	}
}

func TestUnknownOrganisationIsNotFoundEverywhere(t *testing.T) {
	s := open(t)
	if _, err := s.GetOrg(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Error("GetOrg")
	}
	if _, err := s.ListPlans(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Error("ListPlans")
	}
	if _, err := s.CreatePlan(ctx, "nope", "Plan"); !errors.Is(err, ErrNotFound) {
		t.Error("CreatePlan")
	}
	if _, err := s.CreatePerson(ctx, "nope", "Kari"); !errors.Is(err, ErrNotFound) {
		t.Error("CreatePerson")
	}
}

func TestUpdatePersonToggles(t *testing.T) {
	s := open(t)
	o := mustOrg(t, s, "Club")
	p, _ := s.CreatePerson(ctx, o.Slug, "Kari Test")
	off := false
	got, err := s.UpdatePerson(ctx, o.Slug, p.ID, PersonPatch{Proposable: &off})
	if err != nil || got.Proposable {
		t.Fatalf("got %+v, err %v", got, err)
	}
	if _, err := s.UpdatePerson(ctx, o.Slug, p.ID, PersonPatch{Proposable: &off}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no-op update: want ErrInvalid, got %v", err)
	}
}
