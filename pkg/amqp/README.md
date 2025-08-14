# amqp/ (AMQP Protocol Handler and Emulation)

This package provides AMQP protocol compatibility and emulation for Portask, including basic and enhanced AMQP server implementations and test coverage.

## Main Files
- `amqp_go_test.go`: Tests for AMQP protocol
- `basic_server.go`: Basic AMQP server implementation
- `server.go`: Main AMQP server logic

## Key Types, Interfaces, and Methods

### Types & Structs
- `BasicAMQPServer`, `EnhancedAMQPServer` — AMQP server implementations
- `Connection`, `Exchange`, `Queue`, `TLSConfig` — AMQP protocol types
- `MessageStore` — Interface for message storage

### Main Methods (selected)
- `NewBasicAMQPServer(addr string) *BasicAMQPServer`, `Start()`, `Stop()`, `handleConnection()` (BasicAMQPServer)
- `NewEnhancedAMQPServer(addr, store) *EnhancedAMQPServer`, `Start()`, `Stop()`, `handleConnection()`, `handleAMQPFrameWithChannel()`, `handleMethodFrameWithChannel()`, `handleQueueDeclare()`, `handleBasicPublish()`, `handleBasicConsume()`, `handleBasicAck()`, `handleBasicNack()` (EnhancedAMQPServer)

## Usage Example
```go
import "github.com/meftunca/portask/pkg/amqp"

server := amqp.NewBasicAMQPServer(":5672")
err := server.Start()
```

## TODO / Missing Features
- [ ] AMQP 1.0/2.0 support
- [ ] Advanced emulation and error scenarios
- [ ] Performance benchmarking

---

For details, see the code and comments in each file.
