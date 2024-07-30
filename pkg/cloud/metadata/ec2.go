// Copyright 2024 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"k8s.io/klog/v2"
)

const (
	// OutpostArnEndpoint is the ec2 instance metadata endpoint to query to get the outpost arn
	OutpostArnEndpoint string = "outpost-arn"

	// enisEndpoint is the ec2 instance metadata endpoint to query the number of attached ENIs
	EnisEndpoint string = "network/interfaces/macs"

	// blockDevicesEndpoint is the ec2 instance metadata endpoint to query the number of attached block devices
	BlockDevicesEndpoint string = "block-device-mapping"
)

type EC2MetadataClient func() (EC2Metadata, error)

var DefaultEC2MetadataClient = func() (EC2Metadata, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	svc := imds.NewFromConfig(cfg)
	return svc, nil
}

func EC2MetadataInstanceInfo(svc EC2Metadata, regionFromSession string) (*Metadata, error) {
	docOutput, err := svc.GetInstanceIdentityDocument(context.Background(), &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return nil, fmt.Errorf("could not get EC2 instance identity metadata: %w", err)
	}
	doc := docOutput.InstanceIdentityDocument

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

	enisOutput, err := svc.GetMetadata(context.Background(), &imds.GetMetadataInput{Path: EnisEndpoint})
	if err != nil {
		return nil, fmt.Errorf("could not get metadata for ENIs: %w", err)
	}
	enis, err := io.ReadAll(enisOutput.Content)
	if err != nil {
		return nil, fmt.Errorf("could not read ENIs metadata content: %w", err)
	}
	attachedENIs := util.CountMACAddresses(string(enis))
	klog.V(4).InfoS("Number of attached ENIs", "attachedENIs", attachedENIs)

	blockDevMappings := 0
	if !util.IsSBE(doc.Region) {
		mappingsOutput, mappingsOutputErr := svc.GetMetadata(context.Background(), &imds.GetMetadataInput{Path: BlockDevicesEndpoint})
		if mappingsOutputErr != nil {
			return nil, fmt.Errorf("could not get metadata for block device mappings: %w", mappingsOutputErr)
		}
		mappings, mappingsErr := io.ReadAll(mappingsOutput.Content)
		if mappingsErr != nil {
			return nil, fmt.Errorf("could not read block device mappings metadata content: %w", mappingsErr)
		}
		blockDevMappings = strings.Count(string(mappings), "ebs")
	}

	instanceInfo := Metadata{
		InstanceID:             doc.InstanceID,
		InstanceType:           doc.InstanceType,
		Region:                 doc.Region,
		AvailabilityZone:       doc.AvailabilityZone,
		NumAttachedENIs:        attachedENIs,
		NumBlockDeviceMappings: blockDevMappings,
	}

	outpostArnOutput, err := svc.GetMetadata(context.Background(), &imds.GetMetadataInput{Path: OutpostArnEndpoint})
	// "outpust-arn" returns 404 for non-outpost instances. note that the request is made to a link-local address.
	// it's guaranteed to be in the form `arn:<partition>:outposts:<region>:<account>:outpost/<outpost-id>`
	// There's a case to be made here to ignore the error so a failure here wouldn't affect non-outpost calls.
	if err != nil {
		if !strings.Contains(err.Error(), "404") {
			return nil, fmt.Errorf("something went wrong while getting EC2 outpost arn: %w", err)
		}
	} else {
		outpostArnData, err := io.ReadAll(outpostArnOutput.Content)
		if err == nil {
			outpostArn := string(outpostArnData)
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
	}

	return &instanceInfo, nil
}
