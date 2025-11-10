{{- define "node" }}
{{- if .Values.node.enableLinux }}
---
kind: DaemonSet
apiVersion: apps/v1
metadata:
  name: {{ .NodeName }}
  namespace: {{ .Values.node.namespaceOverride | default .Release.Namespace }}
  labels:
    {{- include "aws-ebs-csi-driver.labels" . | nindent 4 }}
  {{- with .Values.node.daemonSetAnnotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if or (kindIs "float64" .Values.node.revisionHistoryLimit) (kindIs "int64" .Values.node.revisionHistoryLimit) }}
  revisionHistoryLimit: {{ .Values.node.revisionHistoryLimit }}
  {{- end }}
  selector:
    matchLabels:
      app: {{ .NodeName }}
      {{- include "aws-ebs-csi-driver.selectorLabels" . | nindent 6 }}
  updateStrategy:
    {{- toYaml .Values.node.updateStrategy | nindent 4 }}
  template:
    metadata:
      labels:
        app: {{ .NodeName }}
        {{- include "aws-ebs-csi-driver.labels" . | nindent 8 }}
        {{- if .Values.node.podLabels }}
        {{- toYaml .Values.node.podLabels | nindent 8 }}
        {{- end }}
      {{- with .Values.node.podAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
    spec:
      {{- with .Values.node.affinity }}
      affinity: {{- toYaml . | nindent 8 }}
      {{- end }}
      nodeSelector:
        kubernetes.io/os: linux
        {{- with .Values.node.nodeSelector }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      serviceAccountName: {{ .Values.node.serviceAccount.name }}
      terminationGracePeriodSeconds: {{ .Values.node.terminationGracePeriodSeconds }}
      priorityClassName: {{ .Values.node.priorityClassName | default "system-node-critical" }}
      tolerations:
        {{- if .Values.node.tolerateAllTaints }}
        - operator: Exists
        {{- else }}
        {{- with .Values.node.tolerations }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
        {{- include "aws-ebs-csi-driver.daemonset-tolerations" . | nindent 8 }}
        {{- end }}
      hostNetwork: {{ .Values.node.hostNetwork }}
      {{- with .Values.node.securityContext }}
      securityContext:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.node.initContainers }}
      initContainers:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: ebs-plugin
          image: {{ include "aws-ebs-csi-driver.fullImagePath" $ }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args:
            - node
            - --endpoint=$(CSI_ENDPOINT)
            {{- with .Values.node.reservedVolumeAttachments }}
            - --reserved-volume-attachments={{ . }}
            {{- end }}
            {{- if .Values.node.enableMetrics }}
            - --http-endpoint=0.0.0.0:3302
            {{- end}}
            {{- with .Values.node.kubeletPath }}
            - --csi-mount-point-prefix={{ . }}/plugins/kubernetes.io/csi/ebs.csi.aws.com/
            {{- end}}
            {{- with .Values.node.volumeAttachLimit }}
            - --volume-attach-limit={{ . }}
            {{- end }}
            {{- with .Values.node.metadataSources }}
            - --metadata-sources={{ . }}
            {{- end }}
            {{- if .Values.node.legacyXFS }}
            - --legacy-xfs=true
            {{- end}}
            {{- with .Values.node.loggingFormat }}
            - --logging-format={{ . }}
            {{- end }}
            {{- if .Values.debugLogs }}
            - --v=7
            {{- else }}
            - --v={{ .Values.node.logLevel }}
            {{- end }}
            {{- if .Values.node.otelTracing }}
            - --enable-otel-tracing=true
            {{- end}}
            {{- range .Values.node.additionalArgs }}
            - {{ . }}
            {{- end }}
          env:
            - name: CSI_ENDPOINT
              value: unix:/csi/csi.sock
            - name: CSI_NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            {{- if .Values.proxy.http_proxy }}
            {{- include "aws-ebs-csi-driver.http-proxy" . | nindent 12 }}
            {{- end }}
            {{- with .Values.node.otelTracing }}
            - name: OTEL_SERVICE_NAME
              value: {{ .otelServiceName }}
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: {{ .otelExporterEndpoint }}
            {{- end }}
            {{- if .Values.fips }}
            - name: AWS_USE_FIPS_ENDPOINT
              value: "true"
            {{- end }}
            {{- if .Values.node.serviceAccount.disableMutation }}
            - name: DISABLE_TAINT_WATCHER
              value: "true"
            {{- end }}
            {{- with .Values.node.env }}
            {{- . | toYaml | nindent 12 }}
            {{- end }}
          {{- with .Values.controller.envFrom }}
          envFrom:
            {{- . | toYaml | nindent 12 }}
          {{- end }}
          volumeMounts:
            - name: kubelet-dir
              mountPath: {{ .Values.node.kubeletPath }}
              mountPropagation: "Bidirectional"
            - name: plugin-dir
              mountPath: /csi
            - name: device-dir
              mountPath: /dev
            {{- if .Values.node.selinux }}
            - name: selinux-sysfs
              mountPath: /sys/fs/selinux
            - name: selinux-config
              mountPath: /etc/selinux/config
              readOnly: true
            {{- end }}
          {{- with .Values.node.volumeMounts }}
          {{- toYaml . | nindent 12 }}
          {{- end }}
          ports:
            - name: healthz
              containerPort: 9808
              protocol: TCP
            {{- if .Values.node.enableMetrics }}
            - name: metrics
              containerPort: 3302
              protocol: TCP
            {{- end }}
          livenessProbe:
            httpGet:
              path: /healthz
              port: healthz
            initialDelaySeconds: 10
            timeoutSeconds: 3
            periodSeconds: 10
            failureThreshold: 5
          readinessProbe:
            httpGet:
              path: /healthz
              port: healthz
            timeoutSeconds: 3
            periodSeconds: 5
            failureThreshold: 3
          {{- with .Values.node.resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .Values.node.containerSecurityContext }}
          securityContext:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          lifecycle:
            preStop:
              exec:
                command: ["/bin/aws-ebs-csi-driver", "pre-stop-hook"]
          terminationMessagePolicy: FallbackToLogsOnError
        - name: node-driver-registrar
          image: {{ printf "%s%s:%s" (default "" .Values.image.containerRegistry) .Values.sidecars.nodeDriverRegistrar.image.repository .Values.sidecars.nodeDriverRegistrar.image.tag }}
          imagePullPolicy: {{ default .Values.image.pullPolicy .Values.sidecars.nodeDriverRegistrar.image.pullPolicy }}
          args:
            - --csi-address=$(ADDRESS)
            - --kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)
            {{- if .Values.debugLogs }}
            - --v=7
            {{- else }}
            - --v={{ .Values.sidecars.nodeDriverRegistrar.logLevel }}
            {{- end }}
            {{- range .Values.sidecars.nodeDriverRegistrar.additionalArgs }}
            - {{ . }}
            {{- end }}
          env:
            - name: ADDRESS
              value: /csi/csi.sock
            - name: DRIVER_REG_SOCK_PATH
              value: {{ printf "%s/plugins/ebs.csi.aws.com/csi.sock" (trimSuffix "/" .Values.node.kubeletPath) }}
            {{- if .Values.proxy.http_proxy }}
            {{- include "aws-ebs-csi-driver.http-proxy" . | nindent 12 }}
            {{- end }}
            {{- with .Values.sidecars.nodeDriverRegistrar.env }}
            {{- . | toYaml | nindent 12 }}
            {{- end }}
          {{- with .Values.controller.envFrom }}
          envFrom:
            {{- . | toYaml | nindent 12 }}
          {{- end }}
          {{- with .Values.sidecars.nodeDriverRegistrar.livenessProbe }}
          livenessProbe:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          volumeMounts:
            - name: plugin-dir
              mountPath: /csi
            - name: registration-dir
              mountPath: /registration
            - name: probe-dir
              mountPath: {{ printf "%s/plugins/ebs.csi.aws.com/" (trimSuffix "/" .Values.node.kubeletPath) }}
          {{- with default .Values.node.resources .Values.sidecars.nodeDriverRegistrar.resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .Values.sidecars.nodeDriverRegistrar.securityContext }}
          securityContext:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          terminationMessagePolicy: FallbackToLogsOnError
        - name: liveness-probe
          image: {{ printf "%s%s:%s" (default "" .Values.image.containerRegistry) .Values.sidecars.livenessProbe.image.repository .Values.sidecars.livenessProbe.image.tag }}
          imagePullPolicy: {{ default .Values.image.pullPolicy .Values.sidecars.livenessProbe.image.pullPolicy }}
          args:
            - --csi-address=/csi/csi.sock
            {{- range .Values.sidecars.livenessProbe.additionalArgs }}
            - {{ . }}
            {{- end }}
          {{- with .Values.controller.envFrom }}
          envFrom:
            {{- . | toYaml | nindent 12 }}
          {{- end }}
          volumeMounts:
            - name: plugin-dir
              mountPath: /csi
          {{- with default .Values.node.resources .Values.sidecars.livenessProbe.resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .Values.sidecars.livenessProbe.securityContext }}
          securityContext:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          terminationMessagePolicy: FallbackToLogsOnError
      {{- if .Values.imagePullSecrets }}
      imagePullSecrets:
      {{- range .Values.imagePullSecrets }}
        - name: {{ . }}
      {{- end }}
      {{- end }}
      volumes:
        - name: kubelet-dir
          hostPath:
            path: {{ .Values.node.kubeletPath }}
            type: Directory
        - name: plugin-dir
          hostPath:
            path: {{ printf "%s/plugins/ebs.csi.aws.com/" (trimSuffix "/" .Values.node.kubeletPath) }}
            type: DirectoryOrCreate
        - name: registration-dir
          hostPath:
            path: {{ printf "%s/plugins_registry/" (trimSuffix "/" .Values.node.kubeletPath) }}
            type: Directory
        - name: device-dir
          hostPath:
            path: /dev
            type: Directory
        {{- if .Values.node.selinux }}
        - name: selinux-sysfs
          hostPath:
            path: /sys/fs/selinux
            type: Directory
        - name: selinux-config
          hostPath:
            path: /etc/selinux/config
            type: File
        {{- end }}
        - name: probe-dir
          {{- if .Values.node.probeDirVolume }}
          {{- toYaml .Values.node.probeDirVolume | nindent 10 }}
          {{- else }}
          emptyDir: {}
          {{- end }}
        {{- with .Values.node.volumes }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      {{- if .Values.node.dnsConfig }}
      dnsConfig:
        {{- toYaml .Values.node.dnsConfig | nindent 8 }}
      {{- end }}
{{- end }}
{{- end }}
