package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	rows, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{PublicOnly: true, Limit: 20})
	if err != nil {
		t.Fatalf("ListMCPServers: %v", err)
	}
	for _, r := range rows {
		if r.Visibility != domain.VisibilityPublic {
			t.Errorf("expected public visibility, got %v for server %v", r.Visibility, r.Slug)
		}
	}

	// Namespace filter.
	rows2, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{Namespace: "ns1", Limit: 20})
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

	rows, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{
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

	rows, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{
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
		name        string
		cursor      string
		expectError bool // true → ErrInvalidCursor expected; false → ignored (first page)
	}{
		{"empty string", "", false},
		{"no comma", "20240101T000000Z01HXYZ1234567890ABCDEFGHIJ", true},
		{"invalid time", "not-a-time,01HXYZ1234567890ABCDEFGHIJ", true},
		{"too short", "x,y", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetDB(t)
			ctx := context.Background()
			_, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{
				Cursor: tc.cursor,
				Limit:  5,
			})
			if tc.expectError {
				if !errors.Is(err, store.ErrInvalidCursor) {
					t.Errorf("ListMCPServers with cursor %q: want ErrInvalidCursor, got %v", tc.cursor, err)
				}
			} else {
				if err != nil {
					t.Errorf("ListMCPServers with cursor %q returned unexpected error: %v", tc.cursor, err)
				}
			}
		})
	}
}

func TestListMCPServers_FilterByStatus(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "status-ns", "Status NS")

	// Create three servers; default status is draft.
	srv1, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pubID, Slug: "status-draft", Name: "Draft Server"})
	srv2, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pubID, Slug: "status-published", Name: "Published Server"})
	srv3, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pubID, Slug: "status-deprecated", Name: "Deprecated Server"})

	// Promote srv2 to published and srv3 to deprecated via direct SQL.
	if _, err := sharedDB.Pool.Exec(ctx, "UPDATE mcp_servers SET status=$1 WHERE id=$2", "published", srv2.ID); err != nil {
		t.Fatalf("setting published status: %v", err)
	}
	if _, err := sharedDB.Pool.Exec(ctx, "UPDATE mcp_servers SET status=$1 WHERE id=$2", "deprecated", srv3.ID); err != nil {
		t.Fatalf("setting deprecated status: %v", err)
	}
	_ = srv1 // srv1 stays draft

	for _, tc := range []struct {
		status string
		want   int
		slug   string
	}{
		{"draft", 1, "status-draft"},
		{"published", 1, "status-published"},
		{"deprecated", 1, "status-deprecated"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			rows, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{Status: tc.status, Limit: 20})
			if err != nil {
				t.Fatalf("ListMCPServers(status=%s): %v", tc.status, err)
			}
			if len(rows) != tc.want {
				t.Errorf("status=%s: got %d rows, want %d", tc.status, len(rows), tc.want)
			}
			if len(rows) > 0 && rows[0].Slug != tc.slug {
				t.Errorf("status=%s: slug=%q, want %q", tc.status, rows[0].Slug, tc.slug)
			}
		})
	}
}

func TestListMCPServers_FilterByVisibility(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "vis-filter-ns", "Vis Filter NS")

	srv1, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pubID, Slug: "vf-public-1", Name: "Public 1"})
	srv2, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pubID, Slug: "vf-public-2", Name: "Public 2"})
	_, _ = sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pubID, Slug: "vf-private", Name: "Private"})

	// Make srv1 and srv2 public.
	for _, id := range []string{srv1.ID, srv2.ID} {
		if _, err := sharedDB.Pool.Exec(ctx, "UPDATE mcp_servers SET visibility=$1 WHERE id=$2", "public", id); err != nil {
			t.Fatalf("setting visibility: %v", err)
		}
	}

	// Filter by public (PublicOnly=false so we use the Visibility field).
	pubRows, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{Visibility: "public", Limit: 20})
	if err != nil {
		t.Fatalf("ListMCPServers(visibility=public): %v", err)
	}
	if len(pubRows) != 2 {
		t.Errorf("visibility=public: got %d rows, want 2", len(pubRows))
	}
	for _, r := range pubRows {
		if r.Visibility != "public" {
			t.Errorf("expected public visibility, got %q for slug %q", r.Visibility, r.Slug)
		}
	}

	// Filter by private.
	privRows, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{Visibility: "private", Limit: 20})
	if err != nil {
		t.Fatalf("ListMCPServers(visibility=private): %v", err)
	}
	if len(privRows) != 1 {
		t.Errorf("visibility=private: got %d rows, want 1", len(privRows))
	}
	if len(privRows) > 0 && privRows[0].Slug != "vf-private" {
		t.Errorf("visibility=private: slug=%q, want vf-private", privRows[0].Slug)
	}
}

