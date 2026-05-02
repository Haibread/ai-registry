# ADR 0001 — Publisher OIDC group mapping and change-approval workflow

- **Status:** Proposed
- **Date:** 2026-05-02
- **Deciders:** @Haibread
- **Supersedes:** —

## TL;DR

Today only realm admins can write to the registry. We want non-admin
publishers to author content for **their** publisher, and we want every
change reviewed before it goes live.

- Each `publishers` row gets a nullable `group_name`. The JWT carries a
  `groups` claim. Non-admin writes are allowed only if the publisher's
  group is in the JWT.
- Each version gets a new `review_state` column (`none` |
  `pending_review` | `rejected`), **orthogonal** to the existing
  `status` / `published_at` fields — the existing publish mechanism is
  untouched. Approve flips the existing `published_at` exactly as today.
- Publishers can keep editing a `pending_review` version (PR-style); a
  `revision` integer protects reviewers from approving stale content via
  a 409.
- One global reviewer group (`registry-reviewers`, configurable) for
  first release. Per-resource-type or per-publisher reviewer groups
  deferred (F1).
- All names/claims are configurable per CLAUDE.md's env + YAML + default
  rule. No user table is introduced.

## Context

The registry currently has two write-authorization states: anonymous (denied)
and `realm_access.roles[] contains "admin"` (allowed for everything). This is
enforced by `RequireAdmin` middleware at [server/internal/auth/middleware.go:80](../../server/internal/auth/middleware.go).
There is no user table — the first migration is explicit that
"auth is stateless JWT; user data never hits the database"
([server/migrations/000001_init.up.sql:11](../../server/migrations/000001_init.up.sql)).

Publishers (`publishers` table) namespace MCP servers and agents but have no
membership semantics. Anyone with the `admin` realm role can write to any
publisher's entries; non-admins can write to none.

The existing schema models entries and versions like this (relevant bits):

- `mcp_servers.status` ∈ `('draft', 'published', 'deprecated', 'deleted')`
  — the entry-level lifecycle (CLAUDE.md decision I).
- `mcp_server_versions(server_id, status, published_at, …)` — the version
  table, where `status` ∈ `('active', 'deprecated', 'deleted')` and
  `published_at` is the marker for "this version is live".
- Agents follow the same shape with `agents.status` and
  `agent_versions(agent_id, status, published_at, …)`.

We want two things, and they are coupled:

1. **Delegate per-publisher writes** to non-admin users via Keycloak groups,
   without introducing a user table or breaking the stateless-JWT principle.
2. **Gate every change behind review** by an approval group, because
   consumers of the MCP and A2A specs cache published versions and silent
   mutation breaks them (decisions C and I in CLAUDE.md).

Shipping (1) without (2) would briefly let publisher group members
self-publish, which is exactly what (2) is meant to prevent. They are
treated here as a single design.

## Decision

### Part A — Map publishers to a Keycloak group (1:1)

Each publisher may name **at most one** Keycloak group. Members of that
group can author content for the publisher; admins can author for any
publisher.

#### JWT claim

Bare group names (e.g. `["mcp-authors", "registry-reviewers"]`), not
Keycloak full paths. Configured via a Keycloak group-membership mapper with
"Full group path" disabled. Token contents:

```json
{
  "sub": "…",
  "email": "…",
  "realm_access": { "roles": ["user", "admin"] },
  "groups": ["mcp-authors"]
}
```

The JWT **claim name** is configurable per the project's config rule (env
+ YAML + default — see CLAUDE.md *Configuration*):

| Env var               | YAML key             | Default  |
|-----------------------|----------------------|----------|
| `AUTH_GROUPS_CLAIM`   | `auth.groups_claim`  | `groups` |

`KeycloakClaims` ([server/internal/auth/claims.go](../../server/internal/auth/claims.go))
gains `Groups []string` populated from this claim, plus a
`HasGroup(name string) bool` helper.

