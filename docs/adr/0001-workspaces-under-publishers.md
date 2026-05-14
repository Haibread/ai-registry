# ADR 0001 — Workspaces under publishers

- **Status:** Accepted (shipped via PR #28; finalising migration via PR #62)
- **Date:** 2026-05-02
- **Deciders:** @Haibread
- **Supersedes:** —
- **Followed by:** [ADR 0002 — Workspace OIDC group binding](0002-workspace-group-binding.md), [ADR 0003 — Change-approval workflow](0003-change-approval-workflow.md)

## TL;DR

Insert a **workspace** entity between publishers and resources.

- New `workspaces (id, publisher_id, slug, name, …)` table.
- `mcp_servers.publisher_id` and `agents.publisher_id` become
  `workspace_id`. A two-step migration backfills one `default`
  workspace per existing publisher.
- Hierarchical URLs: `/v0/publishers/{p}/workspaces/{w}/servers/{s}`.
  ULID lookups (`/v0/servers/{id}`) unchanged.
- Auth and review workflow are untouched in this ADR; they land in 0002
  and 0003 respectively.

## Context

The registry currently has a single namespace layer:

```
publisher  ──owns──▶  mcp_server / agent  ──has──▶  versions
```

`mcp_servers (publisher_id, slug, …)` and `agents (publisher_id, slug,
…)` reference `publishers` directly, with `UNIQUE(publisher_id, slug)`.

This works for one team per publisher. It breaks down once a publisher
has multiple teams: every team's resources live in the same flat
namespace, every team has the same access (one Keycloak group per
publisher in the post-0002 world), and slug collisions force
coordination across teams that shouldn't need to talk.

Mature registries solve this with a sub-org layer: GitHub orgs, GitLab
groups, ArgoCD projects, Grafana orgs. We adopt the same pattern. The
publisher remains the legal/branding entity; the workspace is the
team-level unit that *actually* owns resources and binds to access
(0002).

## Decision

### Entity

```
publisher  ──owns──▶  workspace  ──owns──▶  mcp_server / agent  ──has──▶  versions
```

A publisher has one or more workspaces. A workspace has one publisher.
Every MCP server and agent belongs to exactly one workspace. The
publisher of a resource is reached by joining through the workspace.

### Schema

```sql
CREATE TABLE workspaces (
    id           TEXT        PRIMARY KEY,
    publisher_id TEXT        NOT NULL REFERENCES publishers(id) ON DELETE RESTRICT,
    slug         TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    description  TEXT,
    contact      TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publisher_id, slug)
);

CREATE TRIGGER trg_workspaces_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

(`UNIQUE(publisher_id, slug)` already creates the per-publisher index;
no separate `idx_workspaces_publisher` needed.)

### Migration

Two-step migration as designed. The schema part runs as a forward-only
SQL migration; the data backfill runs separately because **ULID
generation lives at the application layer** (no `generate_ulid()` SQL
function in this codebase).

Step 1 — schema migration `000008_workspaces.{up,down}.sql` (shipped):

1. Create the `workspaces` table.
2. Add `workspace_id` to `mcp_servers` and `agents` as **nullable**
   `REFERENCES workspaces(id) ON DELETE RESTRICT`.

Step 2 — backfill (Go-side one-shot, invoked on server boot via
`db.BackfillWorkspaces` when `workspace_id` is `NULL` anywhere; shipped):

3. For each row in `publishers`, insert a workspace `(generated ULID,
   publisher.id, 'default', 'Default workspace')`.
4. `UPDATE mcp_servers / agents SET workspace_id = (SELECT id FROM
   workspaces WHERE publisher_id = X AND slug = 'default')`.
5. Verify: `SELECT count(*) FROM mcp_servers WHERE workspace_id IS
   NULL` returns 0. Same for `agents`.

Step 3 — finalising migration `000011_workspaces_finalise.{up,down}.sql`
**(shipped 2026-05-14)**:

6. `ALTER TABLE … ALTER COLUMN workspace_id SET NOT NULL`, gated on a
   `DO $$ … RAISE EXCEPTION` check so operators get a friendly error
   if the backfill hasn't completed against the target DB.
7. Drop `publisher_id` columns and the old `UNIQUE(publisher_id, slug)`
   constraints. Add `UNIQUE(workspace_id, slug)` — slug uniqueness is
   now per-workspace, so two workspaces under one publisher may each
   expose a server with the same slug.
8. Drop `idx_*_publisher` (the `idx_*_workspace` indexes from 000008
   cover the new key).

> **Status note (2026-05-14).** Step 3 shipped. `mcp_servers` and
> `agents` no longer carry `publisher_id`; the owning publisher is
> reached via `workspaces.publisher_id` (a single JOIN). The boot-time
> `BackfillWorkspaces` helper was removed in the same change — after
> the NOT NULL constraint lands it has nothing to do, and its UPDATE
> queries referenced the dropped column. Wire-level `publisher_id`
> fields on MCP server / agent API responses are still populated; they
> are derived through the join, so the OpenAPI schema is unchanged.

### Slug uniqueness

Slugs are unique **within a workspace** (`UNIQUE(workspace_id, slug)`),
not within a publisher. Two workspaces under the same publisher can
each have a server named `weather` — they live at different URL paths.
This is the multi-team property we're paying for the new layer to get.

### URL structure

| Style                | Path                                                                  |
|----------------------|-----------------------------------------------------------------------|
| ULID lookup (today)  | `GET /v0/servers/{ulid}` — unchanged                                  |
| Hierarchical (new)   | `GET /v0/publishers/{p}/workspaces/{w}/servers/{s}`                   |
| Workspace-scoped list| `GET /v0/publishers/{p}/workspaces/{w}/servers`                       |
| Workspace CRUD       | `GET/POST/PATCH/DELETE /v0/publishers/{p}/workspaces[/{w}]`           |

Existing `/v0/publishers/{p}/servers/{s}` paths continue to work as
permanent redirects (HTTP 301) to
`/v0/publishers/{p}/workspaces/default/servers/{s}` during a transition
window. ADR 0002 defines how the redirect interacts with auth
(short version: auth is evaluated on the redirect *target*, not the
source).

### Spec-compatible wire formats (unchanged)

- **MCP registry response shape** (CLAUDE.md decision C) identifies
  servers by ULID, not by slug paths. The `_meta` block is the only
  place namespacing leaks; we leave it as today and add no workspace
  reference there in this ADR.
- **A2A Agent Card** (decision H): the per-agent
  `/.well-known/agent-card.json` URL changes to live under the
  workspace path. Card *content* doesn't change.

### Auth (unchanged in this ADR)

All write endpoints — including the new workspace CRUD — require
`realm_access.roles[] contains "admin"`. ADR 0002 introduces non-admin
writes via group binding.

### Workspace deletion

Workspace deletion requires the workspace to be **empty**, i.e.:

- no MCP servers and no agents reference it,
- no `pending_review` versions and no pending-deletion entries point at
  it (formal definitions live in ADR 0003 — for ADR 0001's purposes
  "no resources" is enough).

The handler returns 409 if any of those conditions hold. This avoids
orphaning items in a reviewer's queue.

### API surface (additions)

```
GET    /v0/publishers/{p}/workspaces                         public
GET    /v0/publishers/{p}/workspaces/{w}                     public
POST   /v0/publishers/{p}/workspaces                         admin
PATCH  /v0/publishers/{p}/workspaces/{w}                     admin
DELETE /v0/publishers/{p}/workspaces/{w}                     admin

GET    /v0/publishers/{p}/workspaces/{w}/servers             public
GET    /v0/publishers/{p}/workspaces/{w}/servers/{s}         public
POST   /v0/publishers/{p}/workspaces/{w}/servers             admin
… (mirrored for agents)
```

All MUST be added to `server/api/openapi.yaml` per the API-first rule.

## Consequences

### Positive

- Multi-team usage works without slug-collision pain.
- Maps cleanly onto the Keycloak group structure introduced in 0002:
  one group per workspace, not per publisher.
- Future per-workspace reviewer groups (0003 F1) become a refinement,
  not a redesign.
- Existing data preserved via the `default` workspace.
- ULID-based lookups (the spec-compat path) keep working unchanged.

### Negative

- Two-step migration with a Go-side backfill in the middle. The
  schema and finalise migrations bracket it; ops needs to run the
  backfill cleanly in every environment before the finalise migration.
- **Publisher deletion is now a multi-step cleanup.** With
  `ON DELETE RESTRICT` on workspaces and resources, deleting a
  publisher requires every workspace to be deleted first, which
  requires every workspace to be empty. We accept this — publisher
  deletion is rare and benefits from the friction.
- Two-tier hierarchy is more concept to teach. Some users will have
  exactly one workspace per publisher and find the layer redundant.
- URL shape change requires a redirect window for legacy paths.
- Every repository query that filtered by `publisher_id` now joins
  through `workspaces`. Minor extra query cost.

### Neutral

- Public-facing search/list endpoints can flatten across workspaces
  (e.g. `GET /v0/servers?publisher=anthropic`) so the average reader
  doesn't have to think about workspaces.
- MCP and A2A spec-compatible wire formats don't change in this ADR.

## Alternatives considered

1. **Don't add workspaces; one publisher = one group.** Rejected: it's
   the current model. Single team per publisher is fine for an early
   demo and breaks down on the second team. Putting it off means a
   more painful migration later, with real users.
2. **Make publishers themselves the workspace.** Rejected: publishers
   carry external meaning (verified badges, contact info, branding)
   that doesn't subdivide naturally.
3. **Keep `publisher_id` on resources in addition to `workspace_id`,
   maintained by a trigger.** Worth a serious look. Pro: read-side hot
   paths (`GET /v0/servers?publisher=anthropic`) avoid the join,
   matters at scale. Con: the trigger is one more failure mode (gets
   skipped on bulk inserts that bypass triggers, drifts under
   replication), and Postgres can index the join cheaply. Rejected for
   v1 in favor of the join + an index on `workspaces.publisher_id`;
   revisit if profiling shows the join hot.
4. **Slugs unique within publisher (across workspaces).** Rejected:
   defeats half the reason for adding the layer. Different teams must
   be able to name their servers independently.
5. **Single-table self-referencing hierarchy** (publishers and
   workspaces in one table with a `parent_id`). Rejected: makes every
   query polymorphic and the spec-compat wire formats harder to keep
   stable.

## Out of scope

- **Group binding on workspaces** — ADR 0002.
- **Change-approval workflow** — ADR 0003.
- **Cross-workspace transfers** (moving an MCP server from workspace
  A to workspace B). Defer.
- **Workspace-level visibility controls** (private workspace etc.).
  Today the entry's `visibility` field on agents handles per-resource
  visibility; we don't add a workspace-level one yet.

## Implementation sketch

1. **Schema migration** for `workspaces` table + nullable
   `workspace_id` on resources.
2. **Domain entity** `workspace.go` in `server/internal/domain/`.
3. **Repository** in `server/internal/store/` for workspace CRUD plus
   a helper to resolve `(publisher_slug, workspace_slug) →
   workspace_id`.
4. **Backfill command / boot-time job** that creates the default
   workspace per publisher and rewrites resource FKs. Idempotent.
5. **Finalise migration** flipping `workspace_id` to NOT NULL and
   dropping `publisher_id` from resources.
6. **Handlers** for workspace CRUD, refactored MCP server / agent
   handlers that resolve the workspace from URL path parameters.
7. **Redirect middleware** mapping legacy `/v0/publishers/{p}/servers/{s}`
   to the default-workspace path with a 301.
8. **OpenAPI** — add workspace schemas, new operations, new path
   parameters on existing operations.
9. **Tests**: repository tests on the FK rewrite; handler tests on the
   hierarchical CRUD; redirect test on the legacy path; integration
   test asserting that pre-migration data ends up in default
   workspaces; an empty-check test on workspace delete.

## References

- [CLAUDE.md](../../CLAUDE.md) — project principles (API-first, two
  UIs, version model, decisions C and H).
- [server/migrations/000001_init.up.sql](../../server/migrations/000001_init.up.sql) — current `mcp_servers (publisher_id, slug)` and
  `agents (publisher_id, slug)` shape.
- GitHub orgs / GitLab groups / Grafana orgs / ArgoCD projects — prior
  art for the sub-publisher layer.
