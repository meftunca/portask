package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// NodeState represents the state of a cluster node
type NodeState int

const (
	NodeStateUnknown NodeState = iota
	NodeStateStarting
	NodeStateHealthy
	NodeStateSuspected
	NodeStateFailed
	NodeStateLeaving
	NodeStateLeft
)

func (s NodeState) String() string {
	switch s {
	case NodeStateStarting:
		return "starting"
	case NodeStateHealthy:
		return "healthy"
	case NodeStateSuspected:
		return "suspected"
	case NodeStateFailed:
		return "failed"
	case NodeStateLeaving:
		return "leaving"
	case NodeStateLeft:
		return "left"
	default:
		return "unknown"
	}
}

// ClusterConfig defines the configuration for cluster operations
type ClusterConfig struct {
	NodeID            string        `json:"node_id"`
	Address           string        `json:"address"`
	Port              int           `json:"port"`
	SeedNodes         []string      `json:"seed_nodes"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	ElectionTimeout   time.Duration `json:"election_timeout"`
	ReplicationFactor int           `json:"replication_factor"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
}

// NodeInfo represents information about a cluster node
type NodeInfo struct {
	ID       string            `json:"id"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	Status   NodeState         `json:"status"`
	Tags     map[string]string `json:"tags"`
	JoinTime time.Time         `json:"join_time"`
	LastSeen time.Time         `json:"last_seen"`
}

// ClusterInfo represents information about the entire cluster
type ClusterInfo struct {
	ClusterID    string               `json:"cluster_id"`
	Nodes        map[string]*NodeInfo `json:"nodes"`
	LeaderID     string               `json:"leader_id"`
	NodeCount    int                  `json:"node_count"`
	HealthyNodes int                  `json:"healthy_nodes"`
	Version      int64                `json:"version"`
	Term         int64                `json:"term"`
	LastUpdated  time.Time            `json:"last_updated"`
}

// ClusterNode represents a node in the cluster
type ClusterNode struct {
	config      *ClusterConfig
	info        *NodeInfo
	clusterInfo *ClusterInfo
	httpServer  *http.Server
	ctx         context.Context
	cancel      context.CancelFunc
	stopCh      chan struct{}
	running     int32
	mu          sync.RWMutex
}

// NewClusterNode creates a new cluster node
func NewClusterNode(config *ClusterConfig) (*ClusterNode, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())

	node := &ClusterNode{
		config: config,
		info: &NodeInfo{
			ID:       config.NodeID,
			Address:  config.Address,
			Port:     config.Port,
			Status:   NodeStateStarting,
			Tags:     make(map[string]string),
			JoinTime: time.Now(),
			LastSeen: time.Now(),
		},
		clusterInfo: &ClusterInfo{
			ClusterID:    "cluster-1",
			Nodes:        make(map[string]*NodeInfo),
			LeaderID:     "",
			NodeCount:    0,
			HealthyNodes: 0,
			Version:      1,
			Term:         0,
			LastUpdated:  time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
		stopCh: make(chan struct{}),
	}

	node.clusterInfo.Nodes[node.info.ID] = node.info
	node.clusterInfo.NodeCount = 1

	return node, nil
}

// Start starts the cluster node
func (cn *ClusterNode) Start() error {
	if !atomic.CompareAndSwapInt32(&cn.running, 0, 1) {
		return fmt.Errorf("node already running")
	}

	if err := cn.startHTTPServer(); err != nil {
		atomic.StoreInt32(&cn.running, 0)
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	cn.setState(NodeStateHealthy)
	go cn.heartbeatLoop()

	return nil
}

// Stop stops the cluster node
func (cn *ClusterNode) Stop() error {
	if !atomic.CompareAndSwapInt32(&cn.running, 1, 0) {
		return nil
	}

	cn.setState(NodeStateLeaving)
	close(cn.stopCh)
	cn.cancel()

	if cn.httpServer != nil {
		if err := cn.httpServer.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("failed to stop HTTP server: %w", err)
		}
	}

	cn.setState(NodeStateLeft)
	return nil
}

func (cn *ClusterNode) setState(state NodeState) {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	cn.info.Status = state
	cn.info.LastSeen = time.Now()
	cn.clusterInfo.LastUpdated = time.Now()
}

func (cn *ClusterNode) startHTTPServer() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", cn.handleHealth)
	mux.HandleFunc("/node/info", cn.handleNodeInfo)
	mux.HandleFunc("/cluster/info", cn.handleClusterInfo)

	cn.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cn.config.Address, cn.config.Port),
		Handler:      mux,
		ReadTimeout:  cn.config.ReadTimeout,
		WriteTimeout: cn.config.WriteTimeout,
	}

	go func() {
		if err := cn.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

func (cn *ClusterNode) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := map[string]interface{}{
		"status":    cn.info.Status.String(),
		"timestamp": time.Now(),
		"uptime":    time.Since(cn.info.JoinTime),
		"node_id":   cn.info.ID,
	}

	json.NewEncoder(w).Encode(health)
}

func (cn *ClusterNode) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cn.mu.RLock()
	info := *cn.info
	cn.mu.RUnlock()
	json.NewEncoder(w).Encode(info)
}

func (cn *ClusterNode) handleClusterInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cn.mu.RLock()
	info := *cn.clusterInfo
	cn.mu.RUnlock()
	json.NewEncoder(w).Encode(info)
}

func (cn *ClusterNode) heartbeatLoop() {
	ticker := time.NewTicker(cn.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cn.sendHeartbeats()
		case <-cn.stopCh:
			return
		case <-cn.ctx.Done():
			return
		}
	}
}

func (cn *ClusterNode) sendHeartbeats() {
	log.Printf("Node %s sending heartbeats", cn.info.ID)
}

// RunClusterServer runs a standalone cluster server
func RunClusterServer() {
	config := &ClusterConfig{
		NodeID:            "phase2d-node-1",
		Address:           "127.0.0.1",
		Port:              9080,
		SeedNodes:         []string{},
		HeartbeatInterval: time.Second * 5,
		ElectionTimeout:   time.Second * 10,
		ReplicationFactor: 3,
		ReadTimeout:       time.Second * 30,
		WriteTimeout:      time.Second * 30,
	}

	node, err := NewClusterNode(config)
	if err != nil {
		log.Fatalf("Failed to create cluster node: %v", err)
	}

	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start cluster node: %v", err)
	}

	log.Printf("🚀 Phase 2D Cluster Node Started")
	log.Printf("📍 Address: %s:%d", config.Address, config.Port)
	log.Printf("🆔 Node ID: %s", config.NodeID)
	log.Printf("💓 Heartbeat Interval: %v", config.HeartbeatInterval)
	log.Printf("🗳️  Election Timeout: %v", config.ElectionTimeout)
	log.Printf("🔄 Replication Factor: %d", config.ReplicationFactor)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	log.Println("🛑 Shutting down cluster node...")

	if err := node.Stop(); err != nil {
		log.Printf("❌ Error stopping cluster node: %v", err)
	}

	log.Println("✅ Cluster node stopped successfully")
}
