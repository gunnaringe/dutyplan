package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

type prefIDs struct {
	person, sound, singing int64
	events                 []int64
}

func prefSetup(t *testing.T, e *env, org string) prefIDs {
	t.Helper()
	var p store.Person
	var sound, lov store.Task
	var evs []store.Event
	e.mustDo(201, "POST", "/api/orgs/"+org+"/plans", map[string]string{"name": "Autumn 2026"})
	e.mustDo(201, "POST", "/api/orgs/"+org+"/plans/"+term+"/people", map[string]string{"name": "Kari Test"}).json(&p)
	e.mustDo(201, "POST", "/api/orgs/"+org+"/plans/"+term+"/tasks", map[string]any{"name": "Sound"}).json(&sound)
	e.mustDo(201, "POST", "/api/orgs/"+org+"/plans/"+term+"/tasks", map[string]any{"name": "Singing"}).json(&lov)
	e.mustDo(200, "PATCH", fmt.Sprintf("/api/orgs/%s/plans/%s/tasks/%d", org, term, lov.ID), map[string]any{"planned": false})
	ids := prefIDs{person: p.ID, sound: sound.ID, singing: lov.ID}
	for _, d := range []string{"2026-08-09", "2026-08-16"} {
		e.mustDo(201, "POST", "/api/orgs/"+org+"/plans/autumn-2026/events", map[string]any{"date": d}).json(&evs)
		ids.events = append(ids.events, evs[0].ID)
	}
	return ids
}

// answersText is everyone's answers in the plan, as the table reads them: "t<task>=<level>" and "e<event>=<level>" per person.
func answersText(e *env, org string) string {
	var a store.Answers
	e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/autumn-2026/answers", nil).json(&a)
	out := ""
	for _, x := range a.Tasks {
		out += fmt.Sprintf("p%d t%d=%s ", x.PersonID, x.TaskID, x.Level)
	}
	for _, x := range a.Events {
		out += fmt.Sprintf("p%d e%d=%s ", x.PersonID, x.EventID, x.Level)
	}
	return out
}

func TestAnswersFlow(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	ids := prefSetup(t, e, "org-a")
	if got := answersText(e, "org-a"); got != "" {
		t.Fatalf("nothing answered yet: %q", got)
	}
	e.mustDo(204, "PUT", fmt.Sprintf("/api/orgs/org-a/plans/autumn-2026/people/%d/tasks/%d", ids.person, ids.sound), map[string]string{"level": "good"})
	e.mustDo(204, "PUT", fmt.Sprintf("/api/orgs/org-a/plans/autumn-2026/people/%d/availability", ids.person),
		map[string]any{"levels": map[string]string{fmt.Sprint(ids.events[0]): "never", fmt.Sprint(ids.events[1]): "good"}})
	want := fmt.Sprintf("p%d t%d=good p%d e%d=never p%d e%d=good ", ids.person, ids.sound, ids.person, ids.events[0], ids.person, ids.events[1])
	if got := answersText(e, "org-a"); got != want {
		t.Errorf("after answering: %q, want %q", got, want)
	}
	e.mustDo(204, "DELETE", fmt.Sprintf("/api/orgs/org-a/plans/autumn-2026/people/%d/tasks/%d", ids.person, ids.sound), nil)
	e.mustDo(204, "PUT", fmt.Sprintf("/api/orgs/org-a/plans/autumn-2026/people/%d/availability", ids.person),
		map[string]any{"levels": map[string]string{fmt.Sprint(ids.events[0]): ""}})
	want = fmt.Sprintf("p%d e%d=good ", ids.person, ids.events[1])
	if got := answersText(e, "org-a"); got != want {
		t.Errorf("after clearing: %q, want %q", got, want)
	}
	// taking a task off one person's list leaves the others'
	var ola store.Person
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/"+term+"/people", map[string]string{"name": "Ola Sample"}).json(&ola)
	for _, p := range []int64{ids.person, ola.ID} {
		e.mustDo(204, "PUT", fmt.Sprintf("/api/orgs/org-a/plans/%s/people/%d/tasks/%d", term, p, ids.sound), map[string]string{"level": "good"})
	}
	e.mustDo(204, "DELETE", fmt.Sprintf("/api/orgs/org-a/plans/%s/people/%d/tasks/%d", term, ids.person, ids.sound), nil)
	if got := answersText(e, "org-a"); !strings.Contains(got, fmt.Sprintf("p%d t%d=good", ola.ID, ids.sound)) {
		t.Errorf("Ola's task went with Kari's: %q", got)
	}
}

func TestAnswerErrorsAndIsolation(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	a := prefSetup(t, e, "org-a")
	b := prefSetup(t, e, "org-b")
	task := func(org string, person, task int64) string {
		return fmt.Sprintf("/api/orgs/%s/plans/autumn-2026/people/%d/tasks/%d", org, person, task)
	}
	avail := func(org string, person int64) string {
		return fmt.Sprintf("/api/orgs/%s/plans/autumn-2026/people/%d/availability", org, person)
	}
	notFound := func(method, path string, body any) {
		t.Helper()
		if r := e.do(method, path, body); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("%s %s = %d %s, want 404", method, path, r.code, r.body)
		}
	}
	notFound("PUT", task("org-b", a.person, b.sound), map[string]string{"level": "good"})
	notFound("PUT", task("org-b", b.person, a.sound), map[string]string{"level": "good"})
	notFound("PUT", task("org-b", b.person, b.singing), map[string]string{"level": "good"}) // manual-only task
	notFound("DELETE", task("org-b", b.person, b.sound), nil)                               // not chosen
	notFound("PUT", avail("org-b", b.person), map[string]any{"levels": map[string]string{fmt.Sprint(a.events[0]): "never"}})
	notFound("PUT", avail("org-b", a.person), map[string]any{"levels": map[string]string{fmt.Sprint(b.events[0]): "never"}})
	notFound("PUT", task("org-a", a.person, 99999), map[string]string{"level": "good"})
	notFound("PUT", "/api/orgs/org-a/plans/autumn-2026/people/abc/tasks/1", map[string]string{"level": "good"})

	for name, c := range map[string]struct {
		path string
		body any
	}{
		"bad task level":  {task("org-a", a.person, a.sound), map[string]string{"level": "kanskje"}},
		"bad event level": {avail("org-a", a.person), map[string]any{"levels": map[string]string{fmt.Sprint(a.events[0]): "neutral"}}},
		"bad event id":    {avail("org-a", a.person), map[string]any{"levels": map[string]string{"x": "never"}}},
		"no events":       {avail("org-a", a.person), map[string]any{"levels": map[string]string{}}},
	} {
		if r := e.do("PUT", c.path, c.body); r.code != 400 || r.errCode() != "invalid" {
			t.Errorf("%s: %d %s", name, r.code, r.body)
		}
	}
	// an event of another plan of the same organisation
	var spring []store.Event
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Spring"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/spring/events", map[string]any{"date": "2027-03-07"}).json(&spring)
	notFound("PUT", avail("org-a", a.person), map[string]any{"levels": map[string]string{fmt.Sprint(spring[0].ID): "never"}})
	// nothing was stored by any refused call
	if got := answersText(e, "org-a") + answersText(e, "org-b"); got != "" {
		t.Errorf("refused writes must change nothing: %q", got)
	}
}
