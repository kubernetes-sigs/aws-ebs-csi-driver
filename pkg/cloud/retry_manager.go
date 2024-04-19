package cloud

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
)

const (
	// retryMaxAttempt sets max number of EC2 API call attempts.
	// Set high enough to ensure default sidecar timeout will cancel context long before we stop retrying.
	retryMaxAttempt = 50
)

// retryManager dictates the retry strategies of EC2 API calls.
// Each mutating EC2 API has its own retryer because the AWS SDK throttles on a retryer object level, not by API name.
// While default AWS accounts share request tokens between mutating APIs, users can raise limits for individual APIs.
// Separate retryers ensures that throttling one API doesn't unintentionally throttle others with separate token buckets.
type retryManager struct {
	createVolumeRetryer                            aws.Retryer
	deleteVolumeRetryer                            aws.Retryer
	attachVolumeRetryer                            aws.Retryer
	detachVolumeRetryer                            aws.Retryer
	modifyVolumeRetryer                            aws.Retryer
	createSnapshotRetryer                          aws.Retryer
	deleteSnapshotRetryer                          aws.Retryer
	enableFastSnapshotRestoresRetryer              aws.Retryer
	unbatchableDescribeVolumesModificationsRetryer aws.Retryer
}

func newRetryManager() *retryManager {
	return &retryManager{
		createVolumeRetryer:                            newAdaptiveRetryer(),
		attachVolumeRetryer:                            newAdaptiveRetryer(),
		deleteVolumeRetryer:                            newAdaptiveRetryer(),
		detachVolumeRetryer:                            newAdaptiveRetryer(),
		modifyVolumeRetryer:                            newAdaptiveRetryer(),
		createSnapshotRetryer:                          newAdaptiveRetryer(),
		deleteSnapshotRetryer:                          newAdaptiveRetryer(),
		enableFastSnapshotRestoresRetryer:              newAdaptiveRetryer(),
		unbatchableDescribeVolumesModificationsRetryer: newAdaptiveRetryer(),
	}
}

// newAdaptiveRetryer restricts attempts of API calls that recently hit throttle errors.
func newAdaptiveRetryer() *retry.AdaptiveMode {
	return retry.NewAdaptiveMode(func(ao *retry.AdaptiveModeOptions) {
		ao.StandardOptions = append(ao.StandardOptions, func(so *retry.StandardOptions) {
			so.MaxAttempts = retryMaxAttempt
		})
	})
}
