package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

// ── routers ──────────────────────────────────────────────────────────────────

func newGroupRouter() *chi.Mux {
	h := handlers.NewGroupHandlers(testDB, testDB)
	r := chi.NewRouter()
	r.Get("/api/v1/groups", h.ListGroups)
	r.Post("/api/v1/groups", h.CreateGroup)
	r.Get("/api/v1/groups/{slug}", h.GetGroup)
	r.Patch("/api/v1/groups/{slug}", h.PatchGroup)
	r.Delete("/api/v1/groups/{slug}", h.DeleteGroup)
	r.Get("/api/v1/groups/{slug}/members", h.ListMembers)
	r.Put("/api/v1/groups/{slug}/members/{email}", h.PutMember)
	r.Delete("/api/v1/groups/{slug}/members/{email}", h.DeleteMember)
	return r
}

func newUserRouter() *chi.Mux {
	h := handlers.NewUserHandlers(testDB, testDB)
	r := chi.NewRouter()
	r.Get("/api/v1/users", h.ListUsers)
	r.Post("/api/v1/users", h.CreateUser)
	r.Get("/api/v1/users/{id}", h.GetUser)
	r.Patch("/api/v1/users/{id}", h.PatchUser)
	r.Post("/api/v1/users/{id}/set-password", h.SetPassword)
	return r
}

func newGrantRouter() *chi.Mux {
	h := handlers.NewGrantHandlers(testDB, testDB)
	r := chi.NewRouter()
	r.Get("/api/v1/grants", h.ListGlobalGrants)
	r.Post("/api/v1/grants", h.CreateGlobalGrant)
	r.Delete("/api/v1/grants/{id}", h.DeleteGlobalGrant)
	r.Get("/api/v1/publishers/{slug}/grants", h.ListPublisherGrants)
	r.Post("/api/v1/publishers/{slug}/grants", h.CreatePublisherGrant)
	r.Delete("/api/v1/publishers/{slug}/grants/{id}", h.DeletePublisherGrant)
	return r
}

