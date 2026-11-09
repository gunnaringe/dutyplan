package store

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestWeightSpecsAreConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range WeightSpecs {
		if seen[s.Key] {
			t.Errorf("%s is listed twice", s.Key)
		}
		seen[s.Key] = true
		switch s.Kind {
		case "number":
			d, ok := s.Default.(float64)
			if !ok || d < s.Min || d > s.Max || s.Min > s.Max {
				t.Errorf("%s: default %v outside [%g, %g]", s.Key, s.Default, s.Min, s.Max)
			}
		case "choice":
			found := false
			for _, o := range s.Options {
				found = found || o == s.Default
			}
			if !found {
				t.Errorf("%s: default %v is not one of %v", s.Key, s.Default, s.Options)
			}
		default:
			t.Errorf("%s: kind %q", s.Key, s.Kind)
		}
	}
}

func weightsWorld(t *testing.T) (*Store, Org, Plan, Plan) {
	t.Helper()
	s := open(t)
	o := mustOrg(t, s, "Menighet")
	p1, err := s.CreatePlan(ctx, o.Slug, "Ola", "Høst 2026", "")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.CreatePlan(ctx, o.Slug, "Ola", "Vår 2027", "")
	if err != nil {
		t.Fatal(err)
	}
	return s, o, p1, p2
}

