package auth

import (
	"context"
	"net/http"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// RoleStore is the narrow store slice RequirePublisherRole needs to resolve a
// caller's effective roles on a publisher. *store.DB satisfies it.
type RoleStore interface {
	EffectiveRoles(ctx context.Context, p store.EffectiveRolesParams) (map[domain.Role]bool, error)
}

// PublisherResolver returns the publisher id a request targets. Implementations
// read URL path params (the {namespace} publisher slug) or look up the
// referenced resource. They MUST NOT consume the request body — the guard runs
// before the handler. Returning store.ErrNotFound means "no such publisher",
// which the guard treats as 403 (you hold no role on a publisher that does not
// exist) rather than leaking existence with a 404.
type PublisherResolver func(r *http.Request) (publisherID string, err error)

// RequirePublisherRole returns chi middleware that authorizes a write/review
// request against the caller's effective role on the target publisher
// (ADR 0006 §6). It is the per-publisher replacement for the old
// workspace-group write gate.
//
// The check is a capability check over the role lattice (domain.Satisfies),
// not a threshold: Editor satisfies an Editor requirement, Reviewer satisfies
// a Reviewer requirement, Admin satisfies both, and Server Admin satisfies
// everything (short-circuited up front). Must be chained after Authenticate
// with a PrincipalStore configured.
func RequirePublisherRole(rs RoleStore, required domain.Role, resolve PublisherResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Server Admin satisfies any publisher-scoped requirement.
			if IsServerAdminFromContext(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}

			// Identify the caller. Prefer the resolved Principal, but fall back
			// to the token's claims: a federated group-writer whose access token
			// carries no email isn't provisioned into a users row, yet still
			// names its groups in the claim — and the ADR's model authorizes via
			// the group's grant (claim slug → group → grant), no users row
			// required. Either source must be present (authenticated).
			var userID string
			var groups []string
			authed := false
			if p, ok := PrincipalFromContext(r.Context()); ok && p != nil {
				userID = p.UserID
				groups = p.ClaimGroups
				authed = true
			} else if c, ok := ClaimsFromContext(r.Context()); ok && c != nil {
				groups = c.Groups
				authed = true
			}
			if !authed {
				problem.Write(w, http.StatusUnauthorized, "unauthorized",
					"Missing or invalid bearer token", r.URL.Path)
				return
			}

			publisherID, err := resolve(r)
			if err != nil {
				// Unknown publisher → the caller cannot hold a role there.
				problem.Write(w, http.StatusForbidden, "forbidden",
					"You do not have the required role on this publisher.", r.URL.Path)
				return
			}

			held, err := rs.EffectiveRoles(r.Context(), store.EffectiveRolesParams{
				UserID:          userID,
				ClaimGroupSlugs: groups,
				PublisherID:     publisherID,
			})
			if err != nil {
				problem.Write(w, http.StatusInternalServerError, "internal",
					"Failed to resolve authorization.", r.URL.Path)
				return
			}
			if !domain.Satisfies(held, required) {
				problem.Write(w, http.StatusForbidden, "forbidden",
					"Insufficient role on this publisher: "+string(required)+" required.", r.URL.Path)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IsAuthenticated reports whether the request carries any usable identity — a
// resolved Principal or token claims. Handlers that authorize from the request
// body (e.g. resource create, where the target publisher is only known after
// parsing) call this first to reject anonymous callers with 401 before doing
// any work, so the unauthenticated path never reaches the body parser or the DB.
func IsAuthenticated(ctx context.Context) bool {
	if p, ok := PrincipalFromContext(ctx); ok && p != nil {
		return true
	}
	if c, ok := ClaimsFromContext(ctx); ok && c != nil {
		return true
	}
	return false
}

// CheckPublisherRole is the function form of RequirePublisherRole, for handlers
// that can only resolve the target publisher after parsing the request body
// (resource create carries its publisher in the body, not the path). It mirrors
// the middleware's capability check over the role lattice:
//
//   - Server Admin satisfies any requirement (ok=true).
//   - Otherwise the caller's effective roles on publisherID are resolved from
//     their user grants and effective groups, and checked with domain.Satisfies.
//
// authed is false when the request carries no identity — callers should map
// that to 401 (prefer an IsAuthenticated guard up front). When authed is true,
// ok is the authorization decision (map false to 403).
func CheckPublisherRole(ctx context.Context, rs RoleStore, publisherID string, required domain.Role) (ok, authed bool, err error) {
	if IsServerAdminFromContext(ctx) {
		return true, true, nil
	}
	var userID string
	var groups []string
	if p, ok := PrincipalFromContext(ctx); ok && p != nil {
		userID = p.UserID
		groups = p.ClaimGroups
		authed = true
	} else if c, ok := ClaimsFromContext(ctx); ok && c != nil {
		groups = c.Groups
		authed = true
	}
	if !authed {
		return false, false, nil
	}
	held, err := rs.EffectiveRoles(ctx, store.EffectiveRolesParams{
		UserID:          userID,
		ClaimGroupSlugs: groups,
		PublisherID:     publisherID,
	})
	if err != nil {
		return false, true, err
	}
	return domain.Satisfies(held, required), true, nil
}

// RejectLocalToken is the MCP wall (ADR 0006 §3, non-negotiable): the MCP and
// agent protocol surface is OAuth-only, so a registry-issued local token is
// refused there even though it is valid on the human/admin API. Unauthenticated
// and OIDC-authenticated requests pass through (other guards handle them).
func RejectLocalToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if kind, ok := IssuerKindFromContext(r.Context()); ok && kind == IssuerLocal {
			problem.Write(w, http.StatusForbidden, "forbidden",
				"Local registry tokens are not accepted on the MCP surface; use an OAuth (OIDC) token.", r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}
