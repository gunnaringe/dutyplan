package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// placesText is the plan's places as "e<event>t<task>=<n> ".
func placesText(e *env, org string) string {
	var ps []struct {
		EventID int64 `json:"event_id"`
		TaskID  int64 `json:"task_id"`
		Slots   int   `json:"slots"`
	}
	e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/autumn-2026/places", nil).json(&ps)
	out := ""
	for _, p := range ps {
		out += fmt.Sprintf("e%dt%d=%d ", p.EventID, p.TaskID, p.Slots)
	}
	return out
}

func TestASelectionSetsManyAnswersAtOnce(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	ids := prefSetup(t, e, "org-a")
	var ola store.Person
	e.mustDo(201, "POST", "/api/orgs/org-a/people", map[string]string{"name": "Ola Sample"}).json(&ola)
	path := "/api/orgs/org-a/plans/autumn-2026/answers"
	var all []store.Answer
	for _, p := range []int64{ids.person, ola.ID} {
		for _, ev := range ids.events {
			all = append(all, store.Answer{Person: p, Event: ev, Level: "good"})
		}
	}
	e.mustDo(204, "PUT", path, map[string]any{"answers": all})
	if got := answersText(e, "org-a"); strings.Count(got, "=good") != 4 {
		t.Fatalf("four answers: %q", got)
	}
	// the same again changes nothing
	if r := e.do("PUT", path, map[string]any{"answers": all}); r.code == 204 {
		t.Errorf("setting what is there: %d, want a refusal that nothing changed", r.code)
	}
	for i := range all {
		all[i].Level = ""
	}
	e.mustDo(204, "PUT", path, map[string]any{"answers": all[:2]})
	if got := answersText(e, "org-a"); strings.Count(got, "=good") != 2 || strings.Contains(got, fmt.Sprintf("p%d ", ids.person)) {
		t.Fatalf("cleared for one person: %q", got)
	}
}

func TestASelectionSetsManyPlacesAtOnce(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	ids := prefSetup(t, e, "org-a")
	path := "/api/orgs/org-a/plans/autumn-2026/places"
	cells := []store.Places{{Event: ids.events[0], Task: ids.sound, Slots: 2}, {Event: ids.events[1], Task: ids.sound, Slots: 2}}
	e.mustDo(204, "PUT", path, map[string]any{"places": cells})
	want := fmt.Sprintf("e%dt%d=2 e%dt%d=2 ", ids.events[0], ids.sound, ids.events[1], ids.sound)
	if got := placesText(e, "org-a"); got != want {
		t.Fatalf("places: %q, want %q", got, want)
	}
	if got := e.refused(400, "PUT", path, map[string]any{"places": cells}); got.Code != "nothing_to_change" {
		t.Fatalf("the same places again: %+v, want nothing_to_change", got)
	}
	cells[0].Slots = 0
	e.mustDo(204, "PUT", path, map[string]any{"places": cells[:1]})
	if got := placesText(e, "org-a"); got != fmt.Sprintf("e%dt%d=2 ", ids.events[1], ids.sound) {
		t.Fatalf("after clearing one: %q", got)
	}
}

