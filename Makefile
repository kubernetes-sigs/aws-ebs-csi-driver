IMAGE=quay.io/bertinatto/ebs-csi-driver
VERSION=testing

.PHONY: ebs-csi-driver
ebs-csi-driver:
	mkdir -p bin
	go build -o bin/ebs-csi-driver ./cmd/ebs-csi-driver

.PHONY: test
test:
	go test -v -race github.com/bertinatto/ebs-csi-driver/pkg/...

.PHONY: test-sanity
test-sanity:
	go test -v github.com/bertinatto/ebs-csi-driver/tests/sanity/...

.PHONY: test-e2e
test-e2e:
	go test -v github.com/bertinatto/ebs-csi-driver/tests/e2e/...

.PHONY: image
image: ebs-csi-driver
	cp bin/ebs-csi-driver deploy/docker
	docker build -t $(IMAGE):$(VERSION) deploy/docker
	rm -f deploy/docker/ebs-csi-driver

.PHONY: push
push: image
	docker push $(IMAGE):$(VERSION)
