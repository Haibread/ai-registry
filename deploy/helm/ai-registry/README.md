# ai-registry

![Version: 0.4.0-rc3](https://img.shields.io/badge/Version-0.4.0--rc3-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.4.0-rc3](https://img.shields.io/badge/AppVersion-0.4.0--rc3-informational?style=flat-square)

A centralized registry for AI ecosystem artifacts (MCP servers and AI agents)

**Homepage:** <https://github.com/haibread/ai-registry>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Haibread |  |  |

## Source Code

* <https://github.com/haibread/ai-registry>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| api | object | `{"accessTokenTtl":null,"authGroupsClaim":null,"authReviewerGroup":null,"bootstrap":{"content":null,"enabled":false,"mountPath":"/etc/ai-registry","sampleData":false},"corsAllowedOrigins":null,"database":{"existingSecret":null,"maxConns":25,"minConns":5,"url":null},"image":{"repository":"ghcr.io/haibread/ai-registry/server","tag":null},"jwtSigningKey":null,"jwtSigningKeyExistingSecret":null,"jwtSigningSeed":null,"jwtSigningSeedExistingSecret":null,"livenessProbe":{"failureThreshold":3,"httpGet":{"path":"/healthz","port":"http"},"initialDelaySeconds":10,"periodSeconds":15,"timeoutSeconds":5},"localLogin":{"bootstrapAdmin":{"email":null,"existingSecret":null,"password":null},"enabled":true},"logLevel":"info","oidcAdminRole":null,"oidcClientId":null,"oidcClientSecret":null,"oidcClientSecretExistingSecret":null,"oidcEnabled":false,"oidcInternalUrl":null,"oidcIssuer":null,"oidcJwksUrl":null,"oidcRedirectUrl":null,"oidcRolesClaim":null,"oidcScopes":[],"otlpEndpoint":null,"podSecurityContext":{"fsGroup":65532,"runAsGroup":65532,"runAsUser":65532},"publicBaseURL":null,"readinessProbe":{"failureThreshold":3,"httpGet":{"path":"/readyz","port":"http"},"initialDelaySeconds":5,"periodSeconds":10,"timeoutSeconds":3},"refreshTokenTtl":null,"resources":{"limits":{"ephemeral-storage":"256Mi","memory":"256Mi"},"requests":{"cpu":"100m","ephemeral-storage":"64Mi","memory":"128Mi"}},"service":{"port":8080,"type":"ClusterIP"},"serviceAccount":{"annotations":{},"create":true,"name":null},"serviceMonitor":{"additionalLabels":{},"enabled":false,"interval":"30s","metricRelabelings":[],"namespace":null,"path":"/metrics","relabelings":[],"scrapeTimeout":"10s"},"startupProbe":{"failureThreshold":12,"httpGet":{"path":"/healthz","port":"http"},"initialDelaySeconds":5,"periodSeconds":5},"trustedProxyCIDR":null}` | Backend Go HTTP service. Inherits every `global.*` default and may override any of them here. |
| api.accessTokenTtl | string | `nil` | Access-token lifetime (Go duration). Unset → server default (15m). |
| api.authGroupsClaim | string | `nil` | JWT claim path for the user's group memberships, resolved to publisher-scoped role grants. Unset → server default (`groups`). |
| api.authReviewerGroup | string | `nil` | Reviewer group whose members can approve / reject pending changes. Unset → server default (`registry-reviewers`). |
| api.bootstrap.content | string | `nil` | Inline YAML bootstrap file. Takes precedence over `sampleData`. |
| api.bootstrap.enabled | bool | `false` | Load a bootstrap file on startup. |
| api.bootstrap.mountPath | string | `"/etc/ai-registry"` | Directory where the bootstrap ConfigMap is mounted inside the container. The server reads `<mountPath>/bootstrap.yaml`. |
| api.bootstrap.sampleData | bool | `false` | Load the built-in sample data bundled with the chart. Ignored when `content` is set. |
| api.corsAllowedOrigins | string | `nil` | Comma-separated list of allowed CORS origins, e.g. "https://registry.example.com,https://admin.example.com". |
| api.database.existingSecret | string | `nil` | Name of a pre-existing Secret whose key `DATABASE_URL` holds the connection string (recommended). Wins over `url` and CNPG. |
| api.database.maxConns | int | `25` | Maximum size of the connection pool. |
| api.database.minConns | int | `5` | Minimum number of idle connections kept in the pool. |
| api.database.url | string | `nil` | Plain-text connection string (not recommended for production). |
| api.image.repository | string | `"ghcr.io/haibread/ai-registry/server"` | Server image repository. |
| api.image.tag | string | `nil` | Server image tag. Unset → `.Chart.AppVersion`. |
| api.jwtSigningKey | string | `nil` | Inline PEM PKCS#8 Ed25519 private key (DEV/quickstart only — rendered into a Secret). Generate: `openssl genpkey -algorithm ed25519`. Wins over `jwtSigningSeed`. |
| api.jwtSigningKeyExistingSecret | string | `nil` | Name of a pre-existing Secret holding key `JWT_SIGNING_KEY`. Preferred in production; wins over the inline key. |
| api.jwtSigningSeed | string | `nil` | Inline high-entropy seed (>= 32 chars) the key is derived from deterministically — a simpler single-secret alternative to a PEM. Generate: `openssl rand -hex 32`. |
| api.jwtSigningSeedExistingSecret | string | `nil` | Name of a pre-existing Secret holding key `JWT_SIGNING_SEED`. |
| api.livenessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/healthz","port":"http"},"initialDelaySeconds":10,"periodSeconds":15,"timeoutSeconds":5}` | Liveness probe (full probe object — override any field). |
| api.livenessProbe.failureThreshold | int | `3` | Consecutive failures before the container is restarted. |
| api.livenessProbe.httpGet.path | string | `"/healthz"` | Path polled for liveness. |
| api.livenessProbe.httpGet.port | string | `"http"` | Named container port polled for liveness. |
| api.livenessProbe.initialDelaySeconds | int | `10` | Delay before the first liveness probe. |
| api.livenessProbe.periodSeconds | int | `15` | Interval between liveness probes. |
| api.livenessProbe.timeoutSeconds | int | `5` | Per-probe timeout. |
| api.localLogin.bootstrapAdmin.email | string | `nil` | Login email for the bootstrap admin. Unset → no admin seeded. |
| api.localLogin.bootstrapAdmin.existingSecret | string | `nil` | Name of a pre-existing Secret holding key `AUTH_BOOTSTRAP_ADMIN_PASSWORD`. Wins over the inline password. |
| api.localLogin.bootstrapAdmin.password | string | `nil` | Inline password (DEV/quickstart only — rendered into a Secret). Ignored when `existingSecret` is set. |
| api.localLogin.enabled | bool | `true` | Enable local email + password login. A stock install is a working local-login registry with no external IdP. Set false to run OIDC-only (then `oidcEnabled` must be true). |
| api.logLevel | string | `"info"` | Structured log level: debug | info | warn | error. |
| api.oidcAdminRole | string | `nil` | Role value within `oidcRolesClaim` that grants Server Admin. Unset → server default (`admin`). |
| api.oidcClientId | string | `nil` | [REQUIRED when oidcEnabled] Confidential broker client ID — the SERVER's OAuth client, never served to the browser. |
| api.oidcClientSecret | string | `nil` | [REQUIRED when oidcEnabled] Inline confidential broker client secret (DEV/quickstart only — rendered into a Kubernetes Secret, base64 NOT encrypted). Mutually exclusive with `oidcClientSecretExistingSecret`. |
| api.oidcClientSecretExistingSecret | string | `nil` | Name of a pre-existing Secret holding key `OIDC_CLIENT_SECRET`. Preferred in production (use with external-secrets / sealed-secrets); wins over the inline value. |
| api.oidcEnabled | bool | `false` | Master switch for the OIDC front door. Leave false to run local-login-only (no IdP required). When true the chart requires `oidcIssuer`, `oidcClientId`, and a client-secret source, failing the render if any is missing. At least one of `oidcEnabled` / `localLogin.enabled` must be true. |
| api.oidcInternalUrl | string | `nil` | Base URL (scheme://host) the server uses to reach the IdP for back-channel calls (discovery / token / JWKS) when the public issuer is not reachable in-cluster. Unset → use `oidcIssuer` for everything. |
| api.oidcIssuer | string | `nil` | [REQUIRED when oidcEnabled] Browser-facing OIDC issuer URL; must match the `iss` claim in issued tokens. |
| api.oidcJwksUrl | string | `nil` | Override the JWKS fetch URL for internal network resolution. Usually unset when `oidcInternalUrl` is provided. |
| api.oidcRedirectUrl | string | `nil` | Callback URL the IdP redirects to after login. Unset → derived from `publicBaseURL` + `/api/v1/auth/oidc/callback`. |
| api.oidcRolesClaim | string | `nil` | OIDC id_token claim path the broker reads realm/global roles from at login. Unset → server default (`realm_access.roles`). |
| api.oidcScopes | list | `[]` | OAuth scopes requested at the authorize endpoint. Empty → server default (openid, profile, email). |
| api.otlpEndpoint | string | `nil` | OTLP gRPC endpoint for traces/metrics export, e.g. "http://otel-collector:4317". Unset → export disabled. |
| api.podSecurityContext | object | `{"fsGroup":65532,"runAsGroup":65532,"runAsUser":65532}` | Pod-level security context overrides for the server. Merged over `global.podSecurityContext`; matches the distroless `nonroot` user (UID/GID 65532). |
| api.podSecurityContext.fsGroup | int | `65532` | fsGroup matching the distroless `nonroot` user. |
| api.podSecurityContext.runAsGroup | int | `65532` | GID of the distroless `nonroot` user. |
| api.podSecurityContext.runAsUser | int | `65532` | UID of the distroless `nonroot` user. |
| api.publicBaseURL | string | `nil` | [REQUIRED] Public-facing base URL of the deployment (what clients use), e.g. "https://registry.example.com". Used by the A2A global agent card and OAuth protected-resource metadata. |
| api.readinessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/readyz","port":"http"},"initialDelaySeconds":5,"periodSeconds":10,"timeoutSeconds":3}` | Readiness probe (full probe object — override any field). |
| api.readinessProbe.failureThreshold | int | `3` | Consecutive failures before the pod is marked unready. |
| api.readinessProbe.httpGet.path | string | `"/readyz"` | Path polled for readiness. |
| api.readinessProbe.httpGet.port | string | `"http"` | Named container port polled for readiness. |
| api.readinessProbe.initialDelaySeconds | int | `5` | Delay before the first readiness probe. |
| api.readinessProbe.periodSeconds | int | `10` | Interval between readiness probes. |
| api.readinessProbe.timeoutSeconds | int | `3` | Per-probe timeout. |
| api.refreshTokenTtl | string | `nil` | Refresh-token lifetime (Go duration). Unset → server default (12h). |
| api.resources | object | `{"limits":{"ephemeral-storage":"256Mi","memory":"256Mi"},"requests":{"cpu":"100m","ephemeral-storage":"64Mi","memory":"128Mi"}}` | Resource requests/limits sized for the registry workload. Requests cover CPU, memory and ephemeral storage; limits cap memory and ephemeral storage only — no CPU limit, as CFS throttling degrades latency worse than the oversubscription it prevents. |
| api.resources.limits.ephemeral-storage | string | `"256Mi"` | Ephemeral-storage limit (an unbounded write cannot fill the node). |
| api.resources.limits.memory | string | `"256Mi"` | Memory limit (a leak cannot take down the node). |
| api.resources.requests.cpu | string | `"100m"` | CPU request. |
| api.resources.requests.ephemeral-storage | string | `"64Mi"` | Ephemeral-storage request. |
| api.resources.requests.memory | string | `"128Mi"` | Memory request. |
| api.service.port | int | `8080` | Service (and container) port the server listens on. |
| api.service.type | string | `"ClusterIP"` | Service type for the server. |
| api.serviceAccount.annotations | object | `{}` | Annotations added to the server ServiceAccount. |
| api.serviceAccount.create | bool | `true` | Create a ServiceAccount for the server. |
| api.serviceAccount.name | string | `nil` | Override the ServiceAccount name. Unset → `<fullname>-server`. |
| api.serviceMonitor.additionalLabels | object | `{}` | Extra labels added to the ServiceMonitor (e.g. to match the Prometheus instance selector, `{ release: kube-prometheus-stack }`). |
| api.serviceMonitor.enabled | bool | `false` | Create a ServiceMonitor for the server. |
| api.serviceMonitor.interval | string | `"30s"` | Scrape interval. |
| api.serviceMonitor.metricRelabelings | list | `[]` | Prometheus `metricRelabelings` applied after scraping. |
| api.serviceMonitor.namespace | string | `nil` | Namespace for the ServiceMonitor. Unset → release namespace. |
| api.serviceMonitor.path | string | `"/metrics"` | Metrics path scraped by Prometheus. |
| api.serviceMonitor.relabelings | list | `[]` | Prometheus `relabelings` applied before scraping. |
| api.serviceMonitor.scrapeTimeout | string | `"10s"` | Scrape timeout. |
| api.startupProbe | object | `{"failureThreshold":12,"httpGet":{"path":"/healthz","port":"http"},"initialDelaySeconds":5,"periodSeconds":5}` | Startup probe (full probe object). 12 × 5s = 60s startup budget before liveness takes over. |
| api.startupProbe.failureThreshold | int | `12` | Consecutive failures before startup is considered failed. |
| api.startupProbe.httpGet.path | string | `"/healthz"` | Path polled during startup. |
| api.startupProbe.httpGet.port | string | `"http"` | Named container port polled during startup. |
| api.startupProbe.initialDelaySeconds | int | `5` | Delay before the first startup probe. |
| api.startupProbe.periodSeconds | int | `5` | Interval between startup probes. |
| api.trustedProxyCIDR | string | `nil` | CIDR of the trusted reverse proxy for real-IP extraction via X-Forwarded-For, e.g. "10.0.0.0/8". |
| cnpg | object | `{"enableSuperuserAccess":true,"enabled":false,"initdb":{"database":"ai_registry","owner":"ai_registry"},"instances":1,"postgresVersion":"18","postgresql":{"parameters":{"max_connections":"200","shared_buffers":"256MB"}},"resources":{"limits":{"ephemeral-storage":"512Mi","memory":"512Mi"},"requests":{"cpu":"100m","ephemeral-storage":"128Mi","memory":"256Mi"}},"storageSize":"5Gi"}` | Optional CloudNativePG-managed Postgres cluster. |
| cnpg.enableSuperuserAccess | bool | `true` | Allow the CNPG operator to connect as superuser (needed for migrations). CNPG then auto-creates `<clusterName>-superuser`, which the server references to build DATABASE_URL. |
| cnpg.enabled | bool | `false` | Provision a CloudNativePG Cluster in the release namespace and wire its superuser secret into the server's DATABASE_URL. |
| cnpg.initdb.database | string | `"ai_registry"` | Name of the database created by the initdb bootstrap. |
| cnpg.initdb.owner | string | `"ai_registry"` | Owner role created by the initdb bootstrap. |
| cnpg.instances | int | `1` | Number of Postgres instances (1 primary + N replicas). |
| cnpg.postgresVersion | string | `"18"` | Major version of the CNPG Postgres image (matches the docker-compose stack, postgres:18). |
| cnpg.postgresql.parameters | object | `{"max_connections":"200","shared_buffers":"256MB"}` | Free-form PostgreSQL server parameters (any GUC). |
| cnpg.postgresql.parameters.max_connections | string | `"200"` | Maximum number of concurrent connections. |
| cnpg.postgresql.parameters.shared_buffers | string | `"256MB"` | Shared memory buffer size. |
| cnpg.resources | object | `{"limits":{"ephemeral-storage":"512Mi","memory":"512Mi"},"requests":{"cpu":"100m","ephemeral-storage":"128Mi","memory":"256Mi"}}` | Resource requests/limits for each Postgres instance pod. Requests cover CPU, memory and ephemeral storage; limits cap memory and ephemeral storage only (no CPU limit). |
| cnpg.resources.limits.ephemeral-storage | string | `"512Mi"` | Ephemeral-storage limit. |
| cnpg.resources.limits.memory | string | `"512Mi"` | Memory limit. |
| cnpg.resources.requests.cpu | string | `"100m"` | CPU request. |
| cnpg.resources.requests.ephemeral-storage | string | `"128Mi"` | Ephemeral-storage request. |
| cnpg.resources.requests.memory | string | `"256Mi"` | Memory request. |
| cnpg.storageSize | string | `"5Gi"` | Persistent volume size for each instance. |
| global | object | `{"affinity":{},"autoscaling":{"enabled":false,"maxReplicas":3,"minReplicas":1,"targetCPUUtilizationPercentage":70,"targetMemoryUtilizationPercentage":null},"extraEnv":[],"extraVolumeMounts":[],"extraVolumes":[],"fullnameOverride":null,"image":{"pullPolicy":"IfNotPresent"},"imagePullSecrets":[],"nameOverride":null,"nodeSelector":{},"podAnnotations":{},"podDisruptionBudget":{"enabled":false,"minAvailable":1},"podLabels":{},"podSecurityContext":{"fsGroup":1000,"runAsGroup":1000,"runAsNonRoot":true,"runAsUser":1000,"seccompProfile":{"type":"RuntimeDefault"}},"replicaCount":1,"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true},"tmpDir":"/tmp","tolerations":[]}` | Values shared by more than one component. Any key here can be overridden inside a component block of the same name. |
| global.affinity | object | `{}` | Affinity rules applied to every workload. |
| global.autoscaling | object | `{"enabled":false,"maxReplicas":3,"minReplicas":1,"targetCPUUtilizationPercentage":70,"targetMemoryUtilizationPercentage":null}` | Default autoscaling policy. When `enabled`, an HPA manages the replica count and `replicaCount` is ignored for that component. |
| global.autoscaling.enabled | bool | `false` | Create a HorizontalPodAutoscaler for the component. |
| global.autoscaling.maxReplicas | int | `3` | Upper replica bound enforced by the HPA. |
| global.autoscaling.minReplicas | int | `1` | Lower replica bound enforced by the HPA. |
| global.autoscaling.targetCPUUtilizationPercentage | int | `70` | Target average CPU utilisation (percentage of requests). |
| global.autoscaling.targetMemoryUtilizationPercentage | int | `nil` | Target average memory utilisation (percentage of requests). Unset → no memory-based scaling metric. |
| global.extraEnv | list | `[]` | Extra environment variables injected into every workload container. Example: extraEnv:   - name: MY_FEATURE_FLAG     value: "true" |
| global.extraVolumeMounts | list | `[]` | Extra volume mounts added to every workload container. |
| global.extraVolumes | list | `[]` | Extra volumes added to every workload's pod spec. |
| global.fullnameOverride | string | `nil` | Override the fully-qualified release name used for every resource. Unset → `<release>-<chart>`. |
| global.image.pullPolicy | string | `"IfNotPresent"` | Image pull policy applied to every workload container. |
| global.imagePullSecrets | list | `[]` | Image pull secrets injected into every workload's pod spec. |
| global.nameOverride | string | `nil` | Override the chart name used in resource names and labels. Unset → the chart name (`ai-registry`). |
| global.nodeSelector | object | `{}` | Node selector applied to every workload. |
| global.podAnnotations | object | `{}` | Extra annotations merged into every workload's pod template. |
| global.podDisruptionBudget | object | `{"enabled":false,"minAvailable":1}` | Default PodDisruptionBudget policy. Only rendered when `enabled=true`; with a single replica a `minAvailable=1` PDB blocks node drains, so bump `replicaCount` or enable autoscaling before turning this on in production. |
| global.podDisruptionBudget.enabled | bool | `false` | Create a PodDisruptionBudget for the component. |
| global.podDisruptionBudget.minAvailable | int | `1` | Minimum number of pods that must stay available during a disruption. |
| global.podLabels | object | `{}` | Extra labels merged into every workload's pod template. |
| global.podSecurityContext | object | `{"fsGroup":1000,"runAsGroup":1000,"runAsNonRoot":true,"runAsUser":1000,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod-level security context applied to all workloads. Defaults to the Kubernetes Pod Security Standards "restricted" profile; components override `runAsUser`/`runAsGroup`/`fsGroup` to match their image's non-root user. |
| global.podSecurityContext.fsGroup | int | `1000` | Supplemental GID owning mounted volumes. |
| global.podSecurityContext.runAsGroup | int | `1000` | Primary GID every container runs as. |
| global.podSecurityContext.runAsNonRoot | bool | `true` | Forbid running any container as UID 0. |
| global.podSecurityContext.runAsUser | int | `1000` | UID every container runs as. |
| global.podSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` | Use the runtime's default seccomp profile. |
| global.replicaCount | int | `1` | Default replica count for every Deployment (ignored for a component once its autoscaling is enabled). |
| global.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container-level security context applied to all workload containers. Restricted profile: no privilege escalation, read-only root fs, all capabilities dropped. |
| global.securityContext.allowPrivilegeEscalation | bool | `false` | Block a process from gaining more privileges than its parent. |
| global.securityContext.capabilities.drop | list | `["ALL"]` | Drop every Linux capability. |
| global.securityContext.readOnlyRootFilesystem | bool | `true` | Mount the container root filesystem read-only. |
| global.tmpDir | string | `"/tmp"` | Writable directory mounted as an `emptyDir` so the read-only-root filesystem container has a scratch `/tmp`. |
| global.tolerations | list | `[]` | Tolerations applied to every workload. |
| httpRoute | object | `{"annotations":{},"enabled":false,"gatewayRef":{"name":null,"namespace":null,"sectionName":null},"host":"ai-registry.example.com"}` | Gateway API HTTPRoute attached to a pre-existing Gateway. |
| httpRoute.annotations | object | `{}` | Extra annotations added to the HTTPRoute. |
| httpRoute.enabled | bool | `false` | Create a Gateway API HTTPRoute attached to an existing Gateway. |
| httpRoute.gatewayRef.name | string | `nil` | [REQUIRED when enabled] Name of the existing Gateway this route attaches to. |
| httpRoute.gatewayRef.namespace | string | `nil` | Namespace of the Gateway. Unset → the release namespace. |
| httpRoute.gatewayRef.sectionName | string | `nil` | Specific Gateway listener section (e.g. "https"). Unset → all listeners. |
| httpRoute.host | string | `"ai-registry.example.com"` | Hostname the route matches. Should align with the Gateway listener's hostname. |
| ingress | object | `{"annotations":{},"className":"nginx","enabled":false,"host":"ai-registry.example.com","tls":{"enabled":false,"secretName":null}}` | Classic Ingress routing API paths to the server and everything else to the web SPA. |
| ingress.annotations | object | `{}` | Extra annotations for the Ingress, e.g. cert-manager issuer or proxy-body-size for nginx. |
| ingress.className | string | `"nginx"` | IngressClass name. Unset → the cluster default IngressClass. |
| ingress.enabled | bool | `false` | Create an Ingress routing API paths to the server and everything else to the web SPA. |
| ingress.host | string | `"ai-registry.example.com"` | Hostname the Ingress matches. |
| ingress.tls.enabled | bool | `false` | Terminate TLS at the Ingress for `host`. |
| ingress.tls.secretName | string | `nil` | Name of the TLS Secret. Unset → `<fullname>-tls`. |
| webapp | object | `{"apiUrl":null,"image":{"repository":"ghcr.io/haibread/ai-registry/web","tag":null},"livenessProbe":{"failureThreshold":3,"httpGet":{"path":"/","port":"http"},"initialDelaySeconds":15,"periodSeconds":20,"timeoutSeconds":5},"podSecurityContext":{"fsGroup":101,"runAsGroup":101,"runAsUser":101},"readinessProbe":{"failureThreshold":3,"httpGet":{"path":"/","port":"http"},"initialDelaySeconds":10,"periodSeconds":10,"timeoutSeconds":3},"resources":{"limits":{"ephemeral-storage":"128Mi","memory":"128Mi"},"requests":{"cpu":"50m","ephemeral-storage":"32Mi","memory":"64Mi"}},"service":{"port":8080,"type":"ClusterIP"},"serviceAccount":{"annotations":{},"create":true,"name":null}}` | Frontend SPA served by nginx. Inherits every `global.*` default and may override any of them here. |
| webapp.apiUrl | string | `nil` | Internal URL nginx proxies `/api/`, `/config.json`, … to. Unset → the in-cluster server service URL derived from the release name. |
| webapp.image.repository | string | `"ghcr.io/haibread/ai-registry/web"` | Web image repository. |
| webapp.image.tag | string | `nil` | Web image tag. Unset → `.Chart.AppVersion`. |
| webapp.livenessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/","port":"http"},"initialDelaySeconds":15,"periodSeconds":20,"timeoutSeconds":5}` | Liveness probe (full probe object — override any field). |
| webapp.livenessProbe.failureThreshold | int | `3` | Consecutive failures before the container is restarted. |
| webapp.livenessProbe.httpGet.path | string | `"/"` | Path polled for liveness. |
| webapp.livenessProbe.httpGet.port | string | `"http"` | Named container port polled for liveness. |
| webapp.livenessProbe.initialDelaySeconds | int | `15` | Delay before the first liveness probe. |
| webapp.livenessProbe.periodSeconds | int | `20` | Interval between liveness probes. |
| webapp.livenessProbe.timeoutSeconds | int | `5` | Per-probe timeout. |
| webapp.podSecurityContext | object | `{"fsGroup":101,"runAsGroup":101,"runAsUser":101}` | Pod-level security context overrides for the web SPA. Merged over `global.podSecurityContext`; matches the `nginx` user (UID/GID 101) in nginxinc/nginx-unprivileged. |
| webapp.podSecurityContext.fsGroup | int | `101` | fsGroup matching the nginx-unprivileged `nginx` user. |
| webapp.podSecurityContext.runAsGroup | int | `101` | GID of the nginx-unprivileged `nginx` user. |
| webapp.podSecurityContext.runAsUser | int | `101` | UID of the nginx-unprivileged `nginx` user. |
| webapp.readinessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/","port":"http"},"initialDelaySeconds":10,"periodSeconds":10,"timeoutSeconds":3}` | Readiness probe (full probe object — override any field). |
| webapp.readinessProbe.failureThreshold | int | `3` | Consecutive failures before the pod is marked unready. |
| webapp.readinessProbe.httpGet.path | string | `"/"` | Path polled for readiness. |
| webapp.readinessProbe.httpGet.port | string | `"http"` | Named container port polled for readiness. |
| webapp.readinessProbe.initialDelaySeconds | int | `10` | Delay before the first readiness probe. |
| webapp.readinessProbe.periodSeconds | int | `10` | Interval between readiness probes. |
| webapp.readinessProbe.timeoutSeconds | int | `3` | Per-probe timeout. |
| webapp.resources | object | `{"limits":{"ephemeral-storage":"128Mi","memory":"128Mi"},"requests":{"cpu":"50m","ephemeral-storage":"32Mi","memory":"64Mi"}}` | Resource requests/limits for the nginx SPA server (tiny footprint). See `api.resources` for the no-CPU-limit rationale. |
| webapp.resources.limits.ephemeral-storage | string | `"128Mi"` | Ephemeral-storage limit. |
| webapp.resources.limits.memory | string | `"128Mi"` | Memory limit. |
| webapp.resources.requests.cpu | string | `"50m"` | CPU request. |
| webapp.resources.requests.ephemeral-storage | string | `"32Mi"` | Ephemeral-storage request. |
| webapp.resources.requests.memory | string | `"64Mi"` | Memory request. |
| webapp.service.port | int | `8080` | Service (and container) port. nginx-unprivileged listens on 8080 (non-root cannot bind < 1024). |
| webapp.service.type | string | `"ClusterIP"` | Service type for the web SPA. |
| webapp.serviceAccount.annotations | object | `{}` | Annotations added to the web ServiceAccount. |
| webapp.serviceAccount.create | bool | `true` | Create a ServiceAccount for the web SPA. |
| webapp.serviceAccount.name | string | `nil` | Override the ServiceAccount name. Unset → `<fullname>-web`. |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