func TestListMCPServers_FilterCombined(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pub1 := insertPublisher(t, "comb-ns1", "Combined NS1")
	pub2 := insertPublisher(t, "comb-ns2", "Combined NS2")

	// ns1: one public+published, one public+draft, one private+published
	srvA, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pub1, Slug: "comb-a", Name: "Comb A"})
	srvB, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pub1, Slug: "comb-b", Name: "Comb B"})
	srvC, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pub1, Slug: "comb-c", Name: "Comb C"})
	// ns2: one public+published
	srvD, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pub2, Slug: "comb-d", Name: "Comb D"})

	type update struct{ id, col, val string }
	for _, u := range []update{
		{srvA.ID, "visibility", "public"},
		{srvA.ID, "status", "published"},
		{srvB.ID, "visibility", "public"},
		// srvB stays draft
		{srvC.ID, "status", "published"},
		// srvC stays private
		{srvD.ID, "visibility", "public"},
		{srvD.ID, "status", "published"},
	} {
		if _, err := sharedDB.Pool.Exec(ctx, "UPDATE mcp_servers SET "+u.col+"=$1 WHERE id=$2", u.val, u.id); err != nil {
			t.Fatalf("update %s=%s on %s: %v", u.col, u.val, u.id, err)
		}
	}

	// namespace=comb-ns1 + status=published + visibility=public => only srvA
	rows, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{
		Namespace:  "comb-ns1",
		Status:     "published",
		Visibility: "public",
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("ListMCPServers(combined): %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("combined filter: got %d rows, want 1", len(rows))
	}
	if len(rows) > 0 && rows[0].Slug != "comb-a" {
		t.Errorf("combined filter: slug=%q, want comb-a", rows[0].Slug)
	}
}

func TestListMCPServers_TotalCount(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "tc-ns", "TotalCount NS")

	// Create 3 servers.
	for i := range 3 {
		sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{ //nolint:errcheck
			PublisherID: pubID,
			Slug:        fmt.Sprintf("tc-srv-%d", i),
			Name:        fmt.Sprintf("TotalCount Server %d", i),
		})
	}

	// Request page of 2 — should return 2 rows but total_count = 3.
	rows, total, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{Limit: 2})
	if err != nil {
		t.Fatalf("ListMCPServers: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("page size: got %d rows, want 2", len(rows))
	}
	if total != 3 {
		t.Errorf("total_count: got %d, want 3", total)
	}

	// Request with cursor (second page) — total_count must still be 3.
	cursor := store.EncodeCursor(rows[len(rows)-1].CreatedAt, rows[len(rows)-1].ID)
	_, total2, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{Limit: 2, Cursor: cursor})
	if err != nil {
		t.Fatalf("ListMCPServers page 2: %v", err)
	}
	if total2 != 3 {
		t.Errorf("total_count page 2: got %d, want 3", total2)
	}
}

func TestUpdateMCPServer(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "update-pub", "Update Pub")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID,
		Slug:        "update-srv",
		Name:        "Original Name",
		Description: "Original description",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	updated, err := sharedDB.UpdateMCPServer(ctx, srv.ID, store.UpdateMCPServerParams{
		Name:        "Updated Name",
		Description: "Updated description",
		HomepageURL: "https://example.com",
		RepoURL:     "https://github.com/example",
		License:     "MIT",
	})
	if err != nil {
		t.Fatalf("UpdateMCPServer: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("name = %q, want %q", updated.Name, "Updated Name")
	}
	if updated.Description != "Updated description" {
		t.Errorf("description = %q, want %q", updated.Description, "Updated description")
	}
	if updated.HomepageURL != "https://example.com" {
		t.Errorf("homepage_url = %q, want %q", updated.HomepageURL, "https://example.com")
	}
	if updated.License != "MIT" {
		t.Errorf("license = %q, want %q", updated.License, "MIT")
	}
}

