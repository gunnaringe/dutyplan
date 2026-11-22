package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
)

// User is a person who has signed in.
type User struct {
	ID    int64  `json:"-"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Roles of a member of an organisation. RoleNone: not a member.
const (
	RoleNone   = ""
	RoleOwner  = "owner"
	RoleEditor = "editor"
)

// SignedIn records a sign-in: the person (made the first time, name and email updated every time) and a new session.
func (s *Store) SignedIn(ctx context.Context, issuer, subject, name, email, tokenHash, idToken string, expires time.Time) (User, error) {
	if !validText(name) || len(name) > 200 {
		name = ""
	}
	if name == "" {
		name = email
	}
	if !validText(email) || len(email) > 320 {
		email = ""
	}
	u := User{Name: name, Email: email}
	now := s.stamp()
	err := s.write(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `INSERT INTO app_user (issuer, subject, name, email, created_at, seen_at) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (issuer, subject) DO UPDATE SET name = excluded.name, email = excluded.email, seen_at = excluded.seen_at
			RETURNING id`, issuer, subject, name, email, now, now).Scan(&u.ID)
		if err != nil {
			return err
		}
		// sessions that ran out go now and then: at every sign-in
		if _, err := tx.ExecContext(ctx, `DELETE FROM session WHERE expires_at < ?`, now); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO session (token_hash, user_id, id_token, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
			tokenHash, u.ID, idToken, now, expires.UTC().Format(time.RFC3339))
		return err
	})
	return u, err
}

// SessionUser is the person a session belongs to; ErrNotFound when there is no such session or it has run out.
func (s *Store) SessionUser(ctx context.Context, tokenHash string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT u.id, u.name, u.email FROM session s JOIN app_user u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, s.stamp()).Scan(&u.ID, &u.Name, &u.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// EndSession removes a session and returns its ID token ("" when there was none).
func (s *Store) EndSession(ctx context.Context, tokenHash string) (idToken string, err error) {
	err = s.write(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `DELETE FROM session WHERE token_hash = ? RETURNING id_token`, tokenHash).Scan(&idToken)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	})
	return idToken, err
}

// Role is what a person is in an organisation: RoleOwner, RoleEditor, or RoleNone (also when there is no such organisation).
func (s *Store) Role(ctx context.Context, orgSlug string, userID int64) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx, `SELECT m.role FROM membership m JOIN organization o ON o.id = m.org_id WHERE o.slug = ? AND m.user_id = ?`,
		orgSlug, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return RoleNone, nil
	}
	return role, err
}

// ListOrgsOf lists the organisations a person is a member of.
func (s *Store) ListOrgsOf(ctx context.Context, userID int64) ([]Org, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT o.slug, o.name, o.created_at FROM organization o JOIN membership m ON m.org_id = o.id
		WHERE m.user_id = ? ORDER BY o.name COLLATE NOCASE, o.slug`, userID)
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

// CreateOrgOwnedBy makes an organisation with the person as its owner. Its address comes from the name; when another
// organisation has that address already, a number is added («choir-2»), so nobody learns which names are taken.
func (s *Store) CreateOrgOwnedBy(ctx context.Context, name string, userID int64) (Org, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Org{}, err
	}
	base, err := slugFor(name)
	if err != nil {
		return Org{}, err
	}
	o := Org{Name: name, CreatedAt: s.stamp()}
	err = s.write(ctx, func(tx *sql.Tx) error {
		if err := atMost(ctx, tx, MaxOwnedOrgs, "orgs", `SELECT COUNT(*) FROM membership WHERE user_id = ? AND role = 'owner'`, userID); err != nil {
			return err
		}
		var id int64
		for n := 1; ; n++ {
			o.Slug = base
			if n > 1 {
				o.Slug = base + "-" + strconv.Itoa(n)
			}
			err := tx.QueryRowContext(ctx, `INSERT INTO organization (slug, name, created_at) VALUES (?, ?, ?) RETURNING id`, o.Slug, o.Name, o.CreatedAt).Scan(&id)
			if isUnique(err) && n < 1000 {
				continue
			}
			if err != nil {
				return err
			}
			break
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO membership (org_id, user_id, role, created_at) VALUES (?, ?, ?, ?)`, id, userID, RoleOwner, o.CreatedAt)
		return err
	})
	return o, err
}
