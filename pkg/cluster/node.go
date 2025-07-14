package cluster

import "time"

// Node represents a node in the cluster.
type Node struct {
	ID       string
	Addr     string
	IsLeader bool
	LastSeen time.Time
	Health   string // "healthy", "unhealthy", "unknown"
}

// NewNode creates a new cluster node.
func NewNode(id, addr string) *Node {
	return &Node{
		ID:       id,
		Addr:     addr,
		IsLeader: false,
		LastSeen: time.Now(),
		Health:   "unknown",
	}
}