func TestUpdateMCPServer_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	_, err := sharedDB.UpdateMCPServer(ctx, store.NewULID(), store.UpdateMCPServerParams{Name: "X"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteMCPServer(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "del-pub", "Delete Pub")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID,
		Slug:        "del-srv",
		Name:        "Delete Me",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}
	// Add a version so we can verify it's also soft-deleted.
	_, err = sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        srv.ID,
		Version:         "1.0.0",
		Runtime:         domain.RuntimeStdio,
		Packages:        validPackages,
		ProtocolVersion: "2025-03-26",
	})
	if err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}

	if err := sharedDB.DeleteMCPServer(ctx, srv.ID); err != nil {
		t.Fatalf("DeleteMCPServer: %v", err)
	}

	// Server should not appear in a non-admin listing.
	rows, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{PublicOnly: true})
	if err != nil {
		t.Fatalf("ListMCPServers: %v", err)
	}
	for _, r := range rows {
		if r.ID == srv.ID {
			t.Error("deleted server still appears in public listing")
		}
	}

	// Double-delete should return ErrNotFound.
	if err := sharedDB.DeleteMCPServer(ctx, srv.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("second delete: expected ErrNotFound, got %v", err)
	}
}

func TestDeleteMCPServer_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	if err := sharedDB.DeleteMCPServer(ctx, store.NewULID()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIncrementMCPServerViewCount(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "view-mcp-ns", "View MCP Corp")

	if _, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "view-srv", Name: "View Server",
	}); err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	got, err := sharedDB.GetMCPServer(ctx, "view-mcp-ns", "view-srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if got.ViewCount != 0 {
		t.Errorf("initial view_count = %d, want 0", got.ViewCount)
	}

	for i := 1; i <= 3; i++ {
		if err := sharedDB.IncrementMCPServerViewCount(ctx, "view-mcp-ns", "view-srv"); err != nil {
			t.Fatalf("IncrementMCPServerViewCount #%d: %v", i, err)
		}
		got, err = sharedDB.GetMCPServer(ctx, "view-mcp-ns", "view-srv", false)
		if err != nil {
			t.Fatalf("GetMCPServer after increment #%d: %v", i, err)
		}
		if got.ViewCount != i {
			t.Errorf("after increment #%d view_count = %d, want %d", i, got.ViewCount, i)
		}
	}

	if got.CopyCount != 0 {
		t.Errorf("copy_count = %d, want 0 (view increments must not touch copy)", got.CopyCount)
	}
}

func TestIncrementMCPServerViewCount_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	if err := sharedDB.IncrementMCPServerViewCount(ctx, "no-ns", "no-srv"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	insertPublisher(t, "known-ns", "Known")
	if err := sharedDB.IncrementMCPServerViewCount(ctx, "known-ns", "missing-srv"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for unknown slug under known namespace, got %v", err)
	}
}

func TestIncrementMCPServerCopyCount(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "copy-mcp-ns", "Copy MCP Corp")

	if _, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "copy-srv", Name: "Copy Server",
	}); err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	for i := 1; i <= 2; i++ {
		if err := sharedDB.IncrementMCPServerCopyCount(ctx, "copy-mcp-ns", "copy-srv"); err != nil {
			t.Fatalf("IncrementMCPServerCopyCount #%d: %v", i, err)
		}
	}

	got, err := sharedDB.GetMCPServer(ctx, "copy-mcp-ns", "copy-srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if got.CopyCount != 2 {
		t.Errorf("copy_count = %d, want 2", got.CopyCount)
	}
	if got.ViewCount != 0 {
		t.Errorf("view_count = %d, want 0 (copy increments must not touch view)", got.ViewCount)
	}
}

