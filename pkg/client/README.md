# Portask Go Client

This package provides a Go client for interacting with the Portask server API. It supports submitting tasks, querying status/results, cancelling tasks, and listing recent tasks over HTTP.

## Features
- Submit tasks to Portask server
- Query task status and results
- Cancel running tasks
- List recent tasks
- Optional API token authentication

## Installation

```
go get github.com/meftunca/portask/pkg/client
```

# 📡 Portask Go Client

Official Go client library for Portask - Ultra High Performance Message Queue.

## ✨ Features

- **🚀 High Performance**: Optimized for speed and low latency
- **🔄 Automatic Retries**: Configurable retry logic with exponential backoff
- **📦 Batch Operations**: Publish multiple messages in a single request
- **🔍 Health Checks**: Built-in server health monitoring
- **⚙️ Flexible Configuration**: Customizable timeouts and connection settings
- **📊 Statistics**: Real-time queue and worker statistics
- **🛡️ Error Handling**: Comprehensive error handling and reporting

## 🚀 Quick Start

### Installation

```bash
go get github.com/meftunca/portask/pkg/client
```

### Basic Usage

```go
package main

import (
    "context"
    "log"
    
    "github.com/meftunca/portask/pkg/client"
    "github.com/meftunca/portask/pkg/types"
)

func main() {
    // Create client
    client, err := client.NewPortaskClient("http://localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Publish a message
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
    
    log.Println("✅ Message published successfully!")
}
```

## 📚 Advanced Usage

### Custom Configuration

```go
import "github.com/meftunca/portask/pkg/client"

config := &client.ClientConfig{
    BaseURL:        "http://localhost:8080",
    Timeout:        30 * time.Second,
    MaxRetries:     5,
    RetryDelay:     2 * time.Second,
    EnableLogging:  true,
}

client, err := client.NewPortaskClientWithConfig(config)
```

### Batch Publishing

```go
messages := []*types.PortaskMessage{
    {
        Topic:    "user.events",
        Priority: types.HighPriority,
        Payload:  []byte(`{"user_id": 123, "action": "login"}`),
    },
    {
        Topic:    "user.events", 
        Priority: types.NormalPriority,
        Payload:  []byte(`{"user_id": 456, "action": "logout"}`),
    },
}

err := client.BatchPublish(context.Background(), messages)
```

### Subscribe to Messages

```go
// Subscribe to high-priority messages
messages, err := client.Subscribe(
    context.Background(), 
    "user.events", 
    types.HighPriority,
)

for _, msg := range messages {
    log.Printf("Received: %s", string(msg.Payload))
}
```

### Get System Statistics

```go
stats, err := client.GetStats(context.Background())
if err != nil {
    log.Fatal(err)
}

log.Printf("📊 Messages/sec: %.2f", stats.MessagesPerSec)
log.Printf("📈 Total messages: %d", stats.TotalMessages)
log.Printf("⏱️ Uptime: %s", stats.Uptime)
```

### Health Monitoring

```go
// Check server health
err := client.Health(context.Background())
if err != nil {
    log.Printf("❌ Server unhealthy: %v", err)
} else {
    log.Println("✅ Server is healthy")
}
```

## 🔧 Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `BaseURL` | string | - | Portask server URL |
| `Timeout` | time.Duration | 30s | Request timeout |
| `MaxRetries` | int | 3 | Maximum retry attempts |
| `RetryDelay` | time.Duration | 1s | Delay between retries |
| `EnableLogging` | bool | false | Enable debug logging |

## 📋 API Reference

### Client Methods

#### `NewPortaskClient(baseURL string) (*PortaskClient, error)`
Creates a new client with default configuration.

#### `NewPortaskClientWithConfig(config *ClientConfig) (*PortaskClient, error)`
Creates a new client with custom configuration.

#### `Publish(ctx context.Context, message *types.PortaskMessage) error`
Publishes a single message to the queue.

#### `BatchPublish(ctx context.Context, messages []*types.PortaskMessage) error`
Publishes multiple messages in a single request.

#### `Subscribe(ctx context.Context, topic string, priority types.Priority) ([]types.PortaskMessage, error)`
Retrieves messages from a specific topic and priority.

#### `GetStats(ctx context.Context) (*StatsResponse, error)`
Retrieves system statistics and performance metrics.

#### `Health(ctx context.Context) error`
Performs a health check on the server.

#### `Close() error`
Closes the client (future-proofing for connection pooling).

## 🚨 Error Handling

The client provides detailed error information:

```go
err := client.Publish(ctx, message)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "timeout"):
        log.Println("⏰ Request timed out")
    case strings.Contains(err.Error(), "connection refused"):
        log.Println("🔌 Server unavailable")
    default:
        log.Printf("❌ Publish failed: %v", err)
    }
}
```

## 🧪 Testing

```bash
# Run client tests
go test ./pkg/client/

# Run with race detection
go test -race ./pkg/client/

# Run benchmarks
go test -bench=. ./pkg/client/
```

## 📈 Performance Tips

1. **Use Batch Publishing**: For high throughput, use `BatchPublish` instead of individual `Publish` calls
2. **Configure Timeouts**: Set appropriate timeouts based on your network conditions
3. **Reuse Clients**: Create one client instance and reuse it across your application
4. **Monitor Health**: Regularly check server health to detect issues early

## 🔗 Integration Examples

### With Gin Web Framework

```go
func publishHandler(c *gin.Context, client *client.PortaskClient) {
    var req PublishRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    message := &types.PortaskMessage{
        Topic:    req.Topic,
        Priority: types.NormalPriority,
        Payload:  []byte(req.Payload),
    }
    
    if err := client.Publish(c.Request.Context(), message); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"status": "published"})
}
```

### With gRPC

```go
func (s *server) PublishMessage(ctx context.Context, req *pb.PublishRequest) (*pb.PublishResponse, error) {
    message := &types.PortaskMessage{
        Topic:    req.Topic,
        Priority: types.Priority(req.Priority),
        Payload:  req.Payload,
    }
    
    if err := s.portaskClient.Publish(ctx, message); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to publish: %v", err)
    }
    
    return &pb.PublishResponse{Success: true}, nil
}
```

## 📄 License

MIT License - see [LICENSE](../../LICENSE) file for details.

## API
- `NewClient(baseURL, apiToken string) *Client`
- `Client.SubmitTask(ctx, TaskRequest) (*TaskResponse, error)`
- `Client.GetTaskStatus(ctx, taskID string) (*TaskStatus, error)`
- `Client.CancelTask(ctx, taskID string) error`
- `Client.ListTasks(ctx) (*ListTasksResponse, error)`

## See Also
- See `example_test.go` for a runnable usage example.
