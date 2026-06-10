# Admin UI/UX Review — Action Report

**Date**: 2026-06-10
**Scope**: the authenticated admin SPA (`web/src/pages/admin/`, shared components in
`web/src/components/admin/` and `web/src/components/ui/`, auth plumbing in `web/src/auth/`).
**Sources**: a live walkthrough of the running app (desktop 1440px / ~1040px / mobile 375px,
dark + light themes), a full code read, a heuristic critique pass, and live end-to-end CRUD
journeys executed with all three personas (Editor via OIDC, Reviewer via OIDC, Server Admin
via local login) — see §3a. Findings marked **[verified]** were reproduced against the
running app or the code during this review; treat the rest as accurate pointers that should
be re-confirmed in passing.

**Priority note from the owner**: desktop/web is the priority; mobile findings are still
valid but less urgent — do not lead with them.

This document is self-contained: an agent with no prior context should be able to pick any
finding and act on it.

> **⚠️ Three server-side bugs were found during journey testing — not UI issues, and the
> most severe findings here.** They live in §3a because that's where they surfaced, but
> read them first:
> - **J0 (P0)** — editors can mutate published entries (metadata + visibility) with **no
>   review**, bypassing the approval queue the product is built around.
> - **J0b (P1)** — force-deleting an entry with a pending deletion-request leaves a
>   **permanently stuck review item** that inflates the reviewer badge forever.
> - **J1 step 2 (P1)** — the create form **silently swallows a 403** on publish.
>
> Background task chips were filed for J0 (incl. visibility) and J0b. Everything about the
> UI's pending-change / 409 surfaces is unverifiable until J0 is fixed.

---

## 1. How to run and verify

```bash
docker compose --profile dev up -d --build   # web on :3000, API on :8081
```

- Sign in at `http://localhost:3000/login` with the dev bootstrap admin:
  `admin@example.com` / `devadmin12345` (dev-only credentials, defined in
  `docker-compose.yml`).
- Other seeded users exist (`author@example.com`, `reviewer@example.com`) for
  role-specific testing; check `deploy/bootstrap.example.yaml` / Keycloak realm for
  credentials.
- Playwright e2e tests for admin flows live under `web/` — any behavior fix below needs
  coverage there per project rules (see §6).

---

## 2. What is working — do not regress

These patterns are deliberate and good. Fixes below should reuse them, not replace them.

- **State discipline**: filter-aware empty states ("No servers match your filters." vs
  "No MCP servers yet." + CTA), table skeletons shaped like their content, `ErrorState`
  wired to RFC 7807 `problem+json` details.
- **Workflow transparency**: pending-change banner names the submitter and offers
  Withdraw; rejection reasons render inline; toasts adapt to role ("Submitted for review"
  vs "Now public").
- **Honest bulk failure**: partial failures report "X of Y succeeded" and keep the
  selection for retry.
- **Publisher delete ceremony** (`web/src/components/admin/confirm-delete-panel.tsx`):
  type-the-name confirm + explicit cascade counts ("removes 2 MCP servers and 0
  agents…"). This is the gold-standard pattern the rest of the app should converge on.
- **Publisher scoping**: the sidebar switcher re-shapes the nav (Activity/Members/Settings
  appear), pre-fills create forms, and re-scopes the dashboard.
- **Chrome**: dark/light theming with a FOUC guard in `web/index.html`; mobile drawer with
  Esc/backdrop close and scroll-lock; aria-labels on icon-only buttons; `role="alert"` on
  errors; URL-backed debounced filters on entity lists.

---

## 3. P1 — broken or trust-breaking (fix first)

### P1.1 Lifecycle stepper is a dead control **[verified]**
On a draft entry, the stepper renders "Published" as an enabled transition target
(tooltip "Transition to Published"); clicking it fires no request and no UI change.

- `web/src/components/admin/lifecycle-stepper.tsx:30-43` —
  `defaultAllowedTransitions('draft')` returns `['published']`, so the step renders as a
  valid clickable target.
- `web/src/pages/admin/mcp/detail.tsx:151-152` — the `onTransition` handler only handles
  `target === 'deprecated'`. Every other click is silently dropped.
- Audit `web/src/pages/admin/agents/detail.tsx` for the same pattern.

**Fix**: pass `allowedTransitions` that reflects what the handler actually supports, or
route a "published" click to the Versions section's Publish action (publishing happens
per-version, so scrolling/focusing that section with an explanatory hint is acceptable).
A primary-looking control that does nothing erodes trust in every other control.

