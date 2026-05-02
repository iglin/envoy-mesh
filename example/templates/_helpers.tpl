{{/*
Expand the chart name.
*/}}
{{- define "example.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels attached to every resource.
*/}}
{{- define "example.labels" -}}
helm.sh/chart: {{ include "example.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Selector labels for mesh-app-alpha.
*/}}
{{- define "example.alphaLabels" -}}
app.kubernetes.io/name: {{ .Values.appAlpha.name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Selector labels for mesh-app-beta.
*/}}
{{- define "example.betaLabels" -}}
app.kubernetes.io/name: {{ .Values.appBeta.name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