func TestWeightsAreStoredAsWhatDiffersFromTheDefault(t *testing.T) {
	s, o, p1, p2 := weightsWorld(t)
	if w, err := s.GetWeights(ctx, o.Slug, p1.Slug); err != nil || len(w) != 0 {
		t.Fatalf("a new plan uses the defaults: %v %v", w, err)
	}
	got, err := s.SetWeights(ctx, o.Slug, p1.Slug, "Ola", map[string]any{"w_fair": 12.0, "w_task_fair": 5, "default_combination": "practical", "w_task.good": 2.0 /* the default */})
	if err != nil || fmt.Sprint(got) != "map[default_combination:practical w_fair:12 w_task_fair:5]" {
		t.Fatalf("a value equal to the default is not stored: %v %v", got, err)
	}
	if stored, _ := s.GetWeights(ctx, o.Slug, p1.Slug); fmt.Sprint(stored) != fmt.Sprint(got) {
		t.Errorf("stored %v, returned %v", stored, got)
	}
	log, _ := s.ListLog(ctx, o.Slug, p1.Slug, 1)
	if len(log) == 0 || log[0].Action != "plan.weights" || log[0].Detail != "default_combination impractical -> practical; w_fair 10 -> 12; w_task_fair 0 -> 5" {
		t.Errorf("the log says what changed: %+v", log)
	}
	// the other plan of the organisation is untouched
	if w, _ := s.GetWeights(ctx, o.Slug, p2.Slug); len(w) != 0 {
		t.Errorf("another plan: %v", w)
	}
	// the same weights again, and a change that only restates the defaults, change nothing
	if _, err := s.SetWeights(ctx, o.Slug, p1.Slug, "Ola", map[string]any{"w_fair": 12.0, "w_task_fair": 5.0, "default_combination": "practical"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no change: %v", err)
	}
	// replaced, not merged: leaving a key out sends it back to the default
	// (a choice equal to its default is dropped too, like a number)
	if got, err = s.SetWeights(ctx, o.Slug, p1.Slug, "Ola", map[string]any{"w_fair": 12.0, "default_combination": "impractical"}); err != nil || fmt.Sprint(got) != "map[w_fair:12]" {
		t.Fatalf("replaced: %v %v", got, err)
	}
	if got, err = s.SetWeights(ctx, o.Slug, p1.Slug, "Ola", map[string]any{}); err != nil || len(got) != 0 {
		t.Fatalf("back to the defaults: %v %v", got, err)
	}
	log, _ = s.ListLog(ctx, o.Slug, p1.Slug, 1)
	if !strings.HasPrefix(log[0].Detail, "back to the defaults (w_fair 12 -> 10") {
		t.Errorf("log: %+v", log)
	}
	if _, err := s.SetWeights(ctx, o.Slug, p1.Slug, "Ola", map[string]any{"w_fair": 10.0}); !errors.Is(err, ErrInvalid) {
		t.Errorf("restating the default on a plan that already uses it: %v", err)
	}
}

func TestWeightRefusals(t *testing.T) {
	s, o, p1, _ := weightsWorld(t)
	for name, v := range map[string]map[string]any{
		"unknown key":                          {"w_unfilled": 5.0},
		"another unknown key":                  {"navn": 1.0},
		"an unknown key beside a valid change": {"w_fair": 12.0, "w_unfilled": 0.0}, // alone it would also be refused, as «nothing to change»; only the key check refuses this
		"an unknown key given as text":         {"w_fair": 12.0, "navn": "ti"},
		"above the range":                      {"w_fair": 101.0},
		"below the range":                      {"w_fair": -1.0},
		"a bad preference positive":            {"w_task.poor": 4.0},
		"a good preference negative":           {"w_date.good": -3.0},
		"a number given as text":               {"w_fair": "ti"},
		"a choice not in the list":             {"default_combination": "kanskje"},
		"a choice given as number":             {"default_combination": 3.0},
		"one bad among good":                   {"w_fair": 12.0, "w_task_fair": 500.0},
	} {
		if _, err := s.SetWeights(ctx, o.Slug, p1.Slug, "Ola", v); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	if _, err := s.SetWeights(ctx, o.Slug, p1.Slug, "", map[string]any{"w_fair": 12.0}); !errors.Is(err, ErrActor) {
		t.Errorf("no actor: %v", err)
	}
	if w, _ := s.GetWeights(ctx, o.Slug, p1.Slug); len(w) != 0 {
		t.Errorf("a refused change leaves nothing behind: %v", w)
	}
	if _, err := s.SetWeights(ctx, o.Slug, "ingen", "Ola", map[string]any{"w_fair": 12.0}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown plan: %v", err)
	}
	// the edges of the range are fine
	if _, err := s.SetWeights(ctx, o.Slug, p1.Slug, "Ola", map[string]any{"w_fair": 100.0, "w_task.poor": -100.0, "w_fair_range": 0.0}); err != nil {
		t.Errorf("the edges: %v", err)
	}
}

func TestWeightsBelongToTheirPlanAndOrganisation(t *testing.T) {
	s, a, pa, _ := weightsWorld(t)
	b := mustOrg(t, s, "Annen")
	pb, _ := s.CreatePlan(ctx, b.Slug, "Eva", "Høst 2026", "") // the same plan slug in another organisation
	if pa.Slug != pb.Slug {
		t.Fatal("test needs equal slugs")
	}
	if _, err := s.SetWeights(ctx, a.Slug, pa.Slug, "Ola", map[string]any{"w_fair": 12.0}); err != nil {
		t.Fatal(err)
	}
	if w, _ := s.GetWeights(ctx, b.Slug, pb.Slug); len(w) != 0 {
		t.Errorf("B's plan took A's weights: %v", w)
	}
	if _, err := s.GetWeights(ctx, b.Slug, "var-2027"); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's other plan through B: %v", err)
	}
	if _, err := s.SetWeights(ctx, b.Slug, "var-2027", "Eva", map[string]any{"w_fair": 12.0}); !errors.Is(err, ErrNotFound) {
		t.Errorf("writing to A's other plan through B: %v", err)
	}
	da, _ := s.GetPlanningData(ctx, a.Slug, pa.Slug)
	db, _ := s.GetPlanningData(ctx, b.Slug, pb.Slug)
	if da.Weights["w_fair"] != 12.0 || len(db.Weights) != 0 {
		t.Errorf("planning data: %v / %v", da.Weights, db.Weights)
	}
}
