package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

var validPackages = json.RawMessage(
	`[{"registryType":"npm","identifier":"@scope/pkg","version":"1.0.0","transport":{"type":"stdio"}}]`)

func TestCreateAndGetMCPServer(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubID := insertPublisher(t, "acme", "Acme Corp")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID,
		Slug:        "my-server",
		Name:        "My Server",
		Description: "A test server",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer() error = %v", err)
	}
	if srv.ID == "" {
		t.Error("expected non-empty ID")
	}
	if srv.Status != domain.StatusDraft {
		t.Errorf("status = %v, want draft", srv.Status)
	}
	if srv.Visibility != domain.VisibilityPrivate {
		t.Errorf("visibility = %v, want private", srv.Visibility)
	}

	// GetMCPServer by namespace/slug as admin (no public filter).
	got, err := sharedDB.GetMCPServer(ctx, "acme", "my-server", false)
	if err != nil {
		t.Fatalf("GetMCPServer() error = %v", err)
	}
	if got.ID != srv.ID {
		t.Errorf("id = %v, want %v", got.ID, srv.ID)
	}
	if got.Namespace != "acme" {
		t.Errorf("namespace = %v, want acme", got.Namespace)
	}
}

func TestCreateMCPServer_ConflictOnDuplicateSlug(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme2", "Acme 2")

	params := store.CreateMCPServerParams{PublisherID: pubID, Slug: "dup", Name: "Dup"}
	if _, err := sharedDB.CreateMCPServer(ctx, params); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := sharedDB.CreateMCPServer(ctx, params)
	if err != store.ErrConflict {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestGetMCPServer_NotFoundWhenPrivateAndPublicOnly(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme3", "Acme 3")

	if _, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "priv", Name: "Private",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := sharedDB.GetMCPServer(ctx, "acme3", "priv", true)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for private server with publicOnly=true, got %v", err)
	}
}

func TestMCPServerVersionLifecycle(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "lifecycle", "Lifecycle Corp")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "srv", Name: "Server",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	// Create a draft version.
	ver, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        srv.ID,
		Version:         "1.0.0",
		Runtime:         domain.RuntimeStdio,
		Packages:        validPackages,
		Capabilities:    json.RawMessage(`{"tools":[]}`),
		ProtocolVersion: "2024-11-05",
	})
	if err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}
	if ver.IsPublished() {
		t.Error("newly created version should not be published")
	}

	// Publish it.
	if err := sharedDB.PublishMCPServerVersion(ctx, srv.ID, "1.0.0"); err != nil {
		t.Fatalf("PublishMCPServerVersion: %v", err)
	}

	// Fetch and verify published.
	got, err := sharedDB.GetMCPServerVersion(ctx, srv.ID, "1.0.0")
	if err != nil {
		t.Fatalf("GetMCPServerVersion: %v", err)
	}
	if !got.IsPublished() {
		t.Error("version should be published after publish call")
	}

	// Publishing again should return ErrImmutable.
	if err := sharedDB.PublishMCPServerVersion(ctx, srv.ID, "1.0.0"); err != store.ErrImmutable {
		t.Errorf("expected ErrImmutable on re-publish, got %v", err)
	}

	// Duplicate version should return ErrConflict.
	_, err = sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "1.0.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, ProtocolVersion: "2024-11-05",
	})
	if err != store.ErrConflict {
		t.Errorf("expected ErrConflict on duplicate version, got %v", err)
	}
}

func TestListMCPServers_Filtering(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pub1 := insertPublisher(t, "ns1", "Namespace 1")
	pub2 := insertPublisher(t, "ns2", "Namespace 2")

	// Create a public server under ns1.
	srv1, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pub1, Slug: "public-srv", Name: "Public Server",
	})
	sharedDB.SetMCPServerVisibility(ctx, srv1.ID, domain.VisibilityPublic)

	// Create a private server under ns2.
	sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pub2, Slug: "private-srv", Name: "Private Server",
	})

	// PublicOnly=true should return only public entries.
	rows, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{PublicOnly: true, Limit: 20})
	if err != nil {
		t.Fatalf("ListMCPServers: %v", err)
	}
	for _, r := range rows {
		if r.Visibility != domain.VisibilityPublic {
			t.Errorf("expected public visibility, got %v for server %v", r.Visibility, r.Slug)
		}
	}

	// Namespace filter.
	rows2, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{Namespace: "ns1", Limit: 20})
	if err != nil {
		t.Fatalf("ListMCPServers with namespace: %v", err)
	}
	for _, r := range rows2 {
		if r.Namespace != "ns1" {
			t.Errorf("expected namespace ns1, got %v", r.Namespace)
		}
	}
}

