package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// WebSocketServer provides real-time messaging via WebSocket
type WebSocketServer struct {
	storage  storage.MessageStore
	upgrader websocket.Upgrader

	// Client management
	clients    map[*WebSocketClient]bool
	register   chan *WebSocketClient
	unregister chan *WebSocketClient

	// Message broadcasting
	broadcast chan *types.PortaskMessage

	// Subscriptions
	subscriptions map[string]map[*WebSocketClient]bool // topic -> clients
	subMutex      sync.RWMutex

	// Statistics
	totalConnections  int64
	activeConnections int64
	messagesReceived  int64
	messagesSent      int64
}

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	conn             *websocket.Conn
	send             chan []byte
	server           *WebSocketServer
	id               string
	subscribedTopics map[string]bool
	mu               sync.RWMutex
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type      string                 `json:"type"`
	Topic     string                 `json:"topic,omitempty"`
	Data      interface{}            `json:"data,omitempty"`
	MessageID string                 `json:"message_id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Headers   map[string]interface{} `json:"headers,omitempty"`
}

// WebSocketResponse represents a WebSocket response
type WebSocketResponse struct {
	Type      string      `json:"type"`
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

const (
	// WebSocket message types
	MessageTypeConnect     = "connect"
	MessageTypeSubscribe   = "subscribe"
	MessageTypeUnsubscribe = "unsubscribe"
	MessageTypePublish     = "publish"
	MessageTypeMessage     = "message"
	MessageTypePing        = "ping"
	MessageTypePong        = "pong"
	MessageTypeResponse    = "response"
	MessageTypeError       = "error"

	// WebSocket configuration
	WriteWait      = 10 * time.Second
	PongWait       = 60 * time.Second
	PingPeriod     = (PongWait * 9) / 10
	MaxMessageSize = 512 * 1024 // 512KB
)

// NewWebSocketServer creates a new WebSocket server
func NewWebSocketServer(storage storage.MessageStore) *WebSocketServer {
	return &WebSocketServer{
		storage:       storage,
		clients:       make(map[*WebSocketClient]bool),
		register:      make(chan *WebSocketClient),
		unregister:    make(chan *WebSocketClient),
		broadcast:     make(chan *types.PortaskMessage, 256),
		subscriptions: make(map[string]map[*WebSocketClient]bool),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}
}

// Start starts the WebSocket server hub
func (ws *WebSocketServer) Start(ctx context.Context) {
	go ws.run(ctx)
}

// run handles the main WebSocket server loop
func (ws *WebSocketServer) run(ctx context.Context) {
	ticker := time.NewTicker(PingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case client := <-ws.register:
			ws.clients[client] = true
			ws.activeConnections++
			ws.totalConnections++

			// Send welcome message
			response := WebSocketResponse{
				Type:      MessageTypeConnect,
				Success:   true,
				Data:      map[string]string{"status": "connected", "client_id": client.id},
				Timestamp: time.Now(),
			}
			client.sendResponse(response)

		case client := <-ws.unregister:
			if _, ok := ws.clients[client]; ok {
				delete(ws.clients, client)
				close(client.send)
				ws.activeConnections--

				// Remove from all subscriptions
				ws.unsubscribeAll(client)
			}

		case message := <-ws.broadcast:
			ws.broadcastMessage(message)

		case <-ticker.C:
			// Send ping to all clients
			ws.pingClients()
		}
	}
}

// HandleWebSocket handles WebSocket connections
func (ws *WebSocketServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &WebSocketClient{
		conn:             conn,
		send:             make(chan []byte, 256),
		server:           ws,
		id:               fmt.Sprintf("client_%d", time.Now().UnixNano()),
		subscribedTopics: make(map[string]bool),
	}

	// Register client
	ws.register <- client

	// Start goroutines for client
	go client.writePump()
	go client.readPump()
}

// broadcastMessage broadcasts a message to subscribed clients
func (ws *WebSocketServer) broadcastMessage(message *types.PortaskMessage) {
	topicName := string(message.Topic)

	ws.subMutex.RLock()
	clients, exists := ws.subscriptions[topicName]
	ws.subMutex.RUnlock()

	if !exists {
		return
	}

	wsMessage := WebSocketMessage{
		Type:      MessageTypeMessage,
		Topic:     topicName,
		Data:      message.Payload,
		MessageID: string(message.ID),
		Timestamp: time.Unix(0, message.Timestamp),
		Headers:   message.Headers,
	}

	messageBytes, err := json.Marshal(wsMessage)
	if err != nil {
		log.Printf("Failed to marshal WebSocket message: %v", err)
		return
	}

	for client := range clients {
		select {
		case client.send <- messageBytes:
			ws.messagesSent++
		default:
			// Client buffer is full, remove it
			ws.unregister <- client
		}
	}
}

// pingClients sends ping to all clients
func (ws *WebSocketServer) pingClients() {
	for client := range ws.clients {
		select {
		case client.send <- []byte(`{"type":"ping","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`):
		default:
			ws.unregister <- client
		}
	}
}

// subscribe subscribes a client to a topic
func (ws *WebSocketServer) subscribe(client *WebSocketClient, topic string) {
	ws.subMutex.Lock()
	defer ws.subMutex.Unlock()

	if ws.subscriptions[topic] == nil {
		ws.subscriptions[topic] = make(map[*WebSocketClient]bool)
	}

	ws.subscriptions[topic][client] = true

	client.mu.Lock()
	client.subscribedTopics[topic] = true
	client.mu.Unlock()
}

