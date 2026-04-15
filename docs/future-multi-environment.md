# Design note — Multi-environment entries

**Status:** parked, not scheduled. No implementation work on this until we
come back to it deliberately.
**Origin:** discussion during v0.3.0 planning, derived from the AI Forge
card layout showing `dev, staging, prod` tags next to an agent version.

## The idea

Today a registry entry has one connection block: one URL, one transport, one
auth scheme, one version. In reality a published MCP server or agent is
often deployed to several environments simultaneously — `prod` on Keycloak
+ OAuth2, `staging` on a static API key, `dev` on localhost — and consumers
need to know which one they're wiring into.

We want to represent this without pretending the environments are
interchangeable: they generally are **not**. Different envs typically have
different URLs, different auth, and sometimes different versions pinned.

## Design options considered

### Option A — Environments as connection profiles on the entry
The entry stays as one canonical record. A new `environments[]` field
carries a list of full connection blocks, each with its own URL,
transport, auth scheme, and version pin:

```yaml
environments:
  - name: prod
    url: https://hr-assistant.example.com
    transport: streamable_http
    auth: { scheme: OAuth2, ... }
    version: v2.0.1
  - name: staging
    url: https://hr-assistant.staging.example.com
    transport: streamable_http
    auth: { scheme: ApiKey, ... }
    version: v2.1.0-rc2
```

Shared across envs: name, description, publisher, tools/skills.
Per-env: everything you need to actually connect.
Client-config generator gains an env dropdown.

- **Pro:** one canonical entry, one detail page. Consumers pick an env
  and get the right snippet. Smallest UX shift from today.
- **Con:** if tools or skills differ between envs (staging rolls out an
  experimental tool first), we either pin tools to versions or duplicate
  them per env.

### Option B — Environments as a sibling sub-resource
New `/v0/mcp/servers/{ns}/{slug}/environments` resource. The entry
describes the *product*; environments are the *deployments*. Each env
is its own record with its own lifecycle.

- **Pro:** cleanest separation. Matches reality — the product vs. where
  it runs are genuinely different concepts.
- **Con:** more API surface, more admin UI, more cognitive load for
  casual browsers.

### Option C — Reuse versions for environments
Treat environment as a tag on a version: v2.0.1 is "in prod", v2.1.0-rc2
is "in staging". Zero new schema.

- **Pro:** no migration.
- **Con:** conflates semver version with deployment state. A single
  version can be in several envs simultaneously (v2.0.1 in both prod and
  staging during a rollout). Probably wrong.

**Current lean:** Option A. It keeps the canonical-entry UX we already
have and covers the 80% case. Option B is the right answer the day we
also want per-env observability, per-env access control, or per-env SLOs.
Option C is a trap.

## Open questions (must be answered before we implement)

1. **Are tools/skills environment-specific?** Can staging expose tools
   that prod doesn't? If yes, tools/skills must live on the version, not
   the entry, and each env pins a version.

2. **Is auth scheme always per-env?** Assumption: yes. Keycloak realms,
   API keys, and OAuth client IDs are all deployment-scoped. Confirm
   before finalising the schema.

3. **Can a non-`prod` env be the default shown on the card?** i.e. can
   an entry be staging-only while it's being onboarded, with no prod env
   yet? This affects the "which env do we advertise first" rule for
   list cards.

4. **Do consumers need to filter/search by environment?** "Show me
   everything that has a prod deployment." Affects whether env name
   goes into the entry's indexed fields.

## Things this interacts with (don't forget)

- **OpenAPI spec** — new nested schema for the connection profile; must
  stay MCP-spec-compatible on the wire-format side (`/v0/` endpoints
  may need to project a "default" env for spec consumers that don't
  understand env selection).
- **A2A Agent Card** — an agent card describes a connection. Multi-env
  agents may need either one card per env (`/.well-known/agent-card.json`
  per env) or a single card with alternative endpoints. A2A spec
  reference required before deciding.
- **Client config generator** — gets an env selector. Default to prod
  when present, else first env.
- **Version lifecycle** — if envs pin versions, deprecating a version
  must not silently leave an env pointing at a dead version.
- **Admin UI** — env management form on the edit page; bulk edit less
  obvious.

## When to revisit

When we have a real user asking for it, or when we start building the
API-gateway side of the platform (which is also where the runtime-call
metric belongs — see the parked usage-metric discussion). Until then
this note is the full state of the idea.