**Journey extension [verified live]**: once an entry is deprecated, there is **no
un-deprecate path in the UI at all** — the "Deprecate" button disappears, the Actions row
offers nothing, and the stepper's now-valid "Published" target is the same dead click.
The domain allows deprecated→published (`defaultAllowedTransitions('deprecated')` returns
`['published']`), but the UI accidentally makes `DeprecateButton`'s "cannot be undone"
copy true. Fix the stepper (or add a "Republish" action) and the copy together.

### P1.2 Slug validation is silently dead in four create forms **[verified]**
`pattern="^[a-z0-9-]+"` fails to compile under the `v` regex flag Chromium uses for the
HTML `pattern` attribute (`Invalid character class` — reproduced with
`new RegExp('^(?:^[a-z0-9-]+)$', 'v')`). When the pattern fails to compile the browser
skips validation entirely, so `"Bad Slug!!"` passes client-side and only fails on the
server roundtrip.

Affected (all **[verified]** present):
- `web/src/pages/admin/mcp/new.tsx:250`
- `web/src/pages/admin/agents/new.tsx:243`
- `web/src/pages/admin/publishers/new.tsx:89`
- `web/src/pages/admin/groups/new.tsx:84`

The version patterns (`^\d+\.\d+\.\d+.*` in the same forms and
`web/src/components/admin/new-version-form.tsx:257`) compile fine — only the slug
pattern is broken.

**End-to-end failure confirmed live**: typing `Bad Slug!!` passes `checkValidity()`
(true, `patternMismatch: false`), the form submits, the server correctly rejects with
422 — and the UI then shows only **"Failed to create server — Unprocessable Entity"**.
The server's problem+json `detail` is excellent
(`slug: "Bad Slug!!" is not a valid slug (use lowercase letters, digits, and hyphens;
max 63 chars)`; duplicates get `MCP server 'genai-test/echo' already exists`) but the
form throws it away: `web/src/pages/admin/mcp/new.tsx:94` reads only `error.title`
(`new.tsx:149` same for the version step), even though the shared `problemMessage()`
util (`web/src/lib/utils.ts:17-20`) already extracts `detail ?? title ?? fallback`.

**Fix**: use `pattern="[a-z0-9\-]+"` (HTML pattern is already implicitly anchored; the
`^` anchors are redundant anyway); route all create-form errors through
`problemMessage()` so the server's detail reaches the user; add onBlur inline
validation with `aria-invalid` + a visible message — currently no form in the admin has
inline validation; errors surface only after submit. Audit `agents/new.tsx` for the
same title-only extraction.

### P1.3 Signed-out and under-privileged deep links die silently **[verified]**
- `web/src/auth/RequireAuth.tsx:22` — any `/admin/...` URL visited signed-out redirects
  to `/` (public homepage) with no message. The user isn't told they need to sign in.
- `web/src/pages/login.tsx:18,28` — login always lands on `/admin`, so the original
  destination is lost even after signing in.
- Same silent-bounce pattern in `RequireServerAdmin` for non-admins deep-linking
  server-admin pages (e.g. `/admin/users`).

