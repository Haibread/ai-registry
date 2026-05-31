package handlers

import (
	"context"
	"net/http"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// MeStore is the narrow store slice the /me handler needs: the caller's user
// row (for the display name) and their effective role grants. *store.DB
// satisfies it.
type MeStore interface {
	GetUserByID(ctx context.Context, id string) (*store.User, error)
	ListGrantsForPrincipal(ctx context.Context, userID string, claimGroupSlugs []string) ([]store.PrincipalGrant, error)
}

// MeHandlers serves GET /api/v1/me.
type MeHandlers struct {
	db MeStore
}

// NewMeHandlers constructs the /me handler.
func NewMeHandlers(db MeStore) *MeHandlers {
	return &MeHandlers{db: db}
}

// meResponse is the resolved identity + authorization view returned to the SPA
// so it can gate the admin UI by role without trusting any client-side claim.
// Grants includes both per-publisher and global (empty publisher fields) role
// grants; is_server_admin short-circuits every publisher-scoped check.
type meResponse struct {
	Authenticated bool                   `json:"authenticated"`
	UserID        string                 `json:"user_id,omitempty"`
	Email         string                 `json:"email,omitempty"`
	DisplayName   string                 `json:"display_name,omitempty"`
	IsServerAdmin bool                   `json:"is_server_admin"`
	Issuer        string                 `json:"issuer,omitempty"`
	Grants        []store.PrincipalGrant `json:"grants"`
}

// Me returns the authenticated caller's identity and effective role grants.
// Authentication is required: a request with no resolvable principal and no
// token claims gets 401. A federated user who has a valid token but no
// provisioned users row still resolves (grants come from claim-group grants).
func (h *MeHandlers) Me(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, hasPrincipal := auth.PrincipalFromContext(ctx)
	claims, hasClaims := auth.ClaimsFromContext(ctx)
	if (!hasPrincipal || p == nil) && (!hasClaims || claims == nil) {
		problem.Write(w, http.StatusUnauthorized, "unauthorized",
			"Missing or invalid bearer token", r.URL.Path)
		return
	}

	var userID, email, displayName, issuer string
	var groups []string
	if hasPrincipal && p != nil {
		userID = p.UserID
		email = p.Email
		groups = p.ClaimGroups
		issuer = string(p.Issuer)
	}
	if hasClaims && claims != nil {
		if email == "" {
			email = claims.Email
		}
		if len(groups) == 0 {
			groups = claims.Groups
		}
	}

	// Enrich with the display name from the users row when the caller is
	// provisioned. A federated, not-yet-provisioned user has no row — that is
	// fine; their grants still resolve via claim-group membership.
	if userID != "" {
		if u, err := h.db.GetUserByID(ctx, userID); err == nil && u != nil {
			displayName = u.DisplayName
			if email == "" {
				email = u.Email
			}
		}
	}

	grants, err := h.db.ListGrantsForPrincipal(ctx, userID, groups)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if grants == nil {
		grants = []store.PrincipalGrant{}
	}

	writeJSON(w, r, http.StatusOK, meResponse{
		Authenticated: true,
		UserID:        userID,
		Email:         email,
		DisplayName:   displayName,
		IsServerAdmin: auth.IsServerAdminFromContext(ctx),
		Issuer:        issuer,
		Grants:        grants,
	})
}

// MineScopeStore is the store slice mineScope needs to resolve the publishers a
// caller holds a grant on. *store.DB satisfies it.
type MineScopeStore interface {
	EffectivePublisherIDs(ctx context.Context, userID string, claimGroupSlugs []string) (ids []string, hasGlobal bool, err error)
}

// mineScope resolves the publisher scope for a mine=true admin listing
// (ADR 0006 — "authors only see their own resources"):
//
//   - all=true   → the caller sees every publisher's resources (Server Admin or
//     a global grant). Apply NO publisher filter but DO drop the public-only
//     filter so private/draft entries show.
//   - ids        → the publishers the caller holds a grant on (when all=false).
//     An empty slice means a provisioned author with zero grants: the listing
//     must return nothing rather than the whole public catalog.
//   - authed     → false when the request carries no usable identity (→ 401);
//     "mine" is meaningless without knowing who you are.
func mineScope(ctx context.Context, st MineScopeStore) (ids []string, all, authed bool, err error) {
	if auth.IsServerAdminFromContext(ctx) {
		return nil, true, true, nil
	}
	userID, groups, authed := auth.IdentityFromContext(ctx)
	if !authed {
		return nil, false, false, nil
	}
	ids, hasGlobal, err := st.EffectivePublisherIDs(ctx, userID, groups)
	if err != nil {
		return nil, false, true, err
	}
	if hasGlobal {
		return nil, true, true, nil
	}
	return ids, false, true, nil
}
