package test

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	"github.com/bertinatto/ebs-csi-driver/pkg/driver"
	"github.com/bertinatto/ebs-csi-driver/pkg/util"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	endpoint = "tcp://127.0.0.1:10000"
	nodeID   = "CSINode"
	timeout  = time.Second * 10
	region   = "us-east-1"
	zone     = "us-east-1d"
)

var (
	stdVolCap = []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
	stdVolSize  = int64(1 * 1024 * 1024 * 1024)
	stdCapRange = &csi.CapacityRange{RequiredBytes: stdVolSize}
)

func TestControllerE2E(t *testing.T) {
	go runCSIDriver()

	cc, err := newControllerClient()
	if err != nil {
		t.Fatalf("could not get Controller client: %v", err)
	}

	ec2c := newEC2Client()
	if ec2c == nil {
		t.Fatalf("could not get EC2 client: %v", err)
	}

	testCases := []struct {
		name string
		req  *csi.CreateVolumeRequest
	}{
		{
			name: "success: create and delete volume",
			req: &csi.CreateVolumeRequest{
				Name:               "volume-name-e2e-test",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)

		t.Logf("Creating volume with name %q", tc.req.GetName())
		createResp, err := cc.CreateVolume(context.Background(), tc.req)
		if err != nil {
			t.Fatalf("could not create volume: %v", err)
		}

		volume := createResp.GetVolume()
		if volume == nil {
			t.Fatalf("expected valid volume, got nil")
		}
		t.Logf("Volume %q was created", volume.Id)

		descParams := &ec2.DescribeVolumesInput{
			Filters: []*ec2.Filter{
				&ec2.Filter{
					Name:   aws.String("tag:" + cloud.VolumeNameTagKey),
					Values: []*string{aws.String(tc.req.GetName())},
				},
			},
		}

		t.Logf("Verifying that volume %q was created", volume.Id)
		var volumes []*ec2.Volume
		if waitErr := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
			if volumes, err = describeVolumes(ec2c, descParams); err != nil {
				t.Fatalf("could not get list of volumes: %v", err)
			}
			if len(volumes) != 1 {
				return false, fmt.Errorf("expected 1 volume, got %d", len(volumes))
			}
			return true, nil
		}); waitErr != nil {
			log.Fatal(waitErr)
		}

		if *volumes[0].VolumeId != volume.Id {
			log.Fatalf("expected volume name %q, got %q", volume.Id, *volumes[0].VolumeId)
		}

		t.Logf("Deleting volume %q", volume.Id)
		_, err = cc.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{
			VolumeId: volume.Id,
		})
		if err != nil {
			t.Fatalf("could not delete volume %q: %v", volume.Id, err)
		}

		t.Logf("Verifying that volume %q was deleted", volume.Id)
		if waitErr := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
			if volumes, err = describeVolumes(ec2c, descParams); err != nil {
				t.Fatalf("could not get list of volumes: %v", err)
			}
			if len(volumes) == 1 {
				return false, fmt.Errorf("expected 0 volumes, got %d", len(volumes))
			}
			return true, nil
		}); waitErr != nil {
			log.Fatal(waitErr)
		}

		t.Logf("Deleting volume %q twice", volume.Id)
		_, err = cc.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{
			VolumeId: volume.Id,
		})
		if err != nil {
			t.Fatalf("could not delete volume %q twice: %v", volume.Id, err)
		}

		nonexistentVolume := "vol-0f13f3ff21126cabf"
		if nonexistentVolume != volume.Id {
			t.Logf("Deleting nonexistent volume %q", nonexistentVolume)
			_, err = cc.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{
				VolumeId: nonexistentVolume,
			})
			if err != nil {
				t.Fatalf("could not delete nonexistent volume %q: %v", nonexistentVolume, err)
			}
		} else {
			t.Logf("Skipping nonexistent volume deletion because volume %q does exist", nonexistentVolume)
		}
	}
}

func runCSIDriver() {
	cloud, err := cloud.NewCloud()
	if err != nil {
		log.Fatalln(err)
	}

	metadata := cloud.GetMetadata()

	drv := driver.NewDriver(cloud, endpoint, metadata.InstanceID)
	if err := drv.Run(); err != nil {
		log.Fatalln(err)
	}
}

func newControllerClient() (csi.ControllerClient, error) {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithDialer(
			func(string, time.Duration) (net.Conn, error) {
				scheme, addr, err := util.ParseEndpoint(endpoint)
				if err != nil {
					return nil, err
				}
				return net.Dial(scheme, addr)
			}),
	}
	grpcClient, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return nil, err
	}
	return csi.NewControllerClient(grpcClient), nil
}

func newEC2Client() *ec2.EC2 {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))
	return ec2.New(sess)
}

func describeVolumes(ec2c *ec2.EC2, params *ec2.DescribeVolumesInput) ([]*ec2.Volume, error) {
	var volumes []*ec2.Volume
	var nextToken *string
	for {
		response, err := ec2c.DescribeVolumes(params)
		if err != nil {
			return nil, err
		}
		for _, volume := range response.Volumes {
			volumes = append(volumes, volume)
		}
		nextToken = response.NextToken
		if aws.StringValue(nextToken) == "" {
			break
		}
		params.NextToken = nextToken
	}
	return volumes, nil
}
