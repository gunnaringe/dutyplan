package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// codedBody is the part of an error body the page reads.
type codedBody struct {
	Error   string         `json:"error"`
	Code    string         `json:"code"`
	Params  map[string]any `json:"params"`
	Message string         `json:"message"`
}

func (e *env) refused(want int, method, path string, body any) codedBody {
	e.t.Helper()
	r := e.do(method, path, body)
	if r.code != want {
		e.t.Fatalf("%s %s = %d %s, want %d", method, path, r.code, r.body, want)
	}
	var b codedBody
	r.json(&b)
	return b
}

func TestRefusalsCarryACodeAndParametersForThePage(t *testing.T) {
	e := newEnvWith(t, fakeSolver(func(context.Context, []byte) ([]byte, error) { return emptyAnswer, nil }))
	a := e.cellsWorld("Org A")
	base := "/api/orgs/" + a.org
	plan := base + "/plans/" + a.plan

	for _, c := range []struct {
		what   string
		got    codedBody
		kind   string
		code   string
		params map[string]any
	}{
		{"blank name", e.refused(400, "POST", base+"/people", map[string]string{"name": "  "}), "invalid", "name_invalid", map[string]any{"what": "name"}},
		{"the same person twice", e.refused(409, "POST", base+"/groups", map[string]string{"name": "Choir"}), "conflict", "already_exists", map[string]any{"kind": "group", "name": "Choir"}},
		{"nothing to change", e.refused(400, "PATCH", base+"/people/"+itoa(a.anna.ID), map[string]any{"proposable": true}), "invalid", "nothing_to_change", nil},
		{"a weight above its range", e.refused(400, "PUT", plan+"/weights", map[string]any{"values": map[string]any{"w_fair": 500}}), "invalid", "weight_range", map[string]any{"key": "w_fair", "min": 0.0, "max": 100.0}},
		{"an unknown weight", e.refused(400, "PUT", plan+"/weights", map[string]any{"values": map[string]any{"nope": 1}}), "invalid", "weight_unknown", map[string]any{"key": "nope"}},
		{"no events chosen", e.refused(400, "POST", plan+"/cells", map[string]any{"events": []int64{}, "task": a.sound.ID, "person": a.anna.ID}), "invalid", "choose_events", nil},
		{"a bad date", e.refused(400, "POST", plan+"/events", map[string]any{"date": "09.08.2026"}), "invalid", "date_invalid", nil},
		{"a time limit out of range", e.refused(400, "POST", plan+"/runs", map[string]any{"time_limit": 1}), "invalid", "time_limit", map[string]any{"min": 5.0, "max": 120.0}},
	} {
		if c.got.Error != c.kind || c.got.Code != c.code || c.got.Message == "" {
			t.Errorf("%s: %+v, want %s/%s", c.what, c.got, c.kind, c.code)
		}
		for k, v := range c.params {
			if c.got.Params[k] != v {
				t.Errorf("%s: params[%s] = %v, want %v", c.what, k, c.got.Params[k], v)
			}
		}
	}

	// in use: a group that a manual cell names
	e.mustDo(201, "POST", plan+"/cells", map[string]any{"events": []int64{a.events[0].ID}, "task": a.sang.ID, "group": a.choir.ID})
	if got := e.refused(409, "DELETE", base+"/groups/"+itoa(a.choir.ID), nil); got.Code != "in_use" || got.Params["kind"] != "group" || got.Params["name"] != "Choir" {
		t.Errorf("in use: %+v", got)
	}
	// without a coded cause the English message still comes
	if got := e.refused(404, "GET", base+"/plans/nope", nil); got.Error != "not_found" || got.Code != "" {
		t.Errorf("not found: %+v", got)
	}
}

func TestANothingToPlanRefusalHasItsCode(t *testing.T) {
	e := newEnvWith(t, fakeSolver(func(context.Context, []byte) ([]byte, error) { return emptyAnswer, nil }))
	org, _ := e.plannable("Org A")
	empty, _ := e.st.CreatePlan(context.Background(), org, "Empty", "")
	got := e.refused(400, "POST", "/api/orgs/"+org+"/plans/"+empty.Slug+"/runs", map[string]any{})
	if got.Code != "nothing_to_plan" || got.Error != "invalid" {
		t.Errorf("%+v", got)
	}
}

// unfilledBecauseOfTheDate: a solver that leaves the only place empty because its only candidate cannot attend.
func unfilledBecauseOfTheDate(_ context.Context, input []byte) ([]byte, error) {
	var in struct {
		People []struct{ ID string } `json:"people"`
		Dates  []struct{ ID string } `json:"dates"`
		Tasks  []struct{ ID string } `json:"tasks"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"status": "OPTIMAL", "wall_time": 0.1, "assignments": []any{},
		"unfilled": []map[string]any{{"date": in.Dates[0].ID, "task": in.Tasks[0].ID, "missing": 1,
			"why": map[string]any{"candidates": 0, "event_never": []string{in.People[0].ID}}}},
		"load": map[string]any{}, "violations": []string{},
	})
}

func TestTheRunIsWrittenInTheLanguageThePageAsksFor(t *testing.T) {
	e := newEnvWith(t, fakeSolver(unfilledBecauseOfTheDate))
	org, plan := e.plannable("Org A")
	e.mustDo(202, "POST", "/api/orgs/"+org+"/plans/"+plan+"/runs", map[string]any{"time_limit": 5})
	e.wait()
	explanation := func(query string) string {
		var v struct {
			Events []struct {
				Cells []struct {
					Explanation string `json:"explanation"`
				} `json:"cells"`
			} `json:"events"`
		}
		e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/"+plan+"/runs/latest"+query, nil).json(&v)
		return v.Events[0].Cells[0].Explanation
	}
	for query, want := range map[string]string{
		"":              "Nobody can take this: cannot attend: Aslaug.",
		"?lang=en":      "Nobody can take this: cannot attend: Aslaug.",
		"?lang=nb":      "Ingen kan ta denne: passer ikke: Aslaug.",
		"?lang=no":      "Ingen kan ta denne: passer ikke: Aslaug.",
		"?lang=klingon": "Nobody can take this: cannot attend: Aslaug.",
	} {
		if got := explanation(query); got != want {
			t.Errorf("%q: %q, want %q", query, got, want)
		}
	}
	// the same switch on «mine oppdrag» (no assignments here, but the request must be accepted)
	people, _ := e.st.ListPeople(context.Background(), org)
	if r := e.do("GET", "/api/orgs/"+org+"/plans/"+plan+"/people/"+itoa(people[0].ID)+"/assignments?lang=nb", nil); r.code != 200 || !strings.Contains(string(r.body), `"has_plan":true`) {
		t.Errorf("assignments with ?lang=nb: %d %s", r.code, r.body)
	}
	_ = store.ErrInvalid
}
