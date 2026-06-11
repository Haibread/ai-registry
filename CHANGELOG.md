# Changelog

All notable changes to this project are documented here.

## Unreleased

### Added

- **Instance-wide tags, curated by Server Admins and ticked by publishers.**
  A new vocabulary of registry-wide tags (e.g. pricing or maturity markers)
  lives at `GET /api/v1/tags` (public; each tag carries `slug`, display
  `name`, `description`, badge `color`, and an `active` flag) with
  Server-Admin-only `POST` / `PATCH /{slug}` / `DELETE /{slug}` management
  and a matching `/admin/tags` UI page. Publishers tick tags from the
  vocabulary on the new-version forms (a checkbox group appears once tags are
  defined); the create-version endpoints validate the slugs against the
  active vocabulary (422 otherwise) and freeze the selection into the
  immutable version row. Entry-level `tags` in list/detail responses and the
  existing `?tag=` filter now reflect the latest *published* version's tags,
  the public detail pages and catalog cards render them as colored chips, and
  the catalog list pages gain a tag filter dropdown. Because published
  versions are immutable, an in-use tag cannot be deleted — `DELETE` answers
  409 and the tag is deactivated instead (hidden from new publishes, still
  displayed on the versions that carry it); the never-written entry-level
  `tags` columns from migration 000002 are dropped.

- **The tag vocabulary can be defined declaratively in configuration.** A new
  `instance_tags` config key (`INSTANCE_TAGS` env var as a JSON array; Helm:
  `api.instanceTags`) lists tags that the server reconciles into the database
  on every startup: listed tags are created or updated and flagged
  `managed`, making them read-only through the API and admin UI (PATCH /
  DELETE answer 409, the UI badges them "Managed" and disables the actions);
  removing an entry releases the tag back to admin-UI ownership without
  deleting it. Invalid entries (bad slug/color, duplicates) fail startup
  fast. The version-create checkbox picker also gained the tag description
  as a hover tooltip on the whole row.

## v0.4.0-rc8 — 2026-06-11

### Added

- **The new-version form pre-fills from the previous version.** Authoring
  v(n+1) starts from v(n) instead of a blank slate: transport, remote
  endpoint, package fields, capabilities, the tools list, protocol version —
  and for agents the endpoint, skill, auth scheme, modes, and provider
  metadata — are seeded from the latest version, with a patch-bumped
  version suggestion and a visible "Pre-filled from vX.Y.Z" note.

- **The tools editor explains how to dump `tools/list` from a running
  server.** A "How do I get this list from my server?" panel below the
  editor offers copy-ready recipes for three situations: the MCP Inspector
  CLI (Node), a plain `curl` Streamable-HTTP handshake for remote servers,
  and a raw JSON-RPC stdin pipe for local servers — the latter two need no
  Node at all. The paste paths now accept the commands' output verbatim:
  the `{"tools": […]}` JSON-RPC envelope is unwrapped in the JSON tab too,
  and the MCP spec's `inputSchema` spelling is adopted into `input_schema`
  (previously it was silently carried as an unknown key and never shown).

- **Remote MCP servers carry a first-class endpoint URL.** MCP server
  versions gain a `remotes` array (`[{type, url}]`, mirroring the MCP
  registry server.json) so a hosted server no longer needs a fabricated
  package entry to record where clients connect. The create-version API
  validates the entries (non-stdio transport, absolute http(s) URL), the
  admin create forms show a "Remote endpoint URL" field for non-stdio
  transports, the public detail page lists remotes under Connection /
  Installation, the host-config generator emits `url`-style snippets for
  them, and the list `transport` filter matches remotes as well as
  packages. Bootstrap specs accept `remotes:` per version.

- **Authors can ask for a public release with their submission.** The
  submit-for-review endpoints accept an optional `{"request_public": true}`
  body; approving such a submission publishes the version AND flips the
  entry's visibility to public in the same transaction (a no-op if it's
  already public). Withdrawing drops the request; a resubmission states it
  afresh. In the admin UI the submit dialog and the create forms offer the
  option on private entries, the pending version row and the review queue
  show a "public on approval" badge, and the reviewer's approve confirm
  spells out that the entry goes public.

- **Per-user access view.** New `GET /api/v1/users/{id}/grants` (Server
  Admin) aggregates every role grant contributing to a user's access —
  attached directly or inherited through local group membership — enriched
  with publisher and group handles. The admin user detail page shows it as
  an "Access" section (role / scope / via, with drill-down links), flags
  Server Admins, and notes that roles from IdP claim groups are matched at
  sign-in and aren't listed.

## v0.4.0-rc7 — 2026-06-10

### Added

- **Admin report rows name the reported entry.** `GET /api/v1/reports`
  (admin) now joins `resource_ns` / `resource_slug` / `resource_name` for
  each report, and the admin Reports page links straight to the entry's
  detail page (instead of a text search over a raw ULID). Reports whose
  target was deleted say so explicitly.
- **Entity-list sorting.** The admin MCP and Agent lists expose the API's
  `sort` orders (newest, recently updated, recently published, name A–Z/Z–A)
  in the filter bar, persisted in the URL.
- **Unsaved-changes protection.** The four create forms and the new-version
  editor warn before in-app navigation or page unload would discard typed
  input (shared `DirtyFormGuard`, router-blocker + `beforeunload`).
- **Global-only reviewers get a landing page.** A reviewer with a global
  grant and no publisher membership now lands on their queue count with a
  link to `/admin/review` instead of the "No publishers yet" empty state.
- **Role matrix help page.** `/admin/help/roles` documents what
  Viewer/Editor/Reviewer/Admin/Server Admin can do, linked from the grants
  editor.

- **Deprecation can now be reversed.** New endpoints
  `POST /api/v1/mcp/servers/{ns}/{slug}/undeprecate` and
  `POST /api/v1/agents/{ns}/{slug}/undeprecate` return a deprecated entry to
  `published`. Same review contract as the other entry-level mutations:
  Editors enqueue an `undeprecation` change request (202) that a Reviewer
  approves; Server Admins apply immediately (200); 409 when the entry is not
  deprecated. The admin detail page gains a **Republish** action and the
  lifecycle stepper's deprecated → published step is wired to it.

### Fixed

- **Admin entity lists fit their container.** Entry names are links, column
  visibility now tracks the table's actual width (container queries) so the
  Manage button no longer clips off-screen at laptop widths, dates stop
  wrapping, the selected-row highlight works, and the floating bulk bar no
  longer covers the last row.
- **Detail pages distinguish "not found" from "failed to load".** A 404
  renders the not-found branch; any other failure shows the error surface
  with the server's problem detail and a Try again button. Loading states
  use the detail skeleton, the read view shows every editable field (with an
  explicit "—" when unset), and version rows on a deprecated entry are
  annotated "(entry deprecated)".
- **One transition surface per lifecycle action.** The stepper no longer
  duplicates the Deprecate button with an unconfirmed one-click transition;
  it keeps the draft→published guidance hint and the republish affordance.
- **Audit log ergonomics.** Filters auto-apply (no Apply button), the actor
  filter offers a picker of known users by email instead of demanding a
  remembered subject UUID, raw ULIDs moved into the expanded row detail,
  resource links no longer full-page-reload the SPA, and the client-side
  action filter no longer claims "no matches" while unloaded events may
  match.
- **Grants editor.** Labels stack above their selects, the `config` badge
  explains itself, and config-seeded grants no longer offer a revoke that
  bootstrap would undo on the next start.
- **Accessibility.** WCAG AA contrast for the destructive palette and muted
  badges, a skip-to-content link in the admin shell, one focus-ring language
  across sidebar links and buttons, no heading-level skips on the dashboard,
  and only compositor-friendly properties are animated.
- **Admin create-form slug validation works again, and server errors surface
  in full.** The slug `pattern` attribute (`^[a-z0-9-]+`) failed to compile
  under the `v` regex flag modern browsers apply, so client-side validation
  was silently skipped in all four create forms (MCP server, agent, publisher,
  group). The shared `SlugField` now uses a compatible pattern, validates on
  blur with an inline `aria-invalid` error, and caps length at 63. Create-form
  failures also show the server's problem-details `detail` (e.g. which slug
  collided) instead of the bare status title.
- **One confirmation dialect across the admin.** Every remaining browser
  `confirm()` (bulk delete/deprecate on the entity lists, single-row Delete
  and Deprecate, deletion requests) now uses the shared themed dialog, naming
  the target and the consequence. Button weight now matches reversibility:
  the reversible Deprecate is a quiet outline button, while the irreversible
  break-glass Delete is solid destructive red (previously inverted, and the
  muted Delete read as disabled). Type-to-confirm stays reserved for cascade
  deletes (publishers).
- **Interactive sign-in lands on the admin console.** Completing an OIDC
  sign-in with no pending deep link now lands on `/admin` instead of the
  public homepage the server redirect points at — matching the local-login
  behavior. Deep links captured before sign-in are still honored.
- **The "Report an issue" dialog renders centered.** Tailwind's preflight
  stripped the `margin: auto` that centers a modal `<dialog>`, pinning it to
  the top-left corner; `m-auto` restores centering.
- **Reviewers no longer approve blind, and Approve confirms first.** Version
  items in the review queue gain an expandable "Show submitted content" panel
  (runtime, protocol, packages, tools for MCP; endpoint, skills, auth for
  agents) so the reviewed content is visible without leaving the queue.
  Approve now opens a confirmation naming the entry and consequence —
  "Approve & publish" for versions, a destructive "Approve deletion" for
  deletion requests (previously one unconfirmed click hard-deleted the entry).
  The entry page's Publish button on a pending version now reads
  "Approve & publish" too, so the two surfaces are recognisably the same
  action, and its browser confirm() was replaced by the themed dialog. Queue
  resource links no longer full-page reload (SPA navigation).
- **The create forms tell editors the truth about publishing.** For callers
  without the Reviewer role the "Publish version immediately" checkbox is now
  "Submit version for review" and actually submits the version (previously the
  form fired a publish that 403'd and **silently swallowed the error**,
  leaving a plain draft with no explanation). Publish/submit failures after a
  successful create now surface as a toast with the server's detail. The
  versions section no longer links non-reviewers to the review queue they
  cannot open, and editors get a one-line pipeline explainer on the entry
  page (submit → review → approve → make public).
- **High-blast-radius admin actions now confirm first, and self-lockout is
  blocked.** Disable/Enable account, Grant/Revoke Server Admin, and role-grant
  revocation no longer fire on a single click — a themed confirmation dialog
  names the user/principal and the consequence (new shared `ConfirmDialog`,
  replacing nothing-at-all on these surfaces). The server additionally rejects
  disabling your own account or revoking your own Server Admin role (409), so
  an instance cannot lose its last administrator; the UI disables those
  buttons on your own user page with an explanation.
- **Signed-out and under-privileged deep links no longer die silently.**
  Visiting an `/admin/...` URL signed out now lands on the login page with a
  "Sign in to continue to …" hint, and signing in (local or OIDC) resumes the
  original destination instead of dropping it. A non-admin deep-linking a
  Server-Admin page (e.g. `/admin/users`) is bounced to the dashboard with a
  visible "Server Admin access required" notice instead of a silent redirect.
  Only same-origin paths are honored as return destinations.
