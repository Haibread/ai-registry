# AI Registry — operations runbook

Audience: on-call engineer for an ai-registry deployment. Each section lists
symptoms, triage commands, and remediation. Keep this file in sync with the
production topology — a runbook that lies is worse than no runbook.

---

## 0. Contacts & links

- Source repo: <https://github.com/haibread/ai-registry>
- Helm chart: `deploy/helm/ai-registry`
- Object names: `<fullname>` below is the release name, suffixed with
  `-ai-registry` unless the release name already contains it.
- OpenAPI spec: `/openapi.yaml` (served by the api pod)

---

## 1. Health endpoints

| Path       | Purpose                                                  | Expected status |
|------------|----------------------------------------------------------|-----------------|
| `/healthz` | Liveness — is the process alive?                         | 200             |
| `/readyz`  | Readiness — can this pod serve traffic right now?        | 200 (or 503 during startup / DB outage) |
| `/metrics` | Prometheus scrape endpoint, on the api Service only.     | 200 (404 through the ingress) |

`/readyz` returning 503 is the first signal for most outages: it removes the pod
from rotation (kube-proxy / ingress) before users notice.

---

## 2. API pod crash-looping

**Symptoms**

- `kubectl get pods -l app.kubernetes.io/component=api` shows
  `CrashLoopBackOff`.
- Ingress returns 502 / 503 to users.

**Triage**

```sh
kubectl logs -l app.kubernetes.io/component=api --tail=200 --previous
kubectl describe pod -l app.kubernetes.io/component=api
```

Common root causes:

| Log line contains                           | Cause                               | Fix |
|---------------------------------------------|-------------------------------------|-----|
| `failed to connect to database`             | Postgres unreachable / bad DSN      | §4  |
| `migrations ... failed`                     | Migration SQL error                 | §5  |
| `invalid TRUSTED_PROXY_CIDR`                | Bad config value                    | Correct `api.trustedProxyCIDR` in values.yaml |
| `jwks ... no such host` / `fetch … timeout` | Cannot reach OIDC issuer            | Check `api.oidcJwksUrl`, DNS, NetworkPolicy |
| OOMKilled (in `kubectl describe`)           | Memory limit too low                | Raise `api.resources.limits.memory` |

---

## 3. Readiness flapping

**Symptoms**

- `/readyz` oscillates between 200 and 503.
- Pods show frequent `Ready=False` transitions in `kubectl get pods -w`.

**Triage**

1. Confirm the DB is the culprit — `/readyz` only fails if the DB ping fails.
2. Check Postgres connection-pool saturation:

   ```sql
   SELECT state, count(*) FROM pg_stat_activity
   WHERE datname = current_database() GROUP BY state;
   ```

3. If saturated, raise `api.database.maxConns` or lower the load (rate limit /
   scale horizontally).

**Remediation**

- Short-term: `kubectl rollout restart deploy/<fullname>-api`.
- Long-term: enable `api.autoscaling.enabled=true` with a sensible CPU target,
  and raise `api.database.maxConns` alongside Postgres `max_connections`.

---

## 4. Database unreachable

**Symptoms**

- `/readyz` returns 503 consistently.
- Server logs show `failed to connect to database` or pgx connection errors.

**Triage**

```sh
# Resolve the service
kubectl get svc,ep -l cnpg.io/cluster

# Try the DSN from inside a temporary pod
kubectl run pg-probe --rm -it --image=postgres:18-alpine -- \
  psql "$DATABASE_URL" -c 'select 1'
```

**Remediation paths**

| Cause                                             | Action |
|---------------------------------------------------|--------|
| CNPG primary is not elected                       | `kubectl describe cluster <name>`; promote a replica if stuck |
| Credentials rotated; api has old secret           | `kubectl rollout restart deploy/<fullname>-api` after updating the secret |
| NetworkPolicy blocks api → DB                     | Review / loosen NetworkPolicy |
| PVC full                                          | Raise `cnpg.storageSize` in values.yaml; CNPG resizes online |

See §6 for backup / restore.

---

## 5. Migration failed on startup

**Symptoms**

- The api fails fast at boot with `migrations ... failed`.
- Previous deploys worked; this is the first restart after a new image tag.

**Triage**

```sh
kubectl logs deploy/<fullname>-api | grep -iE 'migration|sql'
# Check which version the DB is at:
kubectl run pg-probe --rm -it --image=postgres:18-alpine -- \
  psql "$DATABASE_URL" -c 'select version, dirty from schema_migrations'
```

`dirty=true` means a migration partially applied and the process crashed
mid-transaction.

**Remediation**

