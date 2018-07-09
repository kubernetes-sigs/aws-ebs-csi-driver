package cloud

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/util/sets"
)

const (
	// createTag* is configuration of exponential backoff for CreateTag call. We
	// retry mainly because if we create an object, we cannot tag it until it is
	// "fully created" (eventual consistency). Starting with 1 second, doubling
	// it every step and taking 9 steps results in 255 second total waiting
	// time.
	createTagInitialDelay = 1 * time.Second
	createTagFactor       = 2.0
	createTagSteps        = 9
)

type VolumeID string

// MapToAWSVolumeID extracts the awsVolumeID from the VolumeID
func (name VolumeID) MapToAWSVolumeID() (awsVolumeID, error) {
	// name looks like aws://availability-zone/awsVolumeId

	// The original idea of the URL-style name was to put the AZ into the
	// host, so we could find the AZ immediately from the name without
	// querying the API.  But it turns out we don't actually need it for
	// multi-AZ clusters, as we put the AZ into the labels on the PV instead.
	// However, if in future we want to support multi-AZ cluster
	// volume-awareness without using PersistentVolumes, we likely will
	// want the AZ in the host.

	s := string(name)

	if !strings.HasPrefix(s, "aws://") {
		// Assume a bare aws volume id (vol-1234...)
		// Build a URL with an empty host (AZ)
		s = "aws://" + "" + "/" + s
	}
	url, err := url.Parse(s)
	if err != nil {
		// TODO: Maybe we should pass a URL into the Volume functions
		return "", fmt.Errorf("Invalid disk name (%s): %v", name, err)
	}
	if url.Scheme != "aws" {
		return "", fmt.Errorf("Invalid scheme for AWS volume (%s)", name)
	}

	awsID := url.Path
	awsID = strings.Trim(awsID, "/")

	// We sanity check the resulting volume; the two known formats are
	// vol-12345678 and vol-12345678abcdef01
	//if !awsVolumeRegMatch.MatchString(awsID) {
	//return "", fmt.Errorf("Invalid format for AWS volume (%s)", name)
	//}

	return awsVolumeID(awsID), nil
}

// AWS volume types
const (
	// Provisioned IOPS SSD
	VolumeTypeIO1 = "io1"
	// General Purpose SSD
	VolumeTypeGP2 = "gp2"
	// Cold HDD (sc1)
	VolumeTypeSC1 = "sc1"
	// Throughput Optimized HDD
	VolumeTypeST1 = "st1"
)

// AWS provisioning limits.
// Source: http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html
const (
	MinTotalIOPS = 100
	MaxTotalIOPS = 20000
)

// DefaultVolumeType specifies which storage to use for newly created Volumes
// TODO: Remove when user/admin can configure volume types and thus we don't
// need hardcoded defaults.
const DefaultVolumeType = "gp2"

// awsVolumeID represents the ID of the volume in the AWS API, e.g. vol-12345678
// The "traditional" format is "vol-12345678"
// A new longer format is also being introduced: "vol-12345678abcdef01"
// We should not assume anything about the length or format, though it seems
// reasonable to assume that volumes will continue to start with "vol-".
type awsVolumeID string

func (i awsVolumeID) awsString() *string {
	return aws.String(string(i))
}

// awsTagNameMasterRoles is a set of well-known AWS tag names that indicate the instance is a master
// The major consequence is that it is then not considered for AWS zone discovery for dynamic volume creation.
var awsTagNameMasterRoles = sets.NewString("kubernetes.io/role/master", "k8s.io/role/master")