#### Schema

```sql
ALTER TABLE publishers
    ADD COLUMN group_name text;

CREATE INDEX publishers_group_name_idx
    ON publishers (group_name)
    WHERE group_name IS NOT NULL;
```

`group_name` is nullable. A publisher with `NULL` is admin-only, preserving
current behavior for migration safety. 1:1 is chosen for now; many-to-many
later is a non-breaking change (add a join table, backfill, drop the
column — see F8).

### Part B — Change-approval workflow

Add a new `review_state` column on each version table, orthogonal to the
existing `status` and `published_at`. The existing publish mechanism
(setting `published_at`, parent entry `status='published'`) is untouched —
the approve handler simply triggers it as today.

```
Draft               review_state='none'           , published_at IS NULL
  │  submit
  ▼
Pending review      review_state='pending_review' , published_at IS NULL
  │  approve  ──▶  Published   review_state='none', published_at IS NOT NULL
  │                            (parent entry status flips to 'published')
  │  reject
  ▼
Rejected            review_state='rejected'       , published_at IS NULL
                    rejection_reason IS NOT NULL
  │  edit + submit
  ▼
back to Pending review (revision++ , rejection_reason cleared)

(also: from Pending review, edit content stays Pending review, revision++)
(also: from Pending review, withdraw goes back to Draft)
```

Once a version is `published_at IS NOT NULL`, it is immutable. To change a
published entry the publisher creates a **new** version row in `Draft` and
submits it; the old one stays published until the new one is approved.

#### PR-style continuous editing

Publishers can keep editing a version while it sits in `pending_review`,
without withdrawing it. Each content edit increments a `revision` integer
on the version row. The approve and reject API calls **must** carry the
revision the reviewer last saw; if it differs from the current one the
server returns **409 Conflict** with a problem body indicating the version
was edited since the reviewer last loaded it. The admin UI surfaces the
mismatch and forces a re-read before another approval attempt.

The content-edit endpoint is **not new** — it's the existing
`PATCH /v0/servers/{id}/versions/{ver}` (and the agent equivalent). The
implementation must:

- Authorize via `RequirePublisherWrite` (was `RequireAdmin`).
- Reject the PATCH if the row is not in `review_state IN ('none',
  'pending_review','rejected')` (i.e. published versions remain
  immutable).
- On a successful PATCH against a `pending_review` row, bump `revision`.
- On a successful PATCH against a `rejected` row, leave `revision` and
  `rejection_reason` as-is (the publisher is iterating; the next submit
  call clears the reason and bumps revision).

To prevent stacking, a **partial unique index** enforces at most one
`pending_review` version per entry at a time:

```sql
CREATE UNIQUE INDEX mcp_server_versions_one_pending_idx
    ON mcp_server_versions (server_id)
    WHERE review_state = 'pending_review';
```

(Same on `agent_versions(agent_id)`.)

#### Operations covered

| Operation | Mechanism                                                               |
|-----------|-------------------------------------------------------------------------|
| Create    | New entry's first version is created as Draft, submitted for review.    |
| Edit      | A new version row in Draft is created, submitted; replaces on approval. |
| Delete    | A `deletion_requested_at` flag on the entry; approver confirms or clears.|

Deprecation transitions (entry `status='published' → 'deprecated'`) also go
through the review queue, since they affect what consumers see; treated as
a 1-field edit on the entry.

#### Approval group

**One global group** for the first release, name configurable:

| Env var                   | YAML key                | Default              |
|---------------------------|-------------------------|----------------------|
| `AUTH_REVIEWER_GROUP`     | `auth.reviewer_group`   | `registry-reviewers` |

Granularity per resource type (`mcp-reviewers`, `agent-reviewers`) or per
publisher is **deferred** — see Future work F1.

Admins (`realm_access.roles[] contains "admin"`) implicitly satisfy the
reviewer check.

#### Submit / withdraw / reject mechanics

