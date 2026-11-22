package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// notesText is the plan's notes as "p<person>e<event>=<note> ".
func notesText(e *env, org string) string {
	var a store.Answers
	e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/autumn-2026/answers", nil).json(&a)
	out := ""
	for _, n := range a.Notes {
		out += fmt.Sprintf("p%de%d=%s ", n.PersonID, n.EventID, n.Note)
	}
	return out
}

func TestANoteOnAnAnswerIsSetShownAndCleared(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	ids := prefSetup(t, e, "org-a")
	path := fmt.Sprintf("/api/orgs/org-a/plans/autumn-2026/events/%d/notes/%d", ids.events[0], ids.person)
	e.mustDo(204, "PUT", path, map[string]string{"note": "  Can come after 12\nnot before  "})
	if got, want := notesText(e, "org-a"), fmt.Sprintf("p%de%d=Can come after 12\nnot before ", ids.person, ids.events[0]); got != want {
		t.Fatalf("notes: %q, want %q", got, want)
	}
	if got := e.refused(400, "PUT", path, map[string]string{"note": "Can come after 12\nnot before"}); got.Code != "nothing_to_change" {
		t.Fatalf("the same note again: %+v", got)
	}
	// the note stays when the answer is set and cleared
	answers := "/api/orgs/org-a/plans/autumn-2026/answers"
	e.mustDo(204, "PUT", answers, map[string]any{"answers": []store.Answer{{Person: ids.person, Event: ids.events[0], Level: "good"}}})
	e.mustDo(204, "PUT", answers, map[string]any{"answers": []store.Answer{{Person: ids.person, Event: ids.events[0], Level: ""}}})
	if notesText(e, "org-a") == "" {
		t.Fatal("clearing the answer took the note")
	}
	e.mustDo(204, "PUT", path, map[string]string{"note": " "})
	if got := notesText(e, "org-a"); got != "" {
		t.Fatalf("an empty note must remove it: %q", got)
	}
}

func TestANoteStaysInItsOrganisationAndPlan(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	a, b := prefSetup(t, e, "org-a"), prefSetup(t, e, "org-b")
	var evs []store.Event
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Spring"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/spring/events", map[string]any{"date": "2027-03-01"}).json(&evs)
	at := func(ev, person int64) string {
		return fmt.Sprintf("/api/orgs/org-a/plans/autumn-2026/events/%d/notes/%d", ev, person)
	}
	for name, path := range map[string]string{
		"another org's person":  at(a.events[0], b.person),
		"another org's event":   at(b.events[0], a.person),
		"another plan's event":  at(evs[0].ID, a.person),
		"a person that is none": at(a.events[0], 999999),
	} {
		if r := e.do("PUT", path, map[string]string{"note": "x"}); r.code != 404 {
			t.Errorf("%s: %d %s, want 404", name, r.code, r.body)
		}
	}
	for name, note := range map[string]string{"too long": strings.Repeat("é", store.MaxNoteRunes+1), "control": "a\x00b"} {
		if r := e.do("PUT", at(a.events[0], a.person), map[string]string{"note": note}); r.code != 400 {
			t.Errorf("%s: %d, want 400", name, r.code)
		}
	}
	e.mustDo(204, "PUT", at(a.events[0], a.person), map[string]string{"note": strings.Repeat("é", store.MaxNoteRunes)})
	if got := notesText(e, "org-b"); got != "" {
		t.Fatalf("org B sees notes: %q", got)
	}
	// a note in another plan of the same organisation stays there
	e.mustDo(204, "PUT", fmt.Sprintf("/api/orgs/org-a/plans/spring/events/%d/notes/%d", evs[0].ID, a.person), map[string]string{"note": "Spring"})
	if got := notesText(e, "org-a"); strings.Contains(got, "Spring") {
		t.Fatalf("autumn shows spring's note: %q", got)
	}
}
