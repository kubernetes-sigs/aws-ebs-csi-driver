IMAGE=quay.io/bertinatto/ebs-csi-driver
VERSION=testing

.PHONY: aws-ebs-csi-driver
aws-ebs-csi-driver:
	mkdir -p bin
	go build -o bin/aws-ebs-csi-driver ./cmd/aws-ebs-csi-driver

.PHONY: test
test:
	go test -v -race ./pkg/...

.PHONY: test-sanity
test-sanity:
	go test -v ./tests/sanity/...

.PHONY: test-e2e
test-e2e:
	go test -v ./tests/e2e/...

.PHONY: image
image: aws-ebs-csi-driver
	cp bin/aws-ebs-csi-driver deploy/docker
	docker build -t $(IMAGE):$(VERSION) deploy/docker
	rm -f deploy/docker/aws-ebs-csi-driver

.PHONY: push
push: image
	docker push $(IMAGE):$(VERSION)
