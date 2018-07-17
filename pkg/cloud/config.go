package cloud

import (
	"io"

	gcfg "gopkg.in/gcfg.v1"
)

// readAWSConfig reads an instance of AWSCloudConfig from config reader.
func readAWSConfig(config io.Reader) (*Config, error) {
	var cfg Config
	var err error
	if config != nil {
		err = gcfg.ReadInto(&cfg, config)
		if err != nil {
			return nil, err
		}
	}
	return &cfg, nil
}

// Config wraps the settings for the AWS cloud provider.
type Config struct {
	Global struct {
		// RoleARN is the IAM role to assume when interaction with AWS APIs.
		RoleARN string
	}
}
