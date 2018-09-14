/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"flag"
	"net"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	"github.com/bertinatto/ebs-csi-driver/pkg/driver"
	"github.com/bertinatto/ebs-csi-driver/pkg/util"
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

const (
	endpoint = "tcp://127.0.0.1:10000"
	region   = "us-east-1"
)

var (
	drv       *driver.Driver
	csiClient *CSIClient
	ec2Client *ec2.EC2
	ebs       cloud.Cloud
)

func TestE2E(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS EBS CSI Driver Tests")
}

var _ = BeforeSuite(func() {
	// Run CSI Driver in its own goroutine
	var err error
	ebs, err = cloud.NewCloud()
	Expect(err).To(BeNil(), "Set up Cloud client failed with error")
	drv = driver.NewDriver(ebs, nil, endpoint)
	go drv.Run()

	// Create CSI Controller client
	csiClient, err = newCSIClient()
	Expect(err).To(BeNil(), "Set up Controller Client failed with error")
	Expect(csiClient).NotTo(BeNil())

	// Create EC2 client
	ec2Client = newEC2Client()
	Expect(ec2Client).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	drv.Stop()
})

type CSIClient struct {
	ctrl csi.ControllerClient
	node csi.NodeClient
}

func newCSIClient() (*CSIClient, error) {
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
	return &CSIClient{
		ctrl: csi.NewControllerClient(grpcClient),
		node: csi.NewNodeClient(grpcClient),
	}, nil
}

func newEC2Client() *ec2.EC2 {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))
	return ec2.New(sess)
}
