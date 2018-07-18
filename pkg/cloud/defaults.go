package cloud

const (
	// DefaultVolumeSize represents the default volume size. TODO: what should be the default size?
	DefaultVolumeSize int64 = 1 * 1024 * 1024 * 1024

	// VolumeNameTagKey is the key value that refers to the volume's name.
	VolumeNameTagKey string = "VolumeName"

	// VolumeTypeIO1 represents a provisioned IOPS SSD type of volume.
	VolumeTypeIO1 = "io1"

	// VolumeTypeGP2 represents a general purpose SSD type of volume.
	VolumeTypeGP2 = "gp2"

	// VolumeTypeSC1 represents a cold HDD (sc1) type of volume.
	VolumeTypeSC1 = "sc1"

	// VolumeTypeST1 represents a throughput-optimized HDD type of volume.
	VolumeTypeST1 = "st1"

	// MinTotalIOPS represents the minimum Input Output per second.
	MinTotalIOPS int64 = 100

	// MaxTotalIOPS represents the maximum Input Output per second.
	MaxTotalIOPS int64 = 20000

	// DefaultVolumeType specifies which storage to use for newly created Volumes.
	DefaultVolumeType = "gp2"
)