func jsonReq(method, url, body string) *http.Request {
	req := httptest.NewRequest(method, url, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ── groups ───────────────────────────────────────────────────────────────────

func TestGroupHandler_CRUD(t *testing.T) {
	resetTables(t)
	router := newGroupRouter()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/groups", `{"slug":"platform","name":"Platform","description":"infra"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d; %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/groups/platform", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPatch, "/api/v1/groups/platform", `{"name":"Platform Team"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d; %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/groups/platform", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/groups/platform", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: %d, want 404", rec.Code)
	}
}

func TestGroupHandler_DuplicateConflict(t *testing.T) {
	resetTables(t)
	router := newGroupRouter()
	router.ServeHTTP(httptest.NewRecorder(), jsonReq(http.MethodPost, "/api/v1/groups", `{"slug":"dup","name":"A"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/groups", `{"slug":"dup","name":"B"}`))
	if rec.Code != http.StatusConflict {
		t.Errorf("duplicate: %d, want 409", rec.Code)
	}
}

func TestGroupHandler_PutMemberAutoCreatesUser(t *testing.T) {
	resetTables(t)
	router := newGroupRouter()
	router.ServeHTTP(httptest.NewRecorder(), jsonReq(http.MethodPost, "/api/v1/groups", `{"slug":"team","name":"Team"}`))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/groups/team/members/new@example.com", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("put member: %d; %s", rec.Code, rec.Body.String())
	}
	// The auto-created user now exists.
	if _, err := testDB.GetUserByEmail(context.Background(), "new@example.com"); err != nil {
		t.Errorf("member-add should auto-create the user: %v", err)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/groups/team/members/new@example.com", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete member: %d, want 204", rec.Code)
	}
}

// TestGroupHandler_PutMemberDecodesPercentEncodedEmail pins the fix for a bug
// the mocked-client unit tests missed: a real browser client (openapi-fetch)
// percent-encodes the {email} path segment, so "a@b.com" arrives as
// "a%40b.com". The handler must URL-decode it before storing/matching, or the
// member is created under a mangled "a%40b.com" address. Drive the encoded
// form end-to-end and assert the user is stored — and listed — with the real
// "@" address.
func TestGroupHandler_PutMemberDecodesPercentEncodedEmail(t *testing.T) {
	resetTables(t)
	router := newGroupRouter()
	router.ServeHTTP(httptest.NewRecorder(), jsonReq(http.MethodPost, "/api/v1/groups", `{"slug":"team","name":"Team"}`))

	const realEmail = "encoded@example.com"
	rec := httptest.NewRecorder()
	// %40 is the percent-encoding of "@" — exactly what the SPA sends.
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/groups/team/members/encoded%40example.com", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("put member (encoded): %d; %s", rec.Code, rec.Body.String())
	}

	// The stored user carries the decoded address, not "encoded%40example.com".
	if _, err := testDB.GetUserByEmail(context.Background(), realEmail); err != nil {
		t.Errorf("member should be stored under the decoded email %q: %v", realEmail, err)
	}

	// The members listing shows the decoded address.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/groups/team/members", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list members: %d", rec.Code)
	}
	var body struct {
		Items []struct {
			Email string `json:"email"`
		} `json:"items"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if len(body.Items) != 1 || body.Items[0].Email != realEmail {
		t.Errorf("members = %+v, want exactly one with email %q", body.Items, realEmail)
	}

	// And the encoded form deletes it (DeleteMember decodes too).
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/groups/team/members/encoded%40example.com", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete member (encoded): %d, want 204", rec.Code)
	}
}

func TestGroupHandler_ListMembers(t *testing.T) {
	resetTables(t)
	router := newGroupRouter()
	router.ServeHTTP(httptest.NewRecorder(), jsonReq(http.MethodPost, "/api/v1/groups", `{"slug":"team","name":"Team"}`))

	// Add two members by email (auto-creates the users).
	for _, email := range []string{"a@x.test", "b@x.test"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/groups/team/members/"+email, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("put member %s: %d", email, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/groups/team/members", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list members: %d", rec.Code)
	}
	var body struct {
		Items []struct {
			Email string `json:"email"`
		} `json:"items"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if len(body.Items) != 2 {
		t.Errorf("members = %d, want 2", len(body.Items))
	}

	// Unknown group → 404.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/groups/ghost/members", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("list members of unknown group: %d, want 404", rec.Code)
	}
}

// ── users ────────────────────────────────────────────────────────────────────

