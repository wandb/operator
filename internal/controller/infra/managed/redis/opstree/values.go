package opstree

// installType represents the Redis installation type
type installType string

const (
	// InstallTypeNone represents no Redis installation
	InstallTypeNone installType = "none"
	// InstallTypeSentinel represents Redis Sentinel installation
	InstallTypeSentinel installType = "sentinel"
	// InstallTypeStandalone represents Redis Standalone installation
	InstallTypeStandalone installType = "standalone"
)
