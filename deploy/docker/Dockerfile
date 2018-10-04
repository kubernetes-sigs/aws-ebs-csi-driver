FROM fedora-minimal

COPY aws-ebs-csi-driver /bin/aws-ebs-csi-driver

RUN microdnf install -y e2fsprogs && microdnf clean all

ENTRYPOINT ["/bin/aws-ebs-csi-driver", "-logtostderr", "-v", "5"]
