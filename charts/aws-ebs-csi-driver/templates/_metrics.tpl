{{- define "metrics" }}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
  namespace: {{ .Release.Namespace }}
  labels:
    app: {{ .Name }}
  {{- if .EnableAnnotations }}
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "{{ .Port }}"
  {{- end }}
spec:
  selector:
    app: {{ .TargetPod }}
  ports:
    - name: metrics
      port: {{ .Port }}
      targetPort: {{ .Port }}
  type: ClusterIP
  {{- with .InternalTrafficPolicy }}
  internalTrafficPolicy: {{ . }}
  {{- end }}
{{- if or .ServiceMonitor.forceEnable (.Capabilities.APIVersions.Has "monitoring.coreos.com/v1/ServiceMonitor") }}
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ .Name }}
  namespace: {{ .Release.Namespace }}
  labels:
    app: {{ .Name }}
    {{- if .ServiceMonitor.labels }}
    {{- toYaml .ServiceMonitor.labels | nindent 4 }}
    {{- end }}
spec:
  selector:
    matchLabels:
      app: {{ .Name }}
  namespaceSelector:
    matchNames:
      - {{ .Release.Namespace }}
  endpoints:
    - targetPort: {{ .Port }}
      path: /metrics
      interval: {{ .ServiceMonitor.interval | default "15s"}}
      {{- if .ServiceMonitor.relabelings }}
      relabelings:
      {{- toYaml .ServiceMonitor.relabelings | nindent 6 }}
      {{- end }}
{{- end }}
{{- end }}
