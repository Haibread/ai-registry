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
// (ADR 0006 §6). It replaces RequireWorkspaceWrite / RequireReviewer.
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

			p, ok := PrincipalFromContext(r.Context())
			if !ok || p == nil {
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
				UserID:          p.UserID,
				ClaimGroupSlugs: p.ClaimGroups,
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
