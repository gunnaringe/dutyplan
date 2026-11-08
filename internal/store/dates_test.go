package store

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestIsoWeekday(t *testing.T) {
	for date, want := range map[string]int{"2026-08-09": 7 /* søndag */, "2026-08-10": 1 /* mandag */, "2026-08-15": 6, "2026-12-24": 4} {
		d, _ := time.Parse("2006-01-02", date)
		if got := IsoWeekday(d); got != want {
			t.Errorf("%s = %d, want %d", date, got, want)
		}
	}
}

func setup(t *testing.T) (*Store, Org, Plan, DayType) {
	t.Helper()
	s := open(t)
	o := mustOrg(t, s, "Menighet")
	p, err := s.CreatePlan(ctx, o.Slug, "Ola", "Høst 2026", "")
	if err != nil {
		t.Fatal(err)
	}
	dt, err := s.CreateDayType(ctx, o.Slug, "Ola", "Gudstjeneste")
	if err != nil {
		t.Fatal(err)
	}
	return s, o, p, dt
}

func TestAddUpdateDeleteDateById(t *testing.T) {
	s, o, p, dt := setup(t)
	for name, date := range map[string]string{"not a date": "2026-02-30", "wrong format": "09.08.2026", "empty": ""} {
		if _, err := s.AddDate(ctx, o.Slug, p.Slug, "Ola", date, dt.ID, ""); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	if _, err := s.AddDate(ctx, o.Slug, p.Slug, "Ola", "2026-08-09", dt.ID, strings.Repeat("x", 201)); !errors.Is(err, ErrInvalid) {
		t.Errorf("long note: %v", err)
	}
	d, err := s.AddDate(ctx, o.Slug, p.Slug, "Ola", "2026-08-09", dt.ID, "Med kveldsmat")
	if err != nil || d.DayType != "Gudstjeneste" || d.Note != "Med kveldsmat" {
		t.Fatalf("add: %+v %v", d, err)
	}
	if _, err := s.AddDate(ctx, o.Slug, p.Slug, "Ola", "2026-08-09", dt.ID, ""); !errors.Is(err, ErrConflict) {
		t.Errorf("same date twice: %v", err)
	}
	other, _ := s.CreateDayType(ctx, o.Slug, "Ola", "Alpha")
	got, err := s.UpdateDate(ctx, o.Slug, p.Slug, "Ola", d.ID, DatePatch{DayTypeID: &other.ID})
	if err != nil || got.DayType != "Alpha" || got.Note != d.Note {
		t.Fatalf("update: %+v %v", got, err)
	}
	if _, err := s.UpdateDate(ctx, o.Slug, p.Slug, "Ola", d.ID, DatePatch{DayTypeID: &other.ID}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no-op update: %v", err)
	}

	// The date belongs to its plan: through another plan's slug it does not exist.
	p2, _ := s.CreatePlan(ctx, o.Slug, "Ola", "Vår 2027", "")
	if _, err := s.UpdateDate(ctx, o.Slug, p2.Slug, "Ola", d.ID, DatePatch{Note: ptr("x")}); !errors.Is(err, ErrNotFound) {
		t.Errorf("date through the wrong plan: %v", err)
	}
	if err := s.DeleteDate(ctx, o.Slug, p2.Slug, "Ola", d.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete through the wrong plan: %v", err)
	}
	if err := s.DeleteDate(ctx, o.Slug, p.Slug, "Ola", d.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListDates(ctx, o.Slug, p.Slug); len(list) != 0 {
		t.Errorf("after delete: %v", list)
	}
	// the same date may exist in two plans, and each plan lists only its own
	if _, err := s.AddDate(ctx, o.Slug, p2.Slug, "Ola", "2026-08-09", dt.ID, ""); err != nil {
		t.Errorf("same date in another plan: %v", err)
	}
	if l1, _ := s.ListDates(ctx, o.Slug, p.Slug); len(l1) != 0 {
		t.Errorf("plan 1 must not list plan 2's dates: %v", l1)
	}
	if l2, _ := s.ListDates(ctx, o.Slug, p2.Slug); len(l2) != 1 {
		t.Errorf("plan 2 lists its own: %v", l2)
	}
}

func TestBulkAddByWeekday(t *testing.T) {
	s, o, p, dt := setup(t)
	s.AddDate(ctx, o.Slug, p.Slug, "Ola", "2026-08-16", dt.ID, "") // one Sunday already there
	created, skipped, err := s.AddDatesBulk(ctx, o.Slug, p.Slug, "Ola", "2026-08-09", "2026-09-06", []int{7}, dt.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range created {
		got = append(got, d.Date)
	}
	if fmt.Sprint(got) != "[2026-08-09 2026-08-23 2026-08-30 2026-09-06]" || fmt.Sprint(skipped) != "[2026-08-16]" {
		t.Errorf("Sundays 9.8 - 6.9: created %v skipped %v", got, skipped)
	}
	if created, _, _ = s.AddDatesBulk(ctx, o.Slug, p.Slug, "Ola", "2026-08-10", "2026-08-16", []int{1, 3}, dt.ID); len(created) != 2 {
		t.Errorf("Monday and Wednesday of one week: %v", created)
	}
	for name, c := range map[string]struct {
		from, to string
		days     []int
	}{
		"from after to": {"2026-09-01", "2026-08-01", []int{7}},
		"too long":      {"2026-01-01", "2027-06-01", []int{7}},
		"no weekday":    {"2026-08-01", "2026-08-31", nil},
		"weekday 0":     {"2026-08-01", "2026-08-31", []int{0}},
		"weekday 8":     {"2026-08-01", "2026-08-31", []int{8}},
		"bad date":      {"2026-13-01", "2026-12-31", []int{7}},
	} {
		if _, _, err := s.AddDatesBulk(ctx, o.Slug, p.Slug, "Ola", c.from, c.to, c.days, dt.ID); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
}

// Both organisations have a plan with the same slug; every write must reach the right one.
func TestSamePlanSlugInTwoOrganisations(t *testing.T) {
	s := open(t)
	a, b := mustOrg(t, s, "Org A"), mustOrg(t, s, "Org B")
	pa, _ := s.CreatePlan(ctx, a.Slug, "Ola", "Høst 2026", "")
	pb, _ := s.CreatePlan(ctx, b.Slug, "Eva", "Høst 2026", "")
	if pa.Slug != pb.Slug {
		t.Fatal("test needs equal slugs")
	}
	dtA, _ := s.CreateDayType(ctx, a.Slug, "Ola", "Helg")
	dtB, _ := s.CreateDayType(ctx, b.Slug, "Eva", "Helg")
	if _, err := s.AddDate(ctx, a.Slug, pa.Slug, "Ola", "2026-08-09", dtA.ID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddDate(ctx, b.Slug, pb.Slug, "Eva", "2026-08-16", dtB.ID, ""); err != nil {
		t.Fatalf("a write to B's own plan: %v", err)
	}
	if _, _, err := s.AddDatesBulk(ctx, b.Slug, pb.Slug, "Eva", "2026-08-23", "2026-08-30", []int{7}, dtB.ID); err != nil {
		t.Fatalf("a bulk add to B's own plan: %v", err)
	}
	la, _ := s.ListDates(ctx, a.Slug, pa.Slug)
	lb, _ := s.ListDates(ctx, b.Slug, pb.Slug)
	if len(la) != 1 || la[0].Date != "2026-08-09" || len(lb) != 3 {
		t.Errorf("A: %v\nB: %v", la, lb)
	}
}
