{{/*
Expand the name of the chart.
*/}}
{{- define "ai-registry.name" -}}
{{- default .Chart.Name .Values.global.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this
(by the DNS naming spec).
*/}}
{{- define "ai-registry.fullname" -}}
{{- if .Values.global.fullnameOverride }}
{{- .Values.global.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.global.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart label.
*/}}
{{- define "ai-registry.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels (applied to all resources).
*/}}
{{- define "ai-registry.labels" -}}
helm.sh/chart: {{ include "ai-registry.chart" . }}
{{ include "ai-registry.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels (stable subset; never change after first deploy).
*/}}
{{- define "ai-registry.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ai-registry.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/* -------------------------------------------------------------------------
Per-component value resolution.

Every key in `.Values.global` is a default that a component block may override
by re-declaring the same key. The merged view is `mergeOverwrite (deepCopy
global) component`: the component wins on conflicts (deep-merged for maps,
replaced for lists/scalars), and component-only keys are carried through.

`ai-registry.componentValues` takes a dict {"global": ..., "component": ...}
and returns the merged values as YAML; callers round-trip through `fromYaml`:

    {{- $v := include "ai-registry.api.values" . | fromYaml }}
------------------------------------------------------------------------- */}}
{{- define "ai-registry.componentValues" -}}
{{- mergeOverwrite (deepCopy .global) .component | toYaml -}}
{{- end -}}

{{- define "ai-registry.api.values" -}}
{{- include "ai-registry.componentValues" (dict "global" .Values.global "component" .Values.api) -}}
{{- end -}}

{{- define "ai-registry.webapp.values" -}}
{{- include "ai-registry.componentValues" (dict "global" .Values.global "component" .Values.webapp) -}}
{{- end -}}

{{/* -------------------------------------------------------------------------
API (backend) helpers
------------------------------------------------------------------------- */}}

{{/*
Validate the server auth configuration. Fails the render with an actionable
message rather than letting the pod crash-loop on boot. Mirrors the server's
own validation (internal/config): at least one login method must be open, and
an enabled OIDC broker needs an issuer, client ID, and a client-secret source.
*/}}
{{- define "ai-registry.api.validateAuth" -}}
{{- $s := .Values.api -}}
{{- if and (not $s.oidcEnabled) (not $s.localLogin.enabled) -}}
{{- fail "No login method enabled: set api.localLogin.enabled=true (local accounts) and/or api.oidcEnabled=true (OIDC)." -}}
{{- end -}}
{{- if $s.oidcEnabled -}}
{{- if not $s.oidcIssuer -}}
{{- fail "api.oidcEnabled=true requires api.oidcIssuer (the IdP issuer URL)." -}}
{{- end -}}
{{- if not $s.oidcClientId -}}
{{- fail "api.oidcEnabled=true requires api.oidcClientId (the confidential broker client ID)." -}}
{{- end -}}
{{- if and (not $s.oidcClientSecret) (not $s.oidcClientSecretExistingSecret) -}}
{{- fail "api.oidcEnabled=true requires a client secret: set api.oidcClientSecret or api.oidcClientSecretExistingSecret." -}}
{{- end -}}
{{- end -}}
{{- if and $s.oidcClientSecret $s.oidcClientSecretExistingSecret -}}
{{- fail "Set only one of api.oidcClientSecret or api.oidcClientSecretExistingSecret, not both." -}}
{{- end -}}
{{- end -}}

{{- define "ai-registry.api.fullname" -}}
{{- printf "%s-api" (include "ai-registry.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "ai-registry.api.labels" -}}
{{ include "ai-registry.labels" . }}
app.kubernetes.io/component: api
{{- end }}

{{- define "ai-registry.api.selectorLabels" -}}
{{ include "ai-registry.selectorLabels" . }}
app.kubernetes.io/component: api
{{- end }}

{{/*
Backend service account name.
*/}}
{{- define "ai-registry.api.serviceAccountName" -}}
{{- if .Values.api.serviceAccount.create }}
{{- default (include "ai-registry.api.fullname" .) .Values.api.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.api.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Backend image reference (repository:tag).
*/}}
{{- define "ai-registry.api.image" -}}
{{- $tag := .Values.api.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.api.image.repository $tag }}
{{- end }}

{{/* -------------------------------------------------------------------------
Webapp helpers
------------------------------------------------------------------------- */}}

{{- define "ai-registry.webapp.fullname" -}}
{{- printf "%s-webapp" (include "ai-registry.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "ai-registry.webapp.labels" -}}
{{ include "ai-registry.labels" . }}
app.kubernetes.io/component: webapp
{{- end }}

{{- define "ai-registry.webapp.selectorLabels" -}}
{{ include "ai-registry.selectorLabels" . }}
app.kubernetes.io/component: webapp
{{- end }}

{{/*
Web service account name.
*/}}
{{- define "ai-registry.webapp.serviceAccountName" -}}
{{- if .Values.webapp.serviceAccount.create }}
{{- default (include "ai-registry.webapp.fullname" .) .Values.webapp.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.webapp.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Web image reference (repository:tag).
*/}}
{{- define "ai-registry.webapp.image" -}}
{{- $tag := .Values.webapp.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.webapp.image.repository $tag }}
{{- end }}

{{/* -------------------------------------------------------------------------
CNPG helpers
------------------------------------------------------------------------- */}}

{{- define "ai-registry.cnpg.clusterName" -}}
{{- printf "%s-postgres" (include "ai-registry.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
CNPG superuser secret name — always "<clusterName>-superuser",
the secret CNPG auto-creates when enableSuperuserAccess is true.
*/}}
{{- define "ai-registry.cnpg.superuserSecretName" -}}
{{- printf "%s-superuser" (include "ai-registry.cnpg.clusterName" .) }}
{{- end }}
