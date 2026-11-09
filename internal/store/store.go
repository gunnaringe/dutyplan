// Package store holds everything the app persists: SQLite with embedded, forward-only migrations.
//
// Tenancy: every row belongs to an organisation, and every lookup is scoped by the organisation's slug.
// A plan or person from another organisation is reported as ErrNotFound, never as a different error.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	_ "modernc.org/sqlite" // pure-Go driver: single static binary, no cgo
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
	ErrInvalid  = errors.New("invalid")
	ErrActor    = errors.New("actor required")
)

// Coded is an error of one of the kinds above that also carries a stable code and parameters, so the web page can write
// the message in the user's language. Msg is the English text, which is what everything else shows.
type Coded struct {
	Kind   error
	Code   string
	Params map[string]any
	Msg    string
}

func (e *Coded) Error() string { return e.Kind.Error() + ": " + e.Msg }
func (e *Coded) Unwrap() error { return e.Kind }

// Invalid is a refused request (400) with a code, parameters (may be nil) and an English message.
func Invalid(code string, params map[string]any, format string, args ...any) error {
	return &Coded{ErrInvalid, code, params, fmt.Sprintf(format, args...)}
}

// Conflict is a request that clashes with what exists (409).
func Conflict(code string, params map[string]any, format string, args ...any) error {
	return &Coded{ErrConflict, code, params, fmt.Sprintf(format, args...)}
}

func exists(kind, name string) error {
	return Conflict("already_exists", map[string]any{"kind": kind, "name": name}, "%s %q", kind, name)
}

func inUse(kind, name, by string) error {
	return Conflict("in_use", map[string]any{"kind": kind, "name": name}, "%s %q is used by %s; remove it there first", kind, name, by)
}

var errNothingToChange = func() error { return Invalid("nothing_to_change", nil, "nothing to change") }

// Store is safe for concurrent use. It uses a single connection: simple, and plenty for this app.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// Open opens (and creates) the database at path and applies pending migrations.
// Foreign keys are SQLite-off by default on every connection, so they are switched on in the DSN.
func Open(path string) (*Store, error) {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, now: func() time.Time { return time.Now().UTC() }}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// DB exposes the handle for tests that need to look underneath the API.
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("migration %q: name must start with a number", name)
		}
		var done int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&done); err != nil {
			return err
		}
		if done > 0 {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, s.stamp()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) stamp() string { return s.now().Format(time.RFC3339) }

// logEntry is what a write reports about itself; the helper stores it in the same transaction.
type logEntry struct {
	orgID  int64
	planID *int64
	action string
	detail string
}

