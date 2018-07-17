package cloud

import (
	"io"

	gcfg "gopkg.in/gcfg.v1"
)

// readAWSCloudConfig reads an instance of AWSCloudConfig from config reader.
func readAWSCloudConfig(config io.Reader) (*CloudConfig, error) {
	var cfg CloudConfig
	var err error
	if config != nil {
		err = gcfg.ReadInto(&cfg, config)
		if err != nil {
			return nil, err
		}
	}
	return &cfg, nil
}

// CloudConfig wraps the settings for the AWS cloud provider.
type CloudConfig struct {
	Global struct {
		// RoleARN is the IAM role to assume when interaction with AWS APIs.
		RoleARN string
	}
}
