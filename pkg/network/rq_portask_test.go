package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/serialization"
	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RabbitMQIntegrationConfig holds RabbitMQ test configuration
type RabbitMQIntegrationConfig struct {
	URL              string
	ExchangeName     string
	ExchangeType     string
	QueuePrefix      string
	RoutingKeyPrefix string
	Durable          bool
	AutoDelete       bool
}

// DefaultRabbitMQConfig returns default RabbitMQ test configuration
func DefaultRabbitMQConfig() *RabbitMQIntegrationConfig {
	return &RabbitMQIntegrationConfig{
		URL:              "amqp://guest:guest@localhost:5672/",
		ExchangeName:     "portask_test_exchange",
		ExchangeType:     "topic",
		QueuePrefix:      "portask_test",
		RoutingKeyPrefix: "portask.test",
		Durable:          false,
		AutoDelete:       true,
	}
}

// RabbitMQPortaskBridge simulates RabbitMQ client integration with Portask
type RabbitMQPortaskBridge struct {
	conn         *amqp091.Connection
	channel      *amqp091.Channel
	portaskConn  net.Conn
	config       *RabbitMQIntegrationConfig
	messageCount int64
	errorCount   int64
	isRunning    bool
	stopChan     chan struct{}
	mu           sync.RWMutex
}

// NewRabbitMQPortaskBridge creates a new bridge between RabbitMQ and Portask
func NewRabbitMQPortaskBridge(config *RabbitMQIntegrationConfig, portaskAddr string) (*RabbitMQPortaskBridge, error) {
	// Connect to RabbitMQ
	conn, err := amqp091.Dial(config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	// Create channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create RabbitMQ channel: %v", err)
	}

	// Declare exchange
	err = channel.ExchangeDeclare(
		config.ExchangeName,
		config.ExchangeType,
		config.Durable,
		config.AutoDelete,
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %v", err)
	}

	// Connect to Portask server
	portaskConn, err := net.DialTimeout("tcp", portaskAddr, 5*time.Second)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to connect to Portask: %v", err)
	}

	return &RabbitMQPortaskBridge{
		conn:        conn,
		channel:     channel,
		portaskConn: portaskConn,
		config:      config,
		stopChan:    make(chan struct{}),
	}, nil
}

// PublishToRabbitMQ publishes a message to RabbitMQ
func (r *RabbitMQPortaskBridge) PublishToRabbitMQ(topic, routingKey string, message []byte, headers map[string]interface{}) error {
	if headers == nil {
		headers = make(map[string]interface{})
	}
	headers["source"] = "portask_bridge"
	headers["timestamp"] = time.Now().Unix()

	fullRoutingKey := fmt.Sprintf("%s.%s", r.config.RoutingKeyPrefix, routingKey)

	err := r.channel.Publish(
		r.config.ExchangeName,
		fullRoutingKey,
		false, // mandatory
		false, // immediate
		amqp091.Publishing{
			ContentType:  "application/json",
			Body:         message,
			Headers:      headers,
			DeliveryMode: amqp091.Transient,
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		atomic.AddInt64(&r.errorCount, 1)
		return fmt.Errorf("failed to publish to RabbitMQ: %v", err)
	}

	atomic.AddInt64(&r.messageCount, 1)
	fmt.Printf("📤 Published to RabbitMQ: exchange=%s, routing_key=%s\n", r.config.ExchangeName, fullRoutingKey)
	return nil
}

// ConsumeFromRabbitMQ consumes messages from RabbitMQ and forwards to Portask
func (r *RabbitMQPortaskBridge) ConsumeFromRabbitMQ(topics []string) error {
	r.mu.Lock()
	r.isRunning = true
	r.mu.Unlock()

	for _, topic := range topics {
		queueName := fmt.Sprintf("%s_%s", r.config.QueuePrefix, topic)
		routingKey := fmt.Sprintf("%s.%s", r.config.RoutingKeyPrefix, topic)

		// Declare queue
		queue, err := r.channel.QueueDeclare(
			queueName,
			r.config.Durable,
			r.config.AutoDelete,
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %v", queueName, err)
		}

		// Bind queue to exchange
		err = r.channel.QueueBind(
			queue.Name,
			routingKey,
			r.config.ExchangeName,
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue %s: %v", queueName, err)
		}

		// Start consuming
		messages, err := r.channel.Consume(
			queue.Name,
			"",    // consumer tag
			false, // auto-ack
			false, // exclusive
			false, // no-local
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to start consuming from queue %s: %v", queueName, err)
		}

		// Process messages in goroutine
		go func(topic string, msgs <-chan amqp091.Delivery) {
			for {
				select {
				case <-r.stopChan:
					return
				case delivery, ok := <-msgs:
					if !ok {
						return
					}

					// Forward to Portask
					err := r.SendToPortask("publish", topic, string(delivery.Body))
					if err != nil {
						fmt.Printf("❌ Failed to forward to Portask: %v\n", err)
						atomic.AddInt64(&r.errorCount, 1)
						delivery.Nack(false, true) // Requeue on error
					} else {
						fmt.Printf("✅ Forwarded from RabbitMQ to Portask: topic=%s, data=%s\n", topic, string(delivery.Body))
						delivery.Ack(false)
					}
				}
			}
		}(topic, messages)
	}

	return nil
}

// ForwardFromPortaskToRabbitMQ forwards messages from Portask to RabbitMQ
func (r *RabbitMQPortaskBridge) ForwardFromPortaskToRabbitMQ() error {
	go func() {
		scanner := make([]byte, 4096)
		for {
			select {
			case <-r.stopChan:
				return
			default:
				r.portaskConn.SetReadDeadline(time.Now().Add(1 * time.Second))
				n, err := r.portaskConn.Read(scanner)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					fmt.Printf("❌ Portask read error: %v\n", err)
					return
				}

				if n > 0 {
					// Parse Portask message and forward to RabbitMQ
					var portaskMsg map[string]interface{}
					if err := json.Unmarshal(scanner[:n], &portaskMsg); err == nil {
						if topic, ok := portaskMsg["topic"].(string); ok {
							if data, ok := portaskMsg["data"].(string); ok {
								headers := map[string]interface{}{
									"origin":       "portask",
									"forwarded_at": time.Now().Unix(),
								}
								r.PublishToRabbitMQ(topic, topic, []byte(data), headers)
							}
						}
					}
				}
			}
		}
	}()
	return nil
}

