package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Each cap lets the last one in and refuses the next with its code. The caps are lowered to 2 for the test.
func TestTheCapsOnWhatOneSignUpCanMake(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	for _, v := range []*int{&MaxOwnedOrgs, &MaxPlansPerOrg, &MaxPeoplePerPlan, &MaxTasksPerPlan, &MaxEventsPerPlan, &MaxLinksPerOrg} {
		old := *v
		*v = 2
		t.Cleanup(func() { *v = old })
	}
	u, err := s.SignedIn(ctx, "iss", "sub", "Kari", "", "hash", "", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	try := func(what string, make func(i int) error) {
		t.Helper()
		for i := 1; i <= 2; i++ {
			if err := make(i); err != nil {
				t.Fatalf("%s %d: %v", what, i, err)
			}
		}
		if err := make(3); !codeIs(err, "too_many_"+what) {
			t.Errorf("the third of %s: %v, want too_many_%s", what, err, what)
		}
	}
	try("orgs", func(i int) error { _, err := s.CreateOrgOwnedBy(ctx, fmt.Sprintf("Org %d", i), u.ID); return err })
	try("plans", func(i int) error { _, err := s.CreatePlan(ctx, "org-1", fmt.Sprintf("Plan %d", i)); return err })
	try("people", func(i int) error {
		_, err := s.CreatePerson(ctx, "org-1", "plan-1", fmt.Sprintf("Person %d", i))
		return err
	})
	try("tasks", func(i int) error {
		_, err := s.CreateTask(ctx, "org-1", "plan-1", fmt.Sprintf("Task %d", i))
		return err
	})
	try("events", func(i int) error {
		_, err := s.AddEvent(ctx, "org-1", "plan-1", fmt.Sprintf("2026-12-0%d", i))
		return err
	})
	try("links", func(i int) error {
		_, err := s.CreateLink(ctx, "org-1", "plan-1", LevelView, "", 0, false, u.ID)
		return err
	})
	// the caps are per organisation and plan: another one starts from nothing
	if _, err := s.CreatePerson(ctx, "org-1", "plan-2", "Person 1"); err != nil {
		t.Fatalf("a person in another plan: %v", err)
	}
	if _, err := s.AddEvent(ctx, "org-1", "plan-2", "2026-12-01"); err != nil {
		t.Fatalf("an event in another plan: %v", err)
	}
}