1. Roll back the api image to the previous tag so traffic stops erroring.
2. Fix the migration manually (apply the remainder by hand or rewind with
   `migrate force <prev_version>` via a one-shot pod).
3. Ship a fixed migration and redeploy.

**Never** edit a migration file already applied in any environment. Roll forward
with a new migration.

---

## 6. Database backup & restore

Backups use CloudNativePG's Barman integration. They are **not** configured in the
default chart — see `docs/db-backup.md` for the full playbook. Summary:

- Continuous WAL archiving + scheduled base backups land in object storage.
- Point-in-time recovery (PITR) is the target recovery mode; RPO ≤ 5 min, RTO
  ~15 min for a single-region restore.
- **Test your restore quarterly.** A backup you've never restored is not a backup.

---

## 7. Certificate / TLS issues

**Symptoms**

- Browsers show certificate errors.
- `curl -v https://ai-registry.example.com` fails TLS handshake.

**Triage**

```sh
kubectl get ingress,certificate,certificaterequest -A
kubectl describe certificate <cert-name>
```

**Common causes**

- cert-manager is missing or the issuer is not `Ready`.
- Let's Encrypt rate-limited the issuer — check `certificaterequest` conditions.
- DNS doesn't resolve the hostname to the ingress IP (HTTP-01 challenge failed).
  Check with `dig +short ai-registry.example.com`.

---

## 8. OIDC / auth outages

**Symptoms**

- Admin UI login loops back to the sign-in page.
- api logs: `jwks fetch failed` or `id_token validation failed` during the OIDC
  callback.

OIDC is brokered server-side and the registry issues its own bearer tokens, so
an IdP outage blocks *new* OIDC logins only. Existing sessions keep refreshing
(refresh never contacts the IdP) until the refresh token expires
(`REFRESH_TOKEN_TTL`), and local email + password login is unaffected.

**Triage**

1. Is the issuer reachable from the cluster? `kubectl exec` into an api pod
   (distroless — use `kubectl debug` with an ephemeral ubuntu container) and
   `curl -v $OIDC_JWKS_URL`. The broker validates the `id_token` against the JWKS at
   `/api/v1/auth/oidc/callback`.
2. Did the issuer rotate its signing key? JWKS is cached briefly; a rotation then a
   restart fixes it.
3. Was the admin role renamed in the IdP? Server Admin requires the claim at
   `OIDC_ROLES_CLAIM` (default `realm_access.roles`) to contain `OIDC_ADMIN_ROLE`
   (default `admin`).

**Remediation**

- Relax admin access only via a maintenance window. Do not disable auth in
  production to debug.

---

## 9. Rate limit firing

**Symptoms**

- Public users see `429 Too Many Requests`.
- Metric `registry_ratelimit_hits_total` is climbing.

**Triage**

- `PUBLIC_RATE_LIMIT_RPM` defaults to 1000 rpm per client IP, per replica.
  Confirm `api.trustedProxyCIDR` is set so the limit keys on the real client IP,
  not the ingress controller IP.

**Remediation**

- Short-term: raise `PUBLIC_RATE_LIMIT_RPM` temporarily (through `api.extraEnv`).
- Long-term: diagnose the abusive client (check top source IPs in access logs) and
  block via ingress WAF / firewall instead of raising the limit.

---

## 10. OOM / memory pressure

**Symptoms**

- Pods restart with `Reason: OOMKilled`.
- Memory metric trends upward over hours.

**Triage**

```sh
kubectl top pods -l app.kubernetes.io/component=api
kubectl describe pod <pod> | grep -A5 'Last State'
```

**Remediation**

1. Raise `api.resources.limits.memory` and redeploy.
2. If memory keeps growing, suspect a leak. The api exposes no `pprof` endpoint;
   track `go_memstats_heap_inuse_bytes` from `/metrics` over time to confirm it.
3. File an issue with the time window, pod name, and `/metrics` snapshot.

---

## 11. Emergency rollback

1. Find the last-known-good image tag:

   ```sh
   kubectl describe deploy/<fullname>-api | grep Image
   git log --oneline -- deploy/helm/ai-registry/Chart.yaml
   ```

2. Roll back:

   ```sh
   helm rollback <release> --wait
   ```

3. Verify: `/readyz` → 200, smoke test passes (`test/load/smoke.js`).

`helm rollback` reverts ConfigMaps/Secrets/Deployments to the previous revision. It
does **not** reverse database migrations — if the bad release shipped a schema
change, follow §5 to roll it forward instead.

---

## 12. Declaring an incident

- Page severity: SEV2 for user-visible outage, SEV3 for degraded.
- Open an incident channel; post every action you take (timestamped).
- Once resolved, write a postmortem within 48 hours under
  `docs/postmortems/<date>-<slug>.md`.
