package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/haibread/ai-registry/internal/domain"
)

// SeedRBACParams carries the config-derived inputs for the boot-time RBAC
// seed. Empty fields disable the corresponding seed step.
type SeedRBACParams struct {
	// ReviewerGroupSlug is the AUTH_REVIEWER_GROUP value. When non-empty, the
	// seed ensures a group of this slug exists and carries a global
	// (all-publishers) Reviewer grant with source = config. Re-applied on
	// every boot, so deleting the grant via the API only sticks if the seed
	// is removed too.
	ReviewerGroupSlug string

	// BootstrapAdminEmail / BootstrapAdminPasswordHash seed a local Server
	// Admin on first boot. Create-only: an existing account is left untouched
	// so a rotated password survives reboots. Empty email skips
	// the step. The hash is the argon2id output from auth.HashPassword — this
	// package never sees the plaintext.
	BootstrapAdminEmail        string
	BootstrapAdminPasswordHash string
}

// SeedRBAC idempotently applies the boot-time authorization seed: the reviewer
// group + its global Reviewer grant, and the bootstrap Server Admin. It is
// safe to call on every boot.
func (db *DB) SeedRBAC(ctx context.Context, p SeedRBACParams) error {
	ctx, span := startSpan(ctx, "SeedRBAC")
	defer span.End()

	if p.ReviewerGroupSlug != "" {
		g, err := db.EnsureGroupBySlug(ctx, p.ReviewerGroupSlug, p.ReviewerGroupSlug)
		if err != nil {
			recordErr(span, err)
			return fmt.Errorf("seeding reviewer group: %w", err)
		}
		if err := db.EnsureGrant(ctx, CreateGrantParams{
			PrincipalType: domain.PrincipalGroup,
			PrincipalID:   g.ID,
			PublisherID:   "", // global
			Role:          domain.RoleReviewer,
			Source:        domain.GrantSourceConfig,
		}); err != nil {
			recordErr(span, err)
			return fmt.Errorf("seeding reviewer grant: %w", err)
		}
	}

	if p.BootstrapAdminEmail != "" {
		_, err := db.GetUserByEmail(ctx, p.BootstrapAdminEmail)
		switch {
		case err == nil:
			// Account already exists — create-only, never touch the password.
		case errors.Is(err, ErrNotFound):
			_, err := db.CreateUser(ctx, CreateUserParams{
				Email:         p.BootstrapAdminEmail,
				PasswordHash:  p.BootstrapAdminPasswordHash,
				IsServerAdmin: true,
			})
			// A concurrent boot may have won the race; treat as success.
			if err != nil && !errors.Is(err, ErrConflict) {
				recordErr(span, err)
				return fmt.Errorf("seeding bootstrap admin: %w", err)
			}
		default:
			recordErr(span, err)
			return fmt.Errorf("checking bootstrap admin: %w", err)
		}
	}

	return nil
}
