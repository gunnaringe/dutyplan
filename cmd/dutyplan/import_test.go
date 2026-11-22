package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

func TestTheImportCommand(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "d.db")
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateOrg(context.Background(), "Choir"); err != nil {
		t.Fatal(err)
	}
	st.Close()
	file := filepath.Join(dir, "in.json")
	os.WriteFile(file, []byte(`{"plan":"Spring 2031","tasks":["Sound"],"events":[{"date":"2031-01-12","start_time":"11:00","places":{"Sound":1}}]}`), 0o600)
	if code := importCmd([]string{"--db", db, "--org", "choir"}); code != 2 {
		t.Fatalf("no file: exit %d, want 2", code)
	}
	if code := importCmd([]string{"--db", db, "--org", "choir", "--dry-run", file}); code != 0 {
		t.Fatalf("dry run: exit %d", code)
	}
	if code := importCmd([]string{"--db", db, "--org", "choir", file}); code != 0 {
		t.Fatalf("import: exit %d", code)
	}
	if code := importCmd([]string{"--db", db, "--org", "choir", file}); code != 1 {
		t.Fatalf("the same plan again: exit %d, want 1", code)
	}
	os.WriteFile(file, []byte(`{"plan":"Other","colour":"red","events":[{"date":"2031-02-01"}]}`), 0o600)
	if code := importCmd([]string{"--db", db, "--org", "choir", file}); code != 1 {
		t.Fatalf("an unknown key: exit %d, want 1", code)
	}
	st, _ = store.Open(db)
	defer st.Close()
	if plans, _ := st.ListPlans(context.Background(), "choir"); len(plans) != 1 {
		t.Fatalf("plans: %+v (the dry run must store nothing)", plans)
	}
}