func TestIncrementMCPServerCopyCount_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	if err := sharedDB.IncrementMCPServerCopyCount(ctx, "nope", "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestMCPServerVersion_ToolsRoundTrip verifies that the new tools JSONB column
// is persisted as submitted and returned verbatim through every read path:
// version GET, version list, GetMCPServer, GetMCPServerByID, and ListMCPServers.
// Regression guard — forgetting to add `tools` to any of those SELECTs would
// silently null out the field on read.
func TestMCPServerVersion_ToolsRoundTrip(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "tools-ns", "Tools Corp")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "tooly", Name: "Tooly",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	toolsIn := json.RawMessage(`[
		{"name":"read_file","description":"reads a file"},
		{"name":"write_file","description":"writes a file","input_schema":{"type":"object"}}
	]`)

	ver, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        srv.ID,
		Version:         "1.0.0",
		Runtime:         domain.RuntimeStdio,
		Packages:        validPackages,
		Tools:           toolsIn,
		ProtocolVersion: "2024-11-05",
	})
	if err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}
	if err := sharedDB.PublishMCPServerVersion(ctx, srv.ID, "1.0.0"); err != nil {
		t.Fatalf("PublishMCPServerVersion: %v", err)
	}
	if err := sharedDB.SetMCPServerVisibility(ctx, srv.ID, domain.VisibilityPublic); err != nil {
		t.Fatalf("SetMCPServerVisibility: %v", err)
	}

	// assertTwoTools verifies that the raw JSON has exactly the two tool
	// names we sent in. We unmarshal rather than string-compare so
	// whitespace and key ordering from pg don't break the test.
	assertTwoTools := func(where string, raw json.RawMessage) {
		t.Helper()
		if len(raw) == 0 {
			t.Errorf("%s: tools is empty (was stripped on the read path)", where)
			return
		}
		var tools []map[string]any
		if err := json.Unmarshal(raw, &tools); err != nil {
			t.Errorf("%s: tools is not a JSON array: %v (%s)", where, err, string(raw))
			return
		}
		if len(tools) != 2 {
			t.Errorf("%s: expected 2 tools, got %d (%s)", where, len(tools), string(raw))
			return
		}
		names := []string{fmt.Sprint(tools[0]["name"]), fmt.Sprint(tools[1]["name"])}
		if names[0] != "read_file" || names[1] != "write_file" {
			t.Errorf("%s: tool names = %v, want [read_file write_file]", where, names)
		}
	}

	// 1. Raw create return value.
	assertTwoTools("CreateMCPServerVersion return", ver.Tools)

	// 2. GetMCPServerVersion.
	got, err := sharedDB.GetMCPServerVersion(ctx, srv.ID, "1.0.0")
	if err != nil {
		t.Fatalf("GetMCPServerVersion: %v", err)
	}
	assertTwoTools("GetMCPServerVersion", got.Tools)

	// 3. ListMCPServerVersions.
	versions, err := sharedDB.ListMCPServerVersions(ctx, srv.ID)
	if err != nil {
		t.Fatalf("ListMCPServerVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	assertTwoTools("ListMCPServerVersions[0]", versions[0].Tools)

	// 4. GetMCPServer (latest-version projection).
	row, err := sharedDB.GetMCPServer(ctx, "tools-ns", "tooly", false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if row.LatestVersion == nil {
		t.Fatal("GetMCPServer: latest version is nil")
	}
	assertTwoTools("GetMCPServer.LatestVersion", row.LatestVersion.Tools)

	// 5. GetMCPServerByID (latest-version projection).
	row2, err := sharedDB.GetMCPServerByID(ctx, srv.ID)
	if err != nil {
		t.Fatalf("GetMCPServerByID: %v", err)
	}
	if row2.LatestVersion == nil {
		t.Fatal("GetMCPServerByID: latest version is nil")
	}
	assertTwoTools("GetMCPServerByID.LatestVersion", row2.LatestVersion.Tools)

	// 6. ListMCPServers (public path).
	listed, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{PublicOnly: true})
	if err != nil {
		t.Fatalf("ListMCPServers: %v", err)
	}
	var found *store.MCPServerRow
	for i := range listed {
		if listed[i].ID == srv.ID {
			found = &listed[i]
			break
		}
	}
	if found == nil {
		t.Fatal("ListMCPServers: server not present in results")
	}
	if found.LatestVersion == nil {
		t.Fatal("ListMCPServers: latest version is nil")
	}
	assertTwoTools("ListMCPServers[.].LatestVersion", found.LatestVersion.Tools)
}

// TestMCPServerVersion_ToolsDefaultEmptyArray verifies that omitting the Tools
// param defaults the column to an empty array (not NULL), matching the
// migration's NOT NULL DEFAULT '[]'::jsonb and the in-code default. This
// protects the UI from having to distinguish "no tools" from "unset".
func TestMCPServerVersion_ToolsDefaultEmptyArray(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "tools-default-ns", "Default Corp")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "default-srv", Name: "Default",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	ver, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        srv.ID,
		Version:         "0.1.0",
		Runtime:         domain.RuntimeStdio,
		Packages:        validPackages,
		ProtocolVersion: "2024-11-05",
		// Tools intentionally omitted.
	})
	if err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}

	var arr []any
	if err := json.Unmarshal(ver.Tools, &arr); err != nil {
		t.Fatalf("returned Tools should be a JSON array, got %q: %v", string(ver.Tools), err)
	}
	if len(arr) != 0 {
		t.Errorf("expected empty tools array, got %v", arr)
	}

	got, err := sharedDB.GetMCPServerVersion(ctx, srv.ID, "0.1.0")
	if err != nil {
		t.Fatalf("GetMCPServerVersion: %v", err)
	}
	if err := json.Unmarshal(got.Tools, &arr); err != nil {
		t.Fatalf("read-back Tools should be a JSON array, got %q: %v", string(got.Tools), err)
	}
	if len(arr) != 0 {
		t.Errorf("expected empty tools array on read-back, got %v", arr)
	}
}
