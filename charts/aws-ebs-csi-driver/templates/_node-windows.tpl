{{- define "node-windows" }}
{{- if (.Values.node).enableWindows }}
---
kind: DaemonSet
apiVersion: apps/v1
metadata:
  name: {{ printf "%s-windows" .NodeName }}
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
        kubernetes.io/os: windows
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
              mountPath: C:\var\lib\kubelet
              mountPropagation: "None"
            - name: plugin-dir
              mountPath: C:\csi
            - name: csi-proxy-disk-pipe
              mountPath: \\.\pipe\csi-proxy-disk-v1
            - name: csi-proxy-volume-pipe
              mountPath: \\.\pipe\csi-proxy-volume-v1
            - name: csi-proxy-filesystem-pipe
              mountPath: \\.\pipe\csi-proxy-filesystem-v1
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
          securityContext:
            windowsOptions:
              runAsUserName: "ContainerAdministrator"
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
              value: unix:/csi/csi.sock
            - name: DRIVER_REG_SOCK_PATH
              value: C:\var\lib\kubelet\plugins\ebs.csi.aws.com\csi.sock
            {{- if (.Values.proxy).http_proxy }}
            {{- include "aws-ebs-csi-driver.http-proxy" . | nindent 12 }}
            {{- end }}
            {{- with ((.Values.sidecars).nodeDriverRegistrar).env }}
            {{- . | toYaml | nindent 12 }}
            {{- end }}
          {{- with (.Values.node).envFrom }}
          envFrom:
            {{- . | toYaml | nindent 12 }}
          {{- end }}
          livenessProbe:
            exec:
              command:
                - /csi-node-driver-registrar.exe
                - --kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)
                - --mode=kubelet-registration-probe
            initialDelaySeconds: 30
            timeoutSeconds: 15
            periodSeconds: 90
          volumeMounts:
            - name: plugin-dir
              mountPath: C:\csi
            - name: registration-dir
              mountPath: C:\registration
            - name: probe-dir
              mountPath: C:\var\lib\kubelet\plugins\ebs.csi.aws.com
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
        - name: liveness-probe
          image: {{ printf "%s%s:%s" (default "" (.Values.image).containerRegistry) (default "public.ecr.aws/eks-distro/kubernetes-csi/livenessprobe" (((.Values.sidecars).livenessProbe).image).repository) (default "v2.10.0-eks-1-28-6" (((.Values.sidecars).livenessProbe).image).tag) }}
          imagePullPolicy: {{ default (default "IfNotPresent" (.Values.image).pullPolicy) (((.Values.sidecars).livenessProbe).image).pullPolicy }}
          args:
            - --csi-address=unix:/csi/csi.sock
            {{- range ((.Values.sidecars).livenessProbe).additionalArgs }}
            - {{ . }}
            {{- end }}
          {{- with (.Values.node).envFrom }}
          envFrom:
            {{- . | toYaml | nindent 12 }}
          {{- end }}
          volumeMounts:
            - name: plugin-dir
              mountPath: C:\csi
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
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
      {{- range . }}
        - name: {{ . }}
      {{- end }}
      {{- end }}
      volumes:
        - name: kubelet-dir
          hostPath:
            path: C:\var\lib\kubelet
            type: Directory
        - name: plugin-dir
          hostPath:
            path: C:\var\lib\kubelet\plugins\ebs.csi.aws.com
            type: DirectoryOrCreate
        - name: registration-dir
          hostPath:
            path: C:\var\lib\kubelet\plugins_registry
            type: Directory
        - name: csi-proxy-disk-pipe
          hostPath:
            path: \\.\pipe\csi-proxy-disk-v1
            type: ""
        - name: csi-proxy-volume-pipe
          hostPath:
            path: \\.\pipe\csi-proxy-volume-v1
            type: ""
        - name: csi-proxy-filesystem-pipe
          hostPath:
            path: \\.\pipe\csi-proxy-filesystem-v1
            type: ""
        - name: probe-dir
          emptyDir: {}
{{- end }}
{{- end }}