- **The admin lifecycle stepper no longer renders dead controls.** Clicking
  "Published" on a draft entry now scrolls to the Versions section with a
  hint (publishing happens per version) instead of silently doing nothing;
  on a deprecated entry it republishes. Transition targets are hidden for
  read-only viewers and while a change is pending review. The Deprecate
  confirm no longer claims the action "cannot be undone".

- **Force-deleting an entry no longer strands review-queue items.** Admin
  force-delete (MCP servers + agents) now also sets the `deleted_at` tombstone,
  cancels any in-flight version submission, and drops any pending entry-change
  request in the same transaction. Previously an entry force-deleted while a
  deletion request was pending left a permanently-stuck queue item that could
  be neither approved nor rejected and inflated the reviewer badge forever. The
  review-queue query additionally excludes `status='deleted'` entries from
  every branch, so pre-fix residue is hidden too.

### Changed

- **Entry-level mutations now route through the review queue.** Changing an
  entry's visibility (`POST .../visibility`), deprecating it
  (`POST .../deprecate`), or editing its metadata (`PATCH .../{ns}/{slug}`) no
  longer takes effect immediately for publisher Editors — the request is now
  **enqueued for review** (HTTP `202`, was `200`) and a Reviewer approves it
  before it applies. Previously only version content and entry deletion went
  through the queue; these three actions bypassed it. **Server Admins keep the
  immediate path** (still `200`) as a break-glass escape hatch, consistent with
  reviewer direct-publish and admin force-delete. New endpoints:
  `POST .../change-request/{approve,reject,withdraw}` (MCP + agents). The review
  queue and its count now include `mcp_change` / `agent_change` items, and the
  admin detail page shows a "pending review" banner with a **Withdraw** action.
  API clients that relied on `200` from these three endpoints as a non-admin
  must handle `202` + the approval step.

### Added

- **Dual-mode tools editor in the admin version-authoring forms.** The
  publisher-declared `tools[]` array is now authored either as structured cards
  (a "Form" tab: per-tool name, description, annotation-hint toggles, and a JSON
  sub-editor for `input_schema`) or as a raw array (a "JSON" tab, for pasting a
  `tools/list` response), replacing the single raw-JSON textarea. Both tabs
  share one validator, so switching is lossless and surfaces the offending JSON
  path (e.g. `tools[2].input_schema`) instead of a generic submit-time error.
  Applies to both the create-server form and the inline New version form.
- **Create a new version of an existing MCP server / agent from the admin UI.**
  Previously a subsequent version could only be created at resource-creation
  time or via the API directly — the detail page's Versions section listed
  versions but offered no way to add one. It now has a **New version** button
  (publisher Editor / Server Admin only) that opens an inline form to author a
  new draft version; submitting it for review stays on the existing Submit
  button / review queue.
- **Publish a version directly from the Versions section.** Reviewers (and
  Server Admins) get a **Publish** button on any unpublished version — the
  go-live action previously existed only as a "publish on create" checkbox, so
  an approved/draft version had no UI to publish.
- **Richer version-authoring fields.** The New version form now exposes fields
  the API already accepted but no UI did: MCP `capabilities` (JSON) and
  package `registryBaseUrl`; agent `capabilities` (JSON), `provider`,
  `documentation_url`, `icon_url`, and skill `examples`.
- **End-to-end user-journey coverage** (`web/e2e/user-journeys.spec.ts`): full
  MCP create → publish → edit → second-version → publish → deprecate → delete
  lifecycle, switcher-scoped lists, create-form publisher pre-select, "Load
  more" append, the filtered empty state, agent rich-field round-trips,
  read-only Settings for a non-admin member, the bulk-deprecate confirmation,
  and the review workflow's separation of duties (an Editor submits but cannot
  approve — UI and API 403 — while a distinct Reviewer approves from the queue)
  — run against the live Docker stack.
- **End-to-end failure-path + RBAC coverage** (`web/e2e/ux-edge-cases.spec.ts`):
  uses request interception to force API 500s and assert the list error state,
  the New-version JSON validation, a detail-action error toast, bulk
  partial-failure reporting, and editor-vs-Server-Admin action gating.

### Changed

- **Admin resource lists respect the selected publisher.** The MCP and Agent
  admin lists were scoped only by `mine=true` (every publisher the caller
  manages); they now also scope to the publisher chosen in the switcher, while
  an explicit publisher filter still takes precedence and a Server Admin's
  All-publishers view stays unscoped.
- **The create-resource forms pre-select the current publisher.** The "New
  Server" / "New Agent" publisher dropdown defaults to the publisher the admin
  area is scoped to (when the caller can author on it). The field is now
  labelled "Publisher" consistently (was "Namespace (publisher)").
- **Publisher Settings is viewable read-only by members.** Non-admin publisher
  members previously hit an "Admin access required" wall; they can now open
  Settings in a read-only view, while editing remains gated on the Admin role
  (and the write endpoint is authorized server-side regardless).
- **List pages show skeletons while loading and a clear error on failure.** The
  publishers/users/groups/MCP/agents lists previously flashed their empty state
  during load and had no error surface; they now use a table skeleton, a shared
  error state, and the standard `EmptyState` (with a create CTA).

### Fixed

- **"Load more" no longer hides earlier rows.** The MCP and Agent admin lists
  paginated by pushing a cursor into the URL, which *replaced* the visible rows
  with the next page. They now use an infinite query that appends pages.
- **Detail-page actions report the real error and no longer silently succeed.**
  The MCP/agent visibility and deprecate mutations ignored the API's
  `{ error }` result (so failures still ran `onSuccess`); they now surface the
  server's RFC 7807 detail via a toast, as do edit/delete. Edit forms show the
  specific message instead of a generic "Update failed".
- **Publisher delete no longer understates its blast radius.** If the owned-
  resource counts failed to load, the delete confirmation claimed the publisher
  "owns no MCP servers or agents"; it now warns that the impact is unknown.
- **Bulk actions run concurrently and tolerate partial failure**, reporting
  "N of M succeeded" instead of aborting the batch on the first error; bulk
  deprecate now asks for confirmation like the single-item action.
- **API errors on list / delete-cascade / bulk paths are no longer swallowed.**
  `openapi-fetch` resolves (rather than rejects) on a non-2xx, so `queryFn`s
  that returned `r.data` and bulk callbacks that ignored the result treated 5xx
  as success — meaning the list error state, the publisher delete-cascade
  "impact unknown" warning, and bulk partial-failure reporting never actually
  fired. Those paths now throw on `error`, so the error UI works as intended.
- **Create-form publisher pre-select now works for a Server Admin.** The
  publisher list loads asynchronously for a Server Admin, so the one-shot
  default missed it; the selection is now derived (and ignores Radix's spurious
  empty-value callback) so the trigger reliably shows the scoped publisher.
- **Dev stack starts again: the Keycloak healthcheck used `curl`,** which the
  `keycloak:26.6` image doesn't ship — so the container was perpetually
  `unhealthy` and the `server` (which `depends_on` it) never started under
  `docker compose --profile dev up`. The healthcheck now uses a bash `/dev/tcp`
  probe.
- **Smaller consistency fixes:** success toasts on create/edit; set-password
  refreshes the user; internal links use the SPA router (no full reloads); the
  audit page title reads "Audit log"; the bulk action bar no longer overflows
  narrow screens; the dashboard stats error no longer leaks internal details.

## v0.4.0-rc6 — 2026-06-09

### Fixed

- **Creating a version without its optional list field no longer fails.** Both
  `POST /api/v1/mcp/servers/{namespace}/{slug}/versions` (with no `packages`)
  and `POST /api/v1/agents/{namespace}/{slug}/versions` (with no `skills`)
  rejected the absent — but OpenAPI-optional — field with `422 Unprocessable
  Entity` ("packages/skills must not be empty"), so the admin "New Server" /
  "New Agent" forms surfaced "… created, but version creation failed" whenever
  the optional package/skill section was left blank. Each handler now validates
  the field's structure only when entries are supplied, and the store defaults
  an empty `packages`/`skills` to `[]` (matching `capabilities`, `tools`, and
  `authentication`).
- **A deleted MCP server / agent slug can be reused.** Deletion is a soft
  delete (the row is kept for audit), but the table-level `UNIQUE
  (publisher_id, slug)` covered deleted rows, so re-creating a previously
  deleted slug failed with a duplicate/conflict error. The constraint is now a
  partial unique index over live (non-deleted) rows only (migration `000017`),
  and the single-row getters and view/copy-count updates filter out deleted
  rows so a lookup resolves to the live entry rather than a shadowing
  tombstone.
- **The admin list refreshes after creating a server/agent.** The create flows
  navigated to the new detail page without invalidating the cached admin list,
  so (with the 30s query `staleTime`) the new entry was missing from the list
  until a hard refresh. Both create mutations now invalidate the `['admin-mcp']`
  / `['admin-agents']` list cache on success.

### Changed

- **Access logs now identify the authenticated caller.** Every `http request`
  log line carries a `user_email` field — the email of the bearer-token
  principal, or `anonymous` for unauthenticated (public) requests. Combined with
  the existing `path`, `method`, and `status` fields this answers "which user
  accessed which path" directly from `docker logs` / the OTLP log stream. The
  `Authenticate` middleware now runs ahead of the request logger so the resolved
  principal is on the request context when the line is emitted; tokens are never
  logged.

## v0.4.0-rc5 — 2026-06-05

### Changed

- **Deleting a publisher now cascades to all of its resources.** `DELETE
  /api/v1/publishers/{slug}` removes every MCP server and agent the publisher
  owns (regardless of status), their versions, and any reports filed against
  them, in a single transaction — instead of returning `409 Conflict` when the
  publisher still owned active entries. The endpoint no longer responds with
  `409`.
- **Admin publisher deletion now requires an explicit, type-to-confirm step.**
  Because the delete is irreversible and cascades, the admin UI replaces the
  plain confirm dialog with a danger panel that spells out how many MCP servers
  and agents will be removed and keeps the delete button disabled until the
  operator types the publisher's exact name.

## v0.4.0-rc4 — 2026-06-05

### Changed

