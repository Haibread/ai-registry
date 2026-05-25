package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/haibread/ai-registry/internal/problem"
)

type contextKey string

const (
	claimsKey  contextKey = "auth_claims"
	isAdminKey contextKey = "auth_is_admin"
)

// Validator validates incoming JWTs and populates request context with claims.
type Validator struct {
	jwks        *JWKSCache
	issuer      string
	audience    string
	groupsClaim string // JSON key in the token payload to read group memberships from; "groups" is the default
}

// NewValidator creates a Validator using the provided JWKSCache and issuer.
// When audience is non-empty, tokens whose `aud` claim does not contain it are
// rejected — required by the MCP authorization spec (OAuth 2.1 resource
// indicators) to prevent cross-client token reuse.
//
// groupsClaim controls which JSON key in the token payload populates
// KeycloakClaims.Groups. The default is "groups" (matches the json tag on
// the typed struct); operators set AUTH_GROUPS_CLAIM when their Keycloak
// realm emits group memberships under a different name. An empty string
// is treated as the default.
func NewValidator(jwks *JWKSCache, issuer, audience, groupsClaim string) *Validator {
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	return &Validator{jwks: jwks, issuer: issuer, audience: audience, groupsClaim: groupsClaim}
}

// Authenticate is chi middleware that parses the Bearer token when present.
// It does NOT block requests without a token — unauthenticated requests
// proceed with no claims in context. Use RequireAdmin to gate write endpoints.
func (v *Validator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		claims := &KeycloakClaims{}
		parseOpts := []jwt.ParserOption{
			jwt.WithIssuedAt(),
			jwt.WithIssuer(v.issuer),
			jwt.WithExpirationRequired(),
		}
		if v.audience != "" {
			parseOpts = append(parseOpts, jwt.WithAudience(v.audience))
		}
		_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			kid, _ := t.Header["kid"].(string)
			return v.jwks.GetKey(r.Context(), kid)
		}, parseOpts...)

		if err != nil {
			// A token was provided but is invalid (expired, bad signature, etc.).
			// Return 401 rather than silently treating it as unauthenticated,
			// so clients with broken tokens get immediate diagnostic feedback.
			problem.Write(w, http.StatusUnauthorized, "unauthorized",
				"Invalid or expired bearer token", r.URL.Path)
			return
		}

		// When operators configure a non-default groups-claim name (e.g. an
		// IdP that emits "realm_groups" instead of "groups"), the typed
		// parse above won't have populated KeycloakClaims.Groups. Re-parse
		// the (already signature-verified) payload as a generic map and
		// extract the configured field.
		if v.groupsClaim != "groups" {
			claims.Groups = extractGroupsClaim(token, v.groupsClaim)
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		ctx = context.WithValue(ctx, isAdminKey, claims.IsAdmin())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractGroupsClaim decodes the token payload (already validated by
// ParseWithClaims) and returns the slice at the configured claim key.
// Anything that isn't a JSON array of strings becomes an empty slice —
// the caller should already treat a missing groups claim as "no
// memberships" rather than as an error.
func extractGroupsClaim(tokenString, claimName string) []string {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil
	}
	arr, ok := raw[claimName].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// WorkspaceLookup resolves the request's target workspace. Implementations
// typically read URL path params or look up the resource referenced in the
// request. They MUST NOT consume the request body — RequireWorkspaceWrite
// runs before the handler and the body must remain readable downstream.
//
// Returning an empty groupName is the legitimate "admin-only workspace"
// signal: the middleware then falls through to the admin check. Errors
// trigger a 500 — they're an indication that the URL or DB state is bad,
// not an authorization decision.
type WorkspaceLookup func(*http.Request) (groupName string, err error)

// RequireWorkspaceWrite is chi middleware that authorizes a write
// request against a workspace's group_name binding. The contract:
//
//   - Admins (realm role "admin") always pass.
//   - Non-admins pass only when the workspace's group_name is non-empty
//     and the JWT's `groups` claim contains it.
//   - Anyone else gets 403.
//   - Missing JWT entirely gets 401 (mirrors RequireAdmin's wording).
//
// The WorkspaceLookup runs after the auth gate so a missing token short-
// circuits before any DB work.
func RequireWorkspaceWrite(lookup WorkspaceLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok || claims == nil {
				problem.Write(w, http.StatusUnauthorized, "unauthorized",
					"Missing or invalid bearer token", r.URL.Path)
				return
			}
			if claims.IsAdmin() {
				next.ServeHTTP(w, r)
				return
			}
			groupName, err := lookup(r)
			if err != nil {
				problem.Write(w, http.StatusInternalServerError, "internal",
					"workspace lookup failed", r.URL.Path)
				return
			}
			// Distinguish "workspace has no binding" from "you lack the
			// binding's group" so a 403 actually tells you what's wrong.
			// We echo the required group but never the user's claim
			// values — the caller already has their own token.
			if groupName == "" {
				problem.Write(w, http.StatusForbidden, "forbidden",
					"This workspace is admin-only (no Keycloak group binding configured). Bind a group via PATCH .../workspaces/{slug} to delegate writes.", r.URL.Path)
				return
			}
			if !claims.HasGroup(groupName) {
				problem.Write(w, http.StatusForbidden, "forbidden",
					fmt.Sprintf("Writes to this workspace require membership in Keycloak group %q.", groupName), r.URL.Path)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireReviewer returns chi middleware that gates a route to admins
// or members of the configured reviewer group. Group name is passed as
// an argument because it is configured per-deployment via
// AUTH_REVIEWER_GROUP / auth.reviewer_group (default "registry-reviewers").
// When the group is empty or the JWT carries no matching membership, only
// admins pass.
//
// Wire this onto approve / reject endpoints and deletion-confirmation
// routes. Publisher-side endpoints (submit, withdraw, edit,
// request-deletion) keep using RequireWorkspaceWrite.
func RequireReviewer(group string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok || claims == nil {
				problem.Write(w, http.StatusUnauthorized, "unauthorized",
					"Missing or invalid bearer token", r.URL.Path)
				return
			}
			if claims.IsAdmin() {
				next.ServeHTTP(w, r)
				return
			}
			if group == "" {
				problem.Write(w, http.StatusForbidden, "forbidden",
					"The change-approval workflow is admin-only on this deployment (AUTH_REVIEWER_GROUP is unset).", r.URL.Path)
				return
			}
			if !claims.HasGroup(group) {
				problem.Write(w, http.StatusForbidden, "forbidden",
					fmt.Sprintf("Reviewing requires membership in Keycloak group %q.", group), r.URL.Path)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin is chi middleware that returns 401/403 if the request is not
// authenticated as an admin. It must be chained after Authenticate.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			problem.Write(w, http.StatusUnauthorized, "unauthorized",
				"Missing or invalid bearer token", r.URL.Path)
			return
		}
		if !claims.IsAdmin() {
			problem.Write(w, http.StatusForbidden, "forbidden",
				"Insufficient permissions: registry:admin role required", r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClaimsFromContext retrieves the parsed Keycloak claims from the context.
func ClaimsFromContext(ctx context.Context) (*KeycloakClaims, bool) {
	c, ok := ctx.Value(claimsKey).(*KeycloakClaims)
	return c, ok
}

// IsAdminFromContext reports whether the current request is authenticated as admin.
func IsAdminFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(isAdminKey).(bool)
	return v
}

// ContextWithClaims injects claims into a context. Used in tests to simulate
// authenticated requests without a real JWT.
func ContextWithClaims(ctx context.Context, claims *KeycloakClaims) context.Context {
	ctx = context.WithValue(ctx, claimsKey, claims)
	ctx = context.WithValue(ctx, isAdminKey, claims.IsAdmin())
	return ctx
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}
