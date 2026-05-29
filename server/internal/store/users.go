package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// User is a principal row (ADR 0006). The principal key is ID (a ULID), never
// the OIDC subject. A row may carry Subject (can log in via OIDC), a password
// (HasPassword true — can log in locally), both (linked), or neither (invited:
// can hold grants/memberships but cannot log in until a credential is set).
//
// The argon2id password hash is never exposed on this struct; use
// CredentialsByEmail to fetch it for verification.
type User struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	DisplayName   string     `json:"display_name,omitempty"`
	Subject       string     `json:"subject,omitempty"` // "" when local-only
	HasPassword   bool       `json:"has_password"`
	IsServerAdmin bool       `json:"is_server_admin"`
	Disabled      bool       `json:"disabled"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CreateUserParams holds the fields needed to insert a users row. Subject and
// PasswordHash are optional: leave both empty to create an "invited" user.
type CreateUserParams struct {
	Email         string
	DisplayName   string
	Subject       string // OIDC sub; empty leaves the column NULL
	PasswordHash  string // argon2id; empty leaves the column NULL
	IsServerAdmin bool
}

// UpdateUserParams holds the mutable fields for a user PATCH. Pointer fields
// are only written when non-nil so callers can patch individual attributes.
type UpdateUserParams struct {
	DisplayName   *string
	Disabled      *bool
	IsServerAdmin *bool
}

// ListUsersParams controls pagination for ListUsers.
type ListUsersParams struct {
	Limit  int32
	Cursor string
}

// normalizeEmail lower-cases and trims an email so lookups and the UNIQUE
// constraint are case-insensitive. The users table stores the normalised form.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (db *DB) scanUser(row pgx.Row) (*User, error) {
	var u User
	var subject, displayName *string
	var hasPassword bool
	var lastSeen *time.Time
	if err := row.Scan(&u.ID, &u.Email, &displayName, &subject, &hasPassword,
		&u.IsServerAdmin, &u.Disabled, &lastSeen, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	if displayName != nil {
		u.DisplayName = *displayName
	}
	if subject != nil {
		u.Subject = *subject
	}
	u.HasPassword = hasPassword
	u.LastSeenAt = lastSeen
	return &u, nil
}

// userColumns is table-qualified so it is safe to embed in a JOIN (e.g.
// ListGroupMembers, where group_members also has created_at). The qualifier is
// equally valid in single-table SELECTs and in RETURNING on the users table.
const userColumns = `users.id, users.email, users.display_name, users.subject,
	(users.password_hash IS NOT NULL) AS has_password,
	users.is_server_admin, users.disabled, users.last_seen_at,
	users.created_at, users.updated_at`

// GetUserByID returns a single user by ULID. Returns ErrNotFound if absent.
func (db *DB) GetUserByID(ctx context.Context, id string) (*User, error) {
	ctx, span := startSpan(ctx, "GetUserByID")
	defer span.End()

	u, err := db.scanUser(db.Pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return u, nil
}

// GetUserBySubject resolves a federated user by OIDC subject.
func (db *DB) GetUserBySubject(ctx context.Context, subject string) (*User, error) {
	ctx, span := startSpan(ctx, "GetUserBySubject")
	defer span.End()

	u, err := db.scanUser(db.Pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE subject = $1`, subject))
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting user by subject: %w", err)
	}
	return u, nil
}

// GetUserByEmail resolves a user by (normalised) email.
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	ctx, span := startSpan(ctx, "GetUserByEmail")
	defer span.End()

	u, err := db.scanUser(db.Pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1`, normalizeEmail(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return u, nil
}

// Credentials carries the fields the local-login path needs to authenticate a
// password: the principal id, the stored argon2id hash, and whether the
// account is disabled. PasswordHash is empty for an OIDC-only / invited user
// (no local login possible).
type Credentials struct {
	UserID       string
	PasswordHash string
	Disabled     bool
}

// CredentialsByEmail returns the login credentials for an email. Returns
// ErrNotFound when no such user exists. The caller verifies the hash and
// checks Disabled — this method never reveals whether the failure was a
// missing user vs. a bad password.
func (db *DB) CredentialsByEmail(ctx context.Context, email string) (*Credentials, error) {
	ctx, span := startSpan(ctx, "CredentialsByEmail")
	defer span.End()

	var c Credentials
	var hash *string
	err := db.Pool.QueryRow(ctx,
		`SELECT id, password_hash, disabled FROM users WHERE email = $1`,
		normalizeEmail(email)).Scan(&c.UserID, &hash, &c.Disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting credentials: %w", err)
	}
	if hash != nil {
		c.PasswordHash = *hash
	}
	return &c, nil
}

// CreateUser inserts a users row. Returns ErrConflict when the email (or
// subject) already exists.
func (db *DB) CreateUser(ctx context.Context, p CreateUserParams) (*User, error) {
	ctx, span := startSpan(ctx, "CreateUser")
	defer span.End()

	id := NewULID()
	now := time.Now().UTC()
	email := normalizeEmail(p.Email)

	var subject, passwordHash, displayName any
	if p.Subject != "" {
		subject = p.Subject
	}
	if p.PasswordHash != "" {
		passwordHash = p.PasswordHash
	}
	if p.DisplayName != "" {
		displayName = p.DisplayName
	}

	u, err := db.scanUser(db.Pool.QueryRow(ctx, `
		INSERT INTO users
			(id, email, display_name, subject, password_hash, is_server_admin, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING `+userColumns,
		id, email, displayName, subject, passwordHash, p.IsServerAdmin, now))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return nil, ErrConflict
		}
		recordErr(span, err)
		return nil, fmt.Errorf("creating user: %w", err)
	}
	return u, nil
}

// ListUsers returns a page of users ordered by created_at DESC.
func (db *DB) ListUsers(ctx context.Context, p ListUsersParams) ([]User, error) {
	ctx, span := startSpan(ctx, "ListUsers")
	defer span.End()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	args := []any{}
	argN := 1
	where := "WHERE 1=1"
	if p.Cursor != "" {
		at, id, err := decodeCursor(p.Cursor)
		if err == nil {
			where += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argN, argN+1)
			args = append(args, at, id)
			argN += 2
		}
	}
	args = append(args, p.Limit)

	rows, err := db.Pool.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM users %s ORDER BY created_at DESC, id DESC LIMIT $%d`,
		userColumns, where, argN), args...)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	var result []User
	for rows.Next() {
		u, err := db.scanUser(rows)
		if err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		result = append(result, *u)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}

