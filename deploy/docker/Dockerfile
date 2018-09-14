FROM fedora-minimal

COPY ebs-csi-driver /bin/ebs-csi-driver

RUN microdnf install -y e2fsprogs && microdnf clean all

ENTRYPOINT ["/bin/ebs-csi-driver", "-logtostderr", "-v", "5"]
