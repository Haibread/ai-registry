{{/*
Expand the name of the chart.
*/}}
{{- define "ai-registry.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this
(by the DNS naming spec).
*/}}
{{- define "ai-registry.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
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
Resolve the image tag: use the component-level tag if set, otherwise fall back
to the chart AppVersion.
Usage: {{ include "ai-registry.imageTag" .Values.backend }}
*/}}
{{- define "ai-registry.imageTag" -}}
{{- .tag | default "" }}
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
Backend helpers
------------------------------------------------------------------------- */}}

{{- define "ai-registry.backend.fullname" -}}
{{- printf "%s-backend" (include "ai-registry.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "ai-registry.backend.labels" -}}
{{ include "ai-registry.labels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{- define "ai-registry.backend.selectorLabels" -}}
{{ include "ai-registry.selectorLabels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Backend service account name.
*/}}
{{- define "ai-registry.backend.serviceAccountName" -}}
{{- if .Values.backend.serviceAccount.create }}
{{- default (include "ai-registry.backend.fullname" .) .Values.backend.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.backend.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Backend image reference (repository:tag).
*/}}
{{- define "ai-registry.backend.image" -}}
{{- $tag := .Values.backend.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.backend.image.repository $tag }}
{{- end }}

{{/* -------------------------------------------------------------------------
Web helpers
------------------------------------------------------------------------- */}}

{{- define "ai-registry.web.fullname" -}}
{{- printf "%s-web" (include "ai-registry.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "ai-registry.web.labels" -}}
{{ include "ai-registry.labels" . }}
app.kubernetes.io/component: web
{{- end }}

{{- define "ai-registry.web.selectorLabels" -}}
{{ include "ai-registry.selectorLabels" . }}
app.kubernetes.io/component: web
{{- end }}

{{/*
Web service account name.
*/}}
{{- define "ai-registry.web.serviceAccountName" -}}
{{- if .Values.web.serviceAccount.create }}
{{- default (include "ai-registry.web.fullname" .) .Values.web.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.web.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Web image reference (repository:tag).
*/}}
{{- define "ai-registry.web.image" -}}
{{- $tag := .Values.web.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.web.image.repository $tag }}
{{- end }}

{{/* -------------------------------------------------------------------------
CNPG helpers
------------------------------------------------------------------------- */}}

{{- define "ai-registry.cnpg.clusterName" -}}
{{- printf "%s-postgres" (include "ai-registry.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
CNPG superuser secret name — defaults to "<clusterName>-superuser" which is
what CNPG creates automatically.
*/}}
{{- define "ai-registry.cnpg.superuserSecretName" -}}
{{- if .Values.cnpg.superuserSecretName }}
{{- .Values.cnpg.superuserSecretName }}
{{- else }}
{{- printf "%s-superuser" (include "ai-registry.cnpg.clusterName" .) }}
{{- end }}
{{- end }}
