{{/*
Expand the name of the chart.
*/}}
{{- define "network-policy-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "network-policy-operator.fullname" -}}
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
Selector labels
*/}}
{{- define "network-policy-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "network-policy-operator.name" . }}
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
Network Policy Operator image.
*/}}
{{- define "operator.image" -}}
    {{ .Values.manager.image }}
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
{{- define "tlsjob.image" -}}
    {{ .Values.dr.tlsJob.image }}
{{- end -}}

{{/*
Restart Job name.
*/}}
{{- define "network-policy-operator.restart.name" -}}
{{- printf "%s-%s" (include "network-policy-operator.name" .) "restart" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Restart Job image.
*/}}
{{- define "restartjob.image" -}}
    {{ .Values.dr.restartJob.image }}
{{- end -}}
