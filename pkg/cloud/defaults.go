package cloud

const (
	// DefaultVolumeSize represents the default volume size. TODO: what should be the default size?
	DefaultVolumeSize int64 = 4000000000

	// VolumeNameTagKey is the key value that refers to the volume's name.
	VolumeNameTagKey string = "VolumeName"

	// AWS volume types:
	// VolumeTypeIO1 represents a provisioned IOPS SSD.
	VolumeTypeIO1 = "io1"

	// VolumeTypeGP2 represents a general purpose SSD.
	VolumeTypeGP2 = "gp2"

	// VolumeTypeSC1 represents a cold HDD (sc1).
	VolumeTypeSC1 = "sc1"

	// VolumeTypeST1 represents a throughput-optimized HDD.
	VolumeTypeST1 = "st1"

	// MinTotalIOPS represents the minimum Input Output per second.
	MinTotalIOPS = 100

	// MaxTotalIOPS represents the maximum Input Output per second.
	MaxTotalIOPS = 20000

	// DefaultVolumeType specifies which storage to use for newly created Volumes.
	DefaultVolumeType = "gp2"
)