// write is the only way to change data. It validates the actor, runs fn and records the change in change_log,
// all in one transaction: a failed change leaves no log line, and a change never lacks one.
func (s *Store) write(ctx context.Context, actor string, fn func(tx *sql.Tx) (logEntry, error)) error {
	actor, err := cleanActor(actor)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	e, err := fn(tx)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO change_log (org_id, plan_id, actor, action, detail, at) VALUES (?, ?, ?, ?, ?, ?)`,
		e.orgID, e.planID, actor, e.action, e.detail, s.stamp()); err != nil {
		return err
	}
	return tx.Commit()
}

// cleanActor accepts the self-declared name as sent by the client: percent-encoded UTF-8 (HTTP headers are not
// safely UTF-8), 1-80 characters, no control characters.
func cleanActor(raw string) (string, error) {
	dec, err := url.PathUnescape(raw)
	if err != nil {
		return "", fmt.Errorf("%w: actor is not valid percent-encoding", ErrActor)
	}
	dec = strings.TrimSpace(dec)
	if dec == "" {
		return "", ErrActor
	}
	if utf8.RuneCountInString(dec) > 80 || !validText(dec) {
		return "", fmt.Errorf("%w: actor must be 1-80 printable characters", ErrActor)
	}
	return dec, nil
}

func validText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func cleanName(raw, what string) (string, error) {
	n := strings.TrimSpace(raw)
	if n == "" || utf8.RuneCountInString(n) > 80 || !validText(n) {
		return "", Invalid("name_invalid", map[string]any{"what": what}, "%s must be 1-80 printable characters", what)
	}
	return n, nil
}

var slugTranslit = strings.NewReplacer("æ", "ae", "ø", "o", "å", "a", "ä", "a", "ö", "o", "ü", "u", "é", "e", "è", "e", "ð", "d", "þ", "th")

// Slugify makes a URL-safe slug: «Høst 2026» becomes «host-2026».
func Slugify(name string) string {
	var b strings.Builder
	dash := true
	for _, r := range slugTranslit.Replace(strings.ToLower(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	return s
}

func cleanSlug(slug, name string) (string, error) {
	if slug == "" {
		slug = Slugify(name)
	}
	if len(slug) < 2 || len(slug) > 40 || slug != Slugify(slug) {
		return "", Invalid("slug_invalid", nil, "slug must be 2-40 characters of a-z, 0-9 and hyphens")
	}
	return slug, nil
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// ---- organisations -------------------------------------------------------------------------------------------

type Org struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) CreateOrg(ctx context.Context, actor, name, slug string) (Org, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Org{}, err
	}
	if slug, err = cleanSlug(slug, name); err != nil {
		return Org{}, err
	}
	o := Org{Slug: slug, Name: name, CreatedAt: s.stamp()}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		res, err := tx.ExecContext(ctx, `INSERT INTO organization (slug, name, created_at) VALUES (?, ?, ?)`, o.Slug, o.Name, o.CreatedAt)
		if isUnique(err) {
			return logEntry{}, exists("organisation", o.Slug)
		}
		if err != nil {
			return logEntry{}, err
		}
		id, _ := res.LastInsertId()
		return logEntry{orgID: id, action: "org.create", detail: o.Name}, nil
	})
	return o, err
}

func (s *Store) ListOrgs(ctx context.Context) ([]Org, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT slug, name, created_at FROM organization ORDER BY name COLLATE NOCASE, slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Org{}
	for rows.Next() {
		var o Org
		if err := rows.Scan(&o.Slug, &o.Name, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) GetOrg(ctx context.Context, slug string) (Org, error) {
	var o Org
	err := s.db.QueryRowContext(ctx, `SELECT slug, name, created_at FROM organization WHERE slug = ?`, slug).Scan(&o.Slug, &o.Name, &o.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Org{}, ErrNotFound
	}
	return o, err
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// orgID resolves a slug. Every scoped lookup starts here.
func orgID(ctx context.Context, q queryer, slug string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM organization WHERE slug = ?`, slug).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// ---- plans -------------------------------------------------------------------------------------------------

type Plan struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) CreatePlan(ctx context.Context, orgSlug, actor, name, slug string) (Plan, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Plan{}, err
	}
	if slug, err = cleanSlug(slug, name); err != nil {
		return Plan{}, err
	}
	p := Plan{Slug: slug, Name: name, CreatedAt: s.stamp()}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO plan (org_id, slug, name, created_at) VALUES (?, ?, ?, ?)`, oid, p.Slug, p.Name, p.CreatedAt)
		if isUnique(err) {
			return logEntry{}, exists("plan", p.Slug)
		}
		if err != nil {
			return logEntry{}, err
		}
		pid, _ := res.LastInsertId()
		return logEntry{orgID: oid, planID: &pid, action: "plan.create", detail: p.Name}, nil
	})
	return p, err
}

func (s *Store) ListPlans(ctx context.Context, orgSlug string) ([]Plan, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT slug, name, created_at FROM plan WHERE org_id = ? ORDER BY id DESC`, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Plan{}
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.Slug, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPlan(ctx context.Context, orgSlug, planSlug string) (Plan, error) {
	var p Plan
	err := s.db.QueryRowContext(ctx,
		`SELECT p.slug, p.name, p.created_at FROM plan p JOIN organization o ON o.id = p.org_id WHERE o.slug = ? AND p.slug = ?`,
		orgSlug, planSlug).Scan(&p.Slug, &p.Name, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Plan{}, ErrNotFound
	}
	return p, err
}

