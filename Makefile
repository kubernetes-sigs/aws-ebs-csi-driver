IMAGE=quay.io/bertinatto/ebs-csi-driver
VERSION=testing

.PHONY: ebs-csi-driver
ebs-csi-driver:
	mkdir -p bin
	go build -o bin/ebs-csi-driver ./cmd/ebs-csi-driver

.PHONY: test
test:
	go test github.com/bertinatto/ebs-csi-driver/pkg/driver

.PHONY: test-sanity
test-sanity:
	go test -timeout 30s github.com/bertinatto/ebs-csi-driver/tests -run ^TestSanity$

.PHONY: test-e2e
test-e2e:
	go test -v github.com/bertinatto/ebs-csi-driver/tests -run ^TestControllerE2E$

.PHONY: image
image: ebs-csi-driver
	cp bin/ebs-csi-driver deploy/docker
	docker build -t $(IMAGE):$(VERSION) deploy/docker

.PHONY: push
push: image
	docker push $(IMAGE):$(VERSION)
