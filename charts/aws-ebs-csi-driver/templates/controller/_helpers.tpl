{{/* vim: set ft=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "aws-ebs-csi-driver.controller.name" -}}
{{- printf "%s-controller" (include "aws-ebs-csi-driver.name" .) }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "aws-ebs-csi-driver.controller.fullname" -}}
{{- printf "%s-controller" (include "aws-ebs-csi-driver.fullname" .) }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "aws-ebs-csi-driver.controller.labels" -}}
{{ include "aws-ebs-csi-driver.labels" . }}
app.kubernetes.io/component: controller
{{- end }}

{{/*
Selector labels
*/}}
{{- define "aws-ebs-csi-driver.controller.selectorLabels" -}}
{{ include "aws-ebs-csi-driver.selectorLabels" . }}
app.kubernetes.io/component: controller
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "aws-ebs-csi-driver.controller.serviceAccountName" -}}
{{- if .Values.controller.serviceAccount.create }}
{{- default (include "aws-ebs-csi-driver.controller.fullname" .) .Values.controller.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.controller.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Convert the `--extra-volume-tags` command line arg from a map.
*/}}
{{- define "aws-ebs-csi-driver.controller.extra-volume-tags" -}}
{{- $result := dict "pairs" (list) -}}
{{- range $key, $value := .Values.controller.extraVolumeTags -}}
{{- $noop := printf "%s=%v" $key $value | append $result.pairs | set $result "pairs" -}}
{{- end -}}
{{- if gt (len $result.pairs) 0 -}}
{{- printf "%s=%s" "- --extra-volume-tags" (join "," $result.pairs) -}}
{{- end -}}
{{- end -}}