// unsubscribe unsubscribes a client from a topic
func (ws *WebSocketServer) unsubscribe(client *WebSocketClient, topic string) {
	ws.subMutex.Lock()
	defer ws.subMutex.Unlock()

	if clients, exists := ws.subscriptions[topic]; exists {
		delete(clients, client)
		if len(clients) == 0 {
			delete(ws.subscriptions, topic)
		}
	}

	client.mu.Lock()
	delete(client.subscribedTopics, topic)
	client.mu.Unlock()
}

// unsubscribeAll unsubscribes a client from all topics
func (ws *WebSocketServer) unsubscribeAll(client *WebSocketClient) {
	client.mu.RLock()
	topics := make([]string, 0, len(client.subscribedTopics))
	for topic := range client.subscribedTopics {
		topics = append(topics, topic)
	}
	client.mu.RUnlock()

	for _, topic := range topics {
		ws.unsubscribe(client, topic)
	}
}

// GetStats returns WebSocket server statistics
func (ws *WebSocketServer) GetStats() map[string]interface{} {
	ws.subMutex.RLock()
	topicCount := len(ws.subscriptions)
	ws.subMutex.RUnlock()

	return map[string]interface{}{
		"total_connections":  ws.totalConnections,
		"active_connections": ws.activeConnections,
		"messages_received":  ws.messagesReceived,
		"messages_sent":      ws.messagesSent,
		"subscribed_topics":  topicCount,
		"active_clients":     len(ws.clients),
	}
}

// WebSocketClient methods

// readPump pumps messages from the WebSocket connection to the hub
func (c *WebSocketClient) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.server.messagesReceived++
		c.handleMessage(messageBytes)
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage handles incoming WebSocket messages
func (c *WebSocketClient) handleMessage(messageBytes []byte) {
	var wsMessage WebSocketMessage
	if err := json.Unmarshal(messageBytes, &wsMessage); err != nil {
		c.sendError("Invalid JSON format", err.Error())
		return
	}

	switch wsMessage.Type {
	case MessageTypeSubscribe:
		if wsMessage.Topic == "" {
			c.sendError("Subscribe requires topic", "")
			return
		}

		c.server.subscribe(c, wsMessage.Topic)
		response := WebSocketResponse{
			Type:      MessageTypeResponse,
			Success:   true,
			Data:      map[string]string{"action": "subscribed", "topic": wsMessage.Topic},
			Timestamp: time.Now(),
		}
		c.sendResponse(response)

	case MessageTypeUnsubscribe:
		if wsMessage.Topic == "" {
			c.sendError("Unsubscribe requires topic", "")
			return
		}

		c.server.unsubscribe(c, wsMessage.Topic)
		response := WebSocketResponse{
			Type:      MessageTypeResponse,
			Success:   true,
			Data:      map[string]string{"action": "unsubscribed", "topic": wsMessage.Topic},
			Timestamp: time.Now(),
		}
		c.sendResponse(response)

	case MessageTypePublish:
		c.handlePublish(wsMessage)

	case MessageTypePing:
		response := WebSocketResponse{
			Type:      MessageTypePong,
			Success:   true,
			Timestamp: time.Now(),
		}
		c.sendResponse(response)

	default:
		c.sendError("Unknown message type", wsMessage.Type)
	}
}

// handlePublish handles message publishing via WebSocket
func (c *WebSocketClient) handlePublish(wsMessage WebSocketMessage) {
	if wsMessage.Topic == "" {
		c.sendError("Publish requires topic", "")
		return
	}

	// Convert data to bytes
	var payload []byte
	var err error

	if dataBytes, ok := wsMessage.Data.([]byte); ok {
		payload = dataBytes
	} else {
		payload, err = json.Marshal(wsMessage.Data)
		if err != nil {
			c.sendError("Failed to serialize data", err.Error())
			return
		}
	}

	// Create Portask message
	message := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("ws_%d", time.Now().UnixNano())),
		Topic:     types.TopicName(wsMessage.Topic),
		Payload:   payload,
		Timestamp: time.Now().UnixNano(),
	}

	// Set headers if provided
	if wsMessage.Headers != nil {
		message.Headers = wsMessage.Headers
	}

	// Store message
	ctx := context.Background()
	if err := c.server.storage.Store(ctx, message); err != nil {
		c.sendError("Failed to store message", err.Error())
		return
	}

	// Broadcast to subscribers
	c.server.broadcast <- message

	// Send confirmation
	response := WebSocketResponse{
		Type:    MessageTypeResponse,
		Success: true,
		Data: map[string]interface{}{
			"action":     "published",
			"topic":      message.Topic,
			"message_id": message.ID,
		},
		Timestamp: time.Now(),
	}
	c.sendResponse(response)
}

// sendResponse sends a response to the client
func (c *WebSocketClient) sendResponse(response WebSocketResponse) {
	messageBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Failed to marshal WebSocket response: %v", err)
		return
	}

	select {
	case c.send <- messageBytes:
	default:
		c.server.unregister <- c
	}
}

// sendError sends an error response to the client
func (c *WebSocketClient) sendError(message, details string) {
	response := WebSocketResponse{
		Type:      MessageTypeError,
		Success:   false,
		Error:     message,
		Data:      details,
		Timestamp: time.Now(),
	}
	c.sendResponse(response)
}