- **Submit** (from Draft or Rejected) — sets `review_state='pending_review'`,
  `submitted_at=now()`, `submitted_by(_email)` from the JWT, clears
  `rejection_reason`, bumps `revision`. Requires that the entry has no other
  `pending_review` version (the partial unique index enforces this).
- **Withdraw** (from Pending review) — sets `review_state='none'`, clears
  `submitted_at` / `submitted_by(_email)`. `revision` is *not* reset; it
  keeps incrementing across the lifecycle of the version.
- **Reject** (from Pending review) — sets `review_state='rejected'`,
  `reviewed_at=now()`, `reviewed_by(_email)` from the JWT,
  `review_decision='rejected'`, `rejection_reason=<body.reason>`. Requires
  matching `revision`.
- **Approve** (from Pending review) — sets `published_at=now()` if not
  already set, marks the parent entry `status='published'` if it was
  `'draft'`, sets `review_state='none'`, `reviewed_at=now()`,
  `reviewed_by(_email)`, `review_decision='approved'`. Requires matching
  `revision`.

#### Visibility

- **Public read endpoints** (`GET /v0/servers`, `/v0/servers/{id}`, agent
  equivalents, the spec-compatible MCP shape per CLAUDE.md decision C, the
  per-agent `/.well-known/agent-card.json`, and the global
  `/.well-known/agent-card.json`) return only versions with
  `published_at IS NOT NULL AND status != 'deleted'` — i.e. the **same
  filter as today**. They never return `review_state != 'none'` rows, never
  reveal pending deletions, and never expose any of the new audit columns
  or `review_state`.
- **Authenticated read endpoints** apply the following rule, evaluated in
  order:
  - Admin → sees all states for all publishers, including pending
    deletions.
  - Reviewer group member → sees `pending_review` versions and pending
    deletions across all publishers (so the review queue works), plus
    everything public.
  - Publisher group member → sees all states (Draft, Pending review,
    Rejected, plus public) **only** for entries whose `publisher_id` is
    bound to a `group_name` the JWT carries.
  - Anyone else → public-only, identical to anonymous.
- An entry with a pending deletion stays visible on public reads (the
  current published version is unchanged) until the deletion is approved,
  at which point it is soft-deleted via the new `deleted_at` column and
  removed from public listings.

#### Schema additions

On every entry-version row (`mcp_server_versions`, `agent_versions`):

```sql
ALTER TABLE mcp_server_versions
    ADD COLUMN review_state       text        NOT NULL DEFAULT 'none',
    ADD COLUMN revision           integer     NOT NULL DEFAULT 1,
    ADD COLUMN submitted_at       timestamptz,
    ADD COLUMN submitted_by       text,            -- JWT subject (UUID)
    ADD COLUMN submitted_by_email text,            -- denormalized at action time
    ADD COLUMN reviewed_at        timestamptz,
    ADD COLUMN reviewed_by        text,            -- JWT subject (UUID)
    ADD COLUMN reviewed_by_email  text,            -- denormalized at action time
    ADD COLUMN review_decision    text,            -- 'approved' | 'rejected'
    ADD COLUMN rejection_reason   text;

ALTER TABLE mcp_server_versions
    ADD CONSTRAINT mcp_server_versions_review_state_chk
    CHECK (review_state IN ('none','pending_review','rejected'));

-- A non-'none' review_state implies the version is not yet published.
ALTER TABLE mcp_server_versions
    ADD CONSTRAINT mcp_server_versions_review_unpublished_chk
    CHECK (review_state = 'none' OR published_at IS NULL);

CREATE INDEX mcp_server_versions_review_state_idx
    ON mcp_server_versions (review_state)
    WHERE review_state != 'none';

CREATE UNIQUE INDEX mcp_server_versions_one_pending_idx
    ON mcp_server_versions (server_id)
    WHERE review_state = 'pending_review';
```