// UpdateUser patches the mutable fields of a user. Only non-nil params are
// written. Returns ErrNotFound if the user does not exist.
func (db *DB) UpdateUser(ctx context.Context, id string, p UpdateUserParams) (*User, error) {
	ctx, span := startSpan(ctx, "UpdateUser")
	defer span.End()

	sets := []string{"updated_at = now()"}
	args := []any{}
	argN := 1
	if p.DisplayName != nil {
		sets = append(sets, fmt.Sprintf("display_name = $%d", argN))
		args = append(args, *p.DisplayName)
		argN++
	}
	if p.Disabled != nil {
		sets = append(sets, fmt.Sprintf("disabled = $%d", argN))
		args = append(args, *p.Disabled)
		argN++
	}
	if p.IsServerAdmin != nil {
		sets = append(sets, fmt.Sprintf("is_server_admin = $%d", argN))
		args = append(args, *p.IsServerAdmin)
		argN++
	}
	args = append(args, id)

	u, err := db.scanUser(db.Pool.QueryRow(ctx, fmt.Sprintf(
		`UPDATE users SET %s WHERE id = $%d RETURNING %s`,
		strings.Join(sets, ", "), argN, userColumns), args...))
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("updating user: %w", err)
	}
	return u, nil
}

// SetPasswordHash sets (or clears, when hash is empty) a user's argon2id
// password hash. Returns ErrNotFound if the user does not exist.
func (db *DB) SetPasswordHash(ctx context.Context, id, hash string) error {
	ctx, span := startSpan(ctx, "SetPasswordHash")
	defer span.End()

	var stored any
	if hash != "" {
		stored = hash
	}
	tag, err := db.Pool.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`,
		stored, id)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("setting password hash: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// BindSubject binds an OIDC subject onto an existing user row, but only when
// the row currently has no subject (bind-once). Rebinding a subject on a later
// email change is a known account-takeover CVE class (ADR 0006 §2), so this
// refuses to overwrite a non-NULL subject and returns ErrConflict instead.
// Returns ErrNotFound if the user does not exist.
func (db *DB) BindSubject(ctx context.Context, userID, subject string) error {
	ctx, span := startSpan(ctx, "BindSubject")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE users SET subject = $1, updated_at = now()
		 WHERE id = $2 AND subject IS NULL`,
		subject, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// subject already claimed by another row.
			recordErr(span, ErrConflict)
			return ErrConflict
		}
		recordErr(span, err)
		return fmt.Errorf("binding subject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Either the user is gone, or it already has a (different) subject.
		recordErr(span, ErrConflict)
		return ErrConflict
	}
	return nil
}

// TouchLastSeen records the most recent authenticated request time for a user.
// Best-effort: a missing row is not an error (the principal may have just been
// removed mid-request).
func (db *DB) TouchLastSeen(ctx context.Context, id string) error {
	ctx, span := startSpan(ctx, "TouchLastSeen")
	defer span.End()

	_, err := db.Pool.Exec(ctx,
		`UPDATE users SET last_seen_at = now() WHERE id = $1`, id)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("touching last_seen_at: %w", err)
	}
	return nil
}

// DeleteUser removes a user and all grants attached to it. group_members rows
// cascade via FK; role_grants are polymorphic (no FK) so they are deleted
// explicitly here. Returns ErrNotFound if the user does not exist.
func (db *DB) DeleteUser(ctx context.Context, id string) error {
	ctx, span := startSpan(ctx, "DeleteUser")
	defer span.End()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM role_grants WHERE principal_type = 'user' AND principal_id = $1`,
		id); err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting user grants: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		recordErr(span, err)
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
