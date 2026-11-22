package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

// Link levels. LevelJoin is an invitation: whoever opens it while signed in becomes an editor of the organisation.
const (
	LevelView = "view"
	LevelEdit = "edit"
	LevelJoin = "join"
)

// Link shares an organisation, or one plan of it (Plan set). Token is the secret in its address (/s/<token>).
type Link struct {
	ID        int64  `json:"id"`
	Org       string `json:"org"`
	OrgName   string `json:"org_name"`
	Plan      string `json:"plan,omitempty"`
	PlanName  string `json:"plan_name,omitempty"`
	Level     string `json:"level"`
	Label     string `json:"label"`
	Token     string `json:"token,omitempty"`
	CreatedBy string `json:"created_by,omitempty"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

const linkColumns = `l.id, o.slug, o.name, COALESCE(p.slug, ''), COALESCE(p.name, ''), l.level, l.label, l.token, COALESCE(u.name, ''), l.created_at, COALESCE(l.expires_at, '')`
const linkFrom = ` FROM share_link l JOIN organization o ON o.id = l.org_id LEFT JOIN plan p ON p.id = l.plan_id LEFT JOIN app_user u ON u.id = l.created_by`

func scanLinks(rows *sql.Rows, err error) ([]Link, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.Org, &l.OrgName, &l.Plan, &l.PlanName, &l.Level, &l.Label, &l.Token, &l.CreatedBy, &l.CreatedAt, &l.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func newToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// CreateLink makes a link to the organisation, or to one of its plans (planSlug not ""). expiresDays 0: it never runs out.
func (s *Store) CreateLink(ctx context.Context, orgSlug, planSlug, level, label string, expiresDays int, userID int64) (Link, error) {
	if level != LevelView && level != LevelEdit && level != LevelJoin {
		return Link{}, Invalid("link_level", nil, "the level must be view, edit or join")
	}
	if level == LevelJoin && planSlug != "" {
		return Link{}, Invalid("link_join_plan", nil, "an invitation is to the whole organisation")
	}
	label = strings.TrimSpace(label)
	if len([]rune(label)) > 80 || !validText(label) {
		return Link{}, Invalid("name_invalid", map[string]any{"what": "label"}, "the label must be at most 80 printable characters")
	}
	if expiresDays < 0 || expiresDays > 366 {
		return Link{}, Invalid("link_expiry", nil, "a link runs out after 1 to 366 days, or never (0)")
	}
	var expires any
	if expiresDays > 0 {
		expires = s.now().Add(time.Duration(expiresDays) * 24 * time.Hour).Format(time.RFC3339)
	}
	var id int64
	err := s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		if err := atMost(ctx, tx, MaxLinksPerOrg, "links", `SELECT COUNT(*) FROM share_link WHERE org_id = ?`, oid); err != nil {
			return err
		}
		var pid any
		if planSlug != "" {
			var p int64
			if err := tx.QueryRowContext(ctx, `SELECT id FROM plan WHERE org_id = ? AND slug = ?`, oid, planSlug).Scan(&p); errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			} else if err != nil {
				return err
			}
			pid = p
		}
		var by any
		if userID != 0 {
			by = userID
		}
		return tx.QueryRowContext(ctx, `INSERT INTO share_link (org_id, plan_id, token, level, label, created_by, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			oid, pid, newToken(), level, label, by, s.stamp(), expires).Scan(&id)
	})
	if err != nil {
		return Link{}, err
	}
	links, err := scanLinks(s.db.QueryContext(ctx, `SELECT `+linkColumns+linkFrom+` WHERE l.id = ?`, id))
	if err != nil || len(links) == 0 {
		return Link{}, err
	}
	return links[0], nil
}

// ListLinks lists an organisation's links, also those that have run out (shown as such, so they can be deleted).
func (s *Store) ListLinks(ctx context.Context, orgSlug string) ([]Link, error) {
	return scanLinks(s.db.QueryContext(ctx, `SELECT `+linkColumns+linkFrom+` WHERE o.slug = ? ORDER BY l.created_at DESC, l.id DESC`, orgSlug))
}