(Same shape on `agent_versions`, keyed on `agent_id`.)

On the entry rows (`mcp_servers`, `agents`) — note these tables do **not**
currently have a `deleted_at` column, so the migration adds it:

```sql
ALTER TABLE mcp_servers
    ADD COLUMN deleted_at                  timestamptz,
    ADD COLUMN deletion_requested_at       timestamptz,
    ADD COLUMN deletion_requested_by       text,
    ADD COLUMN deletion_requested_by_email text;

CREATE INDEX mcp_servers_active_idx
    ON mcp_servers (id)
    WHERE deleted_at IS NULL;
```

(Same shape on `agents`.)

The `deletion_requested_*` columns are cleared on rejection and remain set
through the soft-delete for audit. Email is denormalized alongside subject
because Keycloak emails can change after the fact and the audit column
should reflect the email at action time.

The **down migration** drops every column, constraint, and index added by
the up migration, in reverse order. Per CLAUDE.md, down is for local dev
only; production rolls forward.

### Authorization matrix

Publisher-scoped write endpoints use a `RequirePublisherWrite` middleware
that, for a request URL referencing an entry (e.g. `/v0/servers/{id}/...`),
performs **one DB lookup** to resolve the entry's `publisher_id`, then
applies:

```
allow if  jwt.realm_access.roles contains "admin"
       OR (publisher.group_name is not null AND jwt.groups contains publisher.group_name)
```

Reviewer-scoped endpoints use a `RequireReviewer` middleware that checks:

```
allow if  jwt.realm_access.roles contains "admin"
       OR jwt.groups contains <configured reviewer group>
```

| Action                      | Required principal                                    |
|-----------------------------|-------------------------------------------------------|
| Create new entry / version  | Publisher group member OR admin                       |
| Edit Draft / Rejected       | Publisher group member OR admin                       |
| Edit Pending review         | Publisher group member OR admin (revision bumps)      |
| Submit                      | Publisher group member OR admin                       |
| Withdraw                    | Publisher group member OR admin                       |
| Approve / reject            | Reviewer group member OR admin (revision must match)  |
| Request deletion            | Publisher group member OR admin                       |
| Confirm / cancel delete     | Reviewer group member OR admin                        |
| Create / delete a publisher | Admin only                                            |

**Self-approval is permitted** when a user is in both the publisher group
and the reviewer group. This is intentional for the first release given
the reviewer group is a small admin-adjacent set; revisit if the group
grows. Captured under Future work F2.

### API surface

**Existing endpoint, behavior changed:**

```
PATCH  /v0/servers/{id}/versions/{ver}                publisher
       (now: bumps `revision` when version is in pending_review;
        rejected if version is published_at IS NOT NULL)
```

**New endpoints (mirrored for agents):**

```
POST   /v0/servers/{id}/versions/{ver}/submit          publisher
POST   /v0/servers/{id}/versions/{ver}/withdraw        publisher
POST   /v0/servers/{id}/versions/{ver}/approve         reviewer  body: { revision }
POST   /v0/servers/{id}/versions/{ver}/reject          reviewer  body: { revision, reason }
POST   /v0/servers/{id}/deletion-request               publisher
POST   /v0/servers/{id}/deletion-request/approve       reviewer
POST   /v0/servers/{id}/deletion-request/reject        reviewer  body: { reason }
```

All MUST be added to `server/api/openapi.yaml` per the project's API-first
rule.

Approve and reject return:

- **200** on success;
- **409 Conflict** (RFC 7807 problem body) if `revision` in the body does
  not match the current row, **or** if the row is no longer in
  `review_state='pending_review'`. Both conditions are checked in a single
  `UPDATE … WHERE review_state='pending_review' AND revision=$body_revision`
  and the rows-affected count drives the 200 vs 409 decision — this also
  closes the race between two reviewers clicking approve simultaneously.

#### Response schema split