func TestASelectionStaysInItsOrganisationAndPlan(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	a, b := prefSetup(t, e, "org-a"), prefSetup(t, e, "org-b")
	answers, places := "/api/orgs/org-a/plans/autumn-2026/answers", "/api/orgs/org-a/plans/autumn-2026/places"
	for name, c := range map[string]struct {
		path string
		body any
	}{
		"another org's person": {answers, map[string]any{"answers": []store.Answer{{Person: a.person, Event: a.events[0], Level: "good"}, {Person: b.person, Event: a.events[0], Level: "good"}}}},
		"another org's event":  {answers, map[string]any{"answers": []store.Answer{{Person: a.person, Event: b.events[0], Level: "good"}}}},
		"another org's task":   {places, map[string]any{"places": []store.Places{{Event: a.events[0], Task: b.sound, Slots: 1}}}},
		"another org's event2": {places, map[string]any{"places": []store.Places{{Event: b.events[0], Task: a.sound, Slots: 1}}}},
	} {
		if r := e.do("PUT", c.path, c.body); r.code != 404 {
			t.Errorf("%s: %d %s, want 404", name, r.code, r.body)
		}
	}
	// an event of another plan in the same organisation
	var evs []store.Event
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Spring"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/spring/events", map[string]any{"date": "2027-03-01"}).json(&evs)
	if r := e.do("PUT", answers, map[string]any{"answers": []store.Answer{{Person: a.person, Event: evs[0].ID, Level: "good"}}}); r.code != 404 {
		t.Errorf("another plan's event: %d", r.code)
	}
	if r := e.do("PUT", places, map[string]any{"places": []store.Places{{Event: evs[0].ID, Task: a.sound, Slots: 1}}}); r.code != 404 {
		t.Errorf("another plan's event (places): %d", r.code)
	}
	for name, c := range map[string]struct {
		path string
		body any
	}{
		"bad level":   {answers, map[string]any{"answers": []store.Answer{{Person: a.person, Event: a.events[0], Level: "maybe"}}}},
		"none":        {answers, map[string]any{"answers": []store.Answer{}}},
		"too many":    {places, map[string]any{"places": make([]store.Places, store.MaxCellsPerCall+1)}},
		"places < 0":  {places, map[string]any{"places": []store.Places{{Event: a.events[0], Task: a.sound, Slots: -1}}}},
		"places > 50": {places, map[string]any{"places": []store.Places{{Event: a.events[0], Task: a.sound, Slots: 51}}}},
	} {
		if r := e.do("PUT", c.path, c.body); r.code != 400 {
			t.Errorf("%s: %d %s, want 400", name, r.code, r.body)
		}
	}
	if got := answersText(e, "org-a") + placesText(e, "org-a") + answersText(e, "org-b") + placesText(e, "org-b"); got != "" {
		t.Fatalf("a refused selection stored something: %q", got)
	}
}

func TestLockingTheDraftAndSettingItAsideOverHTTP(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	ids := prefSetup(t, e, "org-a")
	plan := "/api/orgs/org-a/plans/autumn-2026"
	locks := []store.Lock{{Event: ids.events[0], Task: ids.sound, Person: ids.person}, {Event: ids.events[1], Task: ids.sound, Person: ids.person}}
	e.mustDo(204, "PUT", plan+"/locks", map[string]any{"locks": locks, "locked": true})
	var cells []store.ManualCell
	e.mustDo(200, "GET", plan+"/cells", nil).json(&cells)
	if len(cells) != 2 || !cells[0].Locked || !cells[1].Locked {
		t.Fatalf("two locked names: %+v", cells)
	}
	e.mustDo(204, "PUT", plan+"/locks", map[string]any{"locks": locks[:1], "locked": false})
	e.mustDo(200, "GET", plan+"/cells", nil).json(&cells)
	if len(cells) != 1 || cells[0].EventID != ids.events[1] {
		t.Fatalf("one unlocked: %+v", cells)
	}

	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	prefSetup(t, e, "org-b")
	if r := e.do("PUT", "/api/orgs/org-b/plans/autumn-2026/locks", map[string]any{"locks": locks, "locked": true}); r.code != 404 {
		t.Errorf("A's names locked through B: %d %s", r.code, r.body)
	}

	var p store.Plan
	e.mustDo(204, "DELETE", plan+"/draft", nil)
	e.mustDo(200, "GET", plan, nil).json(&p)
	if !p.DraftAside {
		t.Errorf("the drafts are not set aside: %+v", p)
	}
	var pb store.Plan
	e.mustDo(200, "GET", "/api/orgs/org-b/plans/autumn-2026", nil).json(&pb)
	if pb.DraftAside {
		t.Errorf("B's drafts were set aside: %+v", pb)
	}
}
