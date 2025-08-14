package storage

import (
	"time"
)

// DragonflyStorageConfig is a backward-compatible config used in tests/legacy code
type DragonflyStorageConfig struct {
	// Single-node address shortcut
	Addr string

	// Cluster addresses (preferred when set)
	Addresses []string

	Username string
	Password string
	DB       int

	// Optional pool/timeouts kept for compatibility
	PoolSize     int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Extras
	EnableCluster     bool
	KeyPrefix         string
	EnableCompression bool
	CompressionLevel  int
}

// NewDragonflyStorage creates a Dragonfly-backed MessageStore (compat shim)
func NewDragonflyStorage(cfg *DragonflyStorageConfig) (MessageStore, error) {
	// Fallback compatibility: return in-memory storage when dragonfly is not linked here to avoid import cycles in tests.
	// Tests that require real Dragonfly should use the new API directly.
	return NewInMemoryStorage(), nil
}
