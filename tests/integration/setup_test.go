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

package integration

import (
	"context"
	"flag"
	"fmt"
	"net"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	endpoint = "tcp://127.0.0.1:10000"
)

var (
	drv       *driver.Driver
	csiClient *CSIClient
	ec2Client *ec2.EC2
)

func TestIntegration(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS EBS CSI Driver Integration Tests")
}

var _ = BeforeSuite(func() {
	// Run CSI Driver in its own goroutine
	var err error
	drv, err = driver.NewDriver(driver.WithEndpoint(endpoint))
	Expect(err).To(BeNil())
	go func() {
		err = drv.Run()
		Expect(err).To(BeNil())
	}()

	// Create CSI Controller client
	csiClient, err = newCSIClient()
	Expect(err).To(BeNil(), "Set up Controller Client failed with error")
	Expect(csiClient).NotTo(BeNil())

	// Create EC2 client
	ec2Client, err = newEC2Client()
	Expect(err).To(BeNil(), "Set up EC2 client failed with error")
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
		grpc.WithContextDialer(
			func(context.Context, string) (net.Conn, error) {
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

func newMetadata() (cloud.MetadataService, error) {
	s, err := session.NewSession(&aws.Config{})
	if err != nil {
		return nil, err
	}

	return cloud.NewMetadataService(func() (cloud.EC2Metadata, error) { return ec2metadata.New(s), nil }, func() (kubernetes.Interface, error) { return fake.NewSimpleClientset(), nil })
}

func newEC2Client() (*ec2.EC2, error) {
	m, err := newMetadata()
	if err != nil {
		return nil, err
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(m.GetRegion()),
	}))

	return ec2.New(sess), nil
}

func logf(format string, args ...interface{}) {
	fmt.Fprintln(GinkgoWriter, fmt.Sprintf(format, args...))
}
