{{- define "networkpolicy-node" }}
{{- if .Values.node.networkPolicy.enabled }}
{{- if .Values.node.hostNetwork }}
{{- fail (printf "node.networkPolicy.enabled=true has no effect on %s because node.hostNetwork=true: NetworkPolicy does not apply to pods using the host network namespace. Set node.hostNetwork=false to use NetworkPolicy enforcement on the node DaemonSet." .NodeName) }}
{{- end }}
{{- if .Values.node.enableWindows }}
{{- fail (printf "node.networkPolicy.enabled=true has no effect on the %s Windows DaemonSet because Windows node pods always run with hostNetwork=true (NetworkPolicy is not enforced against hostNetwork pods). Set node.enableWindows=false, or do not enable node.networkPolicy, until Windows NetworkPolicy support is added." .NodeName) }}
{{- end }}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ .NodeName }}
  namespace: {{ .Values.node.namespaceOverride | default .Release.Namespace }}
  labels:
    {{- include "aws-ebs-csi-driver.labels" . | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      app: {{ .NodeName }}
      {{- include "aws-ebs-csi-driver.selectorLabels" . | nindent 6 }}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Always allow kubelet health checks (ebs-plugin liveness-probe + node-driver-registrar),
    # regardless of any additional rules below.
    - ports:
        - protocol: TCP
          port: 9808
        - protocol: TCP
          port: {{ .Values.sidecars.nodeDriverRegistrar.healthPort }}
    {{- if .Values.node.enableMetrics }}
    # Always allow metrics scraping when enabled, regardless of any additional rules below.
    - ports:
        - protocol: TCP
          port: 3302
    {{- end }}
    {{- with .Values.node.networkPolicy.ingress }}
    # Additional user-supplied ingress rules, appended to the defaults above.
    {{- toYaml . | nindent 4 }}
    {{- end }}
  egress:
    # Always allow DNS resolution, HTTPS (kube-apiserver, etc.), and IMDS,
    # regardless of any additional rules below.
    - ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    - ports:
        - protocol: TCP
          port: 443
        - protocol: TCP
          port: 6443
    - to:
        - ipBlock:
            cidr: 169.254.169.254/32   # IMDS, for metadata source
        - ipBlock:
            cidr: fd00:ec2::254/128    # IMDS, IPv6
      ports:
        - protocol: TCP
          port: 80
    - to:
        - ipBlock:
            cidr: 169.254.170.23/32    # EKS Pod Identity Agent
      ports:
        - protocol: TCP
          port: 80
    {{- with .Values.node.networkPolicy.egress }}
    # Additional user-supplied egress rules, appended to the defaults above.
    {{- toYaml . | nindent 4 }}
    {{- end }}
{{- end }}
{{- end }}
