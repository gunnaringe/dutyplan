package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/gunnaringe/tjenesteplan/internal/store"
)

func TestPlanWeightsOverHTTP(t *testing.T) {
	e := newEnv(t)
	a, b := e.cellsWorld("Org A"), e.cellsWorld("Org B")
	if a.plan != b.plan {
		t.Fatal("test needs the same plan slug in both organisations")
	}
	path := "/api/orgs/" + a.org + "/plans/" + a.plan + "/weights"
	if _, err := e.st.CreatePlan(context.Background(), a.org, "T", "Kun A", ""); err != nil { // a plan only A has
		t.Fatal(err)
	}
	read := func(method, p string, body any) weightsReply { // a fresh reply each time: Unmarshal would merge into an old map
		var r weightsReply
		e.mustDo(200, method, p, body).json(&r)
		return r
	}

	got := read("GET", path, nil)
	if len(got.Specs) != len(store.WeightSpecs) || len(got.Values) != 0 || got.Specs[0].Key != "w_fair" {
		t.Fatalf("a plan starts on the defaults, and the reply lists every setting with its default and limits: %+v", got)
	}
	got = read("PUT", path, map[string]any{"values": map[string]any{"w_fair": 15, "w_task_fair": 4, "default_combination": "praktisk", "w_task.godt": 2}})
	if fmt.Sprint(got.Values) != "map[default_combination:praktisk w_fair:15 w_task_fair:4]" {
		t.Errorf("values (the default is not kept): %v", got.Values)
	}
	got = read("GET", path, nil)
	if got.Values["w_fair"] != 15.0 {
		t.Errorf("stored: %v", got.Values)
	}
	// the same plan slug in the other organisation, and the other plan, are untouched
	if got = read("GET", "/api/orgs/"+b.org+"/plans/"+b.plan+"/weights", nil); len(got.Values) != 0 {
		t.Errorf("B's plan took A's weights: %v", got.Values)
	}
	if got = read("GET", "/api/orgs/"+a.org+"/plans/var-2027/weights", nil); len(got.Values) != 0 {
		t.Errorf("A's other plan: %v", got.Values)
	}

	for _, c := range []struct {
		code         int
		method, path string
		body         any
		what         string
	}{
		{400, "PUT", path, map[string]any{"values": map[string]any{"w_unfilled": 3}}, "unknown weight"},
		{400, "PUT", path, map[string]any{"values": map[string]any{"w_fair": 500}}, "above the range"},
		{400, "PUT", path, map[string]any{"values": map[string]any{"w_task.darlig": 3}}, "wrong sign"},
		{400, "PUT", path, map[string]any{"values": map[string]any{"w_fair": "mye"}}, "text for a number"},
		{400, "PUT", path, map[string]any{"values": map[string]any{"default_combination": "ja"}}, "a choice not offered"},
		{400, "PUT", path, map[string]any{"values": map[string]any{"w_fair": 15, "w_task_fair": 4, "default_combination": "praktisk"}}, "no change"},
		{400, "PUT", path, map[string]any{"verdier": map[string]any{}}, "unknown field"},
		{404, "PUT", "/api/orgs/" + b.org + "/plans/kun-a/weights", map[string]any{"values": map[string]any{"w_fair": 12}}, "A's other plan through B"},
		{404, "GET", "/api/orgs/" + b.org + "/plans/kun-a/weights", nil, "A's other plan through B (read)"},
		{404, "PUT", "/api/orgs/ingen/plans/" + a.plan + "/weights", map[string]any{"values": map[string]any{"w_fair": 12}}, "unknown organisation"},
	} {
		if r := e.do(c.method, c.path, "T", c.body); r.code != c.code {
			t.Errorf("%s: %s %s = %d %s, want %d", c.what, c.method, c.path, r.code, r.body, c.code)
		}
	}
	if r := e.do("PUT", path, "", map[string]any{"values": map[string]any{"w_fair": 20}}); r.code != 400 || r.errCode() != "actor_required" {
		t.Errorf("no actor: %d %s", r.code, r.body)
	}
	// {} and a missing «values» both go back to the defaults
	if got = read("PUT", path, map[string]any{}); len(got.Values) != 0 {
		t.Errorf("reset: %v", got.Values)
	}
}
