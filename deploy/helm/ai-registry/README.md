# ai-registry

![Version: 0.4.0-rc0](https://img.shields.io/badge/Version-0.4.0--rc0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.4.0-rc0](https://img.shields.io/badge/AppVersion-0.4.0--rc0-informational?style=flat-square)

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
| cnpg | object | `{"enableSuperuserAccess":true,"enabled":false,"initdb":{"database":"ai_registry","owner":"ai_registry"},"instances":1,"postgresVersion":"18","postgresql":{"parameters":{"max_connections":"200","shared_buffers":"256MB"}},"storageSize":"5Gi"}` | -------------------------------------------------------------------------- |
| fullnameOverride | string | `""` |  |
| httpRoute | object | `{"annotations":{},"enabled":false,"gatewayRef":{"name":"","namespace":"","sectionName":""},"host":"ai-registry.example.com"}` | -------------------------------------------------------------------------- |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| imagePullSecrets | list | `[]` |  |
| ingress | object | `{"annotations":{},"className":"nginx","enabled":false,"host":"ai-registry.example.com","tls":{"enabled":false,"secretName":""}}` | -------------------------------------------------------------------------- |
| nameOverride | string | `""` |  |
| server | object | `{"affinity":{},"authGroupsClaim":"","authReviewerGroup":"","authStorage":"","autoscaling":{"enabled":false,"maxReplicas":3,"minReplicas":1,"targetCPUUtilizationPercentage":70},"bootstrap":{"content":"","enabled":false,"mountPath":"/etc/ai-registry","sampleData":false},"corsAllowedOrigins":"","databaseUrl":"","databaseUrlSecret":"","dbMaxConns":25,"dbMinConns":5,"extraEnv":[],"extraVolumeMounts":[],"extraVolumes":[],"image":{"repository":"ghcr.io/haibread/ai-registry/server","tag":""},"livenessProbe":{"failureThreshold":3,"httpGet":{"path":"/healthz","port":"http"},"initialDelaySeconds":10,"periodSeconds":15,"timeoutSeconds":5},"localLogin":{"bootstrapAdmin":{"email":"","existingSecret":"","password":""},"enabled":false},"logLevel":"info","nodeSelector":{},"oidcAudience":"ai-registry-server","oidcClientId":"","oidcIssuer":"","oidcJwksUrl":"","otlpEndpoint":"","podAnnotations":{},"podDisruptionBudget":{"enabled":false,"minAvailable":1},"podLabels":{},"podSecurityContext":{"fsGroup":65532,"runAsGroup":65532,"runAsNonRoot":true,"runAsUser":65532,"seccompProfile":{"type":"RuntimeDefault"}},"publicBaseURL":"","readinessProbe":{"failureThreshold":3,"httpGet":{"path":"/readyz","port":"http"},"initialDelaySeconds":5,"periodSeconds":10,"timeoutSeconds":3},"replicaCount":1,"resources":{"limits":{"memory":"256Mi"},"requests":{"cpu":"100m","memory":"128Mi"}},"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true},"service":{"port":8080,"type":"ClusterIP"},"serviceAccount":{"annotations":{},"create":true,"name":""},"serviceMonitor":{"enabled":false},"startupProbe":{"failureThreshold":12,"httpGet":{"path":"/healthz","port":"http"},"initialDelaySeconds":5,"periodSeconds":5},"tmpDir":"/tmp","tolerations":[],"trustedProxyCIDR":""}` | -------------------------------------------------------------------------- |
| web | object | `{"affinity":{},"apiUrl":"","autoscaling":{"enabled":false,"maxReplicas":3,"minReplicas":1,"targetCPUUtilizationPercentage":70},"extraEnv":[],"extraVolumeMounts":[],"extraVolumes":[],"image":{"repository":"ghcr.io/haibread/ai-registry/web","tag":""},"livenessProbe":{"failureThreshold":3,"httpGet":{"path":"/","port":"http"},"initialDelaySeconds":15,"periodSeconds":20,"timeoutSeconds":5},"nodeSelector":{},"podAnnotations":{},"podDisruptionBudget":{"enabled":false,"minAvailable":1},"podLabels":{},"podSecurityContext":{"fsGroup":101,"runAsGroup":101,"runAsNonRoot":true,"runAsUser":101,"seccompProfile":{"type":"RuntimeDefault"}},"readinessProbe":{"failureThreshold":3,"httpGet":{"path":"/","port":"http"},"initialDelaySeconds":10,"periodSeconds":10,"timeoutSeconds":3},"replicaCount":1,"resources":{"limits":{"memory":"128Mi"},"requests":{"cpu":"50m","memory":"64Mi"}},"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true},"service":{"port":8080,"type":"ClusterIP"},"serviceAccount":{"annotations":{},"create":true,"name":""},"tmpDir":"/tmp","tolerations":[]}` | -------------------------------------------------------------------------- |

