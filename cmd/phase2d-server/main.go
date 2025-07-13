package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/meftunca/portask/pkg/cluster"
)

func main() {
	var (
		nodeID = flag.String("node-id", "node-1", "Node ID")
		addr   = flag.String("addr", "127.0.0.1", "Listen address")
		port   = flag.Int("port", 9080, "Listen port")
	)
	flag.Parse()

	// Create cluster configuration
	config := &cluster.ClusterConfig{
		NodeID:            *nodeID,
		Address:           *addr,
		Port:              *port,
		SeedNodes:         []string{},
		HeartbeatInterval: time.Second * 5,
		ElectionTimeout:   time.Second * 10,
		ReplicationFactor: 3,
		ReadTimeout:       time.Second * 30,
		WriteTimeout:      time.Second * 30,
	}

	// Create and start cluster node
	node, err := cluster.NewClusterNode(config)
	if err != nil {
		log.Fatalf("Failed to create cluster node: %v", err)
	}

	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start cluster node: %v", err)
	}

	log.Printf("Phase 2D cluster node started on %s:%d", *addr, *port)
	log.Printf("Node ID: %s", *nodeID)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	log.Println("Shutting down cluster node...")

	if err := node.Stop(); err != nil {
		log.Printf("Error stopping cluster node: %v", err)
	}

	log.Println("Cluster node stopped")
}
