# api/ (HTTP, WebSocket, Fiber API Endpoints)

This package provides HTTP, WebSocket, and Fiber-based API endpoints for the Portask system. It enables client communication, message publishing, topic management, and system monitoring via multiple protocols.

## Main Files
- `fiber.go`: API implementation using the Fiber framework
- `http.go`: Standard HTTP API server
- `websocket.go`: WebSocket server and client endpoints

## Key Types, Interfaces, and Methods

### Structs & Types
- `FiberServer`, `FiberConfig` — Fiber-based API server and configuration
- `HTTPServer`, `APIResponse`, `HealthResponse`, `MetricsResponse`, `MessageRequest`, `FetchRequest` — HTTP server and API types
- `WebSocketServer`, `WebSocketClient`, `WebSocketMessage`, `WebSocketResponse` — WebSocket server/client and message types

### Main Methods (selected)
- `DefaultFiberConfig() FiberConfig`, `NewFiberServer(config, networkServer, storage) *FiberServer`, `Start`, `Stop`, `setupMiddlewares`, `setupRoutes`, `handleHealthFiber`, `handleMetricsFiber`, `handleStatusFiber`, `handleMessagesFiber`, `handlePublishFiber`, `handleFetchFiber`, `handleTopicsFiber`, `handleTopicOperationsFiber`, `handleConnectionsFiber`, `handleShutdownFiber`, `handleConfigFiber`, `handleConfigUpdateFiber`, `handleWebSocketConnection` (FiberServer)
- `NewHTTPServer(addr, networkServer, storage) *HTTPServer`, `Start`, `Stop`, `setupRoutes`, `handleWebSocket`, `withMetrics`, `handleHealth`, `handleMetrics`, `handleStatus`, `handleMessages`, `handlePublish`, `handleFetch`, `handleMessagesList`, `handleTopics`, `handleTopicOperations`, `handleConnections`, `handleShutdown`, `handleConfig`, `sendResponse`, `sendError`, `updateMetrics`, `getCoreMetrics`, `getNetworkMetrics`, `getStorageMetrics`, `getAPIMetrics`, `getSystemMetrics`, `getWebSocketMetrics` (HTTPServer)
- `NewWebSocketServer(storage) *WebSocketServer`, `Start`, `HandleWebSocket`, `GetStats` (WebSocketServer)
- `subscribe`, `unsubscribe`, `unsubscribeAll`, `broadcastMessage`, `pingClients` (WebSocketServer)
- `readPump`, `writePump`, `handleMessage`, `handlePublish`, `sendResponse`, `sendError` (WebSocketClient)

## Usage Example
```go
import "github.com/meftunca/portask/pkg/api"

server := api.NewHTTPServer(":8080", nil, nil)
err := server.Start()
```

## TODO / Missing Features
- [ ] Complete API documentation (OpenAPI/Swagger)
- [ ] Advanced error handling and rate limiting
- [ ] Authentication middleware integration
- [ ] WebSocket authentication and topic ACLs
- [ ] API versioning

---

For details, see the code and comments in each file.