Per CLAUDE.md decision C, the spec-compatible MCP wire format is strict.
The OpenAPI spec therefore exposes **two** schemas per resource:

- `Server` / `Agent` — public, spec-compatible. **Does not expose**
  `review_state`, `revision`, the `submitted*`/`reviewed*`/
  `rejection_reason` columns, or pending-deletion fields.
- `ServerAdmin` / `AgentAdmin` — used by authenticated read endpoints,
  exposes the additional fields.

The two schemas reuse a common base via `allOf` to avoid drift.

### Audit

The existing `audit_log` table already captures `actor_subject` and
`actor_email` from the JWT for every HTTP call. The new columns on the
version row (`submitted_by(_email)`, `reviewed_by(_email)`,
`review_decision`, `rejection_reason`) are themselves the durable audit
record for state transitions, with the email captured at action time so
later Keycloak email changes do not rewrite history.

## Consequences

### Positive

- Non-admin contributors can publish under their assigned publisher without
  receiving global admin rights.
- Stateless model preserved: no user table, no membership table, no live
  Keycloak Admin API dependency on the request path.
- **Existing publish mechanism untouched.** `published_at` and the entry
  `status` enum keep their meaning. `review_state` is a new orthogonal
  field; rollback is "drop the column".
- Public read API behavior is unchanged for downstream consumers — they
  still only see published content under the spec-compatible schema. The
  read-side filter is identical to today.
- Auditable end-to-end: who submitted, who reviewed, when, with email at
  action time and rejection reason.
- PR-style editing on `pending_review` removes the withdraw/resubmit dance
  for typo-class fixes while the revision check keeps reviewer decisions
  honest.
- Approve/reject is race-safe via a single conditional UPDATE; no separate
  optimistic-lock plumbing needed.

### Negative

- Operators must configure the Keycloak group-membership mapper, or the
  `groups` claim is silently absent and all non-admin writes 403. We will
  document this in `deploy/.env.example` and surface a startup warning when
  the feature is enabled but no publishers have `group_name` set.
- Group renames in Keycloak silently break access until the publisher row
  (or the configured reviewer group name) is updated.
- The registry cannot answer "who can write to publisher X?" without a live
  Keycloak Admin API call. Out of scope (F9).
- Self-approval is allowed by design for the first release. If the global
  reviewer group grows, this becomes a real risk; tracked as F2.
- Deprecation now requires review, adding a step for a previously-cheap
  operation. Acceptable given how rarely it happens.
- The revision check adds a contract requirement on the admin UI: every
  approve / reject button must remember the `revision` it loaded; clients
  that forget will see a 409 on every action.
- Resubmits on the same version row mean `submitted_at` /
  `submitted_by(_email)` reflect the **latest** submission, not the first.
  We accept this; the per-call history lives in `audit_log`.
- 1:1 publisher↔group binding means a user belonging to multiple "author"
  groups still needs each publisher row updated individually.
- `RequirePublisherWrite` introduces one extra DB lookup per write request
  (entry → publisher_id). Acceptable; writes are rare and already inside
  a transaction.

### Neutral

- Admin override (`realm_access.roles=admin`) is preserved for bootstrap,
  recovery, and cross-publisher operations across both middlewares.
- API shape for existing endpoints is unchanged — same 401/403 responses,
  same RFC 7807 problem bodies. Only the *reason* for a 403 shifts on the
  publisher-write path, plus the new 409 on approve/reject.
- Pending changes are not visible to the public API, so cache invalidation
  for downstream consumers happens only on approval — no new cache rules
  needed.

## Alternatives considered

1. **DB-backed user table with explicit publisher membership.** Rejected:
   contradicts the explicit "no user table" decision in migration 000001
   and forces us to reconcile a local user store with Keycloak on every
   login.
2. **Pure realm-role gating** (e.g. role `publisher:anthropic`). Workable
   but couples publisher creation to Keycloak realm config — admins would
   have to mint a role in Keycloak for every new publisher. Group
   membership is the standard Keycloak primitive for "set of users";
   roles are for capabilities.
