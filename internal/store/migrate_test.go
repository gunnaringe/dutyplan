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
