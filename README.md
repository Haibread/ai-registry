<p align="center"><img src="logo.svg" alt="AI Registry logo" width="200"></p>

# AI Registry

A self-hostable registry for **MCP servers**, with a versioned HTTP API, a
public catalog UI and an admin console.

[![Lint](https://github.com/Haibread/ai-registry/actions/workflows/lint.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/lint.yml) [![Quality](https://github.com/Haibread/ai-registry/actions/workflows/quality.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/quality.yml) [![Docker](https://github.com/Haibread/ai-registry/actions/workflows/docker.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/docker.yml)

## Description

Teams building with MCP servers end up with them scattered
across repositories, wikis and chat threads. The AI Registry is one catalog
where publishers declare them, reviewers approve them, and people and machines
discover them.

- **Versioned entries** — every publish creates an immutable version; entries
  move through draft, published and deprecated.
- **Spec-aware** — MCP metadata follows the
  [Model Context Protocol](https://modelcontextprotocol.io/) `server.json`
  shapes.
- **Review workflow** — Editors propose versions, deletions and entry changes;
  Reviewers approve them.
- **Publisher-scoped RBAC** — Viewer, Editor, Reviewer and Admin roles granted
  to users or IdP groups per publisher, enforced by the API.
- **Local or OIDC login** — email + password, or any OIDC provider brokered by
  the server; machine clients can present IdP service-account tokens.
- **API-first** — the two UIs are clients of the same `/api/v1`; the OpenAPI
  3.1 document is served at `/openapi.yaml`.
- **Observable** — OpenTelemetry traces, metrics and logs over OTLP.

It hosts metadata only: it does not run, proxy or sandbox the servers it
lists. The only connections it makes to a listed server are the
handshakes and `tools/list` an author asks for from the MCP forms.

The documentation lives at <https://haibread.github.io/ai-registry/>
([llms.txt](https://haibread.github.io/ai-registry/llms.txt) for models). See
[ARCHITECTURE.md](ARCHITECTURE.md) for the design and the reasoning behind
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

### Usage

Start the full local stack — PostgreSQL, Keycloak, the server and the SPA:

```bash
docker compose --profile dev up -d --build
```

The catalog is then at http://localhost:3000. The other URLs, the Compose
profiles and the dev users are in
[Run the local stack](https://haibread.github.io/ai-registry/guides/local-stack/);
configuration, Kubernetes deployment and operations are on the
[documentation site](https://haibread.github.io/ai-registry/).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT — see [LICENSE](LICENSE).