3. **Live Keycloak Admin API lookup per request.** Rejected: adds a network
   hop, a service-account secret, and a cache layer to the hot path for no
   gain over reading the `groups` claim already in the JWT.
4. **Many-to-many publisher↔group from day one.** Rejected for now: no
   concrete need, and the migration to get there later is straightforward.
5. **Separate `change_requests` table holding diff payloads.** Rejected:
   doubles the source of truth for entry content and forces a merge step.
   The version row already gives us "proposed content sitting next to live
   content" for free, with `published_at IS NULL` as the divider.
6. **Per-resource-type reviewer groups from day one** (`mcp-reviewers` +
   `agent-reviewers`). Deferred — useful but not needed for first release
   (F1).
7. **Forbid self-approval** by checking `submitted_by != reviewed_by`.
   Rejected for first release per explicit decision; revisit when the
   reviewer group grows beyond admin-adjacent membership (F2).
8. **Auto-approve when the actor is admin.** Rejected: admins should still
   click approve so the audit row is populated; admin-ness only bypasses
   the reviewer-group check, not the workflow.
9. **Strict immutable `pending_review` (withdraw to edit).** Rejected:
   forces a withdraw/resubmit dance for trivial typo fixes. Adopted PR-style
   continuous editing instead.
10. **Auto-revert to Draft on any edit while Pending review.** Rejected:
    still forces an explicit re-submit on each edit and offers no advantage
    over the chosen revision-counter approach.
11. **Replace existing `status`/`published_at` with a single new `state`
    enum.** Rejected: would clash with current CHECK constraints and force
    a rewrite of every read path. The orthogonal `review_state` column is
    additive and ships without touching existing publish logic.

## Out of scope (FUTURE WORK — do not lose track)

The following are deliberately deferred and should be tracked as follow-up
ADRs / issues:

- **F1. Per-resource-type or per-publisher reviewer groups.** Move from one
  global reviewer group to (`mcp-reviewers`, `agent-reviewers`) or to a
  per-publisher-configurable approver group.
- **F2. Forbid self-approval** (`submitted_by != reviewed_by`) once the
  reviewer group is no longer admin-adjacent.
- **F3. Notifications.** Email / webhook / Slack on submission, approval,
  rejection, deletion request. Needs templating, retry, and per-user
  delivery prefs.
- **F4. SLA timers.** Auto-escalate or auto-reject submissions that sit in
  `pending_review` past a threshold.
- **F5. Bulk approval.** A reviewer UI action that approves N pending
  versions in one call (e.g. all entries from publisher X).
- **F6. Reviewer comments / discussion thread** on a pending version,
  beyond the single `rejection_reason` text field.
- **F7. Diff view in the admin UI** showing what changed between the
  current published version and the `pending_review` version, and between
  successive `revision` values.
- **F8. Many-to-many publisher↔group** mapping (one publisher belonging to
  multiple author groups).
- **F9. List members of a publisher's group** via the Keycloak Admin API,
  for an admin-UI view of "who can write to this publisher".

These items SHOULD be linked from `PLAN.md` once that file is updated for
the next phase.

## Implementation sketch (for the follow-up PR)

1. **Migration `000008_publisher_groups_and_review.{up,down}.sql`** adding:
   - `publishers.group_name` + index;
   - `review_state`, `revision`, audit columns, both CHECK constraints,
     state index, and the partial unique pending-review index for both
     `mcp_server_versions(server_id)` and `agent_versions(agent_id)`;
   - `deleted_at`, `deletion_requested_at`, `deletion_requested_by`,
     `deletion_requested_by_email` on `mcp_servers` and `agents`, plus the
     `deleted_at IS NULL` partial index.
   - Down migration drops all of the above in reverse order.
