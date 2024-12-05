package config

import "time"

// Config holds the application configuration
type Config struct {
	ServiceName    string
	DiscoveryPort  int

	// Health check settings
	HealthInterval    time.Duration // Interval between health checks
	SuspectTimeout    time.Duration // Time until node is marked suspect
	DeadTimeout       time.Duration // Time until node is marked dead
	NodeRemovalDelay  time.Duration // Time to wait before removing dead node

	// Discovery settings
	DiscoveryRetryInterval time.Duration // Time between discovery attempts
	DiscoveryTimeout       time.Duration // Timeout for each discovery operation
	DiscoveryBufferSize    int          // Size of the discovery entries buffer
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ServiceName:    "sakloud",
		DiscoveryPort:  7946,

		// Health check defaults
		HealthInterval:    time.Second * 5,     // Check every 5 seconds
		SuspectTimeout:    time.Second * 10,    // Mark as suspect after 10 seconds
		DeadTimeout:       time.Second * 20,    // Mark as dead after 20 seconds
		NodeRemovalDelay:  time.Second * 30,    // Remove after 30 seconds

		// Discovery defaults
		DiscoveryRetryInterval: time.Second * 10,
		DiscoveryTimeout:       time.Second * 5,
		DiscoveryBufferSize:    4,
	}
}
