{{/* vim: set ft=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "aws-ebs-csi-driver.node.name" -}}
{{- printf "%s-node" (include "aws-ebs-csi-driver.name" .) }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "aws-ebs-csi-driver.node.fullname" -}}
{{- printf "%s-node" (include "aws-ebs-csi-driver.fullname" .) }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "aws-ebs-csi-driver.node.labels" -}}
{{ include "aws-ebs-csi-driver.labels" . }}
app.kubernetes.io/component: node
{{- end }}

{{/*
Selector labels
*/}}
{{- define "aws-ebs-csi-driver.node.selectorLabels" -}}
{{ include "aws-ebs-csi-driver.selectorLabels" . }}
app.kubernetes.io/component: node
{{- end }}

{{/*
Expand the name of the chart.
*/}}
{{- define "aws-ebs-csi-driver.node-windows.name" -}}
{{- printf "%s-node-windows" (include "aws-ebs-csi-driver.name" .) }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "aws-ebs-csi-driver.node-windows.fullname" -}}
{{- printf "%s-node-windows" (include "aws-ebs-csi-driver.fullname" .) }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "aws-ebs-csi-driver.node-windows.labels" -}}
{{ include "aws-ebs-csi-driver.labels" . }}
app.kubernetes.io/component: node-windows
{{- end }}

{{/*
Selector labels
*/}}
{{- define "aws-ebs-csi-driver.node-windows.selectorLabels" -}}
{{ include "aws-ebs-csi-driver.selectorLabels" . }}
app.kubernetes.io/component: node-windows
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "aws-ebs-csi-driver.node.serviceAccountName" -}}
{{- if .Values.node.serviceAccount.create }}
{{- default (include "aws-ebs-csi-driver.node.fullname" .) .Values.node.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.node.serviceAccount.name }}
{{- end }}
{{- end }}