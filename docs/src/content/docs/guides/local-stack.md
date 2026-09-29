---
title: Run the local stack
description: Start PostgreSQL, Keycloak, the server and the SPA with Docker Compose.
sidebar:
  order: 1
---

From a clone of the repository, with Docker and the Compose plugin, start the
full local stack — PostgreSQL, Keycloak with a pre-seeded realm, the
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
[deploy/keycloak-realm-dev.json](https://github.com/Haibread/ai-registry/blob/main/deploy/keycloak-realm-dev.json). Roles are
granted to users or groups per publisher, from the publisher page in the admin
console or through `/api/v1/publishers/{slug}/grants`.
