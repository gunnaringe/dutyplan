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

// Link levels. A view, answer or edit link is to one plan; LevelJoin is an invitation to the organisation: whoever opens it
// while signed in becomes a member. LevelAnswer sees the plan and sets the answers and notes of anyone on the list.
const (
	LevelView   = "view"
	LevelAnswer = "answer"
	LevelEdit   = "edit"
	LevelJoin   = "join"
)

// Link shares one plan (view, edit), or invites to the organisation (join, Plan empty). Token is the secret in its address (/s/<token>).
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
	SingleUse bool   `json:"single_use"` // an invitation for one person: it ends when someone joins with it
}

const linkColumns = `l.id, o.slug, o.name, COALESCE(p.slug, ''), COALESCE(p.name, ''), l.level, l.label, l.token, COALESCE(u.name, ''), l.created_at, COALESCE(l.expires_at, ''), l.single_use`
const linkFrom = ` FROM share_link l JOIN organization o ON o.id = l.org_id LEFT JOIN plan p ON p.id = l.plan_id LEFT JOIN app_user u ON u.id = l.created_by`

func scanLinks(rows *sql.Rows, err error) ([]Link, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.Org, &l.OrgName, &l.Plan, &l.PlanName, &l.Level, &l.Label, &l.Token, &l.CreatedBy, &l.CreatedAt, &l.ExpiresAt, &l.SingleUse); err != nil {
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

// CreateLink makes a view or edit link to one plan, or an invitation (join, planSlug ""). expiresDays 0: it never runs out.
// singleUse: an invitation that ends when someone becomes a member with it.
func (s *Store) CreateLink(ctx context.Context, orgSlug, planSlug, level, label string, expiresDays int, singleUse bool, userID int64) (Link, error) {
	if level != LevelView && level != LevelAnswer && level != LevelEdit && level != LevelJoin {
		return Link{}, Invalid("link_level", nil, "the level must be view, answer, edit or join")
	}
	if level == LevelJoin && planSlug != "" {
		return Link{}, Invalid("link_join_plan", nil, "an invitation is to the whole organisation")
	}
	if level != LevelJoin && planSlug == "" {
		return Link{}, Invalid("link_plan", nil, "a view, answer or edit link is to one plan")
	}
	if singleUse && level != LevelJoin {
		return Link{}, Invalid("link_single_use", nil, "only an invitation can be for one person")
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
		return tx.QueryRowContext(ctx, `INSERT INTO share_link (org_id, plan_id, token, level, label, created_by, created_at, expires_at, single_use) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			oid, pid, newToken(), level, label, by, s.stamp(), expires, singleUse).Scan(&id)
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
		JOIN organization o ON o.id = m.org_id WHERE o.slug = ? ORDER BY m.role, u.name COLLATE NOCASE, u.id`, orgSlug)
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

// changeMember sets a member's role, or removes them (role RoleNone). The organisation always keeps an admin.
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
		if old == RoleAdmin && role != RoleAdmin {
			var admins int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM membership WHERE org_id = ? AND role = 'admin'`, oid).Scan(&admins); err != nil {
				return err
			}
			if admins < 2 {
				return Conflict("last_admin", nil, "the organisation must keep an admin: make someone else admin first")
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

// SetRole makes a member admin or member.
func (s *Store) SetRole(ctx context.Context, orgSlug string, userID int64, role string) error {
	if role != RoleAdmin && role != RoleMember {
		return Invalid("role_invalid", nil, "the role must be admin or member")
	}
	return s.changeMember(ctx, orgSlug, userID, role)
}

// RemoveMember takes a person's access away (also when they leave themselves).
func (s *Store) RemoveMember(ctx context.Context, orgSlug string, userID int64) error {
	return s.changeMember(ctx, orgSlug, userID, RoleNone)
}

// Join makes the person a member of the organisation an invitation is to (a member already keeps their role), and
// returns that organisation; ErrNotFound when the token is no invitation or has run out. An invitation for one person
// ends when someone becomes a member with it (not when a member opens it: that uses nothing).
func (s *Store) Join(ctx context.Context, token string, userID int64) (Org, error) {
	var o Org
	err := s.write(ctx, func(tx *sql.Tx) error {
		var oid, lid int64
		var once bool
		err := tx.QueryRowContext(ctx, `SELECT o.id, o.slug, o.name, o.created_at, l.id, l.single_use FROM share_link l JOIN organization o ON o.id = l.org_id
			WHERE l.token = ? AND l.level = 'join' AND (l.expires_at IS NULL OR l.expires_at > ?)`, token, s.stamp()).Scan(&oid, &o.Slug, &o.Name, &o.CreatedAt, &lid, &once)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO membership (org_id, user_id, role, created_at) VALUES (?, ?, 'member', ?) ON CONFLICT DO NOTHING`, oid, userID, s.stamp())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 1 && once {
			_, err = tx.ExecContext(ctx, `DELETE FROM share_link WHERE id = ?`, lid)
		}
		return err
	})
	return o, err
}
