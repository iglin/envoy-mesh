{{/*
Expand the name of the release. Always uses .Values.name so Deployment,
Service, and ConfigMap share the exact same identifier.
*/}}
{{- define "envoy.name" -}}
{{- .Values.name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every resource.
*/}}
{{- define "envoy.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | quote }}
app.kubernetes.io/name: {{ include "envoy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels used by Deployment and Service.
*/}}
{{- define "envoy.selectorLabels" -}}
app.kubernetes.io/name: {{ include "envoy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
