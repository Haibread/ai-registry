package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

type contextKey string

const (
	claimsKey     contextKey = "auth_claims"
	isAdminKey    contextKey = "auth_is_admin"
	issuerKindKey contextKey = "auth_issuer_kind"
)

// Validator validates incoming JWTs and populates request context with claims.
// It is multi-issuer (ADR 0006 §3): tokens whose `iss` is the local marker are
// verified against the registry's own key, all others against the OIDC JWKS.
type Validator struct {
	jwks        *JWKSCache
	issuer      string
	audience    string
	groupsClaim string // JSON key in the token payload to read group memberships from; "groups" is the default

	// local verifies registry-issued local tokens. Nil when local login is
	// disabled — then only OIDC tokens are accepted.
	local *LocalIssuer
	// store resolves a verified token into a Principal (users.id + effective
	// admin) and just-in-time provisions federated users. Nil in unit tests
	// that only exercise claim parsing — then no Principal is stored and
	// downstream falls back to claim-only behaviour.
	store PrincipalStore
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

// WithLocalIssuer attaches the registry's local-token issuer so Authenticate
// accepts locally-signed tokens. Returns the receiver for chaining.
func (v *Validator) WithLocalIssuer(li *LocalIssuer) *Validator {
	v.local = li
	return v
}

// WithPrincipalStore attaches the store used to resolve a token into a
// Principal (and to JIT-provision federated users). Returns the receiver.
func (v *Validator) WithPrincipalStore(s PrincipalStore) *Validator {
	v.store = s
	return v
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

		issKind, claims, err := v.verify(r, token)
		if err != nil {
			// A token was provided but is invalid (expired, bad signature, etc.).
			// Return 401 rather than silently treating it as unauthenticated,
			// so clients with broken tokens get immediate diagnostic feedback.
			problem.Write(w, http.StatusUnauthorized, "unauthorized",
				"Invalid or expired bearer token", r.URL.Path)
			return
		}

		// When operators configure a non-default groups-claim name (e.g. an
		// IdP that emits "realm_groups" instead of "groups"), the typed parse
		// won't have populated KeycloakClaims.Groups. Re-parse the (already
		// signature-verified) payload as a generic map and extract the field.
		// Local tokens never carry claim groups.
		if issKind == IssuerOIDC && v.groupsClaim != "groups" {
			claims.Groups = extractGroupsClaim(token, v.groupsClaim)
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		ctx = context.WithValue(ctx, isAdminKey, claims.IsAdmin())
		ctx = context.WithValue(ctx, issuerKindKey, issKind)

		// Resolve the principal (users.id + effective admin) when a store is
		// configured. A claim-admin federated token is never locked out by a
		// DB failure (ADR 0006 §6); other principals fail closed.
		if v.store != nil {
			princ, perr := v.resolvePrincipal(ctx, issKind, claims)
			switch {
			case perr == nil:
				ctx = context.WithValue(ctx, principalKey, princ)
				ctx = context.WithValue(ctx, isAdminKey, princ.IsServerAdmin)
			case claims.IsAdmin():
				// Federated Server Admin escape hatch: proceed without a DB row.
				ctx = context.WithValue(ctx, principalKey, &Principal{
					Email:         claims.Email,
					ClaimGroups:   claims.Groups,
					IsServerAdmin: true,
					Issuer:        issKind,
				})
			case errors.Is(perr, errAccountDisabled):
				problem.Write(w, http.StatusForbidden, "forbidden",
					"This account is disabled.", r.URL.Path)
				return
			default:
				problem.Write(w, http.StatusUnauthorized, "unauthorized",
					"Could not resolve the authenticated principal.", r.URL.Path)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// verify validates the bearer token against the right issuer (local marker →
// registry key; everything else → OIDC JWKS) and returns the parsed claims and
// which front door was used.
func (v *Validator) verify(r *http.Request, token string) (IssuerKind, *KeycloakClaims, error) {
	if v.local != nil && unverifiedIssuer(token) == LocalTokenIssuer {
		claims, err := v.local.Verify(token)
		if err != nil {
			return "", nil, err
		}
		return IssuerLocal, claims, nil
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
		return "", nil, err
	}
	return IssuerOIDC, claims, nil
}

// resolvePrincipal turns verified claims into a Principal, provisioning a
// federated user on first login. Disabled accounts are refused on every
// request (the local kill-switch, ADR 0006 §2).
func (v *Validator) resolvePrincipal(ctx context.Context, issKind IssuerKind, claims *KeycloakClaims) (*Principal, error) {
	if issKind == IssuerLocal {
		u, err := v.store.GetUserByID(ctx, claims.Subject)
		if err != nil {
			return nil, errPrincipalUnresolved
		}
		if u.Disabled {
			return nil, errAccountDisabled
		}
		return &Principal{
			UserID:        u.ID,
			Email:         u.Email,
			IsServerAdmin: u.IsServerAdmin,
			Issuer:        IssuerLocal,
		}, nil
	}

	// OIDC: resolve by subject, JIT-provision on first login.
	u, err := v.store.GetUserBySubject(ctx, claims.Subject)
	if errors.Is(err, store.ErrNotFound) {
		u, err = v.provisionFederated(ctx, claims)
	}
	if err != nil {
		return nil, err
	}
	if u.Disabled {
		return nil, errAccountDisabled
	}
	return &Principal{
		UserID:        u.ID,
		Email:         u.Email,
		ClaimGroups:   claims.Groups,
		IsServerAdmin: claims.IsAdmin() || u.IsServerAdmin,
		Issuer:        IssuerOIDC,
	}, nil
}

// provisionFederated handles a federated first login. With a verified email it
// binds onto a pre-invited row (bind-once — never rebinds a row that already
// has a subject, the account-takeover guard); otherwise it lazily creates a
// JIT row. An email is required because users.email is NOT NULL.
func (v *Validator) provisionFederated(ctx context.Context, claims *KeycloakClaims) (*store.User, error) {
	if claims.Email == "" {
		return nil, errNoEmail
	}

	if claims.EmailVerified {
		existing, err := v.store.GetUserByEmail(ctx, claims.Email)
		switch {
		case err == nil && existing.Subject == "":
			// Pre-invited row: bind this subject once.
			if bindErr := v.store.BindSubject(ctx, existing.ID, claims.Subject); bindErr != nil {
				return nil, bindErr
			}
			existing.Subject = claims.Subject
			return existing, nil
		case err == nil:
			// Email already linked to a different identity — refuse to hijack.
			return nil, errPrincipalUnresolved
		case !errors.Is(err, store.ErrNotFound):
			return nil, err
		}
	}

	created, err := v.store.CreateUser(ctx, store.CreateUserParams{
		Email:   claims.Email,
		Subject: claims.Subject,
	})
	if err != nil {
		return nil, err
	}
	return created, nil
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

// RequireAdmin is chi middleware that gates a route to Server Admins. It must
// be chained after Authenticate. Server Admin is dual-sourced (ADR 0006): the
// OIDC realm-admin claim OR a resolved principal's is_server_admin flag (the
// local bootstrap-admin path), via IsServerAdminFromContext.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			problem.Write(w, http.StatusUnauthorized, "unauthorized",
				"Missing or invalid bearer token", r.URL.Path)
			return
		}
		if !IsServerAdminFromContext(r.Context()) {
			problem.Write(w, http.StatusForbidden, "forbidden",
				"Insufficient permissions: Server Admin required", r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// IssuerKindFromContext reports which front door authenticated the request.
// Returns ("", false) for unauthenticated requests. Used by the MCP wall.
func IssuerKindFromContext(ctx context.Context) (IssuerKind, bool) {
	k, ok := ctx.Value(issuerKindKey).(IssuerKind)
	return k, ok
}

// unverifiedIssuer extracts the `iss` claim from a token WITHOUT verifying its
// signature, used only to route the token to the right verifier. The selected
// verifier then fully validates the signature, issuer, and expiry, so reading
// this untrusted value here is safe.
func unverifiedIssuer(tokenString string) string {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var raw struct {
		Iss string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return ""
	}
	return raw.Iss
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