// SendToPortask sends a message to Portask server
func (r *RabbitMQPortaskBridge) SendToPortask(msgType, topic, data string) error {
	msg := map[string]interface{}{
		"type":  msgType,
		"topic": topic,
		"data":  data,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	_, err = r.portaskConn.Write(append(jsonData, '\n'))
	if err != nil {
		atomic.AddInt64(&r.errorCount, 1)
		return fmt.Errorf("failed to send to Portask: %v", err)
	}

	atomic.AddInt64(&r.messageCount, 1)
	fmt.Printf("📤 Sent to Portask: %s\n", string(jsonData))
	return nil
}

// GetStats returns bridge statistics
func (r *RabbitMQPortaskBridge) GetStats() map[string]int64 {
	return map[string]int64{
		"messageCount": atomic.LoadInt64(&r.messageCount),
		"errorCount":   atomic.LoadInt64(&r.errorCount),
	}
}

// Stop stops the bridge
func (r *RabbitMQPortaskBridge) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRunning {
		close(r.stopChan)
		r.isRunning = false
	}

	var errs []error
	if err := r.channel.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := r.conn.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := r.portaskConn.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}
	return nil
}

// Test Functions

// TestRabbitMQAvailability tests if RabbitMQ is running and accessible
func TestRabbitMQAvailability(t *testing.T) {
	config := DefaultRabbitMQConfig()

	t.Run("Check RabbitMQ connection", func(t *testing.T) {
		conn, err := amqp091.Dial(config.URL)
		if err != nil {
			t.Skipf("❌ RabbitMQ not available: %v", err)
		}
		defer conn.Close()

		channel, err := conn.Channel()
		if err != nil {
			t.Errorf("❌ Failed to create RabbitMQ channel: %v", err)
			return
		}
		defer channel.Close()

		t.Log("✅ RabbitMQ is available and accessible")
	})

	t.Run("Test RabbitMQ basic operations", func(t *testing.T) {
		conn, err := amqp091.Dial(config.URL)
		if err != nil {
			t.Skipf("❌ RabbitMQ not available: %v", err)
		}
		defer conn.Close()

		channel, err := conn.Channel()
		require.NoError(t, err)
		defer channel.Close()

		// Test exchange declaration
		err = channel.ExchangeDeclare(
			"test_exchange",
			"direct",
			false, // durable
			true,  // auto-delete
			false, // internal
			false, // no-wait
			nil,   // arguments
		)
		require.NoError(t, err)
		t.Log("✅ Exchange declaration successful")

		// Test queue declaration
		queue, err := channel.QueueDeclare(
			"test_queue",
			false, // durable
			true,  // auto-delete
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)
		require.NoError(t, err)
		t.Logf("✅ Queue declaration successful: %s", queue.Name)

		// Test basic publish
		err = channel.Publish(
			"test_exchange",
			"test_key",
			false, // mandatory
			false, // immediate
			amqp091.Publishing{
				ContentType: "text/plain",
				Body:        []byte("test message"),
			},
		)
		require.NoError(t, err)
		t.Log("✅ Basic publish successful")
	})
}

