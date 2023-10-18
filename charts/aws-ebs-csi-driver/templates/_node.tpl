{{- define "node" }}
{{- if or (eq (default true (.Values.node).enableLinux) true) }}
---
kind: DaemonSet
apiVersion: apps/v1
metadata:
  name: {{ .NodeName }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "aws-ebs-csi-driver.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      app: {{ .NodeName }}
      {{- include "aws-ebs-csi-driver.selectorLabels" . | nindent 6 }}
  {{- with (.Values.node).updateStrategy }}
  updateStrategy:
    {{- toYaml . | nindent 4 }}
  {{- else }}
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: "10%"
  {{- end }}
  template:
    metadata:
      labels:
        app: {{ .NodeName }}
        {{- include "aws-ebs-csi-driver.labels" . | nindent 8 }}
        {{- with (.Values.node).podLabels }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      {{- with (.Values.node).podAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
    spec:
      {{- with (.Values.node).affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- else }}
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: eks.amazonaws.com/compute-type
                operator: NotIn
                values:
                - fargate
      {{- end }}
      nodeSelector:
        kubernetes.io/os: linux
        {{- with (.Values.node).nodeSelector }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      serviceAccountName: {{ default "ebs-csi-node-sa" ((.Values.node).serviceAccount).name }}
      priorityClassName: {{ default "system-node-critical" (.Values.node).priorityClassName }}
      tolerations:
        {{- if default true (.Values.node).tolerateAllTaints }}
        - operator: Exists
        {{- else }}
        {{- with (.Values.node).tolerations }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
        - key: "ebs.csi.aws.com/agent-not-ready"
          operator: "Exists"
        {{- end }}
      hostNetwork: {{ default "false" (.Values.node).hostNetwork }}
      {{- with (.Values.node).securityContext }}
      securityContext:
        {{- toYaml . | nindent 8 }}
      {{- else }}
      securityContext:
        runAsNonRoot: false
        runAsUser: 0
        runAsGroup: 0
        fsGroup: 0
      {{- end }}
      containers:
        - name: ebs-plugin
          image: {{ printf "%s%s:%s" (default "" (.Values.image).containerRegistry) (default "public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver" (.Values.image).repository) (toString (default (printf "v%s" .Chart.AppVersion) (.Values.image).tag)) }}
          imagePullPolicy: {{ default "IfNotPresent" (.Values.image).pullPolicy }}
          args:
            - node
            - --endpoint=$(CSI_ENDPOINT)
            {{- with (.Values.node).volumeAttachLimit }}
            - --volume-attach-limit={{ . }}
            {{- end }}
            - --logging-format={{ default "text" (.Values.node).loggingFormat }}
            - --v={{ default "2" (.Values.node).logLevel }}
            {{- if (.Values.node).otelTracing }}
            - --enable-otel-tracing=true
            {{- end}}
          env:
            - name: CSI_ENDPOINT
              value: unix:/csi/csi.sock
            - name: CSI_NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            {{- if (.Values.proxy).http_proxy }}
            {{- include "aws-ebs-csi-driver.http-proxy" . | nindent 12 }}
            {{- end }}
            {{- with (.Values.node).otelTracing }}
            - name: OTEL_SERVICE_NAME
              value: {{ .otelServiceName }}
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: {{ .otelExporterEndpoint }}
            {{- end }}
            {{- with (.Values.node).env }}
            {{- . | toYaml | nindent 12 }}
            {{- end }}
          {{- with (.Values.node).envFrom }}
          envFrom:
            {{- . | toYaml | nindent 12 }}
          {{- end }}
          volumeMounts:
            - name: kubelet-dir
              mountPath: {{ default "/var/lib/kubelet" (.Values.node).kubeletPath }}
              mountPropagation: "Bidirectional"
            - name: plugin-dir
              mountPath: /csi
            - name: device-dir
              mountPath: /dev
          {{- with (.Values.node).volumeMounts }}
          {{- toYaml . | nindent 12 }}
          {{- end }}
          ports:
            - name: healthz
              containerPort: 9808
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: healthz
            initialDelaySeconds: 10
            timeoutSeconds: 3
            periodSeconds: 10
            failureThreshold: 5
          {{- with (.Values.node).resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- else }}
          resources:
            requests:
              cpu: 10m
              memory: 40Mi
            limits:
              memory: 256Mi
          {{- end }}
          {{- with (.Values.node).containerSecurityContext }}
          securityContext:
            {{- toYaml . | nindent 12 }}
          {{- else }}
          securityContext:
            readOnlyRootFilesystem: true
            privileged: true
          {{- end }}
          lifecycle:
            preStop:
              exec:
                command: ["/bin/aws-ebs-csi-driver", "pre-stop-hook"]
        - name: node-driver-registrar
          image: {{ printf "%s%s:%s" (default "" (.Values.image).containerRegistry) (default "public.ecr.aws/eks-distro/kubernetes-csi/node-driver-registrar" (((.Values.sidecars).nodeDriverRegistrar).image).repository) (default "v2.9.0-eks-1-28-6" (((.Values.sidecars).nodeDriverRegistrar).image).tag) }}
          imagePullPolicy: {{ default (default "IfNotPresent" (.Values.image).pullPolicy) (((.Values.sidecars).nodeDriverRegistrar).image).pullPolicy }}
          args:
            - --csi-address=$(ADDRESS)
            - --kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)
            - --v={{ default "2" ((.Values.sidecars).nodeDriverRegistrar).logLevel }}
          env:
            - name: ADDRESS
              value: /csi/csi.sock
            - name: DRIVER_REG_SOCK_PATH
              value: {{ printf "%s/plugins/ebs.csi.aws.com/csi.sock" (trimSuffix "/" (default "/var/lib/kubelet" (.Values.node).kubeletPath)) }}
            {{- if (.Values.proxy).http_proxy }}
            {{- include "aws-ebs-csi-driver.http-proxy" . | nindent 12 }}
            {{- end }}
            {{- with ((.Values.sidecars).nodeDriverRegistrar).env }}
            {{- . | toYaml | nindent 12 }}
            {{- end }}
            {{- range ((.Values.sidecars).nodeDriverRegistrar).additionalArgs }}
            - {{ . }}
            {{- end }}
          {{- with (.Values.node).envFrom }}
          envFrom:
            {{- . | toYaml | nindent 12 }}
          {{- end }}
          livenessProbe:
            exec:
              command:
                - /csi-node-driver-registrar
                - --kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)
                - --mode=kubelet-registration-probe
            initialDelaySeconds: 30
            timeoutSeconds: 15
            periodSeconds: 90
          volumeMounts:
            - name: plugin-dir
              mountPath: /csi
            - name: registration-dir
              mountPath: /registration
            - name: probe-dir
              mountPath: {{ printf "%s/plugins/ebs.csi.aws.com/" (trimSuffix "/" (default "/var/lib/kubelet" (.Values.node).kubeletPath)) }}
          {{- with default (.Values.node).resources ((.Values.sidecars).nodeDriverRegistrar).resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- else }}
          resources:
            requests:
              cpu: 10m
              memory: 40Mi
            limits:
              memory: 256Mi
          {{- end }}
          {{- with ((.Values.sidecars).nodeDriverRegistrar).securityContext }}
          securityContext:
            {{- toYaml . | nindent 12 }}
          {{- else }}
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
          {{- end }}
        - name: liveness-probe
          image: {{ printf "%s%s:%s" (default "" (.Values.image).containerRegistry) (default "public.ecr.aws/eks-distro/kubernetes-csi/livenessprobe" (((.Values.sidecars).livenessProbe).image).repository) (default "v2.10.0-eks-1-28-6" (((.Values.sidecars).livenessProbe).image).tag) }}
          imagePullPolicy: {{ default (default "IfNotPresent" (.Values.image).pullPolicy) (((.Values.sidecars).livenessProbe).image).pullPolicy }}
          args:
            - --csi-address=/csi/csi.sock
            {{- range ((.Values.sidecars).livenessProbe).additionalArgs }}
            - {{ . }}
            {{- end }}
          {{- with (.Values.node).envFrom }}
          envFrom:
            {{- . | toYaml | nindent 12 }}
          {{- end }}
          volumeMounts:
            - name: plugin-dir
              mountPath: /csi
          {{- with default (.Values.node).resources ((.Values.sidecars).livenessProbe).resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- else }}
          resources:
            requests:
              cpu: 10m
              memory: 40Mi
            limits:
              memory: 256Mi
          {{- end }}
          {{- with ((.Values.sidecars).livenessProbe).securityContext }}
          securityContext:
            {{- toYaml . | nindent 12 }}
          {{- else }}
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
          {{- end }}
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
      {{- range . }}
        - name: {{ . }}
      {{- end }}
      {{- end }}
      volumes:
        - name: kubelet-dir
          hostPath:
            path: {{ (default "/var/lib/kubelet" (.Values.node).kubeletPath) }}
            type: Directory
        - name: plugin-dir
          hostPath:
            path: {{ printf "%s/plugins/ebs.csi.aws.com/" (trimSuffix "/" (default "/var/lib/kubelet" (.Values.node).kubeletPath)) }}
            type: DirectoryOrCreate
        - name: registration-dir
          hostPath:
            path: {{ printf "%s/plugins_registry/" (trimSuffix "/" (default "/var/lib/kubelet" (.Values.node).kubeletPath)) }}
            type: Directory
        - name: device-dir
          hostPath:
            path: /dev
            type: Directory
        - name: probe-dir
          emptyDir: {}
        {{- with (.Values.node).volumes }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
{{- end }}
{{- end }}
