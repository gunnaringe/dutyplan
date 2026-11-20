package store

import (
	"errors"
	"strings"
	"testing"
)

type sectionWorld struct {
	s                 *Store
	org, other        string
	people            map[string]int64
	away, awaySection int64
}

// newSectionWorld: Club with the people Aslaug, Birger, Cecilie, Dina (added in that order, not by name), and Elsewhere
// with Embla and a section of its own.
func newSectionWorld(t *testing.T) sectionWorld {
	t.Helper()
	s := open(t)
	w := sectionWorld{s: s, org: mustOrg(t, s, "Club").Slug, other: mustOrg(t, s, "Elsewhere").Slug, people: map[string]int64{}}
	for _, n := range []string{"Dina", "Birger", "Aslaug", "Cecilie"} {
		p, err := s.CreatePerson(ctx, w.org, n)
		if err != nil {
			t.Fatal(err)
		}
		w.people[n] = p.ID
	}
	p, err := s.CreatePerson(ctx, w.other, "Embla")
	if err != nil {
		t.Fatal(err)
	}
	w.away = p.ID
	x, err := s.CreateSection(ctx, w.other, "Away")
	if err != nil {
		t.Fatal(err)
	}
	w.awaySection = x.ID
	return w
}

// table is the order of Availability: names, with «| Section:» before each section's people.
func (w sectionWorld) table(t *testing.T, org string) string {
	t.Helper()
	people, err := w.s.ListPeople(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	sections, err := w.s.ListSections(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	name := map[int64]string{}
	for _, x := range sections {
		name[x.ID] = x.Name
	}
	var out []string
	var cur int64
	for _, p := range people {
		if p.Section != cur {
			cur = p.Section
			out = append(out, "| "+name[cur]+":")
		}
		out = append(out, p.Name)
	}
	return strings.Join(out, " ")
}

func (w sectionWorld) section(t *testing.T, name string) int64 {
	t.Helper()
	x, err := w.s.CreateSection(ctx, w.org, name)
	if err != nil {
		t.Fatal(err)
	}
	return x.ID
}

func TestPeopleAreInTheOrderTheyWereAddedAndCanBeDragged(t *testing.T) {
	w := newSectionWorld(t)
	s, org := w.s, w.org
	if got := w.table(t, org); got != "Dina Birger Aslaug Cecilie" {
		t.Fatalf("new people go last, not by name: %s", got)
	}
	if err := s.MovePerson(ctx, org, w.people["Cecilie"], 0, 0); err != nil || w.table(t, org) != "Cecilie Dina Birger Aslaug" {
		t.Errorf("to the top: %s %v", w.table(t, org), err)
	}
	if err := s.MovePerson(ctx, org, w.people["Cecilie"], 0, 99); err != nil || w.table(t, org) != "Dina Birger Aslaug Cecilie" {
		t.Errorf("past the end is the bottom: %s %v", w.table(t, org), err)
	}
	if err := s.MovePerson(ctx, org, w.people["Dina"], 0, 2); err != nil || w.table(t, org) != "Birger Aslaug Dina Cecilie" {
		t.Errorf("two down: %s %v", w.table(t, org), err)
	}
	if _, err := s.CreatePerson(ctx, org, "Frode"); err != nil || w.table(t, org) != "Birger Aslaug Dina Cecilie Frode" {
		t.Errorf("a new person goes last: %s %v", w.table(t, org), err)
	}
	if err := s.MovePerson(ctx, org, w.people["Dina"], 0, -1); !codeIs(err, "position_invalid") {
		t.Errorf("a negative position: %v", err)
	}
}

func TestSectionsPlacePeopleInTheTable(t *testing.T) {
	w := newSectionWorld(t)
	s, org := w.s, w.org
	off := w.section(t, "Not this term")
	if got := w.table(t, org); got != "Dina Birger Aslaug Cecilie" {
		t.Fatalf("a new section is empty: %s", got)
	}
	if err := s.MovePerson(ctx, org, w.people["Birger"], off, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.MovePerson(ctx, org, w.people["Dina"], off, 0); err != nil {
		t.Fatal(err)
	}
	if got := w.table(t, org); got != "Aslaug Cecilie | Not this term: Dina Birger" {
		t.Errorf("people without a section first, then the section: %s", got)
	}
	frode, err := s.CreatePerson(ctx, org, "Frode")
	if err != nil || w.table(t, org) != "Aslaug Cecilie Frode | Not this term: Dina Birger" {
		t.Errorf("a new person has no section and goes last of those: %s %v", w.table(t, org), err)
	}
	if err := s.MovePerson(ctx, org, w.people["Aslaug"], 0, 1); err != nil || w.table(t, org) != "Cecilie Aslaug Frode | Not this term: Dina Birger" {
		t.Errorf("the gaps where people left are closed: %s %v", w.table(t, org), err)
	}
	guests := w.section(t, "Guests")
	if err := s.MovePerson(ctx, org, w.people["Cecilie"], guests, 0); err != nil || w.table(t, org) != "Aslaug Frode | Not this term: Dina Birger | Guests: Cecilie" {
		t.Errorf("sections in their order: %s %v", w.table(t, org), err)
	}
	if err := s.MoveSection(ctx, org, guests, 0); err != nil || w.table(t, org) != "Aslaug Frode | Guests: Cecilie | Not this term: Dina Birger" {
		t.Errorf("a section moved first: %s %v", w.table(t, org), err)
	}
	if err := s.MoveSection(ctx, org, guests, -1); !codeIs(err, "position_invalid") {
		t.Errorf("a negative position: %v", err)
	}
	if err := s.MovePerson(ctx, org, w.people["Birger"], off, 0); err != nil || w.table(t, org) != "Aslaug Frode | Guests: Cecilie | Not this term: Birger Dina" {
		t.Errorf("within a section: %s %v", w.table(t, org), err)
	}
	if err := s.MovePerson(ctx, org, w.people["Birger"], 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.MovePerson(ctx, org, w.people["Cecilie"], off, 0); err != nil {
		t.Fatal(err)
	}
	if got := w.table(t, org); got != "Birger Aslaug Frode | Not this term: Cecilie Dina" {
		t.Errorf("back to no section, and an empty section shows no people: %s", got)
	}
	if err := s.DeleteSection(ctx, org, off); err != nil || w.table(t, org) != "Birger Aslaug Frode Cecilie Dina" {
		t.Errorf("deleting a section puts its people last of those without one, in their order: %s %v", w.table(t, org), err)
	}
	if got, _ := s.ListSections(ctx, org); len(got) != 1 || got[0].ID != guests {
		t.Errorf("the deleted section is gone: %+v", got)
	}
	if err := s.MovePerson(ctx, org, frode.ID, 0, 0); err != nil || w.table(t, org) != "Frode Birger Aslaug Cecilie Dina" {
		t.Errorf("no gap after the delete: %s %v", w.table(t, org), err)
	}
	if err := s.MoveSection(ctx, org, w.section(t, "Third"), 0); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ListSections(ctx, org); len(got) != 2 || got[0].Name != "Third" || got[1].Name != "Guests" {
		t.Errorf("sections after a delete keep no gap: %+v", got)
	}
}

func TestSectionNames(t *testing.T) {
	w := newSectionWorld(t)
	s, org := w.s, w.org
	a := w.section(t, "Guests")
	youth := w.section(t, "Youth")
	if err := s.MoveSection(ctx, org, youth, 0); err != nil {
		t.Fatal(err)
	}
	zed := w.section(t, "Zed")
	if got, _ := s.ListSections(ctx, org); len(got) != 3 || got[2].ID != zed {
		t.Errorf("a new section goes last: %+v", got)
	}
	if err := s.DeleteSection(ctx, org, zed); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSection(ctx, org, "Guests"); !codeIs(err, "already_exists") {
		t.Errorf("the same name twice: %v", err)
	}
	if _, err := s.CreateSection(ctx, org, "  "); !codeIs(err, "name_invalid") {
		t.Errorf("an empty name: %v", err)
	}
	if x, err := s.RenameSection(ctx, org, a, " Visitors "); err != nil || x.Name != "Visitors" {
		t.Errorf("rename: %+v %v", x, err)
	}
	if _, err := s.RenameSection(ctx, org, a, "Visitors"); !codeIs(err, "nothing_to_change") {
		t.Errorf("the same name: %v", err)
	}
	if _, err := s.RenameSection(ctx, org, a, "Youth"); !codeIs(err, "already_exists") {
		t.Errorf("another section's name: %v", err)
	}
	if _, err := s.CreateSection(ctx, w.other, "Youth"); err != nil {
		t.Errorf("another organisation may use the name: %v", err)
	}
	for i := 2; i < maxSections; i++ {
		if _, err := s.CreateSection(ctx, org, "S"+strings.Repeat("x", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateSection(ctx, org, "One more"); !codeIs(err, "too_many_sections") {
		t.Errorf("want too_many_sections, got %v", err)
	}
}

func TestSectionsStayInTheirOrganisation(t *testing.T) {
	w := newSectionWorld(t)
	s, org := w.s, w.org
	mine := w.section(t, "Guests")
	for name, err := range map[string]error{
		"rename another org's section":    func() error { _, err := s.RenameSection(ctx, org, w.awaySection, "X"); return err }(),
		"move another org's section":      s.MoveSection(ctx, org, w.awaySection, 0),
		"delete another org's section":    s.DeleteSection(ctx, org, w.awaySection),
		"own section through another org": s.DeleteSection(ctx, w.other, mine),
		"a person into another org's":     s.MovePerson(ctx, org, w.people["Dina"], w.awaySection, 0),
		"another org's person":            s.MovePerson(ctx, org, w.away, mine, 0),
		"own person through another org":  s.MovePerson(ctx, w.other, w.people["Dina"], 0, 0),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: want not found, got %v", name, err)
		}
	}
	if got, _ := s.ListSections(ctx, org); len(got) != 1 || got[0].ID != mine {
		t.Errorf("only its own sections: %+v", got)
	}
	if got := w.table(t, w.other); got != "Embla" {
		t.Errorf("the other organisation is untouched: %s", got)
	}
	if got := w.table(t, org); got != "Dina Birger Aslaug Cecilie" {
		t.Errorf("this organisation is untouched: %s", got)
	}
	// Moving Embla in her own organisation does not reorder this one's people without a section.
	if err := s.MovePerson(ctx, w.other, w.away, 0, 0); err != nil || w.table(t, org) != "Dina Birger Aslaug Cecilie" {
		t.Errorf("each organisation has its own order: %s %v", w.table(t, org), err)
	}
	if _, err := s.DB().Exec(`UPDATE person SET section_id = ? WHERE id = ?`, w.awaySection, w.people["Dina"]); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Errorf("a person in another organisation's section must fail at the foreign key: %v", err)
	}
}

func TestDeletedSectionsLeaveNoGap(t *testing.T) {
	w := newSectionWorld(t)
	s, org := w.s, w.org
	a, b := w.section(t, "A"), w.section(t, "B")
	w.section(t, "C")
	if err := s.DeleteSection(ctx, org, a); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSection(ctx, org, b); err != nil {
		t.Fatal(err)
	}
	w.section(t, "D")
	if got, _ := s.ListSections(ctx, org); len(got) != 2 || got[0].Name != "C" || got[1].Name != "D" {
		t.Errorf("a new section after two deletes goes last: %+v", got)
	}
}