2. **Config keys** in `server/internal/config/config.go`: `AUTH_GROUPS_CLAIM`
   (default `groups`) and `AUTH_REVIEWER_GROUP` (default
   `registry-reviewers`), wired into env / YAML / default per CLAUDE.md.
   Documented in `deploy/.env.example`.
3. **`KeycloakClaims.Groups` + `HasGroup`** in
   `server/internal/auth/claims.go`, populated from the configured claim
   name. Tests for missing claim, empty array, match, no-match, and a
   non-default claim name.
4. **Two middlewares** in `server/internal/auth/middleware.go`:
   - `RequirePublisherWrite(extractEntryID)` — performs the entry →
     `publisher_id` lookup and applies the admin-or-group-match rule;
   - `RequireReviewer` — admin-or-reviewer-group-match.
5. **Domain entities** gain a `ReviewState` enum and a `Revision` field.
   Public read repositories are **unchanged** (the existing
   `published_at IS NOT NULL AND status != 'deleted'` filter already
   excludes pending content). Authenticated repositories take a
   `reviewStates []ReviewState` filter.
6. **Existing PATCH version handler** is updated to: (a) use
   `RequirePublisherWrite`, (b) bump `revision` when the row is in
   `pending_review`, (c) reject with 409 when the row is published.
7. **New handlers** under `server/internal/mcp/` and
   `server/internal/agents/` for submit / withdraw / approve / reject /
   deletion-request / deletion-confirm / deletion-cancel. Approve and
   reject use a single conditional UPDATE
   (`WHERE review_state='pending_review' AND revision=$body`) and map a
   0-row result to RFC 7807 409.
8. **OpenAPI** (`server/api/openapi.yaml`):
   - add `groupName` (nullable) to the `Publisher` schema;
   - introduce `Server` / `ServerAdmin` and `Agent` / `AgentAdmin` pairs
     with the admin variants exposing `reviewState`, `revision`, audit
     columns, and pending-deletion fields;
   - add the seven new operations per resource type with explicit `409`
     responses on approve/reject for revision/state mismatch.
9. **Admin UI**:
   - a `Group name` field on the publisher edit form;
   - a review-queue page (list of `pending_review` versions across the
     registry, filterable by publisher), with per-version approve / reject
     buttons that carry the loaded `revision` and surface 409 by
     re-fetching the version;
   - a deletion-request inbox.
10. **Tests**:
    - unit tests on the state-machine transitions (legal vs illegal),
      including the "`review_state` non-'none' implies
      `published_at IS NULL`" constraint;
    - middleware unit tests for the publisher / reviewer / admin matrices
      including the null-group path and the configurable claim name;
    - integration tests for the full submit→approve and submit→reject
      flows on both MCP and agent endpoints, including the
      edit-while-pending path that bumps `revision`;
    - a 409 test where reviewer A approves with a stale `revision` after
      publisher edits the version;
    - a 409 test where two reviewers approve simultaneously (one wins,
      one gets 409);
    - a public-read test asserting that pending content and the
      admin-only fields never leak through the spec-compatible MCP shape
      or the `/.well-known/agent-card.json` endpoints;
    - an integration test creating a publisher with a group and asserting
      200 vs 403 on a write endpoint with matching vs non-matching tokens.

## References

- [CLAUDE.md](../../CLAUDE.md) — project principles (API-first, two UIs,
  admin-gated writes, OIDC, version immutability, configuration rule).
- [server/migrations/000001_init.up.sql](../../server/migrations/000001_init.up.sql) — "no users table" decision; current entry/version state model;
  table and column names (`mcp_servers`, `mcp_server_versions(server_id)`,
  `agents`, `agent_versions(agent_id)`).
- [server/internal/auth/middleware.go](../../server/internal/auth/middleware.go) — current `RequireAdmin`.
- Keycloak group-membership mapper: <https://www.keycloak.org/docs/latest/server_admin/#_protocol-mappers>
