package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/haibread/ai-registry/internal/store"
)

// fakePrincipalStore is an in-memory PrincipalStore for the federated
// provisioning unit tests.
type fakePrincipalStore struct {
	byID      map[string]*store.User
	bySubject map[string]*store.User
	byEmail   map[string]*store.User
	created   []store.CreateUserParams
	binds     []string // userID bound to a subject
}

func newFakePrincipalStore() *fakePrincipalStore {
	return &fakePrincipalStore{
		byID:      map[string]*store.User{},
		bySubject: map[string]*store.User{},
		byEmail:   map[string]*store.User{},
	}
}

func (f *fakePrincipalStore) GetUserByID(_ context.Context, id string) (*store.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, store.ErrNotFound
}
func (f *fakePrincipalStore) GetUserBySubject(_ context.Context, sub string) (*store.User, error) {
	if u, ok := f.bySubject[sub]; ok {
		return u, nil
	}
	return nil, store.ErrNotFound
}
func (f *fakePrincipalStore) GetUserByEmail(_ context.Context, email string) (*store.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, store.ErrNotFound
}
func (f *fakePrincipalStore) CreateUser(_ context.Context, p store.CreateUserParams) (*store.User, error) {
	f.created = append(f.created, p)
	return &store.User{ID: "jit-" + p.Subject, Email: p.Email, Subject: p.Subject}, nil
}
func (f *fakePrincipalStore) BindSubject(_ context.Context, userID, _ string) error {
	f.binds = append(f.binds, userID)
	return nil
}

func claims(sub, email string, verified bool) *KeycloakClaims {
	return &KeycloakClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: sub},
		Email:            email,
		EmailVerified:    verified,
	}
}

func TestResolveOrProvisionFederated_ExistingSubject(t *testing.T) {
	st := newFakePrincipalStore()
	st.bySubject["sub-1"] = &store.User{ID: "u1", Email: "a@b.com", Subject: "sub-1"}

	u, err := ResolveOrProvisionFederated(context.Background(), st, claims("sub-1", "a@b.com", true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != "u1" {
		t.Fatalf("resolved user = %q, want u1", u.ID)
	}
	if len(st.created) != 0 || len(st.binds) != 0 {
		t.Fatalf("an already-provisioned subject must not create or bind")
	}
}

func TestResolveOrProvisionFederated_BindsPreInvitedRow(t *testing.T) {
	st := newFakePrincipalStore()
	// Pre-invited local row: has the email, no subject yet.
	st.byEmail["a@b.com"] = &store.User{ID: "u1", Email: "a@b.com", Subject: ""}

	u, err := ResolveOrProvisionFederated(context.Background(), st, claims("sub-1", "a@b.com", true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != "u1" || u.Subject != "sub-1" {
		t.Fatalf("expected bind onto u1 with sub-1, got %+v", u)
	}
	if len(st.binds) != 1 || st.binds[0] != "u1" {
		t.Fatalf("expected one BindSubject on u1, got %v", st.binds)
	}
	if len(st.created) != 0 {
		t.Fatalf("bind path must not create a new row")
	}
}

func TestResolveOrProvisionFederated_JITCreatesWhenNoMatch(t *testing.T) {
	st := newFakePrincipalStore()

	u, err := ResolveOrProvisionFederated(context.Background(), st, claims("sub-9", "new@b.com", true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Subject != "sub-9" || u.Email != "new@b.com" {
		t.Fatalf("unexpected JIT user: %+v", u)
	}
	if len(st.created) != 1 {
		t.Fatalf("expected one CreateUser, got %d", len(st.created))
	}
}

func TestResolveOrProvisionFederated_UnverifiedEmailSkipsBindAndCreates(t *testing.T) {
	st := newFakePrincipalStore()
	// A pre-invited row exists, but the token's email is unverified, so the
	// bind-once path must be skipped and a fresh JIT row created instead.
	st.byEmail["a@b.com"] = &store.User{ID: "u1", Email: "a@b.com", Subject: ""}

	u, err := ResolveOrProvisionFederated(context.Background(), st, claims("sub-1", "a@b.com", false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(st.binds) != 0 {
		t.Fatalf("unverified email must not bind, got %v", st.binds)
	}
	if u.ID != "jit-sub-1" || len(st.created) != 1 {
		t.Fatalf("expected a JIT create, got %+v / created=%d", u, len(st.created))
	}
}

func TestResolveOrProvisionFederated_RefusesEmailHijack(t *testing.T) {
	st := newFakePrincipalStore()
	// Email already linked to a different identity.
	st.byEmail["a@b.com"] = &store.User{ID: "u1", Email: "a@b.com", Subject: "other-sub"}

	_, err := ResolveOrProvisionFederated(context.Background(), st, claims("sub-1", "a@b.com", true))
	if !errors.Is(err, errPrincipalUnresolved) {
		t.Fatalf("expected errPrincipalUnresolved, got %v", err)
	}
	if len(st.binds) != 0 || len(st.created) != 0 {
		t.Fatalf("hijack attempt must neither bind nor create")
	}
}

func TestResolveOrProvisionFederated_NoEmailIsRejected(t *testing.T) {
	st := newFakePrincipalStore()
	_, err := ResolveOrProvisionFederated(context.Background(), st, claims("sub-1", "", true))
	if !errors.Is(err, errNoEmail) {
		t.Fatalf("expected errNoEmail, got %v", err)
	}
}
