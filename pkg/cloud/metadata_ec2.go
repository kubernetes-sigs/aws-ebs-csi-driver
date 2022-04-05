package cloud

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"k8s.io/klog"
)

type EC2MetadataClient func() (EC2Metadata, error)

var DefaultEC2MetadataClient = func() (EC2Metadata, error) {
	sess := session.Must(session.NewSession(&aws.Config{}))
	svc := ec2metadata.New(sess)
	return svc, nil
}

func EC2MetadataInstanceInfo(svc EC2Metadata) (*Metadata, error) {
	doc, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		return nil, fmt.Errorf("could not get EC2 instance identity metadata: %v", err)
	}

	if len(doc.InstanceID) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 instance ID")
	}

	if len(doc.InstanceType) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 instance type")
	}

	if len(doc.Region) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 region")
	}

	if len(doc.AvailabilityZone) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 availability zone")
	}

	enis, err := svc.GetMetadata(enisEndpoint)
	// the ENIs should not be empty; if (somehow) it is empty, return an error
	if enis == "" || err != nil {
		return nil, fmt.Errorf("could not get number of attached ENIs")
	}

	attachedENIs := strings.Count(enis, "\n") + 1

	mappings, err := svc.GetMetadata(blockDevicesEndpoint)
	if err != nil {
		return nil, fmt.Errorf("could not get number of block device mappings")
	}
	// The output contains 1 volume for the AMI. Any other block device contributes to the attachment limit
	blockDevMappings := strings.Count(mappings, "\n")

	instanceInfo := Metadata{
		InstanceID:             doc.InstanceID,
		InstanceType:           doc.InstanceType,
		Region:                 doc.Region,
		AvailabilityZone:       doc.AvailabilityZone,
		NumAttachedENIs:        attachedENIs,
		NumBlockDeviceMappings: blockDevMappings,
	}

	outpostArn, err := svc.GetMetadata(outpostArnEndpoint)
	// "outpust-arn" returns 404 for non-outpost instances. note that the request is made to a link-local address.
	// it's guaranteed to be in the form `arn:<partition>:outposts:<region>:<account>:outpost/<outpost-id>`
	// There's a case to be made here to ignore the error so a failure here wouldn't affect non-outpost calls.
	if err != nil && !strings.Contains(err.Error(), "404") {
		return nil, fmt.Errorf("something went wrong while getting EC2 outpost arn: %s", err.Error())
	} else if err == nil {
		klog.Infof("Running in an outpost environment with arn: %s", outpostArn)
		outpostArn = strings.ReplaceAll(outpostArn, "outpost/", "")
		parsedArn, err := arn.Parse(outpostArn)
		if err != nil {
			klog.Warningf("Failed to parse the outpost arn: %s", outpostArn)
		} else {
			klog.Infof("Using outpost arn: %v", parsedArn)
			instanceInfo.OutpostArn = parsedArn
		}
	}

	return &instanceInfo, nil
}