// ---- people ------------------------------------------------------------------------------------------------

type Person struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Active     bool   `json:"active"`
	Proposable bool   `json:"proposable"`
}

func (s *Store) CreatePerson(ctx context.Context, orgSlug, actor, name string) (Person, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Person{}, err
	}
	p := Person{Name: name, Active: true, Proposable: true}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO person (org_id, name, created_at) VALUES (?, ?, ?)`, oid, p.Name, s.stamp())
		if isUnique(err) {
			return logEntry{}, exists("person", p.Name)
		}
		if err != nil {
			return logEntry{}, err
		}
		p.ID, _ = res.LastInsertId()
		return logEntry{orgID: oid, action: "person.create", detail: p.Name}, nil
	})
	return p, err
}

func (s *Store) ListPeople(ctx context.Context, orgSlug string) ([]Person, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, active, proposable FROM person WHERE org_id = ? ORDER BY name COLLATE NOCASE`, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Person{}
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name, &p.Active, &p.Proposable); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PersonPatch changes only the fields that are set.
type PersonPatch struct {
	Name       *string `json:"name"`
	Active     *bool   `json:"active"`
	Proposable *bool   `json:"proposable"`
}

func (s *Store) UpdatePerson(ctx context.Context, orgSlug, actor string, id int64, patch PersonPatch) (Person, error) {
	var p Person
	if patch.Name != nil {
		n, err := cleanName(*patch.Name, "name")
		if err != nil {
			return p, err
		}
		patch.Name = &n
	}
	err := s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		// The org_id in every statement is the tenancy guard: another organisation's person is simply not found.
		err = tx.QueryRowContext(ctx, `SELECT id, name, active, proposable FROM person WHERE id = ? AND org_id = ?`, id, oid).
			Scan(&p.ID, &p.Name, &p.Active, &p.Proposable)
		if errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		}
		if err != nil {
			return logEntry{}, err
		}
		var changes []string
		if patch.Name != nil && *patch.Name != p.Name {
			changes = append(changes, fmt.Sprintf("name %q -> %q", p.Name, *patch.Name))
			p.Name = *patch.Name
		}
		if patch.Active != nil && *patch.Active != p.Active {
			changes = append(changes, fmt.Sprintf("active %t -> %t", p.Active, *patch.Active))
			p.Active = *patch.Active
		}
		if patch.Proposable != nil && *patch.Proposable != p.Proposable {
			changes = append(changes, fmt.Sprintf("proposable %t -> %t", p.Proposable, *patch.Proposable))
			p.Proposable = *patch.Proposable
		}
		if len(changes) == 0 {
			return logEntry{}, errNothingToChange()
		}
		_, err = tx.ExecContext(ctx, `UPDATE person SET name = ?, active = ?, proposable = ? WHERE id = ? AND org_id = ?`,
			p.Name, p.Active, p.Proposable, id, oid)
		if isUnique(err) {
			return logEntry{}, exists("person", p.Name)
		}
		if err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "person.update", detail: fmt.Sprintf("#%d: %s", id, strings.Join(changes, "; "))}, nil
	})
	return p, err
}

// ---- change log --------------------------------------------------------------------------------------------

type LogEntry struct {
	ID     int64  `json:"id"`
	Plan   string `json:"plan,omitempty"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Detail string `json:"detail"`
	At     string `json:"at"`
}

// ListLog returns the newest entries of one organisation, optionally for one plan only.
func (s *Store) ListLog(ctx context.Context, orgSlug, planSlug string, limit int) ([]LogEntry, error) {
	if limit < 1 || limit > 500 {
		limit = 50
	}
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	q := `SELECT l.id, COALESCE(p.slug, ''), l.actor, l.action, l.detail, l.at
	      FROM change_log l LEFT JOIN plan p ON p.id = l.plan_id WHERE l.org_id = ?`
	args := []any{oid}
	if planSlug != "" {
		q += ` AND p.slug = ?`
		args = append(args, planSlug)
	}
	q += ` ORDER BY l.id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogEntry{}
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.Plan, &e.Actor, &e.Action, &e.Detail, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
