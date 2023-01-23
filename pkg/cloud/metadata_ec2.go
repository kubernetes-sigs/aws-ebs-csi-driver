package cloud

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"k8s.io/klog/v2"
)

type EC2MetadataClient func() (EC2Metadata, error)

var DefaultEC2MetadataClient = func() (EC2Metadata, error) {
	sess := session.Must(session.NewSession(&aws.Config{}))
	svc := ec2metadata.New(sess)
	return svc, nil
}

func EC2MetadataInstanceInfo(svc EC2Metadata, regionFromSession string) (*Metadata, error) {
	doc, err := svc.GetInstanceIdentityDocument()
	klog.InfoS("Retrieving EC2 instance identity metadata", "regionFromSession", regionFromSession)
	if err != nil {
		return nil, fmt.Errorf("could not get EC2 instance identity metadata: %w", err)
	}

	if len(doc.InstanceID) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 instance ID")
	}

	if len(doc.InstanceType) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 instance type")
	}

	if len(doc.Region) == 0 {
		if len(regionFromSession) != 0 && util.IsSBE(regionFromSession) {
			doc.Region = regionFromSession
		} else {
			return nil, fmt.Errorf("could not get valid EC2 region")
		}
	}

	if len(doc.AvailabilityZone) == 0 {
		if len(regionFromSession) != 0 && util.IsSBE(regionFromSession) {
			doc.AvailabilityZone = regionFromSession
		} else {
			return nil, fmt.Errorf("could not get valid EC2 availability zone")
		}
	}

	enis, err := svc.GetMetadata(enisEndpoint)
	if err != nil {
		return nil, fmt.Errorf("could not get number of attached ENIs: %w", err)
	}
	// the ENIs should not be empty; if (somehow) it is empty, return an error
	if enis == "" {
		return nil, fmt.Errorf("the ENIs should not be empty")
	}

	attachedENIs := strings.Count(enis, "\n") + 1

	//As block device mapping contains 1 volume for the AMI.
	blockDevMappings := 1

	if !util.IsSBE(doc.Region) {
		mappings, mapErr := svc.GetMetadata(blockDevicesEndpoint)
		// The output contains 1 volume for the AMI. Any other block device contributes to the attachment limit
		blockDevMappings = strings.Count(mappings, "\n")
		if mapErr != nil {
			return nil, fmt.Errorf("could not get number of block device mappings: %w", err)
		}
	}

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
		return nil, fmt.Errorf("something went wrong while getting EC2 outpost arn: %w", err)
	} else if err == nil {
		klog.InfoS("Running in an outpost environment with arn", "outpostArn", outpostArn)
		outpostArn = strings.ReplaceAll(outpostArn, "outpost/", "")
		parsedArn, err := arn.Parse(outpostArn)
		if err != nil {
			klog.InfoS("Failed to parse the outpost arn", "outpostArn", outpostArn)
		} else {
			klog.InfoS("Using outpost arn", "parsedArn", parsedArn)
			instanceInfo.OutpostArn = parsedArn
		}
	}

	return &instanceInfo, nil
}