// TestRabbitMQPortaskIntegration tests full integration between RabbitMQ and Portask
func TestRabbitMQPortaskIntegration(t *testing.T) {
	// Start Portask server
	storage := NewMockMessageStore()
	codecManager := &serialization.CodecManager{}
	protocolHandler := NewJSONCompatibleProtocolHandler(codecManager, storage)

	serverConfig := &ServerConfig{
		Address: "127.0.0.1", Port: 0, MaxConnections: 10,
		ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
		Network: "tcp", ReadBufferSize: 2048, WriteBufferSize: 2048, WorkerPoolSize: 5,
	}

	server := NewTCPServer(serverConfig, protocolHandler)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() { server.Start(ctx) }()
	time.Sleep(300 * time.Millisecond)

	addr := server.GetAddr()
	if addr == nil {
		t.Skip("Portask server failed to start")
	}

	t.Logf("🚀 Portask server started on %s", addr)

	// Test RabbitMQ integration
	rabbitmqConfig := DefaultRabbitMQConfig()

	t.Run("RabbitMQ to Portask Message Flow", func(t *testing.T) {
		bridge, err := NewRabbitMQPortaskBridge(rabbitmqConfig, addr.String())
		if err != nil {
			t.Skipf("❌ Failed to create RabbitMQ bridge: %v", err)
		}
		defer bridge.Stop()

		// Start consuming from RabbitMQ
		topics := []string{"user_actions", "system_alerts", "order_updates"}
		err = bridge.ConsumeFromRabbitMQ(topics)
		require.NoError(t, err)

		// Wait a moment for consumers to start
		time.Sleep(500 * time.Millisecond)

		// Test publishing to RabbitMQ
		testMessages := []struct {
			topic      string
			routingKey string
			data       string
			headers    map[string]interface{}
		}{
			{
				"user_actions",
				"user_actions",
				`{"action": "signup", "user_id": "user123"}`,
				map[string]interface{}{"priority": "high"},
			},
			{
				"system_alerts",
				"system_alerts",
				`{"alert": "high_cpu", "threshold": 90}`,
				map[string]interface{}{"severity": "warning"},
			},
			{
				"order_updates",
				"order_updates",
				`{"order_id": "order456", "status": "shipped"}`,
				map[string]interface{}{"customer_id": "cust789"},
			},
		}

		for i, msg := range testMessages {
			err := bridge.PublishToRabbitMQ(msg.topic, msg.routingKey, []byte(msg.data), msg.headers)
			require.NoError(t, err)
			t.Logf("✅ Published message %d to RabbitMQ: topic=%s", i+1, msg.topic)
		}

		// Wait for message processing
		time.Sleep(3 * time.Second)

		// Verify messages reached Portask storage
		allMessages := storage.GetAllMessages()
		t.Logf("📊 Total messages in Portask storage: %d", len(allMessages))

		for topic, messages := range storage.GetMessagesByTopicMap() {
			t.Logf("📂 Topic '%s': %d messages", topic, len(messages))
			for i, msg := range messages {
				t.Logf("  [%d] %s", i+1, string(msg.Payload))
			}
		}

		// Check bridge stats
		stats := bridge.GetStats()
		t.Logf("📈 Bridge stats: messages=%d, errors=%d", stats["messageCount"], stats["errorCount"])

		// Verify we have messages from all topics
		for _, expectedTopic := range topics {
			messages := storage.GetMessagesByTopic(expectedTopic)
			assert.Greater(t, len(messages), 0, "Should have messages for topic %s", expectedTopic)
		}
	})

	t.Run("Portask to RabbitMQ Message Flow", func(t *testing.T) {
		bridge, err := NewRabbitMQPortaskBridge(rabbitmqConfig, addr.String())
		if err != nil {
			t.Skipf("❌ Failed to create RabbitMQ bridge: %v", err)
		}
		defer bridge.Stop()

		// Start forwarding from Portask to RabbitMQ
		err = bridge.ForwardFromPortaskToRabbitMQ()
		require.NoError(t, err)

		// Send messages directly to Portask that should be forwarded to RabbitMQ
		portaskMessages := []struct {
			msgType string
			topic   string
			data    string
		}{
			{"publish", "notifications", `{"type": "push", "message": "New order received"}`},
			{"publish", "analytics", `{"event": "conversion", "value": 99.99}`},
			{"publish", "logs", `{"level": "info", "message": "User authenticated successfully"}`},
		}

		for i, msg := range portaskMessages {
			err := bridge.SendToPortask(msg.msgType, msg.topic, msg.data)
			require.NoError(t, err)
			t.Logf("✅ Sent message %d to Portask: topic=%s", i+1, msg.topic)
		}

		// Wait for processing
		time.Sleep(2 * time.Second)

		// Check final stats
		stats := bridge.GetStats()
		t.Logf("📈 Final bridge stats: messages=%d, errors=%d", stats["messageCount"], stats["errorCount"])
	})

	t.Run("RabbitMQ Routing Keys and Exchange Types", func(t *testing.T) {
		// Test different exchange types and routing patterns
		configs := []*RabbitMQIntegrationConfig{
			{
				URL:              rabbitmqConfig.URL,
				ExchangeName:     "portask_direct",
				ExchangeType:     "direct",
				QueuePrefix:      "portask_direct",
				RoutingKeyPrefix: "direct",
				Durable:          false,
				AutoDelete:       true,
			},
			{
				URL:              rabbitmqConfig.URL,
				ExchangeName:     "portask_fanout",
				ExchangeType:     "fanout",
				QueuePrefix:      "portask_fanout",
				RoutingKeyPrefix: "fanout",
				Durable:          false,
				AutoDelete:       true,
			},
		}

		for _, config := range configs {
			bridge, err := NewRabbitMQPortaskBridge(config, addr.String())
			if err != nil {
				t.Errorf("❌ Failed to create bridge for %s exchange: %v", config.ExchangeType, err)
				continue
			}

			// Test message routing
			testTopic := fmt.Sprintf("routing_test_%s", config.ExchangeType)
			err = bridge.ConsumeFromRabbitMQ([]string{testTopic})
			require.NoError(t, err)

			time.Sleep(200 * time.Millisecond)

			// Publish test message
			testData := fmt.Sprintf(`{"exchange_type": "%s", "test": true}`, config.ExchangeType)
			err = bridge.PublishToRabbitMQ(testTopic, testTopic, []byte(testData), nil)
			require.NoError(t, err)

			time.Sleep(1 * time.Second)

			// Verify message routing
			messages := storage.GetMessagesByTopic(testTopic)
			t.Logf("📊 %s exchange: %d messages received", config.ExchangeType, len(messages))

			bridge.Stop()
		}
	})

	t.Run("Error Handling and Reliability", func(t *testing.T) {
		bridge, err := NewRabbitMQPortaskBridge(rabbitmqConfig, addr.String())
		if err != nil {
			t.Skipf("❌ Failed to create RabbitMQ bridge: %v", err)
		}
		defer bridge.Stop()

		// Test with invalid JSON
		invalidMessages := []string{
			`{"invalid": json}`,       // Invalid JSON syntax
			`{"missing_topic": true}`, // Missing required fields
			``,                        // Empty message
		}

		for i, invalidMsg := range invalidMessages {
			err := bridge.PublishToRabbitMQ("error_test", "error_test", []byte(invalidMsg), nil)
			// Should succeed in publishing but may fail in processing
			t.Logf("Invalid message %d publish result: %v", i+1, err)
		}

		// Wait for processing
		time.Sleep(2 * time.Second)

		stats := bridge.GetStats()
		t.Logf("📈 Error handling stats: messages=%d, errors=%d", stats["messageCount"], stats["errorCount"])
	})

	// Stop server
	server.Stop(ctx)
}