- **Reworked CI into focused, parallel workflows.** The monolithic `ci.yml`,
  `e2e.yml`, and `publish.yml` are split into `lint.yml`, `quality.yml` (build,
  unit, integration, and Playwright e2e against native host processes with
  Postgres/Keycloak service containers), `docker.yml`, `helm-publish.yml`, and
  `release.yml` (#125, #127).
- **Consolidated Docker Compose onto a single root `docker-compose.yml`.** The
  separate `deploy/docker-compose.yml` and `docker-compose.ci.yml` are removed;
  CI no longer runs compose at all (#125).
- **Restructured the Helm chart for clarity.** Templates are grouped per
  component (`api/`, `webapp/`) with per-kind files and split secrets, the
  values layout is reorganised, and a `helm-docs`-generated chart `README.md` is
  now published alongside it (#126).

## v0.4.0-rc3 — 2026-06-04

### Added

- **Accept IdP-issued service-account access tokens for machine-to-machine
  callers.** A deployment can now opt into honouring an IdP-minted (e.g.
  Keycloak service-account, `client_credentials`) access token directly on the
  API, for non-interactive clients that cannot run the browser login flow (a
  Kubernetes operator, a CI job). A bearer that is not a registry token is
  verified offline against the IdP JWKS and accepted **only** when its `aud`
  claim contains the value configured in the new `OIDC_AUDIENCE` knob — a token
  minted for another client in the same realm is never honoured, so the
  audience pin is the security boundary (configure a Keycloak audience mapper so
  the SA's tokens carry it). The realm-admin role (`OIDC_ADMIN_ROLE`) maps to
  Server Admin and the groups claim drives publisher-scoped RBAC, exactly as a
  brokered login. The path requires the OIDC broker to be configured
  (`OIDC_CLIENT_ID` + `OIDC_CLIENT_SECRET`) and is **disabled by default**:
  with `OIDC_AUDIENCE` empty, only registry-issued tokens are accepted, leaving
  the IdP-less / break-glass deployment unchanged.
- **Configurable email claim path (`AUTH_EMAIL_CLAIM`, default `email`).** The
  broker reads the user's email from this `id_token` claim at login; dotted
  paths address nested objects. Override it for IdPs that emit the email under a
  different name (the email is required — `users.email` is `NOT NULL`).

## v0.4.0-rc2 — 2026-06-04

### Changed

- **Auth reworked from BFF cookie sessions to registry-issued bearer tokens.**
  The `Secure; HttpOnly` registry session cookie introduced in v0.4.0-rc1 is
  gone. Both front doors (local email+password and brokered OIDC) now mint a
  short-lived **Ed25519 access token** (a registry JWT sent as
  `Authorization: Bearer <token>`, validated by signature alone and never
  stored) plus a long-lived, **single-use rotating refresh token** (opaque;
  only its SHA-256 hash is persisted, so a DB leak yields no usable token).
  `POST /api/v1/auth/refresh` rotates the pair — replaying an already-rotated
  token is treated as theft and revokes the whole lineage — and
  `POST /api/v1/auth/logout` revokes the presented refresh token. The brokered
  OIDC flow no longer relies on a transaction cookie: login-transaction state
  lives server-side in `oidc_auth_requests`, and the callback hands minted
  tokens to the SPA via a one-time code (`#code=...` fragment) exchanged at
  `POST /api/v1/auth/oidc/exchange`, so tokens never appear in a URL. The SPA
  holds its tokens in `tokens.ts` and attaches the bearer header in
  `api-client.ts`; OIDC claim group membership and the Server-Admin flag are
  still snapshotted at login (now into the refresh token). Server-side OIDC
  brokering, PKCE, and the IdP token never reaching the browser are unchanged.

  > **Breaking config change.** Migration `000016_bearer_auth` drops the
  > `sessions` table and adds `refresh_tokens`, `oidc_auth_requests`, and
  > `auth_handoff_codes`. The `AUTH_SESSION_*` knobs
  > (`AUTH_SESSION_COOKIE_NAME`, `AUTH_SESSION_TTL`, `AUTH_SESSION_SECURE`,
  > `AUTH_SESSION_SAMESITE`) are removed. New knobs (env + YAML + default per
  > CLAUDE.md): `JWT_SIGNING_KEY` / `JWT_SIGNING_SEED` (the Ed25519 signing
  > credential — supply via env/secret; an empty value generates an **ephemeral
  > dev-only key** that does not survive a restart), `ACCESS_TOKEN_TTL`
  > (default `15m`), `REFRESH_TOKEN_TTL` (default `12h`), `OIDC_ROLES_CLAIM`
  > (default `realm_access.roles`), and `OIDC_ADMIN_ROLE`. The Helm chart adds a
  > `jwt-secret` template wiring the signing key through a Kubernetes Secret.
- **Group role grants are now Server-Admin-only.** A group grant binds an IdP
  claim to a role, so its membership is controlled outside the registry. A
  publisher Admin may still grant/revoke roles for individual **users** on their
  publisher, but creating or deleting a **group** grant now requires Server Admin
  (the server returns 403 otherwise). The grants UI hides the group option and
  the revoke control on group rows for non-Server-Admins.

### Fixed

- **The "New MCP server" / "New agent" publisher dropdowns now list only the
  publishers the caller can author on.** Both create forms fetched the full
  `GET /api/v1/publishers` list, so a non-admin author saw (and could select)
  every publisher — the write then 403'd server-side. The dropdowns now derive
  their options from the caller's grants (via `PublisherContext`) filtered to
  publishers where they hold an authoring role; a Server Admin still sees all.
- **No more logging in twice after a brokered OIDC sign-in.** The SPA exchanged
  the one-time handoff code for tokens but did not refresh its auth state before
  the first guarded navigation, so the user landed back on the login screen and
  had to authenticate again. `AuthContext` now hydrates from the freshly
  exchanged tokens immediately, so a single OIDC login lands the user in the
  admin UI.

### Security

- **Server-Admin admin routes are now guarded in the SPA.** The cross-publisher
  management surfaces (users, groups, publishers, global grants, reports, audit,
  API keys) are wrapped in a `RequireServerAdmin` route guard, so a
  non-Server-Admin navigating directly to one of those URLs is redirected to the
  dashboard instead of rendering a shell that 403s on every request. The
  per-publisher detail page stays accessible to publisher Admins. This is
  defence-in-depth — the APIs were already enforced server-side.
- **Default `REFRESH_TOKEN_TTL` lowered from `720h` (30 days) to `12h`.** OIDC
  claim group memberships are snapshotted at login and only re-read on a fresh
  login, so the refresh-token lifetime bounds how long a group removed in the
  IdP keeps conferring roles in the registry. A shorter default tightens that
  revocation window. Operators who relied on long-lived sessions can restore the
  old behaviour by setting `REFRESH_TOKEN_TTL=720h`.

## v0.4.0-rc1 — 2026-06-02

### 📡 OTLP export for all three signals (metrics + logs, not just traces)

The CLAUDE.md mandate is "OpenTelemetry for all signals — traces, metrics, and
logs — exported via OTLP", but only traces were exported. Now, when an OTLP
endpoint is configured:

- **Metrics** are pushed via an OTLP periodic reader **in addition to** the
  Prometheus pull endpoint (`/metrics`). The same OTel instruments feed both, so
  Prometheus scraping is unchanged.
- **Logs** are bridged to OTLP via `otelslog`. Structured JSON to stdout is
  retained (a fan-out slog handler writes to both), so `kubectl logs` stays
  useful while logs also reach the collector — still carrying `trace_id` /
  `span_id`.
- All three exporters share one gRPC connection to the collector.
- The dev collector config gains a `logs` pipeline.

When no OTLP endpoint is configured, behaviour is unchanged (stdout logs +
Prometheus `/metrics` only).
### 🏗️ Multi-arch images + supply-chain attestations + stronger password floor

- **Images are now built for `linux/amd64` and `linux/arm64`.** Both Dockerfiles
  pin their build stage to `$BUILDPLATFORM`: the Go server cross-compiles
  (`GOARCH=$TARGETARCH`) and the web build emits arch-independent static assets,
  so arm64 builds run **without QEMU emulation**. This also fixes a latent bug —
  the server Dockerfile's `ARG TARGETARCH=amd64` default would have shadowed the
  injected target arch and shipped an amd64 binary inside an arm64 image.
- **Supply-chain attestations:** the publish workflow now generates an **SBOM**
  and **max-mode provenance** for both images and pushes them to GHCR.
- **Local-account password floor raised from 8 to 12 characters** (OWASP
  guidance) for the self-service set-password endpoint.
- The web image declares an explicit non-root `USER 101`; `web/.next` is added to
  `.dockerignore`.
### 🧹 Hygiene: untrack `deploy/.env`, drop stale copy + debug test

- **`deploy/.env` is no longer tracked.** It held dev quick-start values
  (`OIDC_CLIENT_SECRET=dev-broker-secret`, a default bootstrap-admin password)
  and was committed before `.gitignore` listed it, so the ignore rule was inert
  and the file would eventually catch a real secret. Untracked via
  `git rm --cached`; the local file is kept and `deploy/.env.example` remains the
  reference.
- **API Keys page copy fixed:** it told users to "use your Keycloak access token"
  for automation, but after the BFF rework the browser holds no token. Updated to
  explain the session-cookie model and that M2M API keys are still on the roadmap.
- Removed `web/e2e/debug-agent.spec.ts`, a leftover debugging scratch test with a
  5 s hard sleep and no assertions.
### 📈 Fix: `/metrics` is now scrapeable (metrics were silently dead on k8s)

`/metrics` was gated behind `RequireAdmin`, which needs a registry session
cookie. The Prometheus Operator `ServiceMonitor` scrapes the in-cluster
ClusterIP Service and cannot present that cookie, so every scrape got `403` and
**no OTel metric was ever collected on Kubernetes**. The endpoint is now
unauthenticated: its payload is non-sensitive (request counters, latency
histograms, registry-entry gauges) and the shipped Ingress does not route
`/metrics` externally. Add a `NetworkPolicy` if you need to restrict pod-to-pod
scraping.

### 🛡️ CSRF defense + security-header hardening

- **CSRF protection is now actually enforced.** A new `EnforceSameOrigin`
  middleware rejects cross-site state-changing requests (`POST/PUT/PATCH/DELETE`)
  using the Fetch-Metadata `Sec-Fetch-Site` header, falling back to an `Origin`
  allowlist (same list as CORS) for clients that don't send it. This closes the
  gap where the docs claimed a "double-submit token" that did not exist; the real
  defense is now `SameSite` cookies + same-origin enforcement + the
  `application/json` content-type requirement, and it holds even when
  `AUTH_SESSION_SAMESITE=none` is used for a cross-origin SPA.
- **`Content-Security-Policy`** is sent on every response — a deny-by-default
  policy for the JSON/YAML API surface, with a tailored policy on `/docs` so the
  Scalar reference UI still loads.
- **`Strict-Transport-Security`** is sent when the deployment serves over HTTPS
  (mirrors the Secure-cookie setting).
- **CORS now matches the cookie model.** For an exact allowlisted origin the
  server emits `Access-Control-Allow-Credentials: true` (required for a
  cross-origin SPA fetching with `credentials: 'include'`); the `*` wildcard
  still never carries credentials. The stale "bearer-only" rationale is removed.

### 🔐 Brokered OIDC + registry cookie sessions; `/v0` removed

Authentication is reworked into a **backend-for-frontend (BFF)** model. The SPA
is no longer an OIDC client: it holds **no token**, just a `Secure; HttpOnly`
registry **session cookie**. OIDC is **brokered server-side** — the registry is a
single **confidential** client that runs the Authorization Code + PKCE flow and
maps the external identity to an internal user; the IdP token never reaches the
browser. Local email + password login sets the same session cookie. Session
tokens are stored only as a SHA-256 hash, and OIDC claim group membership +
Server-Admin flag are snapshotted into the session at login.

- **Sign-out now ends the IdP session too**: an OIDC logout bounces through the
  provider's RP-initiated logout (`end_session_endpoint`, with `id_token_hint`)
  so the SSO session is terminated, not just the registry cookie.
- **The MCP-registry-spec `/v0` surface is removed** along with the OAuth
  resource-server role and the multi-issuer token validator; MCP servers are
  exposed only via `/api/v1`. A2A Agent Card compatibility is unchanged.
- **Bootstrap can seed RBAC**: `groups` and `grants` sections in the bootstrap
  file let a stack provision group→publisher role grants on boot (the dev seed
  maps the Keycloak demo groups so the `author` fixture is a publisher Admin).
- Dead auth config removed (`AUTH_STORAGE`, `AUTH_LOCAL_SIGNING_KEY`,
  `AUTH_LOCAL_TOKEN_TTL`, `OIDC_AUDIENCE`); new knobs: `OIDC_CLIENT_SECRET`,
  `OIDC_INTERNAL_URL`, `AUTH_SESSION_*`. The SPA drops the `oidc-client-ts`
  dependency. E2E + CI updated to the cookie/brokered flow.

## v0.4.0-rc0 — 2026-05-31

### 🔑 MIT license + Helm local-login support

The repository now ships a top-level **MIT `LICENSE`** (it was previously
unlicensed). The Helm chart gains local email + password login for parity with
docker-compose: `server.localLogin.enabled` plus
`server.localLogin.bootstrapAdmin.{email,password,existingSecret}`. The
bootstrap-admin password is wired through a Kubernetes Secret — an inline value
is rendered into one, or reference an `existingSecret` you manage
(external-secrets / sealed-secrets) — and is **never** placed in the ConfigMap.

### 🔖 Release tooling + roadmap aligned for 0.4.0

The publish workflow now marks hyphenated SemVer tags (e.g. `v0.4.0-rc0`) as
GitHub **pre-releases**, so a release candidate is never ranked "Latest" over
the previous stable tag. Roadmap docs (README / PLAN / CLAUDE) updated: **0.4.0
ships the authorization epic** (publisher-scoped RBAC + local accounts
+ the publisher-scoped admin home; the workspace layer is removed), and API-key
(M2M) auth plus a production `docker-compose.prod.yml` profile move to a later
minor.

### ⚙️ Publisher Admins can edit their publisher's metadata

`PATCH /api/v1/publishers/{slug}` is now a publisher **Admin** action instead of
Server-Admin-only: the guard moves from `RequireAdmin` to publisher-scoped
`RequirePublisherRole(Admin)` (Server Admin keeps break-glass access; a publisher
Editor/Viewer gets 403, an anonymous caller 401). A new scoped **Settings** page
(`/admin/settings`, in the publisher nav for Admins) lets them edit the name +
contact without the Server-Admin Publishers page. The slug stays permanent and
**deleting** a publisher remains Server-Admin-only (it removes the whole tenant).

### 🛠️ Server Admins can scope the admin home to any publisher (+ e2e)

The publisher switcher now offers a Server Admin **every** publisher, not just
the ones they happen to hold a grant on, so they can open any publisher's scoped
Overview / Members / Activity. Server Admins still default to the global "All
publishers" dashboard. Adds Playwright e2e covering the switcher and the scoped
home (Overview, Members, Activity), and documents the feature in the README.

### 👥 Per-publisher Members + Activity pages

Two publisher-scoped admin pages, reachable from the sidebar when a publisher is
selected. **Members** (`/admin/members`) manages the selected
publisher's role grants — a first-class home for a publisher Admin, who
previously had no nav path to it; gated to Admins, with a hint otherwise.
**Activity** (`/admin/activity`) is the full, paginated activity feed for the
publisher (any member). The global audit nav item is renamed **Audit log** to
distinguish it from the per-publisher feed. (Editing publisher *metadata* is now
a publisher-Admin action too — see the Settings entry above.)

### 🏠 Publisher-scoped admin Overview

The `/admin` landing page is now a publisher-scoped Overview when a publisher is
selected (Server Admins viewing "All publishers" keep the global dashboard; a
caller with no publishers gets a short empty state). The Overview answers, in
order: what needs you (an attention strip of role-gated tiles — items awaiting
your review, drafts in progress — shown only when non-zero), the state of the
publisher (MCP/agent/member counts with status bars, from
`/publishers/{slug}/stats`), and what happened (a recent-activity timeline from
`/publishers/{slug}/activity`). Includes onboarding and empty states, and a
staggered timeline reveal that collapses under `prefers-reduced-motion`.

### 🧭 Publisher switcher + grouped admin nav (scoped admin home, frontend)

The admin shell now centers on a **publisher context**. A switcher in the
sidebar lists the publishers you hold a role on (derived from `GET /api/v1/me`,
so it matches the server's authorization); a Server Admin also gets an "All
publishers" scope. The selection persists across reloads. The sidebar nav is
split into a publisher-scoped group (Dashboard, MCP Servers, Agents, Review
queue) and a visually separated **Server admin** group (Publishers, Groups,
Users, Global grants, Reports, Activity, API Keys) shown only to Server Admins.

This is the foundation for the publisher-scoped dashboard; the Overview and the
per-publisher pages consume `currentSlug` in follow-up changes.

### 📊 Per-publisher stats + activity endpoints (scoped admin home, backend)

Two read endpoints that let a publisher member see their own publisher without
the Server-Admin-only global `/stats` and `/audit`:

- `GET /api/v1/publishers/{slug}/stats` — MCP/agent counts + status breakdowns,
  member counts by role, and a pending-review count, all scoped to one
  publisher.
- `GET /api/v1/publishers/{slug}/activity` — the publisher's audit feed (newest
  first, paginated), covering the lifecycle of every MCP server and agent under
  it. Unlike the public per-resource feed, this members-only feed names the
  actor.

Both are gated to any member of the publisher (Viewer and up) or a Server Admin;
a non-member gets 403. This is the backend for an upcoming publisher-scoped
admin dashboard.

### 🔒 Auth hardening: required audience binding + login enumeration fixes

**`OIDC_AUDIENCE` is now required (fail-closed).** The server previously skipped
the JWT `aud` check when no audience was configured, so any token the realm
signed for *any* client was accepted — contrary to the OAuth 2.1 resource-
indicator requirement for the MCP surface. The server now refuses to boot when
`OIDC_AUDIENCE` (YAML `auth.oidc_audience`, Helm `server.oidcAudience`) is empty.
**Action required:** set it to this resource server's audience (the bundled
stack uses `ai-registry-server`) and ensure your IdP emits that `aud`. The
docker-compose stack and the example config / Helm values already default to
`ai-registry-server`.

> **Breaking config change.** `OIDC_AUDIENCE` (YAML `auth.oidc_audience`, Helm
> `server.oidcAudience`) is now mandatory — the server aborts startup with a
> clear error when it is empty. Deployments that previously relied on the
> empty default (no `aud` check) MUST set it before upgrading, and the IdP must
> issue tokens carrying that audience or every request will be rejected with
> 401. The bundled docker-compose stack and the example config / Helm values
> already default to `ai-registry-server`.

**Local login no longer leaks which accounts exist.** Two side channels were
closed on `POST /api/v1/auth/login`: (1) unknown emails and password-less
accounts now run a constant-cost dummy password verification so response timing
no longer distinguishes "account exists" from "doesn't"; (2) a disabled account
with a *wrong* password now returns the uniform `401` instead of a distinct
`403`. A caller who supplies the correct password still gets the explanatory
`403 "this account is disabled"`.

### 🔒 Publisher-scoped read visibility for detail reads and the review queue

Closed two gaps where reads weren't scoped to the caller's publisher.

**Detail / versions / activity reads are now publisher-role-aware.** Previously
a resource's detail (`GET …/mcp/servers/{ns}/{slug}`, its `/versions[/…]`, and
`/activity`, plus the agent equivalents) only revealed private/draft entries to
a **Server Admin** — so a publisher's own Editor/Admin could see their private
draft in the `mine=true` list but got a **404** opening it. These reads now
honor the caller's role on the owning publisher: a member (Viewer and up) sees
their publisher's private/draft entries, while a member of a *different*
publisher (or an anonymous caller) still gets public-only — one publisher's
private data is never exposed to another's.

**The review queue (`GET /api/v1/review-queue`) is now per-publisher scoped.**
It previously required membership in the Keycloak reviewer group (so a
per-publisher Reviewer granted in-registry got 403) and listed pending items
across **all** publishers. Now a Server Admin or a holder of a global Reviewer
grant sees every publisher's items, a per-publisher Reviewer sees only the
publishers they review, and a caller who holds no Reviewer role anywhere gets
403. (A local bootstrap Server Admin can now reach the queue too.)

### 🛂 Reviewer is the sole approver; going public needs approval

Tightened the authorization lattice for separation of duties. A
publisher **Admin can now do everything on the publisher except approve
changes** — `domain.Satisfies` no longer treats `admin` as satisfying
`reviewer`. Approving a submitted version (and publishing a version directly)
requires the **Reviewer** role; the global **Server Admin** is the one
break-glass exception. So no single per-publisher principal can both author and
sign off the same change.

Going public is now a gated, two-step flow: a Reviewer approves (publishes) the
version, then an **Editor or Admin** switches visibility to `public`. The
`POST …/visibility` endpoint moved from Server-Admin-only to Editor/Admin, and
**rejects `public` (409) unless the entry already has an approved (published)
version** — an unreviewed draft can no longer be exposed. The admin UI reflects
this: `Make public` is disabled until the entry is approved, and `canReview` no
longer follows from the admin role.

### 🔭 Publisher-scoped admin visibility + `GET /api/v1/me`

The admin list endpoints (`GET /api/v1/mcp/servers`, `GET /api/v1/agents`)
gained a `mine=true` query parameter that scopes the listing to the resources
the authenticated caller can manage: Server Admins and global-grant
holders still see every publisher, an author sees only the publishers they hold
a role on — **including their own private and draft entries** — and a caller
with no grants sees nothing. This is how the admin UI keeps multiple authors
from seeing each other's resources.

A new `GET /api/v1/me` returns the caller's resolved identity and effective
role grants (per-publisher and global, plus `is_server_admin`), so the SPA can
gate the admin UI by role without trusting any client-side claim.

The **admin UI is now role-aware**. The MCP/agent list pages default
to `mine=true`, so authors see only their own resources. Actions and navigation
are gated by a new `usePermissions` hook: `New` appears only for Editors; edit /
deprecate / submit need Editor on the resource's publisher; approve / reject
need Reviewer; visibility flips and the direct-delete escape hatch stay
Server-Admin-only; and the Server-Admin-only nav (Publishers, Groups, Users,
Global grants, Reports, Activity) is hidden from non-admins. The server still
enforces every write.

### ✍️ Publisher Editors can author resources

Creating an MCP server or agent (`POST /api/v1/mcp/servers`, `POST
/api/v1/agents`) no longer requires Server Admin — a publisher **Editor** may
author for their own publisher (Admin / Server Admin still satisfy it). The new
resource is created **private + draft**, so it only reaches the public catalog
once a version is published and an Admin flips visibility; the approval gate is
unchanged. Anonymous create attempts get 401, non-Editors 403.

### 🔧 Pre-commit hooks + CI lint gate

Added a root [`.pre-commit-config.yaml`](.pre-commit-config.yaml) — baseline
hygiene, gitleaks, gofmt, golangci-lint, eslint, tsc, helm lint/docs, hadolint,
actionlint — plus a `pre-commit` CI job that runs the hygiene hooks, gofmt,
golangci-lint, and actionlint (the rest are already covered by dedicated jobs).
This closes the gap where Go formatting and linting weren't gated in CI. A new
[`server/.golangci.yml`](server/.golangci.yml) pins the v2 standard linter set.
Cleared the findings it surfaced (unchecked `Close()` on three `defer`s, an
unused first-call result in an auth test) and gofmt-normalised the tree.
Stopped tracking the regenerated `web/tsconfig.tsbuildinfo` build cache.

### 🚀 Tag pushes now cut a GitHub Release

The `Publish` workflow gained a `github-release` job: on a `v*.*.*` tag it
cuts a GitHub Release whose body is the matching `CHANGELOG.md` section,
gated behind a successful image + chart publish. The step is idempotent —
an existing release (e.g. cut by hand) has its notes refreshed instead of
erroring — and omits `--latest` so `gh` picks "Latest" by semver. Closes the
gap where `v0.3.2` and `v0.3.3` shipped images/charts but no Release.

### 🧹 Remove the workspace layer

Workspaces are gone. Resources are scoped directly to their owning
publisher again, and authorization is publisher-scoped RBAC (roles
granted to users/groups) rather than a per-workspace Keycloak-group
binding. This is the destructive second half (the additive RBAC +
local-auth half shipped earlier).

> **Breaking schema change.** Migration `000013_workspaces_remove`
> backfills `publisher_id` from the workspace link, flips it `NOT NULL`,
> restores the `(publisher_id, slug)` uniqueness key, drops
> `workspace_id`, and `DROP`s the `workspaces` table. It aborts with a
> friendly error if any resource still has a NULL `publisher_id` or a
> `(publisher_id, slug)` collision — resolve those before upgrading. The
> down migration recreates one `default` workspace per publisher.

- Removes the seven `/api/v1/publishers/{slug}/workspaces…` endpoints
  and the `Workspace` / `CreateWorkspaceRequest` / `WorkspaceList`
  schemas from [openapi.yaml](server/api/openapi.yaml); regenerates
  `web/src/lib/schema.d.ts`.
- Drops the `workspace` field from the MCP-server and agent create
  payloads, the workspace `<Select>` from both admin create forms, and
  the Workspaces section from the publisher detail page.
- Deletes the workspace store/handlers/domain types and the dead
  `RequireWorkspaceWrite` middleware; write/approve routes already moved
  to `RequirePublisherRole` in the previous PR.
- Removes the CI "no stray publisher_id reads" guard — `publisher_id` is
  the canonical column on `mcp_servers` / `agents` once more.

### 📐 OpenAPI server / agent status enum gains `deleted`

Follow-up to the previous entry. `MCPServer.status` and `Agent.status`
in [openapi.yaml](server/api/openapi.yaml) declared
`enum: [draft, published, deprecated]`, but the code also returns
`deleted` for soft-deleted tombstones (see
[server/internal/domain/mcp.go](server/internal/domain/mcp.go) —
`ServerStatus`, used by both entities). Public list responses filter
deleted parents out, so most callers never see one — but
`GetMCPServer` / `GetAgent` don't filter on status, so an admin hitting
`/api/v1/mcp/servers/{ns}/{slug}` after a delete can land on a deleted
record. The `/v0/servers?include_deleted=true` path is also explicit
about returning them.

- Add `deleted` to both `MCPServer.status` and `Agent.status`
  enums. Status query-param filters keep the three-value union
  intact — they're filter inputs and the list endpoints still don't
  accept `deleted`.
- Regenerate `web/src/lib/schema.d.ts` (CI gate from PR #72 enforces
  the sync).
- `StatusBadge` ([web/src/components/ui/badge.tsx](web/src/components/ui/badge.tsx)
  and `statusVariant` in
  [badge-variants.ts](web/src/components/ui/badge-variants.ts)) now
  accept `"deleted"` and render an outline-style tombstone with an
  `XCircle` icon — visually distinct from the muted `draft`, the
  green `published`, and the red `deprecated`. New unit test pins
  the variant mapping; the distinct-variants invariant is widened
  from 3 to 4 colours.

### 📐 OpenAPI version-status enum matches the code

`MCPServerVersion.status` and `AgentVersion.status` in
[openapi.yaml](server/api/openapi.yaml) declared `enum: [draft,
published, deprecated]` but the server has always returned values
from `[active, deprecated, deleted]` (see
[server/internal/domain/mcp.go](server/internal/domain/mcp.go) —
`VersionStatus`). The spec and the wire were silently inconsistent
and generated TS types mislead the SPA: `version-history.tsx`
compares `v.status !== 'active'`, which was unreachable per the
declared types. The PR-#70 E2E test originally asserted
`status === 'published'` and had to be rewritten to use
`published_at`-truthy to work around the gap.

- Fix the two version schemas to declare the actual code shape.
- Regenerate `web/src/lib/schema.d.ts` (PR #72's CI gate now keeps
  this honest going forward).
- No backend or frontend code change required — the wire shape was
  already correct.

Not in this PR: `MCPServer.status` and `Agent.status` declare
`[draft, published, deprecated]` but the code also allows `deleted`
([domain.ServerStatus](server/internal/domain/mcp.go)). That's a
missing value rather than wrong values; deleted parents are filtered
out of list responses by default, so the SPA never sees them today.
Worth a separate small change.

### 🗂️ Workspace selector on MCP / agent create forms

The MCP and agent "new" forms only let you pick a publisher; the
target workspace was always hardcoded to `default`, which made the
per-workspace group binding model invisible from the most common
entry point. A publisher with two workspaces bound to different
Keycloak groups had no UI to send a new server / agent to anything
other than `default`.

- `CreateMCPServerRequest` and `CreateAgentRequest` gain an optional
  `workspace` field (slug under the same `namespace`). Omitting it
  preserves the legacy default-workspace fallback so existing API
  callers are unaffected.
- Handlers route through `ResolveWorkspace` when set, returning 422
  with a friendly message if the workspace doesn't exist under the
  publisher. The default fallback short-circuits
  `EnsureDefaultWorkspaceID` so picking an explicit workspace never
  lazily creates `default`.
- Admin UI: a `Workspace` Select sits below `Namespace` on both
  create forms. It lists the publisher's workspaces with their group
  bindings inline (`anthropic-labs — bound to anthropic-team`) so the
  admin can see what they're picking. Selecting a publisher clears
  any stale workspace pick.
- Tests: backend handler tests cover the routed-to-explicit-workspace
  path and the unknown-workspace 422; vitest cases cover the form
  forwarding the slug into the POST body.
- Vitest `testTimeout` bumped to 15s — the new dependent
  workspace-fetch chain pushed 4 admin-form tests past the 5s ceiling
  under parallel-suite load.

### 🔎 Diagnostics, contract guard, version sync

Three small, unrelated fixes bundled to reduce ongoing operator and
reviewer friction.

- **Richer 403 detail on workspace / reviewer middleware**
  ([server/internal/auth/middleware.go](server/internal/auth/middleware.go)).
  The previous `"Insufficient permissions: workspace group membership
  required"` body was identical whether the workspace was admin-only,
  the user's JWT lacked the group, or the configured group was empty.
  Now the detail field names the required group ("Writes to this
  workspace require membership in Keycloak group `\"anthropic-core\"`.")
  and distinguishes the admin-only / empty-group cases from a
  group-mismatch. Two new unit tests pin the wording so a future edit
  can't silently re-collapse them.
- **CI lock on the generated TS schema.** `web/src/lib/schema.d.ts`
  was gitignored, so PR reviewers couldn't see openapi.yaml changes
  flow through to the SPA's typed surface. The file is now tracked
  and CI runs `npm run generate && git diff --exit-status -- src/lib/schema.d.ts`
  so any edit to `openapi.yaml` that forgets the regenerate step
  fails the build. (Bootstrap commit ships the current generated
  file; subsequent PRs touching the spec will show the schema diff.)
- **Version-string sync.** `web/package.json`,
  `web/package-lock.json`, and `deploy/helm/ai-registry/Chart.yaml`
  were all frozen at `0.1.0` while git tags reached `v0.3.3`.
  Bumped to `0.3.3` so `helm show chart` / `npm pack` /
  `package.json` no longer mislead. The publish workflow continues
  to override these from the git tag at build time
  ([.github/workflows/publish.yml](.github/workflows/publish.yml)),
  so the bump only affects local-from-source flows.

### 🔑 Dev realm refreshed for Phase 7

`deploy/keycloak-realm-dev.json` predated the workspaces / change-approval
work and only shipped an `admin` realm role — no groups, no
group-membership mapper, no demo non-admin users. That meant the Phase 7
authoring and review paths returned `403` out of the box because JWTs
never carried a `groups` claim.

- Seed four Keycloak groups matching `deploy/bootstrap.example.yaml`:
  `anthropic-core`, `anthropic-labs`, `openai-platform`, plus the
  `registry-reviewers` reviewer group (default
  `AUTH_REVIEWER_GROUP`).
- Add the `oidc-group-membership-mapper` (bare names, `full.path: false`)
  to the `ai-registry-web` and `ai-registry-cli` clients so access
  tokens actually carry `groups[]`.
- Add `author@example.com` (member of `anthropic-core` +
  `anthropic-labs`) and `reviewer@example.com` (member of
  `registry-reviewers`). `admin@example.com` and `user@example.com`
  are unchanged and stay as the admin / 403-baseline reference cases.
- README dev-stack section now lists all four users, their passwords,
  and what each one exercises.

Dev only — production realm setup is still the operator's job until the
Keycloak-reconciler work lands.

## v0.3.3 — 2026-05-25

Two workstreams shipped together as the chunky tail of v0.3.x:

1. **Phase 7 access-control + change-approval bundle plus an admin UI
   polish sweep.** Server-side work landed across PRs #28–#32; UI
   polish landed in PR #37.
2. **Project-audit follow-ups (PRs #52–#59 + #62).** A four-front
   audit (server, web, infra/config, docs) produced a P0/P1
   punch-list; the high-impact findings shipped as a batch of small,
   surgical PRs, capped by the workspaces-finalise migration (#62)
   that dropped the legacy `publisher_id` FK from resource tables.

> **Breaking schema change.** Migration `000011_workspaces_finalise`
> drops `publisher_id` from `mcp_servers`/`agents` and swaps slug
> uniqueness to `(workspace_id, slug)`. Operators upgrading from
> v0.3.2 must let the prior image's boot-time backfill run once
> before applying `000011` — the up migration aborts with a friendly
> *workspace backfill not complete* error otherwise.

### 🏢 Workspaces under publishers

A new `workspaces` entity groups MCP servers and agents under each
publisher and binds each set to a Keycloak group whose members can
author content (no group → admin-only).

- New `workspaces` table; two-step migration creates one `default`
  workspace per existing publisher and pivots resources onto it. The
  follow-up finalising migration (`000011`, shipped 2026-05-14 — see
  the "Workspaces finalise" entry below) drops the legacy `publisher_id`
  FK on MCP servers / agents. Forward-only — down migrations are
  dev-only.
- Hierarchical API:
  `GET /api/v1/publishers/{p}/workspaces`,
  `POST /api/v1/publishers/{p}/workspaces`,
  `GET/PATCH/DELETE /api/v1/publishers/{p}/workspaces/{w}`,
  `GET .../workspaces/{w}/servers`, `.../agents`.
- `RequireWorkspaceWrite` middleware: write endpoints require the
  caller's JWT `groups` claim to include the workspace's `group_name`
  (or the `admin` realm role). Configurable claim path
  (`AUTH_GROUPS_CLAIM`, default `groups`).
- Admin UI: workspace section on the publisher detail page, with
  expandable rows showing the MCPs and agents scoped to each
  workspace, plus a modal Edit dialog for renaming or rebinding.
- Bootstrap: optional top-level `workspaces:` list and per-entry
  `workspace:` reference field. Validation rejects unknown
  publisher / workspace refs up front. `group_name` is applied on
  first creation only so re-runs don't silently overwrite operator
  edits. Example YAML now seeds four demo workspaces and pins ten
  entries to them so the UI demo is populated out of the box.

### ✅ Change-approval workflow

A draft → pending review → published lifecycle that lets non-admin
group members propose changes that a global reviewer group approves
before they go live.

- New `review_state` column on MCP / agent versions, orthogonal to
  `status` / `published_at`. States: `none`, `pending_review`,
  `rejected`. A monotonic `revision` counter tracks edits across the
  version's lifetime so concurrent edits surface a discriminated 409
  (`review-revision-mismatch`) instead of clobbering each other.
- New endpoints (per resource kind):
  `POST .../versions/{v}/submit`, `.../withdraw`, `.../approve`,
  `.../reject`, plus `POST .../deletion-request` for proposing an
  entry deletion. The reviewer-only `GET /api/v1/review-queue`
  surfaces every pending item across the registry.
- Reviewer authorisation via `RequireReviewer` middleware;
  configurable via `AUTH_REVIEWER_GROUP` (default `registry-reviewers`).
- RFC 7807 error model uses discriminated `type` URIs:
  `review-state-mismatch`, `review-revision-mismatch`,
  `review-already-pending`, `already-published`. The admin UI maps
  each to a friendly error message inline.
- Admin UI: `/admin/review` queue page with approve / reject (with
  required reason) actions, a per-version history table on the entry
  detail pages with submit / withdraw / resubmit controls, a
  `RequestDeletionButton` on every entry, and a live-pinging count
  badge on the sidebar.

### 🎨 Admin UI polish

A coordinated polish pass over the admin section (PR #37) once the
new workflow surfaces had landed:

- Mobile hamburger drawer (Esc-key dismiss, body scroll lock,
  auto-close on navigation); the desktop sidebar is `hidden md:block`
  and the drawer reuses `AdminSidebar`.
- Loading skeletons on the queue, workspaces, and versions sections;
  toasts (sonner) on every change-approval mutation (submit, withdraw,
  approve, reject, request deletion) and on workspace CRUD; inline
  form-level error placement next to submit buttons. The review-queue
  badge cache is invalidated alongside change-approval toasts so the
  sidebar count stays in sync.
- Workspace edit form lives in a modal dialog (Esc to close, body
  scroll lock, `aria-modal`) instead of pushing the table down.
- Table-row primary actions (Edit, Manage) promoted from `ghost` to
  `outline` with leading icons; `DeleteButton` quieted to outline
  with destructive text so the visual hierarchy across the row stops
  collapsing into "two labels and one filled red button".
- List tables hide low-priority columns on small viewports and
  surface the slug inline under the name where the dedicated column
  is hidden; page headers wrap.

### 🧭 Project-audit follow-ups (PRs #52–#59)

A four-front audit (server, web, infra/config, docs) surfaced a P0/P1
punch-list. The high-impact items shipped as small, surgical PRs;
the deferred items are documented inline in the relevant PR
descriptions.

- **PLAN refresh** (#52) — mark v0.2.2 / v0.3.0 / v0.3.1 / v0.3.2 as
  shipped; PLAN was lagging behind the actual release state.
- **Doc + dead-code cleanup** (#54) — flip the workspace / approval
  design docs from `Proposed` → `Accepted`, drop the stale `next-themes`
  row from PLAN's Phase 6 migration table, prune dead Renovate rules
  (`next`, `eslint-config-next`, `next-auth`, `autoprefixer`,
  `tailwindcss-animate`), bump `@types/node` pin from `^22.0.0` →
  `^24.0.0` (CI runs Node 24), delete the unused
  `compatibility-info.tsx` component.
- **Config-layer fixes** (#55) — `PUBLIC_BASE_URL` and
  `BOOTSTRAP_FILE` were bypassing the config layer (read directly
  via `os.Getenv` from handlers and main), violating CLAUDE.md's
  env+YAML+default mandate. Both are now reachable via env, YAML
  (`http.public_base_url`, `bootstrap_file`), and a built-in
  default. The `--bootstrap-file` CLI flag still wins. The
  `OAuthProtectedResource` handler also dropped its silent
  `localhost:8081` fallback — empty `PublicBaseURL` now returns
  HTTP 500, mirroring `GlobalAgentCard`. 8 new tests pin the
  three-place rule.
- **Helm CNPG postgres bump** (#56) — `cnpg.postgresVersion: "16"`
  → `"18"`. Closes the version drift with the docker-compose stack,
  which moved to `postgres:18-alpine` in PR #41. `pg-probe` snippets
  in `docs/runbook.md` updated to match.
- **Migration rationale backfill** (#57) — Phase 6 (Next.js → Vite
  migration) was a cross-cutting decision but its rationale was never
  written down; #57 documented the rationale, alternatives considered,
  and historical implementation steps so the decision survives the next
  "why aren't we on Next.js?" question.
- **OTel uplift** (#58) — three observability test gaps closed:
  - `router_otel_test.go` only pinned spans for 4 hand-picked routes;
    new `router_otel_walk_test.go` enumerates every chi-registered
    route and asserts every request lands inside an `otelhttp` span,
    so a future router change that drops instrumentation on any
    real handler fails CI.
  - `internal/observability/` was at 0.0% coverage; new
    `observability_test.go` pins log-level mapping, trace_id /
    span_id injection, and metric-instrument registration.
  - `internal/problem/` was at 0.0% coverage; new tests pin the
    RFC 7807 wire shape, `omitempty` semantics, and slug-as-URL
    contract.
  - Bonus: `handlers/config.go` (the SPA's `/config.json` bootstrap)
    gained its first tests covering the auth_storage coercion and
    the empty-issuer dev-boot case.
- **P2 quality fixes** (#59) — three small surgical fixes:
  - **Counter drift on delete.** `CreateServer` / `CreateAgent`
    incremented `MCPServersTotal` / `AgentsTotal`; the matching
    delete handlers never decremented. The OTel `UpDownCounter`
    monotonically inflated. Fixed.
  - **Audit metadata silent-drop.** A `json.Marshal` failure in
    `store.LogAuditEvent` dropped metadata without a log entry;
    now we `slog.Warn` and continue with `metadata=NULL`.
  - **`flag.Parse` error swallow.** `_ = fs.Parse(os.Args[1:])` is
    now a structured `slog.Warn` so log aggregators see flag
    typos.

Deferred (documented in the audit synthesis but not shipped):
`DisallowUnknownFields` rollout (no `additionalProperties: false`
in the OpenAPI spec — needs per-endpoint risk analysis first),
rate-limiter time-based janitor (bigger change), per-handler
internal child spans, eager markdown chunk on detail pages
(`React.lazy` deferral).

### 🏗️ Workspaces finalise (Step 3, PR #62)

The finalising migration designed alongside the original workspaces
rollout but parked at the time. After this PR, MCP servers and agents
no longer carry `publisher_id`; the owning publisher is reached via
`workspaces.publisher_id` through a single JOIN.

- New migration `000011_workspaces_finalise.{up,down}.sql`. The up
  migration is gated by a `DO $$ … RAISE EXCEPTION` block: if any row
  still has `NULL workspace_id`, it aborts with a friendly
  *workspace backfill not complete* error so operators know to redeploy
  the prior image once and let the boot-time backfill run before
  retrying.
- Slug uniqueness is now **per-workspace**: `UNIQUE(workspace_id,
  slug)`. Two workspaces under one publisher may each expose a
  resource with the same slug.
- Every store query that previously read `s.publisher_id` /
  `a.publisher_id` is rewritten to JOIN through workspaces;
  `mcp_servers.publisher_id` and `agents.publisher_id` are dropped.
- `CreateMCPServerParams.PublisherID` and
  `CreateAgentParams.PublisherID` are removed. Handlers now resolve
  the workspace via the newly-exported
  `DB.EnsureDefaultWorkspaceID(publisherID)` before calling create.
- The boot-time `BackfillWorkspaces` helper and its `cmd/server/main.go`
  caller are removed — after the NOT NULL constraint lands the
  function is a no-op by construction, and its UPDATE queries
  referenced the dropped column.
- Wire-level `publisher_id` fields on MCP server / agent API
  responses remain populated (derived through the JOIN); the OpenAPI
  spec is unchanged.
- CI gains a `grep` guard that fails the build if a future change
  reintroduces `s.publisher_id` / `a.publisher_id` / INSERTs of
  `publisher_id` into the resource tables.

## v0.3.2

Helm-chart-only patch release. Four fixes that unblock a fresh
`cnpg.enabled=true` install; no server/web/API changes.

### 🐘 CNPG superuser secret is auto-created again

The Cluster resource set `spec.superuserSecret.name`, which CNPG
interprets as "the user is providing this secret" and suppresses
auto-generation. A fresh install therefore left the backend pod stuck
in `CreateContainerConfigError` with `secret
"<cluster>-superuser" not found`.

- `superuserSecret` removed from `templates/cnpg-cluster.yaml`. CNPG
  now auto-generates `<clusterName>-superuser` as intended.
- Unused `cnpg.superuserSecretName` value and helper branch deleted;
  the helper always returns `<clusterName>-superuser`.

### 🎯 DATABASE_URL targets the actual database

CNPG's auto-generated superuser `uri` hardcodes `dbname=*` (wildcard),
so even once the secret existed the server crashed on start with
`database "*" does not exist`.

- Backend deployment now builds `DATABASE_URL` from the superuser
  secret's `username` + `password`, the CNPG `-rw` service, and
  `cnpg.initdb.database`. The `uri` key is no longer consumed.
- Scheme is `postgres://` — `golang-migrate` registers its driver
  under `postgres` and fails on `postgresql://` with
  `unknown driver postgresql`.

### 🚪 Ingress disabled by default

`ingress.enabled` now defaults to `false`, matching the existing
`httpRoute.enabled=false` and `cnpg.enabled=false` defaults.
Operators opt in to the networking path they actually use instead of
discovering a stray Ingress resource on first install.

## v0.3.1

Security bugfix release. Four high-severity findings from an internal
security review are fixed; no feature changes. All API shapes stable.

### 🔐 JWT audience binding (H1)

The JWT validator now enforces the `aud` claim when `OIDC_AUDIENCE` is
set. Previously the server accepted any token minted by the configured
issuer, even one intended for a different client on the same realm —
a straight violation of the MCP authorization spec (OAuth 2.1
resource indicators).

- `auth.Validator` takes an `audience` string; when non-empty it is
  passed to `jwt.WithAudience` during parse.
- `OIDC_AUDIENCE` wired through env (`OIDC_AUDIENCE`), YAML
  (`auth.oidc_audience`), and defaults — per the CLAUDE.md config
  rule. Example + `.env.example` updated.
- Keycloak dev realm (`deploy/keycloak-realm-dev.json`) now emits
  tokens with `aud=ai-registry-server` via an inline
  `oidc-audience-mapper` on both `ai-registry-web` and
  `ai-registry-cli` clients.
- Docker Compose (`dev`, prod, CI) and Helm default
  `OIDC_AUDIENCE=ai-registry-server`.
- Tests: reject tokens missing `aud`, reject tokens with wrong `aud`,
  accept tokens with matching `aud`, and the audience check is
  skipped when `OIDC_AUDIENCE` is empty (dev-only escape hatch).

### 🔒 SPA token storage defaults to sessionStorage (H2)

Access and refresh tokens were previously held in `localStorage`,
meaning any XSS on the admin UI could exfiltrate them and reuse them
across tabs indefinitely. The SPA now defaults to `sessionStorage`
(scoped to a single tab, cleared on close), and `localStorage` is an
opt-in chosen by the server.

- `GET /config.json` returns a new `auth_storage` field
  (`"session"` | `"local"`, default `"session"`). The server rejects
  any other value and falls back to `"session"`.
- `oidc-client-ts` `UserManager` is constructed with a
  `WebStorageStateStore` backed by the chosen store.
- The Playwright-friendly `"local"` mode is still available for E2E
  because `storageState()` only captures `localStorage`. CI compose
  sets `AUTH_STORAGE=local`; no production deployment should.
- `AUTH_STORAGE` wired through env + YAML + default per CLAUDE.md.
- Tests: defaults to sessionStorage, honours `auth_storage=local`
  when served, coerces unknown values back to `"session"`.

### 🛰 Trusted-proxy gate on reporter IPs (H3)

`POST /reports` (user bug/abuse reports) was honouring
`X-Forwarded-For` from every client, letting anyone forge the
`reporter_ip` column. The endpoint now goes through the existing
`middleware.ClientIP` helper, which only accepts XFF / X-Real-IP
from peers inside `TRUSTED_PROXY_CIDR`.

- `middleware.ClientIP` was exported so handlers share the same
  trust policy as the rate-limit middleware.
- `ReportHandlers` takes a `*net.IPNet` at construction; `nil`
  disables proxy trust entirely (safe default).
- The ad-hoc `reporterIP` helper was deleted.
- Tests: XFF ignored when no trusted proxy configured; XFF honoured
  only when the remote peer is inside the configured CIDR.

### 🌐 CORS never allows credentials (H4)

Our API is bearer-only — no cookies — so echoing
`Access-Control-Allow-Credentials: true` was a latent footgun in
case a future change ever added cookie auth. The middleware now
guarantees the header is never set, and wildcard origins emit a
literal `*` instead of reflecting the request `Origin`.

- `slices.Contains(allowedOrigins, "*")` → `Allow-Origin: *`, no
  `Vary`.
- Non-wildcard match → `Allow-Origin: <origin>` + `Vary: Origin`.
- No code path sets `Allow-Credentials`, and a regression test pins
  the invariant.

### 🧪 Verification

- Unit tests: 9 Go packages green (`go test -count=1 ./...`), 539/540
  Vitest (`web` unit + component), `tsc --noEmit` clean, ESLint 0
  warnings.
- End-to-end against a fresh Keycloak re-import: admin token →
  `GET /api/v1/stats` 200; non-admin token → 403; anonymous → 401;
  issued access tokens carry `aud=ai-registry-server` and
  `realm_access.roles`.

## v0.3.0

Browse-polish release. Three of the four v0.3.0 tasks from `PLAN.md`
land here (Task 2's card redesign was delivered ahead of schedule in
v0.2.x and only needed an icon-tile polish this cycle) plus the
bootstrap + audit-log work needed to make the new activity feed
interesting on a fresh stack. Zero breaking changes.

### ✨ MCP tools become a first-class field (Task 1)

MCP clients negotiate `capabilities.tools` as a boolean feature flag
(`{listChanged: bool}`), NOT a tool list — the actual list is only
returned at runtime via `tools/list`. The registry was previously
reading the capabilities flag as if it were a list, which silently
under-counted servers that advertised tools. v0.3.0 introduces a typed
`tools[]` field on `mcp_server_versions` so the registry can display
tool counts and metadata offline, and ends the semantic collision
with the spec's capabilities flag.

- Migration `000007_mcp_tools` adds `tools JSONB NOT NULL DEFAULT '[]'`
  to `mcp_server_versions`. Additive — no backfill needed.
- `domain.MCPTool` struct + `domain.ValidateTools` (non-empty name,
  unique within array, optional `description` / `input_schema` /
  `annotations`). Empty array is valid.
- Store, handler, and OpenAPI all carry the new field end-to-end.
  `POST /api/v1/mcp/servers/{ns}/{slug}/versions` accepts `tools` and
  validates via `ValidateTools`. The `/v0/` spec-shaped endpoints are
  unchanged.
- Bootstrap: `MCPVersionSpec.Tools` YAML field, with realistic tools
  populated for 7 versions across 6 servers (filesystem, computer-use,
  github, web-search, postgres, kubernetes) so local dev has data.
- New **Tools tab** on the MCP server detail page: one card per tool
  (name + description + annotation badges + collapsible `input_schema`
  viewer), with an empty state referencing the spec's runtime
  `tools/list` path. Tab label shows count (`Tools (3)`) when
  populated.
- MCP card chip rewired to `lv.tools.length`, hides when absent or
  empty. Regression test: `capabilities.tools: {listChanged: true}`
  alone does NOT render the chip.
- Admin new-server form: JSON textarea for declaring tools when
  creating the first version. Client + server both re-validate.

### 🗂 Namespace landing pages (Task 3)

Every publisher now has a scoped landing page for each catalogue half:
`/mcp/{namespace}` and `/agents/{namespace}`. Until now the only way
to see "everything by this publisher" was the flat list filtered via a
query string — now it's a first-class route that can be linked to,
bookmarked, and crawled.

- New pages fetch the publisher header (`GET /api/v1/publishers/{slug}`)
  and the filtered list (`GET /api/v1/mcp/servers?namespace=X` /
  `GET /api/v1/agents?namespace=X`) in parallel; three distinct states
  (loading skeleton, 404 when the publisher doesn't exist, empty-state
  when the publisher exists with zero entries of that kind).
- Namespace chip on every card, detail-page breadcrumbs, and the
  publisher-row link now point at the path-param URLs instead of
  `?namespace=X` query strings. Filter behaviour on the flat lists is
  preserved — existing e2e pagination tests pass unchanged.
- 10 new Vitest cases covering render / loading / empty / 404 /
  links-out across both namespace pages. Playwright `coverage-public`
  gains 5 new smoke tests: seeded entries appear, private-MCP is
  hidden, detail-page link works, unknown-namespace 404 renders, chip
  navigation from the flat list lands on the new route.

### 📜 Per-entry activity feed + admin audit page (Task 4)

Every MCP server and agent detail page now shows a privacy-scrubbed
lifecycle log: creations, publishes, visibility changes,
deprecations. The new admin `/audit` page is the full-fidelity view
with actor-identity columns and filters, so operators can drill from
the global log into a single entry's history and back. Both surfaces
share one backing endpoint per resource kind.

- **Public endpoints** `GET /api/v1/mcp/servers/{ns}/{slug}/activity`
  and the agents equivalent. Project from `audit_log` filtered by
  `(resource_type, resource_id)`, apply a privacy scrub (drop
  `actor_subject` / `actor_email`; metadata key allowlist: `from`,
  `to`, `visibility`, `reason`, `version`, `field`), and drop draft
  `*version.created` events so the public feed only shows
  lifecycle-relevant actions. Cursor pagination on
  `(created_at, id) DESC`. Rate-limited through the same per-IP bucket
  as the other public reads.
- **Admin `/audit` page**: filterable full-fidelity view of the audit
  log with actor identity (subject + email + role) and per-row
  drill-down links to the affected resource. Filter by resource type
  to narrow the feed; cursor paginates the same way.
- **Bootstrap** now emits synthetic audit events
  (`actor_subject = system:bootstrap`,
  `actor_email = bootstrap@ai-registry.local`,
  `metadata.source = "bootstrap"`) for publisher / server / version /
  agent / agent-version first-time inserts so a freshly-brought-up
  stack has realistic activity to render. Re-running the bootstrap is
  idempotent — it does not double-emit.
- **Layout**: the publisher README now renders at full container width
  directly under the short description (above the tabs) on MCP + agent
  detail pages, so the narrative content is always visible regardless
  of which tab the reader has open. Old `ActivityStrip` component
  renamed to `EngagementStrip` to free the "Activity" name for the
  lifecycle feed.
- **Tests**: new Playwright `activity` project exercises admin +
  public surfaces end-to-end, including a wire-level assertion that
  the public endpoint never leaks `actor_subject` / `actor_email` /
  `client_ip` / `user_agent` / `internal_note`. Vitest gains the
  `ActivityFeed` component suite (loading / empty / populated /
  load-more / privacy scrub / per-resource endpoint selection) and the
  `admin/audit` page suite. Bootstrap test covers audit emission shape
  + idempotency.

### 💅 UX polish

- **Card icon tile** — a small rounded identity anchor (`Boxes` for
  MCP servers, `Bot` for agents) renders before the name on both
  catalogue cards. Long names truncate with ellipsis instead of
  pushing the right-side badge cluster off-card. The rest of each
  card — version/status cluster, runtime/ecosystem chips, tools row,
  description, transport block, footer — is byte-for-byte unchanged.
- **Pointer cursors** on the Button, Tabs, and Select primitives so
  every interactive surface in the UI gets the hand cursor on hover.
  Previously only a handful of ad-hoc components set it.

### ⚠️ Upgrade notes

No breaking API changes. The `tools` field is additive. Namespace
URLs become first-class — existing bookmarks pointing at
`?namespace=X` query strings continue to work on the flat list pages.
The `audit_log` table is unchanged; bootstrap's synthetic events
reuse the existing shape with a sentinel `source = "bootstrap"`
metadata marker so they can be filtered out by operators who don't
want them in analytics.

**Full changelog:** `v0.2.2...v0.3.0`

## v0.2.2

Coverage-depth release. Zero user-visible feature changes — this patch
closes the test pyramid gaps called out in `PLAN.md` § v0.2.2, plus one
bundle-size win for first-time public visitors and the Node-20 → Node-24
Actions migration ahead of GitHub's June 2026 force-cut. Every
non-negotiable rule in `CLAUDE.md` (API-first, spec compatibility, OTel
instrumentation, admin-only writes) now has a mechanical contract test
enforcing it in CI.

### 🧪 Protocol & spec conformance (server)

- **`/v0/` MCP wire-format conformance suite** — 40 tests pinning the
  response shape to the MCP registry spec (top-level `servers` key,
  `metadata.count`/`nextCursor`, single-object detail, `_meta`, package
  `registryType`/`identifier`/`version`/`transport.type`, error envelope
  shape, RFC 3339 timestamps). No more `t.Skip` gaps — the old dead
  package-shape skip now fails loudly on an empty seeder.
- **A2A Agent Card JSON Schema conformance** — `server/api/a2a-agent-card.schema.json`
  pins the a2a-project/a2a **June 2025** shape (CLAUDE.md decision G) as
  a machine-checkable schema, embedded alongside `openapi.yaml` via
  `go:embed`. New handler tests validate every per-agent and global card
  emission, catching regressions like `defaultInputModes` going nil or
  a `securitySchemes` type outside the decision-K allow-list.
  Misconfiguration path is also covered: unset `PUBLIC_BASE_URL` must
  return `application/problem+json` 500, never silently advertise
  localhost.
- **`openapi.yaml` ↔ router bijection** — `router_contract_test.go`
  walks every chi route and every documented path/operation and fails
  if either side drifts. The allow-list is one line (`/config.json`)
  with a comment explaining why it's spec-exempt.
- **Admin-guard router contract** — `router_admin_guard_test.go`
  enumerates every `POST`/`PUT`/`PATCH`/`DELETE` route via `chi.Walk`,
  subtracts the public-writes allow-list (`view`, `copy`, `reports`),
  and asserts each remaining route returns 401 without a token. A
  sibling test identity-compares middleware chains to catch the other
  direction (an accidental `RequireAdmin` on a public telemetry route).
  This is the mechanical enforcement of CLAUDE.md's non-negotiable
  rule #3: *"All writes go through admins."*
- **OTel span emission contract** — `router_otel_test.go` installs a
  tracetest `SpanRecorder` as the global provider, fires DB-free public
  routes through the fully-wrapped (`otelhttp.NewHandler`) production
  router, and asserts every request produces a span carrying both
  method and status-code semantic-convention attributes. If anyone ever
  replaces `otelhttp.NewHandler` with a bare mux in `NewRouter`, the
  test fails immediately — the exact bug CLAUDE.md warns about.
- **Migration forward-apply + idempotency** — a fresh testcontainers
  Postgres, `Migrate()` twice, assert every core table and a sample of
  per-migration columns (`featured`, `tags`, `verified`, `readme`,
  `view_count`, `copy_count`) exist.
- **Public rate-limit wiring test** — proves `RouterDeps.PublicRateLimitRPM`
  actually reaches the per-IP bucket (3 requests at limit=2 → third
  gets 429) and that `0` maps to the documented 1000-rpm default, not
  to "reject everything".

To make the contract tests possible, `NewRouter` was split into
`buildMux()` + `NewRouterForTest()` so `chi.Walk` can descend into the
raw `*chi.Mux` without the `otelhttp` wrapper in the way. Production
`NewRouter` still returns the fully-wrapped handler.

### 🧪 Coverage depth (web)

- **Interactive admin-detail coverage** — `admin/mcp/detail.tsx` and
  `admin/agents/detail.tsx` gained 25 tests between them covering the
  LifecycleStepper Deprecated transition, DeprecateButton confirm
  accept/decline, edit-form cancel, delete confirm (with navigate
  assertion) and decline, visibility-mutation failure surfacing, the
  published-only deprecate guard, and the A2A `/.well-known/agent-card.json`
  link href (CLAUDE.md decision H: a cached URL regression silently
  breaks every A2A client).
- **OIDC token lifecycle in `AuthContext`** — 4 new tests capture the
  `addUserLoaded` / `addUserUnloaded` / initial-hydration / unmount
  cleanup paths on the `UserManager.events` subscription. The silent
  cleanup bug (fresh arrow-fn on unmount becomes a no-op) is now
  gated.
- **Radix Select jsdom shims** centralised in `src/test/setup.ts` —
  `hasPointerCapture`, `releasePointerCapture`, `scrollIntoView`.
  Individual test files stop re-declaring them in `beforeEach`.
- **Admin-page coverage floor is verifiable** — the stale
  `"src/pages/**"` exclusion in `vitest.config.ts` hid admin pages
  from the coverage report entirely. Narrowed to public user pages
  only; every admin page now reports ≥86% statements (lowest:
  `mcp/detail.tsx` at 86.4%; highest: 100%), comfortably above the
  v0.2.2 DoD floor of 80%.

Vitest is now **64 files / 490 passing / 1 skipped** (the skipped test
is the `admin/api-keys.tsx` interactive flow, blocked on Phase 5 API-key
endpoints per `PLAN.md`).

### 🔧 CI gates

- **Named conformance suite step** in `ci.yml` re-runs the `/v0/`, A2A,
  OpenAPI-contract, and admin-guard tests with `-v` so their names
  appear in the CI log. A silent deletion or rename now surfaces as a
  CI failure instead of quietly reducing coverage.
- **Go coverage floor at 70%** — `go tool cover -func` against the
  aggregated profile, floor-checked in CI. Current number: 72.2%. The
  floor catches regressions from silent test deletions without gating
  normal development on a moving target.
- **Node 24 readiness** — all third-party actions across `ci.yml`,
  `e2e.yml`, and `publish.yml` bumped to their Node-24 majors
  (`checkout@v5`, `setup-node@v5`, `setup-go@v6`, `upload-artifact@v5`,
  `setup-helm@v5`, `setup-buildx-action@v4`). The Docker action suite
  and `upload-artifact@v5` still bundle Node 20; `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24`
  is set in `publish.yml` + `e2e.yml` as the documented interim escape
  hatch ahead of the June 2, 2026 force-cut.
- **Playwright HTML report upload fix** — CI reporter was `github`
  only, so `upload-artifact` in `e2e.yml` had no `playwright-report/`
  to grab. Now emits both `github` annotations and an HTML report.

### 🚀 Performance

- **Lazy-loaded admin bundle.** All 13 admin pages are now
  `React.lazy()` with a single `Suspense` boundary inside `RequireAuth`.
  First-time public visitors no longer pay for the admin surface
  (forms, editors, bulk actions).
  **Main bundle: 729 KB → 207 KB (gzip: 215 KB → 55 KB).** The vite
  "chunk larger than 500 kB" warning is gone.
- **Long-lived vendor chunks.** `vite.config.ts` `manualChunks` splits
  `react`/`react-dom`/`react-router`, `@tanstack/react-query`,
  `oidc-client-ts`, and the `react-markdown` + `remark`/`rehype` chain
  into dedicated vendor chunks so app-code changes no longer bust
  their long-term browser caches.

### 🐛 Fixes

- **`any`-free web codebase.** The v0.2.1 unblock commit had temporarily
  dimmed `no-explicit-any` to `warn`. v0.2.2 reverts that downgrade
  and fixes every underlying site: hook call sites branch on path so
  openapi-fetch's literal-string typing survives the ternary; related
  / version views use the generated `MCPServer`/`Agent` schema types;
  test mocks are typed against the schema (which surfaced two fixture
  drifts — `status: 'active'` → `'published'`, `runtime: 'python'` →
  a valid transport enum value); `(globalThis as any)` → `vi.stubGlobal`.
- **React Fast Refresh compliance.** Split `cva` variants out of
  `button.tsx`/`badge.tsx` into dedicated `*-variants.ts` files so
  each component module only exports components —
  `react-refresh/only-export-components` clean.
- **Test-fixture drift.** Several MCP fixtures had bogus `runtime`
  values (`'node'`, `'python'`) hidden behind `as MCPServer` casts.
  The MCP `runtime` field is the **transport mechanism** (`stdio`,
  `http`, `sse`, `streamable_http`), not a language. Replaced with
  valid enum values and added comments pointing to
  `server/internal/domain/mcp.go`.
- **Dependabot bumps.** `vite ^6.2.5 → ^6.4.2`, `vitest` +
  `@vitest/coverage-v8 ^2.1.9 → ^3.2.4`, `esbuild ^0.25.0` override.
  Closes the two web advisories; the two Go advisories were test-only
  transitives of `testcontainers-go` and were dismissed as `not_used`.

### ⚠️ Upgrade notes

No schema changes. No breaking API changes. No config changes.
Operators do not need to touch anything to adopt v0.2.2.

**Full changelog:** `v0.2.1...v0.2.2`

## v0.2.1

Coverage backfill release. No user-visible feature changes — the focus is on
filling in test gaps left by the v0.2.0 sprint and tightening one piece of
operator config that showed up under load.

### 🧪 Tests added

- **Server (Go):** new handler tests for `view_count` / `copy_count` event
  recording on both MCP servers and agents, and parity tests for
  `PATCH /v0/servers/{ns}/{slug}/versions/{version}/status`. Store-level tests
  for the matching repository methods.
- **Web (Vitest):** ~18 new test files covering every admin page (`new` /
  `list` / `detail` for publishers, MCP servers, and agents), the admin
  dashboard, layout, and api-keys placeholder, plus shared components
  (server-card, agent-card, theme-toggle, delete-button, deprecate-button,
  raw-json-viewer, install-command, activity-strip, related-entries,
  section-header). Vitest run is now 64 files / 473 passing / 1 skipped
  (Phase 5 api-keys flow).
- **Web (Playwright):** new `coverage-admin.spec.ts` and `coverage-public.spec.ts`
  suites — bulk actions, publish-via-UI through the new-form flow, and a
  22-server pagination walkthrough on the public MCP list. Full Playwright
  suite is now 50 tests across 7 projects, all green.

### 🔧 Server

- **Configurable public rate limit.** The per-IP budget for unauthenticated
  reads on `/api/v1` is now driven by `PUBLIC_RATE_LIMIT_RPM` (env) /
  `http.public_rate_limit_rpm` (YAML), defaulting to **1000 req/min** (was a
  hard-coded 100). Documented in `deploy/.env.example`. The previous limit
  was easy to trip from a browser SPA or the e2e suite under normal use.

### 🐛 Fixes

- Playwright `testMatch` regexes were unanchored and silently pulled
  `coverage-admin.spec.ts` into the `admin` project (and similarly for
  `public`), causing duplicate runs and project-config mismatches. Now
  anchored with `(^|\/)admin\.spec\.ts$`.
- A handful of public-page locators were ambiguous (`getByText(slug)` matched
  both the Name and the Namespace/Slug cell; `getByLabel('Search')` matched
  checkbox aria-labels). Switched to role-based locators with `exact: true`.

### ⚠️ Upgrade notes

No schema changes. No breaking API changes. Operators running behind the
default rate limit will see the public budget rise from 100 to 1000 req/min
per IP — pin `PUBLIC_RATE_LIMIT_RPM=100` if you want the old behaviour.

**Full changelog:** `v0.2.0...v0.2.1`

## v0.2.0

Major UX overhaul of the public browse experience, plus new admin workflow tooling and a richer server API.

### ✨ Highlights

- **Redesigned detail pages** for MCP servers and agents — new Connection card surfaces endpoint URL, transport, protocol version and authentication at a glance, with tabs for Overview / Installation / Versions / JSON (MCP) and Overview / Skills / Connect / Versions / JSON (agents).
- **Version history** with inline diffs between published versions.
- **MCP client config generator** — copy-paste configs for Claude Desktop, Cursor, Windsurf, and other MCP hosts.
- **Agent client snippet generator** — multi-language connection snippets with per-scheme auth guidance.
- **README rendering** on every detail page.
- **Report an entry** dialog for takedown / correction requests.

### 📄 New pages

- **`/explore`** — cross-entity search and discovery.
- **`/publishers/:slug`** — publisher profile pages.
- **`/getting-started`** — MCP + A2A onboarding walkthrough.
- **`/changelog`** — public feed of recently published / updated entries.
- **Homepage rewrite** with a protocol explainer and featured entries.

### 🛠 Admin workflow

- **Bulk actions** — multi-select publish / unpublish / feature / delete on admin lists.
- **Lifecycle stepper** — visual draft → published → deprecated state machine.
- **Reports triage queue** for user-submitted reports.
- **`PATCH` / `DELETE`** endpoints (and delete buttons) for MCP servers, agents and publishers.

### 🔌 API

- **Reports API** — full CRUD for user-submitted reports.
- **Public changelog API** — feed of recent changes.
- **View / copy event tracking** exposed as `view_count` / `copy_count` on every entry.
- **New filters** on listing endpoints: `featured`, `verified`, `tags`, `transport`.
- **New fields** on entries: `featured`, `verified`, `tags[]`, `readme`, engagement counts.

### 🐛 Fixes

- Admin UI no longer breaks when a session expires mid-navigation.
- Several e2e test flakes fixed and CI pipelines stabilized.
- Dev deployment (docker-compose) regressions fixed.

### ⚠️ Upgrade notes

Five new database migrations (`000002` → `000006`) must be applied before rolling out the new server binary. No breaking API changes — all new fields are additive.

**Full changelog:** `v0.1.4...v0.2.0`
