package auth

import (
	"context"
	"errors"

	"github.com/haibread/ai-registry/internal/store"
)

// ResolveOrProvisionFederated maps a validated OIDC identity to a registry
// users row: resolve by subject, else just-in-time provision.
// Called by the OIDC callback once the broker has validated the id_token.
func ResolveOrProvisionFederated(ctx context.Context, st PrincipalStore, claims *KeycloakClaims) (*store.User, error) {
	u, err := st.GetUserBySubject(ctx, claims.Subject)
	if errors.Is(err, store.ErrNotFound) {
		return provisionFederated(ctx, st, claims)
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// provisionFederated handles a federated first login. With a verified email it
// binds onto a pre-invited row (bind-once — never rebinds a row that already
// has a subject, the account-takeover guard); otherwise it lazily creates a JIT
// row. An email is required because users.email is NOT NULL.
func provisionFederated(ctx context.Context, st PrincipalStore, claims *KeycloakClaims) (*store.User, error) {
	if claims.Email == "" {
		return nil, errNoEmail
	}
	if claims.EmailVerified {
		existing, err := st.GetUserByEmail(ctx, claims.Email)
		switch {
		case err == nil && existing.Subject == "":
			// Pre-invited row: bind this subject once.
			if bindErr := st.BindSubject(ctx, existing.ID, claims.Subject); bindErr != nil {
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
	return st.CreateUser(ctx, store.CreateUserParams{
		Email:   claims.Email,
		Subject: claims.Subject,
	})
}
