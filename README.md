# AI Registry

A self-hostable registry for AI ecosystem artifacts — **MCP servers** and
**A2A agents** — with a versioned HTTP API, a public catalog UI and an admin
console.

[![Lint](https://github.com/Haibread/ai-registry/actions/workflows/lint.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/lint.yml) [![Quality](https://github.com/Haibread/ai-registry/actions/workflows/quality.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/quality.yml) [![Docker](https://github.com/Haibread/ai-registry/actions/workflows/docker.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/docker.yml)

## Description

Teams building with MCP servers and A2A agents end up with them scattered
across repositories, wikis and chat threads. The AI Registry is one catalog
where publishers declare them, reviewers approve them, and people and machines
discover them.

- **Versioned entries** — every publish creates an immutable version; entries
  move through draft, published and deprecated.
- **Spec-aware** — MCP metadata follows the
  [Model Context Protocol](https://modelcontextprotocol.io/) `server.json`
  shapes; every agent gets an [A2A](https://a2a-protocol.org/) Agent Card at
  `/agents/{namespace}/{slug}/.well-known/agent-card.json`, and the registry
  publishes its own at `/.well-known/agent-card.json`.
- **Review workflow** — Editors propose versions, deletions and entry changes;
  Reviewers approve them.
- **Publisher-scoped RBAC** — Viewer, Editor, Reviewer and Admin roles granted
  to users or IdP groups per publisher, enforced by the API.
- **Local or OIDC login** — email + password, or any OIDC provider brokered by
  the server; machine clients can present IdP service-account tokens.
- **API-first** — the two UIs are clients of the same `/api/v1`; the OpenAPI
  3.1 document is served at `/openapi.yaml`.
- **Observable** — OpenTelemetry traces, metrics and logs over OTLP.

It hosts metadata only: it does not run, proxy or sandbox the servers and
agents it lists.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the design and the reasoning behind
it.

## Getting started

### Prerequisites

- Docker with the Compose plugin (`docker compose`), for the local stack.
- `helm` ≥ 3.8 and a Kubernetes cluster, for a Kubernetes deployment.

### Installation

```bash
git clone git@github.com:Haibread/ai-registry.git
```

```bash
cd ai-registry
```

### Configuration

Every setting can be given as an environment variable, as a key in a YAML file
passed with `CONFIG_FILE` or `--config`, or left to its default — in that order
of precedence. Every key is documented in
[deploy/config.example.yaml](deploy/config.example.yaml) and
[deploy/.env.example](deploy/.env.example). The ones a real deployment has to
set:

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | — | PostgreSQL connection string |
| `PUBLIC_BASE_URL` | yes | — | External URL of the deployment, used in agent cards and login redirects |
| `JWT_SIGNING_KEY` or `JWT_SIGNING_SEED` | yes | ephemeral key | PEM Ed25519 key, or a ≥ 32-char secret the key is derived from; without either, tokens survive neither a restart nor a second replica |
| `CORS_ALLOWED_ORIGINS` | when the SPA is on another origin | — | Comma-separated allowed origins |
| `AUTH_BOOTSTRAP_ADMIN_EMAIL` / `AUTH_BOOTSTRAP_ADMIN_PASSWORD` | no | — | Seeds a local Server Admin on first start |
| `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL` | for OIDC login | — | Brokered OIDC client; leave empty for local login only |
| `OIDC_AUDIENCE` | no | — | Accept IdP-issued access tokens carrying this audience (machine clients) |
| `TRUSTED_PROXY_CIDR` | behind a proxy | — | CIDR whose `X-Forwarded-For` is trusted for rate limiting |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | — | OTLP endpoint; telemetry export is off when empty |

### Usage

Start the full local stack — PostgreSQL, Keycloak with a pre-seeded realm, the
server, and the SPA on the Vite dev server with hot reload:

```bash
docker compose --profile dev up -d --build
```

To run the SPA from its production nginx image instead, use the `prod` profile
(`dev` and `prod` are mutually exclusive). Add `--profile observability` to
either for an OTel Collector and Jaeger:

```bash
docker compose --profile prod --profile observability up -d --build
```

| URL | What |
| --- | --- |
| http://localhost:3000 | Public catalog |
| http://localhost:3000/admin | Admin console (sign in through Keycloak) |
| http://localhost:8081/openapi.yaml | OpenAPI 3.1 document |
| http://localhost:8081/api/v1/mcp/servers | JSON API |
| http://localhost:8081/.well-known/agent-card.json | The registry's own A2A Agent Card |
| http://localhost:8080 | Keycloak, realm `ai-registry` |
| http://localhost:16686 | Jaeger, with the `observability` profile |

The dev realm's users, one per authorization path, are defined in
[deploy/keycloak-realm-dev.json](deploy/keycloak-realm-dev.json). Roles are
granted to users or groups per publisher, from the publisher page in the admin
console or through `/api/v1/publishers/{slug}/grants`.

#### Seeding the catalog from a file

Point `BOOTSTRAP_FILE` (or `--bootstrap-file`) at a YAML or JSON file and the
server upserts the publishers, MCP servers and agents it declares on every
start. Existing rows are left untouched, except that newly declared `tools[]`
are backfilled; role grants are managed through the API, not this file. See
[deploy/bootstrap.example.yaml](deploy/bootstrap.example.yaml).

#### Registry-wide tags

Server Admins manage the tag vocabulary publishers pick from at
`/admin/tags` or `/api/v1/tags`. Tags can also be declared in configuration
(`instance_tags` key, `INSTANCE_TAGS` JSON, or the chart's `api.instanceTags`);
those are reconciled on start and read-only in the UI and API.

### Deployment

Images are published to GHCR as `ghcr.io/haibread/ai-registry/server` and
`ghcr.io/haibread/ai-registry/web`. The Helm chart is published as an OCI
artifact:

```bash
helm install ai-registry oci://ghcr.io/haibread/ai-registry/charts/ai-registry --version <chart-version>
```

Chart values, including the optional CloudNativePG database, are documented in
[deploy/helm/ai-registry/README.md](deploy/helm/ai-registry/README.md).
Operating a deployment is covered by [docs/runbook.md](docs/runbook.md) and
[docs/db-backup.md](docs/db-backup.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT — see [LICENSE](LICENSE).
