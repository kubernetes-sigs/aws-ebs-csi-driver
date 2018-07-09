package cloud

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"
)

func newEc2Filter(name string, values ...string) *ec2.Filter {
	filter := &ec2.Filter{
		Name: aws.String(name),
	}
	for _, value := range values {
		filter.Values = append(filter.Values, aws.String(value))
	}
	return filter
}

// getCandidateZonesForDynamicVolume retrieves  a list of all the zones in which nodes are running
// It currently involves querying all instances
func (c *awsEBS) getCandidateZonesForDynamicVolume() (sets.String, error) {
	// We don't currently cache this; it is currently used only in volume
	// creation which is expected to be a comparatively rare occurrence.

	// TODO: Caching / expose v1.Nodes to the cloud provider?
	// TODO: We could also query for subnets, I think

	filters := []*ec2.Filter{newEc2Filter("instance-state-name", "running")}

	instances, err := c.describeInstances(filters)
	if err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances returned")
	}

	zones := sets.NewString()

	for _, instance := range instances {
		// We skip over master nodes, if the installation tool labels them with one of the well-known master labels
		// This avoids creating a volume in a zone where only the master is running - e.g. #34583
		// This is a short-term workaround until the scheduler takes care of zone selection
		master := false
		for _, tag := range instance.Tags {
			tagKey := aws.StringValue(tag.Key)
			if awsTagNameMasterRoles.Has(tagKey) {
				master = true
			}
		}

		if master {
			glog.V(4).Infof("Ignoring master instance %q in zone discovery", aws.StringValue(instance.InstanceId))
			continue
		}

		if instance.Placement != nil {
			zone := aws.StringValue(instance.Placement.AvailabilityZone)
			zones.Insert(zone)
		}
	}

	glog.V(2).Infof("Found instances in zones %s", zones)
	return zones, nil
}

// TODO: Move to instanceCache
func (c *awsEBS) describeInstances(filters []*ec2.Filter) ([]*ec2.Instance, error) {
	filters = c.tagging.addFilters(filters)
	request := &ec2.DescribeInstancesInput{
		Filters: filters,
	}

	//response, err := c.ec2.DescribeInstances(request)
	//if err != nil {
	//return nil, err
	//}

	// Instances are paged
	results := []*ec2.Instance{}
	var nextToken *string
	for {
		response, err := c.ec2.DescribeInstances(request)
		if err != nil {
			return nil, fmt.Errorf("error listing AWS instances: %q", err)
		}

		for _, reservation := range response.Reservations {
			results = append(results, reservation.Instances...)
		}

		nextToken = response.NextToken
		if aws.StringValue(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}

	var matches []*ec2.Instance
	for _, instance := range results {
		if c.tagging.hasClusterTag(instance.Tags) {
			matches = append(matches, instance)
		}
	}

	return matches, nil
}