func TestUserHandler_CreateGetPatch(t *testing.T) {
	resetTables(t)
	router := newUserRouter()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/users", `{"email":"u@example.com","password":"hunter2hunter2","is_server_admin":true}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d; %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID          string `json:"id"`
		HasPassword bool   `json:"has_password"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&created)
	if !created.HasPassword {
		t.Error("has_password should be true")
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/users/"+created.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPatch, "/api/v1/users/"+created.ID, `{"disabled":true}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d; %s", rec.Code, rec.Body.String())
	}
	var patched struct {
		Disabled bool `json:"disabled"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&patched)
	if !patched.Disabled {
		t.Error("disabled should be true after patch")
	}
}

func TestUserHandler_CreateDuplicateConflict(t *testing.T) {
	resetTables(t)
	router := newUserRouter()
	router.ServeHTTP(httptest.NewRecorder(), jsonReq(http.MethodPost, "/api/v1/users", `{"email":"dup@example.com"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/users", `{"email":"DUP@example.com"}`))
	if rec.Code != http.StatusConflict {
		t.Errorf("duplicate email: %d, want 409", rec.Code)
	}
}

func TestUserHandler_SetPassword(t *testing.T) {
	resetTables(t)
	router := newUserRouter()
	u, err := testDB.CreateUser(context.Background(), store.CreateUserParams{Email: "target@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	adminCtx := func(r *http.Request) *http.Request {
		ctx := auth.ContextWithClaims(r.Context(), &auth.KeycloakClaims{RealmAccess: auth.RealmAccess{Roles: []string{"admin"}}})
		return r.WithContext(ctx)
	}
	selfCtx := func(r *http.Request) *http.Request {
		ctx := auth.ContextWithClaims(r.Context(), &auth.KeycloakClaims{})
		ctx = auth.ContextWithPrincipal(ctx, &auth.Principal{UserID: u.ID})
		return r.WithContext(ctx)
	}
	otherCtx := func(r *http.Request) *http.Request {
		ctx := auth.ContextWithClaims(r.Context(), &auth.KeycloakClaims{})
		ctx = auth.ContextWithPrincipal(ctx, &auth.Principal{UserID: "someone-else"})
		return r.WithContext(ctx)
	}

	// Unauthenticated → 401.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/users/"+u.ID+"/set-password", `{"password":"longenough1"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauth: %d, want 401", rec.Code)
	}

	// A different non-admin principal → 403.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, otherCtx(jsonReq(http.MethodPost, "/api/v1/users/"+u.ID+"/set-password", `{"password":"longenough1"}`)))
	if rec.Code != http.StatusForbidden {
		t.Errorf("other: %d, want 403", rec.Code)
	}

	// Self → 204.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, selfCtx(jsonReq(http.MethodPost, "/api/v1/users/"+u.ID+"/set-password", `{"password":"longenough1"}`)))
	if rec.Code != http.StatusNoContent {
		t.Errorf("self: %d, want 204; %s", rec.Code, rec.Body.String())
	}

	// Admin → 204; and too-short password → 422.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, adminCtx(jsonReq(http.MethodPost, "/api/v1/users/"+u.ID+"/set-password", `{"password":"short"}`)))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("short password: %d, want 422", rec.Code)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, adminCtx(jsonReq(http.MethodPost, "/api/v1/users/"+u.ID+"/set-password", `{"password":"adminsetpw1"}`)))
	if rec.Code != http.StatusNoContent {
		t.Errorf("admin: %d, want 204", rec.Code)
	}
}

// ── grants ───────────────────────────────────────────────────────────────────

func TestGrantHandler_GlobalCRUD(t *testing.T) {
	resetTables(t)
	router := newGrantRouter()
	g, _ := testDB.CreateGroup(context.Background(), store.CreateGroupParams{Slug: "revs", Name: "Reviewers"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/grants",
		`{"principal_type":"group","principal_id":"`+g.ID+`","role":"reviewer"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create global grant: %d; %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&created)

	// Duplicate → 409.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/grants",
		`{"principal_type":"group","principal_id":"`+g.ID+`","role":"reviewer"}`))
	if rec.Code != http.StatusConflict {
		t.Errorf("duplicate grant: %d, want 409", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/grants", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/grants/"+created.ID, nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d, want 204", rec.Code)
	}
}

func TestGrantHandler_PublisherGrant(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")
	router := newGrantRouter()
	u, _ := testDB.CreateUser(context.Background(), store.CreateUserParams{Email: "editor@acme.test"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/publishers/acme/grants",
		`{"principal_type":"user","principal_id":"`+u.ID+`","role":"editor"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create publisher grant: %d; %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/publishers/acme/grants", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list publisher grants: %d", rec.Code)
	}
	var list struct {
		Items []struct {
			PrincipalLabel string `json:"principal_label"`
		} `json:"items"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list.Items) != 1 || list.Items[0].PrincipalLabel != "editor@acme.test" {
		t.Errorf("publisher grants = %+v, want one labelled editor@acme.test", list.Items)
	}
}

func TestGrantHandler_Validation(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")
	router := newGrantRouter()

	// Invalid role.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/grants",
		`{"principal_type":"group","principal_id":"x","role":"superuser"}`))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid role: %d, want 422", rec.Code)
	}

	// Unknown principal.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/grants",
		`{"principal_type":"group","principal_id":"does-not-exist","role":"reviewer"}`))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown principal: %d, want 422", rec.Code)
	}

	// Unknown publisher.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, jsonReq(http.MethodPost, "/api/v1/publishers/ghost/grants",
		`{"principal_type":"group","principal_id":"x","role":"editor"}`))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown publisher: %d, want 404", rec.Code)
	}
}
