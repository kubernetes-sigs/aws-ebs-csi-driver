package test

import (
	"os"
	"testing"

	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	"github.com/bertinatto/ebs-csi-driver/pkg/driver"
	sanity "github.com/kubernetes-csi/csi-test/pkg/sanity"
)

func TestSanity(t *testing.T) {
	const (
		mountPath = "/tmp/csi/mount"
		stagePath = "/tmp/csi/stage"
		socket    = "/tmp/csi.sock"
		endpoint  = "unix://" + socket
	)

	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		t.Fatalf("could not remove socket file %s: %v", socket, err)
	}

	awsDriver := driver.NewDriver(cloud.NewFakeCloudProvider(), endpoint, "")
	defer awsDriver.Stop()

	go func() {
		if err := awsDriver.Run(); err != nil {
			t.Fatalf("could not run CSI driver: %v", err)
		}
	}()

	config := &sanity.Config{
		Address:     endpoint,
		TargetPath:  mountPath,
		StagingPath: stagePath,
	}

	sanity.Test(t, config)
}
