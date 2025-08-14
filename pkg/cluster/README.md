# cluster/ (Cluster, Health, Node Management)

This package provides cluster management, health checking, and node operations for distributed Portask deployments. It enables node discovery, state management, and cluster health monitoring.

## Main Files
- `cluster.go`: Cluster management and orchestration
- `health.go`: Health checking logic
- `node.go`: Node operations and representation

## Key Types, Interfaces, and Methods

### Types & Structs
- `Node`, `NodeInfo`, `ClusterInfo`, `ClusterNode`, `ClusterConfig`, `NodeState` — Cluster and node types
- `HealthChecker` — Health check logic

### Main Methods (selected)
- `NewNode(id, addr string) *Node` — Create a new node
- `NewHealthChecker() *HealthChecker`, `CheckNode(addr string) (bool, error)` — Health checking
- `NewClusterNode(config *ClusterConfig) (*ClusterNode, error)`, `Start()`, `Stop()` — Cluster node lifecycle
- `setState(state NodeState)`, `startHTTPServer()`, `handleHealth()`, `handleNodeInfo()`, `handleClusterInfo()` — Cluster node internals
- `heartbeatLoop()`, `sendHeartbeats()`, `RunClusterServer()` — Cluster orchestration

## Usage Example
```go
import "github.com/meftunca/portask/pkg/cluster"

node := cluster.NewNode("node1", "127.0.0.1:9000")
```

## TODO / Missing Features
- [ ] Automatic node discovery
- [ ] HA/replication support
- [ ] Cluster health dashboard
- [ ] Dynamic scaling

---

For details, see the code and comments in each file.
