---
title: AI Registry
description: A self-hostable registry for MCP servers and A2A agents.
template: splash
hero:
  tagline: A self-hostable registry for MCP servers and A2A agents, with a versioned HTTP API, a public catalog and an admin console.
  image:
    file: ../../../../logo.svg
---

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
agents it lists. The only connections it makes to a listed server are the
handshakes and `tools/list` an author asks for from the MCP forms.

## Where to start

- [Run the local stack](/guides/local-stack/) to try it on your machine.
- [Deploy on Kubernetes](/guides/deployment/) with the Helm chart.
- [Configuration](/reference/configuration/) lists the settings a deployment has to set.
- [ARCHITECTURE.md](https://github.com/Haibread/ai-registry/blob/main/ARCHITECTURE.md) explains the design and the reasoning behind it.
