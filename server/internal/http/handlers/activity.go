// Package handlers — public per-resource activity feed.
//
// These handlers power the public "Activity" tab on MCP server and agent
// detail pages. They return a privacy-scrubbed view of the underlying audit
// log: actor identity (subject, email) is NEVER exposed, metadata is
// whitelisted, and draft-only events are filtered out.
//
// Unlike /api/v1/audit (admin-only, returns the raw audit_log rows), these
// endpoints are PUBLIC and rate-limited. The DTO returned here is defined in
// openapi.yaml as PublicActivityEvent — deliberately distinct from the
// internal AuditEvent schema so the scrub is enforced at the type level.
package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// publicActionWhitelist defines which audit actions are safe to surface on the
// public activity feed. Actions not in this set are dropped. The primary
// exclusion is *version.created (drafts are admin-only by contract — we must
// not leak their existence). .deleted is also excluded because a soft-deleted
// resource returns 404 from the parent endpoint, so the event is unreachable.
var publicActionWhitelist = map[domain.AuditAction]struct{}{
	// MCP
	domain.ActionMCPServerCreated:    {},
	domain.ActionMCPVersionPublished: {},
	domain.ActionMCPServerDeprecated: {},
	domain.ActionMCPServerVisibility: {},
	domain.ActionMCPServerUpdated:    {},
	// Agents
	domain.ActionAgentCreated:          {},
	domain.ActionAgentVersionPublished: {},
	domain.ActionAgentDeprecated:       {},
	domain.ActionAgentVisibility:       {},
	domain.ActionAgentUpdated:          {},
}

// publicMetadataAllowlist lists metadata keys that are safe to echo back on
// the public feed. Anything else (IP addresses, user-agents, diff payloads,
// internal correlation IDs) is dropped by scrubMetadata.
var publicMetadataAllowlist = map[string]struct{}{
	"from":       {}, // e.g. previous value of a changed field
	"to":         {}, // new value of a changed field
	"visibility": {}, // "public" | "private"
	"reason":     {}, // short deprecation / visibility reason
	"version":    {}, // semver string for version events
	"field":      {}, // which field changed on an update event
}

// scrubMetadata returns a copy of m containing only whitelisted keys whose
// values are primitive JSON types (string, number, bool). Nested objects are
// dropped defensively — if a future action needs to expose a structured blob,
// add a new flat key instead of loosening this filter.
func scrubMetadata(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if _, ok := publicMetadataAllowlist[k]; !ok {
			continue
		}
		switch v.(type) {
		case string, bool, float64, float32, int, int32, int64:
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// extractVersion pulls the semver string off the event — either from the
// metadata map or by inspecting action-type conventions. Returns empty string
// when the event is not version-scoped.
func extractVersion(e domain.AuditEvent) string {
	if v, ok := e.Metadata["version"].(string); ok && v != "" {
		return v
	}
	return ""
}

// ListMCPServerActivity handles GET /api/v1/mcp/servers/{namespace}/{slug}/activity.
func (h *MCPHandlers) ListMCPServerActivity(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	// 404 if the parent resource is not public (privacy contract: if the
	// resource itself isn't visible, its activity history isn't either).
	publicOnly := !auth.IsAdminFromContext(r.Context())
	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	writePublicActivity(w, r, h.db, srv.ID, "mcp_server")
}

// ListAgentActivity handles GET /api/v1/agents/{namespace}/{slug}/activity.
func (h *AgentHandlers) ListAgentActivity(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	publicOnly := !auth.IsAdminFromContext(r.Context())
	ag, err := h.db.GetAgent(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	writePublicActivity(w, r, h.db, ag.ID, "agent")
}

// writePublicActivity is the shared core of both activity endpoints. It pages
// through the audit_log filtered by resource ID, applies the action whitelist
// + metadata scrub, and emits PublicActivityEvent objects.
//
// Because the whitelist can drop rows inside a page, we may deliver fewer than
// `limit` items even when more rows exist upstream. That's fine — the cursor
// reflects the last raw row scanned, so pagination is still consistent; the
// client just needs to follow next_cursor if it wants more.
func writePublicActivity(w http.ResponseWriter, r *http.Request, db *store.DB, resourceID, resourceType string) {
	limit := int32(25)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	p := store.ListAuditParams{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Limit:        limit + 1, // fetch one extra to detect next page
		Cursor:       r.URL.Query().Get("cursor"),
	}

	events, err := db.ListAuditEvents(r.Context(), p)
	if err != nil {
		internalError(w, r, err)
		return
	}

	var nextCursor string
	if int32(len(events)) > limit {
		events = events[:limit]
		last := events[len(events)-1]
		nextCursor = store.EncodeCursorFromTime(last.CreatedAt, last.ID)
	}

	// Public DTO — field names/shape must match openapi.yaml PublicActivityEvent.
	type publicEvent struct {
		ID        string         `json:"id"`
		Action    string         `json:"action"`
		ActorRole string         `json:"actor_role"`
		Version   string         `json:"version,omitempty"`
		Metadata  map[string]any `json:"metadata,omitempty"`
		CreatedAt string         `json:"created_at"`
	}

	items := make([]publicEvent, 0, len(events))
	for _, e := range events {
		if _, allowed := publicActionWhitelist[e.Action]; !allowed {
			continue
		}
		// All current writers are admins (CLAUDE.md rule 3). The enum is
		// kept so future per-publisher roles can slot in without a
		// breaking DTO change.
		items = append(items, publicEvent{
			ID:        e.ID,
			Action:    string(e.Action),
			ActorRole: "admin",
			Version:   extractVersion(e),
			Metadata:  scrubMetadata(e.Metadata),
			CreatedAt: e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
	})
}
