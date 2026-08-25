{{- define "metrics" }}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Name | quote }}
  namespace: {{ .Release.Namespace | quote }}
  labels:
    app: {{ .Name | quote }}
  {{- if .EnableAnnotations }}
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "{{ .Port }}"
  {{- end }}
spec:
  selector:
    app: {{ .TargetPod | quote }}
  ports:
    - name: metrics
      port: {{ .Port }}
      targetPort: {{ .Port }}
  type: ClusterIP
  {{- with .InternalTrafficPolicy }}
  internalTrafficPolicy: {{ . | quote }}
  {{- end }}
{{- if or .ServiceMonitor.forceEnable (.Capabilities.APIVersions.Has "monitoring.coreos.com/v1/ServiceMonitor") }}
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ .Name | quote }}
  namespace: {{ .Release.Namespace | quote }}
  labels:
    app: {{ .Name | quote }}
    {{- if .ServiceMonitor.labels }}
    {{- toYaml .ServiceMonitor.labels | nindent 4 }}
    {{- end }}
spec:
  selector:
    matchLabels:
      app: {{ .Name | quote }}
  namespaceSelector:
    matchNames:
      - {{ .Release.Namespace | quote }}
  endpoints:
    - targetPort: {{ .Port }}
      path: /metrics
      interval: {{ .ServiceMonitor.interval | default "15s" | quote }}
      {{- if .ServiceMonitor.relabelings }}
      relabelings:
      {{- toYaml .ServiceMonitor.relabelings | nindent 6 }}
      {{- end }}
{{- end }}
{{- end }}
