{{/*
Expand the name of the chart.
*/}}
{{- define "network-policy-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 58 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "network-policy-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 58 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 58 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 58 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "network-policy-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "network-policy-operator.labels" -}}
helm.sh/chart: {{ include "network-policy-operator.chart" . }}
{{ include "network-policy-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Local Operator labels
*/}}
{{- define "network-policy-operator-local.labels" -}}
helm.sh/chart: {{ include "network-policy-operator.chart" . }}
{{ include "network-policy-operator-local.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
DR Operator labels
*/}}
{{- define "network-policy-operator-dr.labels" -}}
helm.sh/chart: {{ include "network-policy-operator.chart" . }}
{{ include "network-policy-operator-dr.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "network-policy-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "network-policy-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Local Operator Selector labels
*/}}
{{- define "network-policy-operator-local.selectorLabels" -}}
app.kubernetes.io/name: {{ include "network-policy-operator.name" . }}-local
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
DR Operator Selector labels
*/}}
{{- define "network-policy-operator-dr.selectorLabels" -}}
app.kubernetes.io/name: {{ include "network-policy-operator.name" . }}-dr
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "network-policy-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "network-policy-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Expand the namespace of the release.
Allows overriding it for multi-namespace deployments in combined charts.
*/}}
{{- define "network-policy-operator.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Network Policy Local Operator image.
*/}}
{{- define "operator-local.image" -}}
    {{ printf "%s:%s" .Values.manager.image.local.repository (.Values.manager.image.local.tag | default .Chart.AppVersion) }}
{{- end -}}

{{/*
Network Policy DR Operator image.
*/}}
{{- define "operator-dr.image" -}}
    {{ printf "%s:%s" .Values.manager.image.dr.repository (.Values.manager.image.dr.tag | default .Chart.AppVersion) }}
{{- end -}}

{{/*
Auth Proxy image.
*/}}
{{- define "authProxy.image" -}}
    {{ printf "%s:%s" .Values.authProxy.image.repository .Values.authProxy.image.tag }}
{{- end -}}

{{/*
Remote operator connection Role
*/}}
{{- define "network-policy-operator.remoteRole" -}}
{{- printf "%s-%s" .Values.manager.rbac.role "dr" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
TLS Job name.
*/}}
{{- define "network-policy-operator.tls.name" -}}
{{- printf "%s-%s" (include "network-policy-operator.name" .) "tls" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
TLS Job image.
*/}}
{{- define "job.image" -}}
    {{ printf "%s:%s" .Values.dr.tlsJob.image.repository (.Values.dr.tlsJob.image.tag | default .Chart.AppVersion) }}
{{- end -}}

{{/*
Restricted environment.
*/}}
{{- define "network-policy-operator.restrictedEnvironment" -}}
    {{- if ne (.Values.INFRA_RESTRICTED_ENVIRONMENT | toString) "<nil>" -}}
        {{- .Values.INFRA_RESTRICTED_ENVIRONMENT }}
    {{- else -}}
        "false"
    {{- end -}}
{{- end -}}
