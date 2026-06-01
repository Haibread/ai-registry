package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

// fakeMeStore is a hand-written fake satisfying both handlers.MeStore and
// handlers.MineScopeStore so the /me handler can be exercised without a DB.
type fakeMeStore struct {
	user   *store.User
	grants []store.PrincipalGrant
	pubIDs []string
	global bool
}

func (f *fakeMeStore) GetUserByID(_ context.Context, _ string) (*store.User, error) {
	return f.user, nil
}

func (f *fakeMeStore) ListGrantsForPrincipal(_ context.Context, _ string, _ []string) ([]store.PrincipalGrant, error) {
	return f.grants, nil
}

func (f *fakeMeStore) EffectivePublisherIDs(_ context.Context, _ string, _ []string) ([]string, bool, error) {
	return f.pubIDs, f.global, nil
}

type meBody struct {
	Authenticated bool   `json:"authenticated"`
	UserID        string `json:"user_id"`
	Email         string `json:"email"`
	DisplayName   string `json:"display_name"`
	IsServerAdmin bool   `json:"is_server_admin"`
	Issuer        string `json:"issuer"`
	Grants        []struct {
		Role          string `json:"role"`
		PublisherID   string `json:"publisher_id"`
		PublisherSlug string `json:"publisher_slug"`
		PublisherName string `json:"publisher_name"`
	} `json:"grants"`
}

func TestMeHandler_returns_401_when_unauthenticated(t *testing.T) {
	h := handlers.NewMeHandlers(&fakeMeStore{})
	rr := httptest.NewRecorder()
	h.Me(rr, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestMeHandler_returns_identity_and_grants(t *testing.T) {
	st := &fakeMeStore{
		user: &store.User{ID: "u1", Email: "dev@acme.test", DisplayName: "Dev"},
		grants: []store.PrincipalGrant{
			{Role: domain.RoleEditor, PublisherID: "p1", PublisherSlug: "acme", PublisherName: "Acme"},
			{Role: domain.RoleReviewer}, // global grant: empty publisher fields
		},
	}
	h := handlers.NewMeHandlers(st)

	ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{
		UserID: "u1", Email: "dev@acme.test", AuthMethod: "local",
	})
	rr := httptest.NewRecorder()
	h.Me(rr, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil).WithContext(ctx))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rr.Code, rr.Body.String())
	}
	var got meBody
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Authenticated || got.UserID != "u1" || got.Email != "dev@acme.test" || got.DisplayName != "Dev" {
		t.Errorf("identity = %+v, want u1/dev@acme.test/Dev", got)
	}
	if got.IsServerAdmin {
		t.Errorf("is_server_admin = true, want false for a plain editor")
	}
	if got.Issuer != "local" {
		t.Errorf("issuer = %q, want local", got.Issuer)
	}
	if len(got.Grants) != 2 {
		t.Fatalf("grants = %d, want 2", len(got.Grants))
	}
}

func TestMeHandler_reports_server_admin(t *testing.T) {
	h := handlers.NewMeHandlers(&fakeMeStore{user: &store.User{ID: "admin", Email: "root@acme.test", IsServerAdmin: true}})
	ctx := auth.ContextWithPrincipal(context.Background(), &auth.Principal{
		UserID: "admin", Email: "root@acme.test", IsServerAdmin: true, AuthMethod: "local",
	})
	rr := httptest.NewRecorder()
	h.Me(rr, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil).WithContext(ctx))

	var got meBody
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.IsServerAdmin {
		t.Errorf("is_server_admin = false, want true")
	}
}