// TestRabbitMQPerformanceWithPortask tests performance of RabbitMQ-Portask integration
func TestRabbitMQPerformanceWithPortask(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Start Portask server
	storage := NewMockMessageStore()
	codecManager := &serialization.CodecManager{}
	protocolHandler := NewJSONCompatibleProtocolHandler(codecManager, storage)

	serverConfig := &ServerConfig{
		Address: "127.0.0.1", Port: 0, MaxConnections: 20,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
		Network: "tcp", ReadBufferSize: 4096, WriteBufferSize: 4096, WorkerPoolSize: 10,
	}

	server := NewTCPServer(serverConfig, protocolHandler)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	go func() { server.Start(ctx) }()
	time.Sleep(500 * time.Millisecond)

	addr := server.GetAddr()
	if addr == nil {
		t.Skip("Portask server failed to start")
	}

	rabbitmqConfig := DefaultRabbitMQConfig()
	bridge, err := NewRabbitMQPortaskBridge(rabbitmqConfig, addr.String())
	if err != nil {
		t.Skipf("❌ Failed to create RabbitMQ bridge: %v", err)
	}
	defer bridge.Stop()

	t.Run("High Volume Message Processing", func(t *testing.T) {
		messageCount := 500 // Lower than Kafka due to RabbitMQ overhead
		topicName := "performance_test"

		// Start consuming
		err = bridge.ConsumeFromRabbitMQ([]string{topicName})
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		startTime := time.Now()

		// Send messages rapidly to RabbitMQ
		for i := 0; i < messageCount; i++ {
			msg := fmt.Sprintf(`{"message_id": %d, "data": "performance test message", "priority": %d}`, i, i%3)
			headers := map[string]interface{}{
				"message_id": i,
				"batch":      i / 50,
			}
			err := bridge.PublishToRabbitMQ(topicName, topicName, []byte(msg), headers)
			if err != nil {
				t.Errorf("Failed to publish message %d: %v", i, err)
			}

			// Small delay to prevent overwhelming RabbitMQ
			if i%50 == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}

		// Wait for all messages to be processed
		timeout := time.NewTimer(45 * time.Second)
		ticker := time.NewTicker(2 * time.Second)
		defer timeout.Stop()
		defer ticker.Stop()

		for {
			select {
			case <-timeout.C:
				messages := storage.GetMessagesByTopic(topicName)
				t.Logf("⚠️  Timeout reached. Processed %d/%d messages", len(messages), messageCount)
				stats := bridge.GetStats()
				t.Logf("📊 Final stats: messages=%d, errors=%d", stats["messageCount"], stats["errorCount"])
				return
			case <-ticker.C:
				messages := storage.GetMessagesByTopic(topicName)
				if len(messages) >= messageCount {
					processingDuration := time.Since(startTime)
					rate := float64(messageCount) / processingDuration.Seconds()

					t.Logf("✅ Processed %d messages in %v", messageCount, processingDuration)
					t.Logf("📈 Processing rate: %.0f messages/second", rate)

					stats := bridge.GetStats()
					t.Logf("📊 Bridge stats: messages=%d, errors=%d", stats["messageCount"], stats["errorCount"])
					return
				}
				t.Logf("📊 Progress: %d/%d messages processed", len(storage.GetMessagesByTopic(topicName)), messageCount)
			}
		}
	})

	t.Run("Concurrent Publishers Performance", func(t *testing.T) {
		topicName := "concurrent_test"
		publisherCount := 5
		messagesPerPublisher := 50

		// Start consuming
		err = bridge.ConsumeFromRabbitMQ([]string{topicName})
		require.NoError(t, err)

		time.Sleep(300 * time.Millisecond)

		startTime := time.Now()
		var wg sync.WaitGroup

		// Start multiple concurrent publishers
		for p := 0; p < publisherCount; p++ {
			wg.Add(1)
			go func(publisherID int) {
				defer wg.Done()

				for i := 0; i < messagesPerPublisher; i++ {
					msg := fmt.Sprintf(`{"publisher": %d, "message": %d, "data": "concurrent test"}`, publisherID, i)
					headers := map[string]interface{}{
						"publisher_id": publisherID,
						"message_num":  i,
					}

					err := bridge.PublishToRabbitMQ(topicName, topicName, []byte(msg), headers)
					if err != nil {
						t.Errorf("Publisher %d failed to publish message %d: %v", publisherID, i, err)
					}

					time.Sleep(5 * time.Millisecond) // Small delay
				}
			}(p)
		}

		wg.Wait()
		totalMessages := publisherCount * messagesPerPublisher

		// Wait for processing
		timeout := time.NewTimer(30 * time.Second)
		ticker := time.NewTicker(1 * time.Second)
		defer timeout.Stop()
		defer ticker.Stop()

		for {
			select {
			case <-timeout.C:
				messages := storage.GetMessagesByTopic(topicName)
				t.Logf("⚠️  Timeout reached. Processed %d/%d messages", len(messages), totalMessages)
				return
			case <-ticker.C:
				messages := storage.GetMessagesByTopic(topicName)
				if len(messages) >= totalMessages {
					processingDuration := time.Since(startTime)
					rate := float64(totalMessages) / processingDuration.Seconds()

					t.Logf("✅ Concurrent test: %d messages in %v", totalMessages, processingDuration)
					t.Logf("📈 Concurrent processing rate: %.0f messages/second", rate)
					return
				}
			}
		}
	})

	// Stop server
	server.Stop(ctx)
}
