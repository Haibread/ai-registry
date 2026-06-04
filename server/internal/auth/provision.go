package auth

import (
	"context"
	"errors"

	"github.com/haibread/ai-registry/internal/store"
)

// ResolveOrProvisionFederated maps a validated OIDC identity to a registry
// users row: resolve by subject, else just-in-time provision.
// Called by the OIDC callback once the broker has validated the id_token.
func ResolveOrProvisionFederated(ctx context.Context, st PrincipalStore, id *BrokeredIdentity) (*store.User, error) {
	u, err := st.GetUserBySubject(ctx, id.Subject)
	if errors.Is(err, store.ErrNotFound) {
		return provisionFederated(ctx, st, id)
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// provisionFederated handles a federated first login. It binds onto a
// pre-invited local row matching the email (bind-once — it never rebinds a row
// that already has a subject, the account-takeover guard), otherwise it lazily
// creates a JIT row. The IdP is trusted to assert the email (the registry does
// not require an email_verified claim); an email is required because
// users.email is NOT NULL.
func provisionFederated(ctx context.Context, st PrincipalStore, id *BrokeredIdentity) (*store.User, error) {
	if id.Email == "" {
		return nil, errNoEmail
	}
	existing, err := st.GetUserByEmail(ctx, id.Email)
	switch {
	case err == nil && existing.Subject == "":
		// Pre-invited row: bind this subject once.
		if bindErr := st.BindSubject(ctx, existing.ID, id.Subject); bindErr != nil {
			return nil, bindErr
		}
		existing.Subject = id.Subject
		return existing, nil
	case err == nil:
		// Email already linked to a different identity — refuse to hijack.
		return nil, errPrincipalUnresolved
	case !errors.Is(err, store.ErrNotFound):
		return nil, err
	}
	return st.CreateUser(ctx, store.CreateUserParams{
		Email:   id.Email,
		Subject: id.Subject,
	})
}
