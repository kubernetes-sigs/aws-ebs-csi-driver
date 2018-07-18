all: ebs-csi-driver

ebs-csi-driver:
	mkdir -p bin
	go build -o bin/ebs-csi-driver ./cmd/ebs-csi-driver

test:
	go test github.com/bertinatto/ebs-csi-driver/pkg/driver

test-sanity:
	go test -timeout 30s github.com/bertinatto/ebs-csi-driver/tests -run ^TestSanity$