**Fix**: redirect unauthenticated users to `/login` with a `returnTo` (router `state` or
query param), honor it after both local and OIDC login, and show a "sign in to continue"
hint. For authorization bounces, land with a toast/notice ("Server Admin access
required") instead of a silent redirect.

### P1.4 One-click, unconfirmed, high-blast-radius actions **[verified]**
- `web/src/pages/admin/users/detail.tsx:111-119` — "Grant/Revoke Server Admin" and
  "Disable/Enable account" fire the mutation immediately on click. No confirm of any
  kind. (Check whether an admin can disable or demote *themselves* — lockout risk.)
- `web/src/components/admin/grants-section.tsx:181-191` — revoking a role grant is a
  single X-click, no confirm, no undo. This is the RBAC surface.

Contrast: bulk-deprecating gets a `window.confirm` and deleting a publisher requires
typing its name. Ceremony is currently proportional to *entity type*, not *blast
radius*.

**Fix**: confirmation (or toast-with-undo for grant revoke) on all four actions, reusing
whatever shared confirm primitive comes out of P2.1.

---

## 3a. User journeys — live CRUD walkthrough **[all verified live]**

The full MCP-entry lifecycle was executed end-to-end on the dev stack:
Editor (`genai-author@example.com`, OIDC) created `genai-test/ux-journey` with a v1.0.0
→ submitted for review → Reviewer (`genai-reviewer@example.com`, OIDC) approved →
Server Admin deprecated and force-deleted it. The agent lifecycle (J3c) and the
unhappy paths (validation, reject/resubmit, deletion-request, bulk) were also run live
with the relevant personas. Findings are ordered server-bugs-first (J0, J0b), then the
editor/reviewer journeys (J1–J5).

### J0. ⚠️ SERVER BUG, P0: Editor metadata edits bypass the review queue entirely

> **RESOLVED 2026-06-10 — stale test environment, not a code bug.** The walkthrough ran
> against a server image built 2026-06-09 17:28, *before* commit `c338b38` (the
> review-queue routing, merged 2026-06-10 08:17) — the `change-request` routes didn't
> even exist in the running container (404). After rebuilding (`docker compose --profile
> dev up -d --build server`), all three editor mutations were re-verified live as
> `genai-author@example.com`: PATCH metadata → **202 pending_review** (nothing applied),
> visibility → **202**, deprecate → **202**, duplicate enqueue → **409
> change-already-pending**. Agent-endpoint handler tests (patch/visibility/deprecate
> enqueue, admin immediate path, approve applies) were added in
> `server/internal/http/handlers/entrychange_test.go` to close the coverage gap noted
> below. The UI's pending/409 surfaces are now reachable and testable.

Not a UI finding — a contract violation found while journey-testing, reported here
because the next agent must know about it before touching the related UI:

- As `genai-author@example.com` (role: **editor** on `genai-test`, nothing more),
  `PATCH /api/v1/mcp/servers/genai-test/echo` (a **published, public** entry) returned
  **200 OK and applied immediately** — description changed in place, nothing enqueued,
  no reviewer involved. Reproduced twice (draft and published entries) and confirmed in
  the network trace.
- **Visibility changes bypass too** [verified live]: as the same editor,
  `POST /api/v1/mcp/servers/genai-test/echo/visibility` returned **200 OK** and flipped
  a public seed entry private (and back) instantly — meaning an editor can also expose
  a private entry publicly with zero review. The contract
  (`setMCPServerVisibility`, `openapi.yaml:1111`) likewise reserves 200 for the
  Server Admin escape hatch. Deprecation-as-editor was *not* live-tested (no UI path
  back from deprecated — see P1.1) but almost certainly shares the same hole; cover it
  in server tests along with the agent endpoints (`patchAgent`, `setAgentVisibility`).
- The OpenAPI contract (`server/api/openapi.yaml:1035` `patchMCPServer`) says non-admin
  Editors get **202 + EntryChangeAccepted** (enqueued for review); 200 is documented as
  the Server Admin escape hatch. Commit `c338b38` ("route entry-level mutations through
  the review queue") describes the intended behavior.
- Consequence: any Editor can silently rewrite the public-facing name/description/URLs
  /license of a published entry with no review — the exact thing the queue exists to
  prevent. The UI's pending-change banner never shows because nothing is ever pending.
- Side effect on this review: the documented 409 ("change already pending") path could
  not be UI-tested, since changes never enqueue.

**Fix server-side first** (route Editor PATCHes through the change queue per the spec,
with an integration test asserting 202 for editors and 200 for admins), then re-verify
the UI's pending/409 surfaces, which exist in code but are currently unreachable.

### J0b. ⚠️ SERVER BUG, P1: force-delete leaves a permanently-stuck "zombie" review item

> **RESOLVED 2026-06-10.** Root cause: `DeleteMCPServer`/`DeleteAgent` set
> `status='deleted'` but never set `deleted_at` (the column the queue's
> deletion branch filters on) and never cancelled in-flight reviews. Fixed in
> `server/internal/store/{mcp,agent}.go`: force-delete now sets `deleted_at`,
> cancels pending version submissions, and drops pending entry-change requests
> in the same transaction; `ListReviewQueue` (`store/review.go`) additionally
> excludes `status='deleted'` from every union branch (hides pre-fix residue).
> Store integration tests added (`TestForceDelete*`,
> `TestListReviewQueue_ExcludesLegacyZombies`). Verified live: the stuck
> `genai-test/ux-agent` item no longer appears in the queue; the residue row
> noted below was normalized (`deleted_at` set) and kept as a tombstone.

Found while cleaning up test data; verified live + in the DB:

- An editor filed a deletion-request on agent `genai-test/ux-agent` (status pending in
  the review queue). A Server Admin then **force-deleted** the same agent.
- Admin force-delete is a **soft tombstone**: it sets `status='deleted'` but leaves
  `deletion_requested_at` populated (confirmed in Postgres: the `agents` row survives
  with `status=deleted`, `deletion_requested_at` non-null).
- The review-queue query keys off `deletion_requested_at IS NOT NULL` and **does not
  exclude `status='deleted'`**, so the deletion-request stays in the queue forever and
  keeps the nav badge count elevated.
- The item is **unresolvable from the UI**: its resource link goes to a "Not found"
  page; **both Approve and Reject fail** with `agent 'genai-test/ux-agent' does not
  exist` (the handlers treat a deleted entry as 404) and the item never leaves the
  queue. A reviewer can never clear it.

**Fix** (server, pick one or combine): on force-delete, clear `deletion_requested_at`
(and cancel any pending version-review rows); AND/OR exclude `status='deleted'` from the
review-queue query; AND/OR make approve/reject of a deletion-request **idempotent** —
treat an already-deleted target as success (204) so a stuck item can be cleared. Same
logic path exists for MCP servers (`deletion_requested_at` on `mcp_servers`) — fix both.
Add an integration test: file deletion-request → force-delete → assert the item is gone
from the queue (or resolvable).

**Residue note**: this left one throwaway row in the dev DB —
`agents` id `01KTRAGW9C1QHQMANBPCRG1FKT` (`genai-test/ux-agent`, status `deleted`) with a
stuck deletion-request review item. I did not delete it (raw SQL on the shared DB is out
of scope for a review); it's harmless but you'll want to clean it when fixing the bug —
it doubles as a live repro.

### J1. The editor's "publish" journey is a maze with misleading signage (P1)
What the editor experiences, step by step:

1. The create form shows **"Publish version immediately"**, checked by default
   (`web/src/pages/admin/mcp/new.tsx:407-413` — static label, `defaultChecked`, no
   role-aware copy).
2. Submitting the form fires `POST …/versions/1.0.0/publish`, which returns
   **403 Forbidden — and the UI swallows the error silently** (confirmed in the network
   trace). Root cause: `web/src/pages/admin/mcp/new.tsx:153-157` — the publish step's
   `authFetch` response is never checked (`await authFetch(...)` with no `.ok` test).
   The version lands as a plain `draft` with a "Submit" button; no toast, no hint. The
   user asked for X, the system failed at X, and said nothing.
3. After clicking Submit, the row shows "pending review" + submitter + Withdraw (good),
   and the section helper says approvals "happen on the review queue" with a link —
   but for an editor that link lands on **"Failed to load review queue."** (403 rendered
   as a generic error; the route isn't gated, the nav item is).
4. After the reviewer approves, the version is published — but the entry is **still
   `private`**. Making it actually visible requires "Make public", which for an editor
   is *another* review round-trip.

Nothing anywhere explains the real pipeline (draft → submit → approve → make public →
approve). **Fixes**: role-aware copy/behavior on the create-form checkbox ("Submit for
review on create"); hide or reword the review-queue link for non-reviewers; a pipeline
hint (or the lifecycle stepper, repurposed honestly) on the entry detail page; consider
bundling "make public" into the first approval or asking the editor up front.

### J2. Reviewers approve blind, with two competing affordances (P1/P2)
- The queue card identifies the item only as `ns/slug` + version — no display name and
  **none of the submitted content**: no description, tools, package, or capabilities.
  The only way to inspect what's being approved is to open the entry and expand the
  **"Raw API response"** JSON viewer. For the action that gates what ships to a public
  registry, the reviewer is effectively approving blind.
- Approve is **one click, no confirmation**, and applies/publishes immediately — and the
  same single unconfirmed click approves **deletion requests**, the most destructive
  action in the workflow (verified live: one click hard-deleted an entry). Reject, by
  contrast, has a proper inline required-reason form (good — keep it).
- The reviewer's entry detail page shows a prominent **"Publish"** button on the pending
  version row — a second approve-ish affordance with different wording than the queue's
  "Approve". Pick one verb and one surface (or make detail-page Publish open the same
  review action explicitly).

**Fixes**: render the version's content (description, tools list, package) on the queue
card or an expandable panel; add a lightweight confirm on Approve naming the entry and
version; unify Approve/Publish vocabulary.

### J3. OIDC sign-in lands on the public homepage (P2)
After completing Keycloak login, the user lands on `/` (public homepage) and must spot
the small "Admin" link in the header to reach the console. Combined with P1.3 (no
`returnTo`), every OIDC session starts with a detour. Since auth is expected to be
predominantly OIDC, the post-login landing should be `/admin` (or the preserved deep
link).

### J3b. Report dialog renders in the top-left corner, not centered (P2) **[verified live]**
The "Report an issue" dialog (`web/src/components/shared/report-dialog.tsx`, a native
`<dialog>` opened with `showModal()`) renders pinned to the **top-left corner** instead
of centered. Root cause confirmed via computed styles: the dialog is `position: fixed`
with `inset: 0` but **`margin: 0`** — Tailwind's preflight resets `margin` on every
element, overriding the browser's default `margin: auto` that centers modal dialogs.
**Fix**: add `m-auto` (or `place-self-center`) to the `<dialog>` className. This is the
public-UI report entry point (not strictly admin), but it's the front half of the admin
**Reports** flow and uses the same design system. Audit for any other native `<dialog>`
that relies on UA centering.

### J3c. Agent CRUD journey — parity confirmed, one extra surface (mostly good) **[verified live]**
Created → submitted → withdrew an agent (`genai-test/ux-agent`) as the editor. The agent
form is genuinely richer than MCP (auth scheme, default input/output mode checkboxes,
skill id/name/description/tags, A2A protocol version) and all rendered/submitted fine;
the lifecycle, submit, and withdraw surfaces behave identically to MCP (same components),
so MCP findings J1/P1.1/etc. carry over verbatim. One agent-only surface: the detail page
shows an **"A2A Agent Card — View agent card"** link to the well-known path; for a draft,
private agent that path correctly returns 404 (verified). The same J0 bypass applies
(agent create/submit path); the same J0b zombie bug was in fact found via this agent.

### J4. State vocabulary divergence after lifecycle transitions (P3)
After deprecating an entry, the entry badge reads `deprecated` while the version row
still reads `published` — both true in the domain model, but unexplained side by side.
A one-line legend (entry status vs version state) or a muted "(entry deprecated)"
annotation on version rows would prevent the double-take. Also observed: "Make public"
remains offered on a deprecated entry — confirm that's intended.

### J5. Journey smoke notes (mostly working well)
- Editor scoping is correct end-to-end: no server-admin nav, no review queue, publisher
  pre-filled and locked to their grant, role chip on the dashboard.
- The submit → pending → approved trail on the version row (submitter, reviewer, dates)
  is a genuinely good audit surface.
- **The reject → resubmit loop works and reads well** (verified live): after a reviewer
  rejects with a reason, the author's version row shows a red `rejected` badge, the
  reviewer + date, the full reason text, and a "Resubmit" button. Keep this pattern.
- **The deletion-request flow has good feedback** (verified live): after requesting,
  the button is replaced by "Pending review — Submitted. A reviewer will approve or
  reject the request from the review queue." (Same caveat as J1 step 3: the editor
  can't actually open that review queue.)
- Bulk actions work (verified live): selecting rows raises the floating bar; bulk
  delete refreshes the list into the correct filter-aware empty state.
- Admin force-delete redirects to the list and the entry disappears cleanly.
- Federated users appear in the Users list only after first login (JIT provisioning) —
  expected, but worth a hint on the Users page since admins may look for a user who
  "exists" in the IdP but hasn't signed in yet.

### Additional verified-live coverage (this round)
- **Validation surfaces** (P1.2 updated): bad slug and duplicate slug both reach the
  server, which rejects with excellent problem+json detail — but the create form shows
  only the generic title ("Unprocessable Entity" / "Conflict"). Fix is in P1.2.
- **Agent CRUD** (J3c): create/submit/withdraw as editor — parity with MCP confirmed.
- **Visibility-as-editor** (J0): bypasses review, same as PATCH.
- **Withdraw** on a pending version: button renders and was clicked during the agent
  journey without error.
- **Report dialog** (J3b): centering bug.

### Journeys still NOT exercised live (assess from code or test later)
- Filing a community report end-to-end into `/admin/reports` and acting on it
  (mark reviewed / dismissed / reopen) — the dialog's centering bug (J3b) was found but
  no report was actually submitted (would leave residue against seed data). The
  `/admin/reports` action buttons are unverified.
- The 409 pending-change conflict surface — blocked by J0 (changes never enqueue), so it
  stays code-only until J0 is fixed.
- Token/session expiry and refresh behavior (the `authFetch` 401→refresh→retry path).

---

## 4. P2 — high-impact UX gaps

### P2.1 Three destructive-confirmation dialects coexist **[verified]**
- Type-to-confirm inline panel: publisher/entry delete (`confirm-delete-panel.tsx`) — excellent.
- Native `window.confirm`: bulk delete/deprecate (`web/src/pages/admin/mcp/list.tsx:112,122`,
  same in `agents/list.tsx`), version publish
  (`web/src/components/admin/versions-section.tsx:338` — "Publish v1.0.0? This takes it
  live immediately." in a browser dialog), deletion request
  (`request-deletion-button.tsx:71`), single-row delete/deprecate buttons
  (`delete-button.tsx:26`, `deprecate-button.tsx:10`).
- Nothing at all: grant revoke, Server Admin grant (see P1.4).

Additional weight inversion: `DeprecateButton` (reversible) is loud red while the
irreversible force-`DeleteButton` is visually quiet.

**Fix**: one shared confirm dialog component (themed, keyboard-accessible), with the
type-to-confirm variant reserved for cascade/irreversible deletes. Make publish — the
action public consumers actually feel — a designed moment with version + entry named in
the dialog, not a browser `confirm()`. Align button weight with reversibility.

### P2.2 List tables: the only navigation affordance clips off-screen **[verified]**
- Entry names are not links and rows are not clickable; the right-most "Manage" button is
  the only path into an entry (verified by DOM inspection).
- At a 1038px viewport the table (822px content) overflows its 751px container with
  `overflow-x: auto` and no visible affordance — "Manage" is simply invisible at common
  laptop/split-screen widths, and on mobile (375px) it needs an undiscoverable horizontal
  scroll.
- Date cells wrap to two lines ("Jun 9," / "2026") — no `whitespace-nowrap`.
- At 1440px the content max-width leaves roughly a third of the screen empty: the layout
  has a narrow "sweet spot" (~1100–1300px) and degrades on both sides.
- No column sorting anywhere even though `FilterBar` already supports it
  (`web/src/components/ui/filter-bar.tsx:43` `sortOptions`) — the admin lists just never
  pass options.

**Fix**: linkify the name (keep Manage if desired), `whitespace-nowrap` dates, rethink
the responsive column set in `web/src/pages/admin/mcp/list.tsx` and `agents/list.tsx`,
wire `sortOptions` (updated/name/status).

### P2.3 Read views hide part of the metadata admins come to check **[verified, corrected]**
The MCP detail read view renders description and license *when set*
(`web/src/pages/admin/mcp/detail.tsx:190,208-211`) — but **homepage URL and repository
URL never render outside the Edit form**, and unset fields are omitted entirely, so a
reader can't distinguish "not set" from "not shown". Same component structure for
agents. Show all editable fields in the read view, with an explicit "—" for unset ones.

### P2.4 The *global-only* reviewer persona has no home **[scoped down after live test]**
Live testing showed a **publisher-scoped** reviewer already gets a good home: "1
awaiting your review" callout, badged Review queue nav item, Reviewer role chip on the
dashboard. The problem is specifically a reviewer with only a **global** grant and no
publisher membership (e.g. the `registry-reviewers` group): they land on "No publishers
yet — ask a publisher admin for an Editor or Viewer grant"
(`web/src/pages/admin/dashboard.tsx:28-35`) — told they don't belong while having
work to do.

**Fix**: branch that empty-publisher dashboard on reviewer permission — show queue
count + recent decisions, or redirect straight to `/admin/review`.

### P2.5 No unsaved-changes protection anywhere **[verified]**
Every inline edit form and the long create forms discard state silently on Cancel or
navigation. Worst case: the new-version form with a hand-built tools list
(`new-version-form.tsx`, tools editor) — one stray sidebar click loses everything.

**Fix**: a shared dirty-form guard (router blocker + `beforeunload`) applied to create
forms and the version editor at minimum.

### P2.6 Every detail-page failure reads "Not found."
`web/src/pages/admin/mcp/detail.tsx:111` (and siblings) render "Not found." for *any*
query error — including 500s and network failures. Branch on status: 404 → not-found,
everything else → `ErrorState` with retry.

---

## 5. P3 — consistency, accessibility, polish

### Component vocabulary
- Three form-control dialects coexist: Radix `Select` (forms), native `<select>`
  (FilterBar, grants role picker), raw `<input type="checkbox">` (table selection, with
  no focus styling or indeterminate state). Standardize on one select and one checkbox
  primitive.
- Three error-display dialects coexist too: red alert box at the top of create forms,
  inline `<p role="alert">` under inline-edit forms, and `toast.error` — chosen per page
  rather than per situation. Define one rule (e.g. field errors inline, form errors in
  the form's alert region, background-mutation errors as toasts) and apply it everywhere.
- Review queue and audit log link resources with raw `<a href>` → full SPA reload
  mid-triage; everything else uses `<Link>`. Convert.
- Loading states: lists get skeletons, detail pages get bare "Loading…" text. Add detail
  skeletons.
- Date formats vary by page: entity lists "Jun 9, 2026", audit full timestamps, activity
  relative times. The mix is defensible per context, but pick it deliberately and keep
  table dates from wrapping (see P2.2).

### Accessibility (from in-page detector + manual pass)
- Contrast below WCAG AA on shared primitives, present on every page: muted badge variant
  4.3:1 (`#64748b` on `#f1f5f9`, needs 4.5:1), red count badge 3.6:1 (`#f8fafc` on
  `#ef4444`), destructive button text 3.8:1. Darken `muted-foreground` on tinted badge
  backgrounds and the destructive palette.
- No skip-to-content link: ~10+ tab stops through header/switcher/nav before content on
  every page.
- Two focus languages: sidebar links get UA-default outline; buttons get the design
  system's `focus-visible:ring-2`. Unify.
- Heading order skips h1→h3 on the dashboard.
- A shared layout component animates `transition: height` (layout property — jank +
  motion concerns); animate transform/opacity instead.
- Breadcrumbs: at least the users detail page renders a proper
  `<nav aria-label="Breadcrumb">` landmark **[verified]** — audit the other detail pages
  and align them with whichever component does it right
  (`web/src/components/ui/breadcrumbs.tsx`).
- Muted helper/description text runs 110–128 characters per line on detail and users
  pages — well past comfortable reading measure (~75ch). Cap with `max-w-prose` or
  similar.

### Grants editor (`web/src/components/admin/grants-section.tsx`)
- Add-grant row is visually broken: the "Group" label collapses inline/floats centered
  ("Principal[Group ▾]") — labels need block layout (`:108-151`).
- The `config` badge (`:178`) is unexplained — no tooltip; also unclear whether revoking
  a config-sourced grant sticks or is re-seeded at boot. Add a tooltip and, if revoke
  doesn't stick, disable the X with an explanation.
- Grant button's disabled state is opacity-only.

### Filters
- Audit log requires a manual "Apply" while entity lists auto-apply — unify on
  auto-apply.
- Audit's client-side action filter can show "0 events" while matches exist past the
  loaded cursor — filter server-side or say "no matches in loaded events".
- When scoped to a publisher, lists still show the free-text "Publisher…" filter —
  redundant and able to contradict the scope. Hide it when scoped.
- Audit collapsed rows print raw ULIDs (`SUBJECT: 01KT…`, `RESOURCE ID: …`) on every
  line; move them into the expanded detail. The actor filter demands a remembered
  Keycloak subject UUID (`audit.tsx:272`) — offer a user picker or email match.

### Users
- Heading shows the email twice when display name is empty (`users/detail.tsx`); no
  display-name edit after creation.
- No view anywhere of a user's grants/group memberships — auditing one person's access
  means walking every publisher page plus global grants. Add a per-user access section.

### Copy and affordance details
- "Request deletion (review)" is workflow jargon as primary button copy; "rev 1",
  "Namespace / Slug", and `config` are never explained; disabled buttons explain
  themselves only via hover `title` (invisible to keyboard/touch).
- Recognition over recall: the review queue identifies entries only by `ns/slug`, never
  display names; reports rows print raw ULIDs (`web/src/pages/admin/reports.tsx:148`);
  the audit actor filter demands a remembered UUID (see Filters above). Surface
  human-readable names alongside identifiers.
- Duplicate deprecate affordances on entry detail: the stepper's "Deprecated" step and
  the "Deprecate" action button do the same thing **[verified]**. After fixing P1.1,
  decide which surface owns transitions and demote the other.
- No deeper help anywhere: page-level one-liners are good, but there's no role matrix
  (what can Viewer/Editor/Reviewer/Admin actually do?) and no link from the grants UI to
  one. A short docs page linked from Members/Grants would remove most of the jargon
  burden above.
- Two delete paths sit side-by-side with equal weight on entry detail: governed "Request
  deletion (review)" and break-glass "Delete". De-emphasize the escape hatch and label it
  (admin-only, bypasses review).
- `DeprecateButton` copy says "cannot be undone" while the domain allows
  deprecated→published. Fix the copy.
- The force-Delete button's muted-red outline reads as *disabled* while enabled — and
  genuinely disabled stepper steps look similar **[verified]**.
- Solid-blue `public` badge outshines muted `private` — inverted emphasis for an admin
  scanning for accidental exposure.
- Bulk action bar floats over the last table row; add bottom padding.
- Reports drill-down links to `/admin/mcp?q=<ULID>` — text-search over an ID, likely an
  empty result (`reports.tsx:64-71`). Link to the entry detail instead.
- Dashboard "Quick Actions" duplicate the New buttons one click away — consider dropping.
- "API Keys" occupies a permanent nav slot for a Coming-soon placeholder. Honest, but
  consider demoting it (e.g. a "Planned" affix in the nav item) until Phase 5 ships.
- After "Submitted for review", nothing tells the author where to watch progress — link
  the toast/banner to the entry's pending-change state.
- No keyboard shortcuts or command palette (design.md promises one) — backlog item, not
  a quick fix.

---

## 6. Product decisions needed (ask the owner before building)

These are real gaps, but they need an API change and/or a scope decision first
(project rule: API-first — update `server/api/openapi.yaml`, regenerate types, then
implement; no UI-only features).

1. **`featured`, `verified`, `tags`, `readme` have no write path at all.** The public UI
   renders all four prominently (Featured rail, Verified badges, tags), but neither the
   admin UI nor the API can set them: `patchMCPServer` accepts only
   name/description/homepage/repo/license (`server/api/openapi.yaml:1035`), and
   `patchPublisher` only name/contact (`:761`). Publisher `verified` likewise has no
   setter. Today the only write path is the bootstrap file or direct DB. **[verified]**
2. **Publish ceremony.** If publish is the product's signature moment (it's what
   consumers feel), it deserves more than a confirm dialog — preview of what goes live,
   versioned diff, etc. Scope question.
3. **Reviewer-first review queue.** P2.4 fixes the landing; a deeper investment (filters,
   diffs, batch decisions) is a roadmap question.

---

## 7. Constraints for whoever implements this

From the repo's `CLAUDE.md` — non-negotiable here:

- **API-first**: any new capability goes in the versioned HTTP API + `openapi.yaml`
  first, then regenerate the TS types, then the UI.
- **Tests for everything**: unit tests for logic, Playwright e2e for changed admin
  flows. No change without coverage.
- **Conventional commits**, feature branches (`feat/`, `fix/`…), never push `main`.
- **Errors**: RFC 7807 end-to-end — keep using `problemMessage()` /
  `friendlyProblem()` extraction.
- Reads are public; writes are authorized server-side. UI gating is convenience, not
  security — don't move authorization client-side while refactoring.

### Suggested sequencing
0. **Server bugs first** (task chips filed): **J0** — editor metadata *and visibility*
   mutations bypass review (everything about pending changes/409 in the UI is
   unverifiable until this is fixed); **J0b** — force-delete leaves a stuck review item.
   Both span MCP servers + agents.
1. P1.1–P1.4 + J1 (editor publish journey, incl. the swallowed-403 at `new.tsx:153-157`)
   — small-to-medium, highest trust impact.
2. J2 reviewer queue content + Approve confirm; P2.1 shared confirm dialog (retires
   `window.confirm`, unblocks P1.4 and J2 polish).
3. P2.2 list/table fixes + P2.3 read views + P2.6 error branching + J3 OIDC landing.
4. P2.4 global-reviewer landing, P2.5 dirty-form guard.
5. P3 batches: a11y/contrast pass on shared primitives, grants editor, filters, copy, J4.
6. §6 items after owner sign-off.

Mobile-specific items (375px Manage-column overflow, drawer polish) are lower priority
per the owner — fold them into the P2.2 table work only where the same fix covers both;
don't schedule separate mobile passes first.