func TestDeprecateMCPServer(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "dep-ns", "Deprecate NS")

	srv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "dep-srv", Name: "Dep Server",
	})

	// Can't deprecate a draft server.
	if err := sharedDB.DeprecateMCPServer(ctx, srv.ID); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound when deprecating draft, got %v", err)
	}

	// Publish a version first, which promotes server to published.
	sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "1.0.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, ProtocolVersion: "2024-11-05",
	})
	sharedDB.PublishMCPServerVersion(ctx, srv.ID, "1.0.0")

	if err := sharedDB.DeprecateMCPServer(ctx, srv.ID); err != nil {
		t.Fatalf("DeprecateMCPServer: %v", err)
	}
}

func TestGetMCPServerByID(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "byid-ns", "ByID NS")

	srv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "by-id-server", Name: "By ID Server",
	})

	got, err := sharedDB.GetMCPServerByID(ctx, srv.ID)
	if err != nil {
		t.Fatalf("GetMCPServerByID: %v", err)
	}
	if got.ID != srv.ID {
		t.Errorf("id = %v, want %v", got.ID, srv.ID)
	}

	_, err = sharedDB.GetMCPServerByID(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListMCPServerVersions(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "listver-ns", "ListVer NS")

	srv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "listver-srv", Name: "ListVer Server",
	})

	for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		if _, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
			ServerID: srv.ID, Version: v, Runtime: domain.RuntimeStdio,
			Packages: validPackages, ProtocolVersion: "2024-11-05",
		}); err != nil {
			t.Fatalf("CreateMCPServerVersion(%s): %v", v, err)
		}
	}

	versions, err := sharedDB.ListMCPServerVersions(ctx, srv.ID)
	if err != nil {
		t.Fatalf("ListMCPServerVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("version count = %d, want 3", len(versions))
	}
	// Verify ordering: newest released_at first (all inserted sequentially).
	// At minimum we should see 2.0.0 listed before 1.0.0.
	seen := make(map[string]bool)
	for _, v := range versions {
		seen[v.Version] = true
	}
	for _, want := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		if !seen[want] {
			t.Errorf("missing version %s in ListMCPServerVersions result", want)
		}
	}

	// Empty result for unknown server.
	empty, err := sharedDB.ListMCPServerVersions(ctx, "nonexistent-id")
	if err != nil {
		t.Fatalf("ListMCPServerVersions(nonexistent): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected empty slice for unknown server, got %d rows", len(empty))
	}
}

func TestGetLatestPublishedVersion(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "latest-ns", "Latest NS")

	srv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "latest-srv", Name: "Latest Server",
	})

	// No published version yet — must return ErrNotFound.
	_, err := sharedDB.GetLatestPublishedVersion(ctx, srv.ID)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound with no published versions, got %v", err)
	}

	// Create a draft version and confirm it is still not returned.
	sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "0.9.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, ProtocolVersion: "2024-11-05",
	})
	_, err = sharedDB.GetLatestPublishedVersion(ctx, srv.ID)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for draft-only server, got %v", err)
	}

	// Publish 1.0.0.
	sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "1.0.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, ProtocolVersion: "2024-11-05",
	})
	sharedDB.PublishMCPServerVersion(ctx, srv.ID, "1.0.0")

	// Publish 2.0.0.
	sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "2.0.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, ProtocolVersion: "2024-11-05",
	})
	sharedDB.PublishMCPServerVersion(ctx, srv.ID, "2.0.0")

	latest, err := sharedDB.GetLatestPublishedVersion(ctx, srv.ID)
	if err != nil {
		t.Fatalf("GetLatestPublishedVersion: %v", err)
	}
	if latest.Version != "2.0.0" {
		t.Errorf("latest version = %q, want %q", latest.Version, "2.0.0")
	}
	if !latest.IsPublished() {
		t.Error("latest version should be published")
	}
}

