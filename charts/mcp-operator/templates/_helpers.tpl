{{/*
Expand the name of the chart.
*/}}
{{- define "mcp-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "mcp-operator.fullname" -}}
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
Name of the clusterrole(binding) if in-cluster config is used for the crate cluster.
*/}}
{{- define "mcp-operator.clusterrole" -}}
{{- print "openmcp.cloud:" ( include "mcp-operator.fullname" . ) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Name of the clusterrole(binding) if in-cluster config is used for the laas cluster.
*/}}
{{- define "mcp-operator.landscaper.clusterrole" -}}
{{- print "openmcp.cloud:laas:" ( include "mcp-operator.fullname" . ) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Name of the clusterrole(binding) if in-cluster config is used for the cloudorchestrator cluster.
*/}}
{{- define "mcp-operator.cloudorchestrator.clusterrole" -}}
{{- print "openmcp.cloud:co:" ( include "mcp-operator.fullname" . ) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Name of the clusterrole(binding) if in-cluster config is used for the core cluster.
*/}}
{{- define "mcp-operator.v2bridge.clusterrole" -}}
{{- print "openmcp.cloud:v2:" ( include "mcp-operator.fullname" . ) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Use <image>:<tag> or <image>@<sha256>, depending on which is given.
*/}}
{{- define "image" -}}
{{- if hasPrefix "sha256:" (required "$.tag is required" $.tag) -}}
{{ required "$.repository is required" $.repository }}@{{ required "$.tag is required" $.tag }}
{{- else -}}
{{ required "$.repository is required" $.repository }}:{{ required "$.tag is required" $.tag }}
{{- end -}}
{{- end -}}

{{/*
Renders a list of controllers that have not been deactivated.
If all controllers are active, the result of this is:
- managedcontrolplane
- apiserver
- landscaper
- cloudorchestrator
- authentication
- authorization
*/}}
{{- define "mcp-operator.activeControllers" -}}
{{- range tuple "managedcontrolplane" "apiserver" "landscaper" "cloudOrchestrator" "authentication" "authorization"}}
{{- if not ( "disabled" | get ( . | get $ )) }}
- {{ . | lower }}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Same as 'mcp-operator.activeControllers', but as a comma-separated string.
If all controllers are active, the result of this is:
managedcontrolplane,apiserver,landscaper,cloudorchestrator,authentication
*/}}
{{- define "mcp-operator.activeControllersString" -}}
{{ join "," ( include "mcp-operator.activeControllers" $ | fromYamlArray ) }}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "mcp-operator.labels" -}}
helm.sh/chart-name: {{ .Chart.Name }}
helm.sh/chart-version: {{ .Chart.Version | quote }}
{{ include "mcp-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "mcp-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mcp-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