// DeleteLink ends a link at once.
func (s *Store) DeleteLink(ctx context.Context, orgSlug string, id int64) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM share_link WHERE id = ? AND org_id = (SELECT id FROM organization WHERE slug = ?)`, id, orgSlug)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// LinksByTokens are the links that the tokens open and that have not run out (unknown tokens are left out).
func (s *Store) LinksByTokens(ctx context.Context, tokens []string) ([]Link, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	args := []any{s.stamp()}
	for _, t := range tokens {
		args = append(args, t)
	}
	q := `SELECT ` + linkColumns + linkFrom + ` WHERE (l.expires_at IS NULL OR l.expires_at > ?) AND l.token IN (?` + strings.Repeat(", ?", len(tokens)-1) + `)`
	return scanLinks(s.db.QueryContext(ctx, q, args...))
}

// ---- members --------------------------------------------------------------------------------------------------

// Member is a person with access to an organisation.
type Member struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Role  string `json:"role"`
}

func (s *Store) Members(ctx context.Context, orgSlug string) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.id, u.name, u.email, m.role FROM membership m JOIN app_user u ON u.id = m.user_id
		JOIN organization o ON o.id = m.org_id WHERE o.slug = ? ORDER BY m.role DESC, u.name COLLATE NOCASE, u.id`, orgSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// changeMember sets a member's role, or removes them (role RoleNone). The organisation always keeps an owner.
func (s *Store) changeMember(ctx context.Context, orgSlug string, userID int64, role string) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		var old string
		if err := tx.QueryRowContext(ctx, `SELECT role FROM membership WHERE org_id = ? AND user_id = ?`, oid, userID).Scan(&old); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if old == RoleOwner && role != RoleOwner {
			var owners int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM membership WHERE org_id = ? AND role = 'owner'`, oid).Scan(&owners); err != nil {
				return err
			}
			if owners < 2 {
				return Conflict("last_owner", nil, "the organisation must keep an owner: make someone else owner first")
			}
		}
		if role == RoleNone {
			_, err = tx.ExecContext(ctx, `DELETE FROM membership WHERE org_id = ? AND user_id = ?`, oid, userID)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE membership SET role = ? WHERE org_id = ? AND user_id = ?`, role, oid, userID)
		}
		return err
	})
}

// SetRole makes a member owner or editor.
func (s *Store) SetRole(ctx context.Context, orgSlug string, userID int64, role string) error {
	if role != RoleOwner && role != RoleEditor {
		return Invalid("role_invalid", nil, "the role must be owner or editor")
	}
	return s.changeMember(ctx, orgSlug, userID, role)
}

// RemoveMember takes a person's access away (also when they leave themselves).
func (s *Store) RemoveMember(ctx context.Context, orgSlug string, userID int64) error {
	return s.changeMember(ctx, orgSlug, userID, RoleNone)
}

// Join makes the person an editor of the organisation an invitation is to (a member already keeps their role), and
// returns that organisation; ErrNotFound when the token is no invitation or has run out.
func (s *Store) Join(ctx context.Context, token string, userID int64) (Org, error) {
	var o Org
	err := s.write(ctx, func(tx *sql.Tx) error {
		var oid int64
		err := tx.QueryRowContext(ctx, `SELECT o.id, o.slug, o.name, o.created_at FROM share_link l JOIN organization o ON o.id = l.org_id
			WHERE l.token = ? AND l.level = 'join' AND (l.expires_at IS NULL OR l.expires_at > ?)`, token, s.stamp()).Scan(&oid, &o.Slug, &o.Name, &o.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO membership (org_id, user_id, role, created_at) VALUES (?, ?, 'editor', ?) ON CONFLICT DO NOTHING`, oid, userID, s.stamp())
		return err
	})
	return o, err
}
