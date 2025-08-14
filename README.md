# 🚀 Portask v1.0 - Ultra High Performance Message Queue

[![Go](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Performance](https://img.shields.io/badge/Performance-2M+_msg/sec-red.svg)](docs/performance.md)
[![Production](https://img.shields.io/badge/Status-Production_Ready-brightgreen.svg)](https://github.com/meftunca/portask)

**Portask** is an ultra-high performance, production-ready message queue system designed to outperform Redis Queue (RQ) and compete with Apache Kafka. Built with Go's concurrency primitives and lock-free algorithms.

## 🏆 Performance Highlights

- **🚀 2M+ messages/second** throughput
- **⚡ Sub-microsecond** message processing latency  
- **💯 100% reliability** - zero message loss
- **🔄 Lock-free** MPMC queue implementation
- **⚙️ Event-driven** workers (0% CPU when idle)
- **📈 Linear scalability** with CPU cores

## 🆚 Competitive Advantages

| Feature | Portask v1.0 | Redis Queue (RQ) | Apache Kafka |
|---------|--------------|------------------|--------------|
| **Throughput** | 2M+ msg/sec | 400K-1M msg/sec | 1-3M msg/sec |
| **Latency** | <1μs | 1-10ms | 1-5ms |
| **Memory Usage** | Ultra-low | High | Medium |
| **Setup Complexity** | Simple | Medium | Complex |
| **Zero Message Loss** | ✅ | ✅ | ✅ |
| **Multi-Priority** | ✅ | ❌ | ❌ |
| **Admin UI** | ✅ | ❌ | ✅ |

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/meftunca/portask.git
cd portask

# Build production binary
go build -ldflags="-s -w" -o build/portask ./cmd/server

# Start the server
./build/portask
```

### Basic Configuration

```yaml
# configs/config.yaml
server:
  host: "localhost"
  port: 8080
  
message_bus:
  high_priority_queue_size: 262144    # 256K
  normal_priority_queue_size: 4194304 # 4M  
  low_priority_queue_size: 131072     # 128K
  
worker_pool:
  worker_count: 32        # 4x CPU cores
  batch_size: 2000        # Ultra batches
  batch_timeout: "500ns"  # Ultra-responsive
```

## 📚 Client Usage

### Go Client

```go
package main

import (
    "context"
    "log"
    
    "github.com/meftunca/portask/pkg/client"
    "github.com/meftunca/portask/pkg/types"
)

func main() {
    // Connect to Portask server
    client, err := client.NewPortaskClient("localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Publish a high-priority message
    message := &types.PortaskMessage{
        ID:       "msg-001",
        Topic:    "user.events",
        Priority: types.HighPriority,
        Payload:  []byte(`{"user_id": 123, "action": "login"}`),
    }
    
    err = client.Publish(context.Background(), message)
    if err != nil {
        log.Printf("Failed to publish: %v", err)
        return
    }
    
    log.Println("Message published successfully!")
}
```

### HTTP REST API

#### Publish Message
```bash
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "id": "msg-001",
    "topic": "user.events", 
    "priority": "high",
    "payload": "{\"user_id\": 123, \"action\": \"login\"}"
  }'
```

#### Subscribe to Topic
```bash
curl -X GET "http://localhost:8080/api/v1/subscribe?topic=user.events&priority=high"
```

#### Get Queue Statistics
```bash
curl -X GET http://localhost:8080/api/v1/stats
```

### WebSocket Real-time

```javascript
// Connect to Portask WebSocket
const ws = new WebSocket('ws://localhost:8080/ws');

// Subscribe to topic
ws.send(JSON.stringify({
    action: 'subscribe',
    topic: 'user.events',
    priority: 'high'
}));

// Receive messages
ws.onmessage = function(event) {
    const message = JSON.parse(event.data);
    console.log('Received:', message);
};

// Publish message
ws.send(JSON.stringify({
    action: 'publish',
    message: {
        id: 'msg-002',
        topic: 'user.events',
        priority: 'normal',
        payload: JSON.stringify({user_id: 456, action: 'logout'})
    }
}));
```

## 🌐 Language Clients

### Python Client

```python
import requests
import json

class PortaskClient:
    def __init__(self, base_url="http://localhost:8080"):
        self.base_url = base_url
        
    def publish(self, topic, payload, priority="normal", message_id=None):
        message = {
            "topic": topic,
            "priority": priority,
            "payload": json.dumps(payload) if isinstance(payload, dict) else payload
        }
        if message_id:
            message["id"] = message_id
            
        response = requests.post(
            f"{self.base_url}/api/v1/messages",
            json=message
        )
        return response.json()
        
    def subscribe(self, topic, priority="normal"):
        response = requests.get(
            f"{self.base_url}/api/v1/subscribe",
            params={"topic": topic, "priority": priority}
        )
        return response.json()

# Usage
client = PortaskClient()
client.publish("user.events", {"user_id": 123, "action": "login"}, "high")
```

### Node.js Client

```javascript
const axios = require('axios');

class PortaskClient {
    constructor(baseURL = 'http://localhost:8080') {
        this.client = axios.create({ baseURL });
    }
    
    async publish(topic, payload, priority = 'normal', id = null) {
        const message = {
            topic,
            priority,
            payload: typeof payload === 'object' ? JSON.stringify(payload) : payload
        };
        if (id) message.id = id;
        
        const response = await this.client.post('/api/v1/messages', message);
        return response.data;
    }
    
    async subscribe(topic, priority = 'normal') {
        const response = await this.client.get('/api/v1/subscribe', {
            params: { topic, priority }
        });
        return response.data;
    }
    
    async getStats() {
        const response = await this.client.get('/api/v1/stats');
        return response.data;
    }
}

// Usage
const client = new PortaskClient();
await client.publish('user.events', {user_id: 123, action: 'login'}, 'high');
```

## 📊 Monitoring & Admin UI

Access the web-based admin interface at: `http://localhost:8080/admin`

### Features:
- **📈 Real-time Performance Metrics**
- **📋 Queue Status & Statistics** 
- **👥 Worker Pool Monitoring**
- **🔍 Message Tracing & Debugging**
- **⚙️ Dynamic Configuration**
- **🚨 Alerts & Notifications**

### API Monitoring

```bash
# Get detailed statistics
curl http://localhost:8080/api/v1/stats | jq

# Get worker pool status  
curl http://localhost:8080/api/v1/workers | jq

# Get queue metrics
curl http://localhost:8080/api/v1/queues | jq
```

## 🧪 Performance Testing

### Benchmark Tool

```bash
# Run performance benchmark
./build/portask-benchmark

# Expected Results: 2M+ messages/sec with 100% reliability
```

### Ultra Performance Results
```
🏆 ULTRA PERFORMANCE RESULTS:
════════════════════════════════════════
⏱️  Duration: 15.00 seconds
📤 Messages Published: 31,052,832
📊 Messages Processed: 31,052,832  
🚀 Publish Rate: 2,069,784 msg/sec
⚡ Process Rate: 2,069,784 msg/sec
💾 Throughput: 1,319.78 MB/sec
👥 Active Workers: 96/96
⚙️  Processing Efficiency: 100.0%
🏆 ACHIEVEMENT: 2M+ messages/sec! ULTRA CHAMPION!
```

## 🔧 Advanced Configuration

### Ultra Performance Mode

```yaml
# configs/ultra-config.yaml
message_bus:
  high_priority_queue_size: 524288     # 512K
  normal_priority_queue_size: 8388608  # 8M
  low_priority_queue_size: 262144      # 256K
  
worker_pool:
  worker_count: 64          # 8x CPU cores
  batch_size: 4000          # Mega batches  
  batch_timeout: "100ns"    # Ultra-ultra-fast
  enable_profiling: false
  
performance:
  enable_simd: true         # SIMD optimization
  enable_zero_copy: true    # Zero-copy operations
  memory_pool_size: 1000000 # 1M pre-allocated objects
```

### Load Balancing

```yaml
# configs/cluster-config.yaml
cluster:
  mode: "load_balancer"
  nodes:
    - "portask-1:8080"
    - "portask-2:8080" 
    - "portask-3:8080"
  strategy: "round_robin"    # round_robin, least_connections, hash
  
health_check:
  interval: "30s"
  timeout: "5s"
  retries: 3
```

## 🚀 Production Deployment

### Docker Deployment

```dockerfile
# Dockerfile
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY build/portask .
COPY configs/ configs/
CMD ["./portask"]
```

```bash
# Build and run
docker build -t portask:v1.0 .
docker run -p 8080:8080 -v $(pwd)/configs:/root/configs portask:v1.0
```

### Kubernetes Deployment

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: portask
spec:
  replicas: 3
  selector:
    matchLabels:
      app: portask
  template:
    metadata:
      labels:
        app: portask
    spec:
      containers:
      - name: portask
        image: portask:v1.0
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "512Mi"
            cpu: "1000m"
          limits:
            memory: "2Gi" 
            cpu: "4000m"
```

## 📈 Performance Tuning

### OS Level Optimizations

```bash
# Increase file descriptor limits
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# Optimize TCP settings
echo "net.core.rmem_max = 134217728" >> /etc/sysctl.conf
echo "net.core.wmem_max = 134217728" >> /etc/sysctl.conf
echo "net.ipv4.tcp_rmem = 4096 65536 134217728" >> /etc/sysctl.conf

sysctl -p
```

### Go Runtime Tuning

```bash
# Set optimal Go runtime parameters
export GOGC=100
export GOMAXPROCS=32
export GOMEMLIMIT=8GiB

./build/portask
```

## 🛠️ Development

### Building from Source

```bash
# Development build
go build -o build/portask-dev ./cmd/server

# Production build with optimizations
go build -ldflags="-s -w" -o build/portask ./cmd/server

# Cross-platform builds
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/portask-linux ./cmd/server
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o build/portask.exe ./cmd/server
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Benchmark tests
go test -bench=. ./pkg/queue/

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 📋 API Reference

### REST API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/messages` | Publish message |
| GET | `/api/v1/subscribe` | Subscribe to topic |
| GET | `/api/v1/stats` | Get system statistics |
| GET | `/api/v1/queues` | Get queue metrics |
| GET | `/api/v1/workers` | Get worker status |
| GET | `/api/v1/health` | Health check |

### Message Format

```json
{
  "id": "unique-message-id",
  "topic": "topic.name",
  "priority": "high|normal|low",
  "payload": "message content",
  "timestamp": "2025-08-14T09:30:07Z",
  "retry_count": 0,
  "ttl": 3600
}
```

## 🔧 Troubleshooting

### Common Issues

**High Memory Usage**
```bash
# Check memory statistics
curl http://localhost:8080/api/v1/stats | jq '.memory'

# Tune garbage collector
export GOGC=50
```

**CPU Usage Spikes**
```bash
# Enable profiling
curl http://localhost:8080/debug/pprof/profile > cpu.prof
go tool pprof cpu.prof
```

**Connection Issues**
```bash
# Check port binding
netstat -tlnp | grep :8080

# Verify firewall
sudo ufw allow 8080
```

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/meftunca/portask/issues)
- **Discussions**: [GitHub Discussions](https://github.com/meftunca/portask/discussions)
- **Email**: support@portask.dev

---

**🚀 Portask v1.0 - Where Performance Meets Reliability!**
