package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// A database made before the schema was squashed into one file has migrations this binary does not know. Opening it must
// fail with a clear error rather than mix two schemas.
func TestADatabaseWithUnknownMigrationsIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
		INSERT INTO schema_migrations VALUES (1, 'then'), (12, 'then')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := Open(path); !errors.Is(err, ErrIncompatible) {
		if s != nil {
			s.Close()
		}
		t.Fatalf("want ErrIncompatible, got %v", err)
	}
}

// Migration 006 turned an event's description into a details row: a plan with descriptions gets the row «Description» at
// the top (another name when the plan has a row of that name), on one line per event, and the other rows move down.
func TestDescriptionsBecomeADetailsRowAtTheTop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	o := mustOrg(t, s, "Club")
	p, _ := s.CreatePlan(ctx, o.Slug, "Autumn 2026")
	taken, _ := s.CreatePlan(ctx, o.Slug, "Spring 2027")
	none, _ := s.CreatePlan(ctx, o.Slug, "Summer 2027")
	a, _ := s.AddEvent(ctx, o.Slug, p.Slug, "2026-08-09")
	b, _ := s.AddEvent(ctx, o.Slug, p.Slug, "2026-08-16")
	c, _ := s.AddEvent(ctx, o.Slug, taken.Slug, "2027-01-10")
	s.AddEvent(ctx, o.Slug, none.Slug, "2027-06-06")
	theme, _ := s.AddField(ctx, o.Slug, p.Slug, "Theme")
	if err := s.SetFieldValue(ctx, o.Slug, p.Slug, a.ID, theme.ID, "Harvest"); err != nil {
		t.Fatal(err)
	}
	s.AddField(ctx, o.Slug, taken.Slug, "Description")
	// back to how a database was before 006
	for _, q := range []struct {
		sql string
		id  int64
	}{
		{`ALTER TABLE event ADD COLUMN description TEXT NOT NULL DEFAULT ''`, 0},
		{`UPDATE event SET description = ' Doors at 9:30' || char(13) || char(10) || 'Bring chairs ' WHERE id = ?`, a.ID},
		{`UPDATE event SET description = '  ' WHERE id = ?`, b.ID},
		{`UPDATE event SET description = 'Guest choir' WHERE id = ?`, c.ID},
		{`DELETE FROM schema_migrations WHERE version = 6`, 0},
	} {
		args := []any{}
		if q.id != 0 {
			args = append(args, q.id)
		}
		if _, err := s.DB().Exec(q.sql, args...); err != nil {
			t.Fatal(q.sql, err)
		}
	}
	s.Close()
	if s, err = Open(path); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	got, _ := s.ListFields(ctx, o.Slug, p.Slug)
	if len(got.Fields) != 2 || got.Fields[0].Name != "Description" || got.Fields[0].Position != 0 || got.Fields[1].ID != theme.ID || got.Fields[1].Position != 1 {
		t.Fatalf("rows: %+v", got.Fields)
	}
	values := map[[2]int64]string{}
	for _, v := range got.Values {
		values[[2]int64{v.EventID, v.FieldID}] = v.Value
	}
	desc := got.Fields[0].ID
	if len(values) != 2 || values[[2]int64{a.ID, desc}] != "Doors at 9:30 Bring chairs" || values[[2]int64{a.ID, theme.ID}] != "Harvest" {
		t.Errorf("values (a blank description gives no value): %v", values)
	}
	if got, _ := s.ListFields(ctx, o.Slug, taken.Slug); len(got.Fields) != 2 || got.Fields[0].Name != "Event description" || len(got.Values) != 1 || got.Values[0].Value != "Guest choir" {
		t.Errorf("a plan with a row named Description: %+v", got)
	}
	if got, _ := s.ListFields(ctx, o.Slug, none.Slug); len(got.Fields) != 0 {
		t.Errorf("a plan without descriptions gets no row: %+v", got)
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('event') WHERE name = 'description'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("the column is gone: %d %v", n, err)
	}
}
