package cloud

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

const metadataURL = "http://169.254.169.254/latest/meta-data/%s"

type Metadata struct {
	InstanceID       string
	Region           string
	AvailabilityZone string
}

func NewMetadata() (*Metadata, error) {
	instanceID, err := fetchMetadata("instance-id")
	if err != nil {
		return nil, fmt.Errorf("could not fetchMetadata instance ID: %v", err)
	}

	availabilityZone, err := fetchMetadata("placement/availability-zone")
	if err != nil {
		return nil, fmt.Errorf("could not fetchMetadata availability zone: %v", err)
	}

	region, err := azToRegion(availabilityZone)
	if err != nil {
		return nil, fmt.Errorf("could not parse region: %v", err)
	}

	return &Metadata{instanceID, region, availabilityZone}, nil
}

func fetchMetadata(path string) (string, error) {
	url := fmt.Sprintf(metadataURL, path)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("could not read response body: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpect response code: %d", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("could not read response body: %v", err)
	}

	return string(data), nil
}

func azToRegion(az string) (string, error) {
	if len(az) < 2 {
		return "", fmt.Errorf("invalid availability zone")
	}
	region := az[:len(az)-1]
	return region, nil
}