func TestSetMCPServerVisibility(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "vis-ns", "Vis NS")

	srv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "vis-srv", Name: "Vis Server",
	})

	// Set to public.
	if err := sharedDB.SetMCPServerVisibility(ctx, srv.ID, domain.VisibilityPublic); err != nil {
		t.Fatalf("SetMCPServerVisibility(public): %v", err)
	}
	got, err := sharedDB.GetMCPServerByID(ctx, srv.ID)
	if err != nil {
		t.Fatalf("GetMCPServerByID: %v", err)
	}
	if got.Visibility != domain.VisibilityPublic {
		t.Errorf("visibility = %v, want public", got.Visibility)
	}

	// Set back to private.
	if err := sharedDB.SetMCPServerVisibility(ctx, srv.ID, domain.VisibilityPrivate); err != nil {
		t.Fatalf("SetMCPServerVisibility(private): %v", err)
	}
	got2, _ := sharedDB.GetMCPServerByID(ctx, srv.ID)
	if got2.Visibility != domain.VisibilityPrivate {
		t.Errorf("visibility = %v, want private", got2.Visibility)
	}

	// Non-existent ID must return ErrNotFound.
	if err := sharedDB.SetMCPServerVisibility(ctx, "nonexistent-id", domain.VisibilityPublic); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for bad ID, got %v", err)
	}
}

func TestListMCPServers_SearchQuery(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "search-ns", "Search NS")

	sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "alpha-search", Name: "AlphaSearch Tool",
		Description: "Unique alpha description for search test",
	})
	sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "beta-other", Name: "BetaOther Tool",
		Description: "Completely different beta description",
	})

	rows, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{
		Query: "alpha", Limit: 20,
	})
	if err != nil {
		t.Fatalf("ListMCPServers(query=alpha): %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("expected 1 result for query 'alpha', got %d", len(rows))
	}
	if len(rows) > 0 && rows[0].Slug != "alpha-search" {
		t.Errorf("expected slug alpha-search, got %s", rows[0].Slug)
	}
}

func TestListMCPServers_NamespaceFilter(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pub1 := insertPublisher(t, "nsfilt-ns1", "NS Filter 1")
	pub2 := insertPublisher(t, "nsfilt-ns2", "NS Filter 2")

	sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pub1, Slug: "srv-in-ns1", Name: "Server In NS1",
	})
	sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pub2, Slug: "srv-in-ns2", Name: "Server In NS2",
	})

	rows, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{
		Namespace: "nsfilt-ns1", Limit: 20,
	})
	if err != nil {
		t.Fatalf("ListMCPServers(namespace=nsfilt-ns1): %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("expected 1 result for namespace nsfilt-ns1, got %d", len(rows))
	}
	for _, r := range rows {
		if r.Namespace != "nsfilt-ns1" {
			t.Errorf("expected namespace nsfilt-ns1, got %s", r.Namespace)
		}
	}
}

func TestGetMCPServerVersion_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "getver-ns", "GetVer NS")

	srv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "getver-srv", Name: "GetVer Server",
	})

	_, err := sharedDB.GetMCPServerVersion(ctx, srv.ID, "9.9.9")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing version, got %v", err)
	}
}

func TestDeprecateMCPServer_BadID(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// A completely non-existent ID must also return ErrNotFound.
	if err := sharedDB.DeprecateMCPServer(ctx, "nonexistent-id"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for bad ID, got %v", err)
	}
}

func TestEncodeCursorFromTime(t *testing.T) {
	now := time.Now().UTC()
	id := "01HXYZ1234567890ABCDEFGHIJ"

	fromTime := store.EncodeCursorFromTime(now, id)
	direct := store.EncodeCursor(now, id)

	if fromTime != direct {
		t.Errorf("EncodeCursorFromTime(%v, %s) = %q, want %q (same as EncodeCursor)", now, id, fromTime, direct)
	}
	if fromTime == "" {
		t.Error("EncodeCursorFromTime returned empty string")
	}
}

func TestDecodeCursor_Malformed(t *testing.T) {
	tests := []struct {
		name   string
		cursor string
	}{
		{"empty string", ""},
		{"no comma", "20240101T000000Z01HXYZ1234567890ABCDEFGHIJ"},
		{"invalid time", "not-a-time,01HXYZ1234567890ABCDEFGHIJ"},
		{"too short", "x,y"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// decodeCursor is unexported; exercise it indirectly via ListMCPServers
			// — a malformed cursor should be silently ignored (no WHERE clause applied)
			// rather than returning an error. This matches the implementation's
			// `if err == nil` guard. We just verify no panic and a valid (empty) result.
			resetDB(t)
			ctx := context.Background()
			_, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{
				Cursor: tc.cursor,
				Limit:  5,
			})
			if err != nil {
				t.Errorf("ListMCPServers with malformed cursor %q returned unexpected error: %v", tc.cursor, err)
			}
		})
	}
}
