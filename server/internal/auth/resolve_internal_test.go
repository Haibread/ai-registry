package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/haibread/ai-registry/internal/store"
)

// fakePrincipalStore is an in-memory PrincipalStore for unit-testing principal
// resolution without a database.
type fakePrincipalStore struct {
	byID      map[string]*store.User
	bySubject map[string]*store.User
	byEmail   map[string]*store.User

	createCalls int
	bindCalls   int
}

func newFakeStore() *fakePrincipalStore {
	return &fakePrincipalStore{
		byID:      map[string]*store.User{},
		bySubject: map[string]*store.User{},
		byEmail:   map[string]*store.User{},
	}
}

func (f *fakePrincipalStore) add(u *store.User) {
	f.byID[u.ID] = u
	if u.Subject != "" {
		f.bySubject[u.Subject] = u
	}
	f.byEmail[u.Email] = u
}

func (f *fakePrincipalStore) GetUserByID(_ context.Context, id string) (*store.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, store.ErrNotFound
}

func (f *fakePrincipalStore) GetUserBySubject(_ context.Context, subject string) (*store.User, error) {
	if u, ok := f.bySubject[subject]; ok {
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
	if _, ok := f.byEmail[p.Email]; ok {
		return nil, store.ErrConflict
	}
	f.createCalls++
	u := &store.User{ID: "new-" + p.Subject, Email: p.Email, Subject: p.Subject}
	f.add(u)
	return u, nil
}

func (f *fakePrincipalStore) BindSubject(_ context.Context, userID, subject string) error {
	u, ok := f.byID[userID]
	if !ok {
		return store.ErrNotFound
	}
	if u.Subject != "" {
		return store.ErrConflict
	}
	f.bindCalls++
	u.Subject = subject
	f.bySubject[subject] = u
	return nil
}

func validatorWithStore(f PrincipalStore) *Validator {
	return NewValidator(nil, "issuer", "", "").WithPrincipalStore(f)
}

func TestResolvePrincipal_Local(t *testing.T) {
	f := newFakeStore()
	f.add(&store.User{ID: "u1", Email: "a@x.test", IsServerAdmin: true})
	v := validatorWithStore(f)

	p, err := v.resolvePrincipal(context.Background(), IssuerLocal, &KeycloakClaims{Email: "a@x.test"})
	// Local resolution keys on Subject == users.id.
	if err == nil {
		t.Fatal("expected unresolved when subject does not match a user id")
	}

	claims := &KeycloakClaims{}
	claims.Subject = "u1"
	p, err = v.resolvePrincipal(context.Background(), IssuerLocal, claims)
	if err != nil {
		t.Fatalf("resolvePrincipal: %v", err)
	}
	if p.UserID != "u1" || !p.IsServerAdmin || p.Issuer != IssuerLocal {
		t.Errorf("principal = %+v, want u1/admin/local", p)
	}
}

func TestResolvePrincipal_LocalDisabled(t *testing.T) {
	f := newFakeStore()
	f.add(&store.User{ID: "u1", Email: "a@x.test", Disabled: true})
	v := validatorWithStore(f)

	claims := &KeycloakClaims{}
	claims.Subject = "u1"
	_, err := v.resolvePrincipal(context.Background(), IssuerLocal, claims)
	if !errors.Is(err, errAccountDisabled) {
		t.Errorf("err = %v, want errAccountDisabled", err)
	}
}

func TestResolvePrincipal_OIDCExisting(t *testing.T) {
	f := newFakeStore()
	f.add(&store.User{ID: "u1", Email: "a@x.test", Subject: "sub-1"})
	v := validatorWithStore(f)

	claims := &KeycloakClaims{Groups: []string{"team-a"}}
	claims.Subject = "sub-1"
	p, err := v.resolvePrincipal(context.Background(), IssuerOIDC, claims)
	if err != nil {
		t.Fatalf("resolvePrincipal: %v", err)
	}
	if p.UserID != "u1" || p.Issuer != IssuerOIDC {
		t.Errorf("principal = %+v, want u1/oidc", p)
	}
	if len(p.ClaimGroups) != 1 || p.ClaimGroups[0] != "team-a" {
		t.Errorf("claim groups = %v, want [team-a]", p.ClaimGroups)
	}
	if f.createCalls != 0 {
		t.Error("existing user must not be re-created")
	}
}

func TestResolvePrincipal_OIDCAdminFromClaim(t *testing.T) {
	f := newFakeStore()
	u := &store.User{ID: "u1", Email: "a@x.test", Subject: "sub-1"}
	f.add(u)
	v := validatorWithStore(f)

	claims := &KeycloakClaims{RealmAccess: RealmAccess{Roles: []string{"admin"}}}
	claims.Subject = "sub-1"
	p, err := v.resolvePrincipal(context.Background(), IssuerOIDC, claims)
	if err != nil {
		t.Fatalf("resolvePrincipal: %v", err)
	}
	if !p.IsServerAdmin {
		t.Error("realm admin claim should make the principal a Server Admin")
	}
}

func TestProvisionFederated_JITCreate(t *testing.T) {
	f := newFakeStore()
	v := validatorWithStore(f)

	claims := &KeycloakClaims{Email: "new@x.test", EmailVerified: true}
	claims.Subject = "sub-new"
	p, err := v.resolvePrincipal(context.Background(), IssuerOIDC, claims)
	if err != nil {
		t.Fatalf("resolvePrincipal: %v", err)
	}
	if f.createCalls != 1 {
		t.Errorf("createCalls = %d, want 1 (JIT)", f.createCalls)
	}
	if p.UserID == "" {
		t.Error("JIT-provisioned principal must have a user id")
	}
}

func TestProvisionFederated_BindOnce(t *testing.T) {
	f := newFakeStore()
	// Pre-invited row: email set, no subject.
	f.add(&store.User{ID: "invited-1", Email: "invitee@x.test"})
	v := validatorWithStore(f)

	claims := &KeycloakClaims{Email: "invitee@x.test", EmailVerified: true}
	claims.Subject = "sub-fed"
	p, err := v.resolvePrincipal(context.Background(), IssuerOIDC, claims)
	if err != nil {
		t.Fatalf("resolvePrincipal: %v", err)
	}
	if f.bindCalls != 1 {
		t.Errorf("bindCalls = %d, want 1 (bind-once onto invited row)", f.bindCalls)
	}
	if f.createCalls != 0 {
		t.Error("bind path must not create a new user")
	}
	if p.UserID != "invited-1" {
		t.Errorf("principal user id = %q, want invited-1", p.UserID)
	}
}

func TestProvisionFederated_RefusesHijackOfLinkedEmail(t *testing.T) {
	f := newFakeStore()
	// Email already linked to a DIFFERENT subject.
	f.add(&store.User{ID: "u1", Email: "taken@x.test", Subject: "sub-original"})
	v := validatorWithStore(f)

	claims := &KeycloakClaims{Email: "taken@x.test", EmailVerified: true}
	claims.Subject = "sub-attacker"
	_, err := v.resolvePrincipal(context.Background(), IssuerOIDC, claims)
	if !errors.Is(err, errPrincipalUnresolved) {
		t.Errorf("err = %v, want errPrincipalUnresolved (no hijack)", err)
	}
	if f.bindCalls != 0 {
		t.Error("must not bind onto an already-linked email")
	}
}

func TestProvisionFederated_UnverifiedEmailDoesNotBind(t *testing.T) {
	f := newFakeStore()
	// Pre-invited row exists, but the token's email is NOT verified.
	f.add(&store.User{ID: "invited-1", Email: "invitee@x.test"})
	v := validatorWithStore(f)

	claims := &KeycloakClaims{Email: "invitee@x.test", EmailVerified: false}
	claims.Subject = "sub-fed"
	_, err := v.resolvePrincipal(context.Background(), IssuerOIDC, claims)
	// Unverified email must not bind onto the invited row; it tries to create,
	// which collides on the unique email — fail closed.
	if err == nil {
		t.Error("unverified email must not bind onto a pre-invited row")
	}
	if f.bindCalls != 0 {
		t.Error("must not bind on an unverified email")
	}
}

func TestProvisionFederated_NoEmail(t *testing.T) {
	f := newFakeStore()
	v := validatorWithStore(f)

	claims := &KeycloakClaims{}
	claims.Subject = "sub-noemail"
	_, err := v.resolvePrincipal(context.Background(), IssuerOIDC, claims)
	if !errors.Is(err, errNoEmail) {
		t.Errorf("err = %v, want errNoEmail", err)
	}
}
