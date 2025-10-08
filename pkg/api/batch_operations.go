package api

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/meftunca/portask/pkg/types"
)

// ==================== UNIFIED BATCH OPERATIONS ====================
// These operations work for BOTH Kafka and AMQP translators

// ==================== BATCH PUBLISH ====================

// BatchPublishRequest for publishing multiple messages
type BatchPublishRequest struct {
	Messages      []PublishMessage `json:"messages" validate:"required,min=1,max=1000"`
	TransactionID string           `json:"transaction_id"` // Optional: for transactional writes
}

// PublishMessage represents a single message to publish
type PublishMessage struct {
	Topic     string                 `json:"topic" validate:"required"`
	Partition int32                  `json:"partition"` // -1 or 0 = auto-assign
	Key       string                 `json:"key"`
	Value     interface{}            `json:"value" validate:"required"`
	Headers   map[string]interface{} `json:"headers"`
	TTL       *int64                 `json:"ttl_ms"` // Time-to-live in milliseconds
}

// BatchPublishResponse after publishing messages
type BatchPublishResponse struct {
	Published int             `json:"published"`
	Failed    int             `json:"failed"`
	Results   []PublishResult `json:"results"`
	Duration  string          `json:"duration"`
}

// PublishResult for a single published message
type PublishResult struct {
	Index     int    `json:"index"`
	MessageID string `json:"message_id"`
	Topic     string `json:"topic"`
	Partition int32  `json:"partition"`
	Offset    int64  `json:"offset"`
	Error     string `json:"error,omitempty"`
	Success   bool   `json:"success"`
}

// ==================== BATCH FETCH ====================

// BatchFetchRequest for fetching multiple messages
type BatchFetchRequest struct {
	Topics         []TopicFetchRequest `json:"topics" validate:"required"`
	MaxMessages    int                 `json:"max_messages"`    // Default: 100, Max: 1000
	MaxWaitMs      int                 `json:"max_wait_ms"`     // Default: 1000
	MinBytes       int                 `json:"min_bytes"`       // Default: 1
	IsolationLevel string              `json:"isolation_level"` // "read_uncommitted" or "read_committed"
}

// TopicFetchRequest for fetching from a topic
type TopicFetchRequest struct {
	Topic      string                  `json:"topic" validate:"required"`
	Partitions []PartitionFetchRequest `json:"partitions"`
}

// PartitionFetchRequest for fetching from a partition
type PartitionFetchRequest struct {
	Partition   int32 `json:"partition" validate:"min=0"`
	FetchOffset int64 `json:"fetch_offset" validate:"min=0"`
	MaxBytes    int   `json:"max_bytes"` // Max bytes to fetch from this partition
}

// BatchFetchResponse after fetching messages
type BatchFetchResponse struct {
	Topics   []TopicFetchResponse `json:"topics"`
	Total    int                  `json:"total_messages"`
	Duration string               `json:"duration"`
}

// TopicFetchResponse for a topic's fetched messages
type TopicFetchResponse struct {
	Topic      string                   `json:"topic"`
	Partitions []PartitionFetchResponse `json:"partitions"`
}

// PartitionFetchResponse for a partition's fetched messages
type PartitionFetchResponse struct {
	Partition     int32            `json:"partition"`
	HighWaterMark int64            `json:"high_water_mark"`
	Messages      []FetchedMessage `json:"messages"`
	Error         string           `json:"error,omitempty"`
}

// FetchedMessage represents a fetched message
type FetchedMessage struct {
	MessageID string                 `json:"message_id"`
	Offset    int64                  `json:"offset"`
	Key       string                 `json:"key"`
	Value     interface{}            `json:"value"`
	Headers   map[string]interface{} `json:"headers"`
	Timestamp string                 `json:"timestamp"`
	Size      int                    `json:"size_bytes"`
}

// ==================== BATCH ACKNOWLEDGMENT ====================

// BatchAckRequest for acknowledging multiple messages
type BatchAckRequest struct {
	MessageIDs []string `json:"message_ids" validate:"required,min=1,max=1000"`
	GroupID    string   `json:"group_id"` // Optional: for consumer groups
}

// BatchAckResponse after acknowledging messages
type BatchAckResponse struct {
	Acknowledged int      `json:"acknowledged"`
	Failed       int      `json:"failed"`
	Errors       []string `json:"errors,omitempty"`
}

// BatchNackRequest for negative acknowledgment
type BatchNackRequest struct {
	MessageIDs []string `json:"message_ids" validate:"required"`
	Reason     string   `json:"reason"`
	Requeue    bool     `json:"requeue"` // Requeue for retry
	GroupID    string   `json:"group_id"`
}

// ==================== API HANDLERS ====================

// handleBatchPublish publishes multiple messages
// POST /api/v1/messages/batch/publish
func (s *FiberServer) handleBatchPublish(c *fiber.Ctx) error {
	startTime := time.Now()

	var req BatchPublishRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate
	if len(req.Messages) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "At least one message is required",
		})
	}

	if len(req.Messages) > 1000 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Max 1000 messages per batch",
		})
	}

	// Convert to Portask messages
	results := make([]PublishResult, len(req.Messages))
	batch := make([]*types.PortaskMessage, 0, len(req.Messages))
	published, failed := 0, 0

	for i, msg := range req.Messages {
		// Validate message
		if msg.Topic == "" {
			results[i] = PublishResult{
				Index:   i,
				Success: false,
				Error:   "Topic is required",
			}
			failed++
			continue
		}

		// Serialize value
		var payload []byte
		var err error

		if byteValue, ok := msg.Value.([]byte); ok {
			payload = byteValue
		} else {
			payload, err = json.Marshal(msg.Value)
			if err != nil {
				results[i] = PublishResult{
					Index:   i,
					Topic:   msg.Topic,
					Success: false,
					Error:   "Failed to serialize value: " + err.Error(),
				}
				failed++
				continue
			}
		}

		// Create Portask message
		messageID := types.MessageID(fmt.Sprintf("msg_%d_%s_%d", time.Now().UnixNano(), msg.Topic, i))

		// Initialize headers and metadata
		headers := make(types.MessageHeaders)
		if msg.Headers != nil {
			headers = msg.Headers
		}

		metadata := make(map[string]string)

		portaskMsg := &types.PortaskMessage{
			ID:        messageID,
			Topic:     types.TopicName(msg.Topic),
			Partition: msg.Partition,
			Key:       msg.Key,
			Payload:   payload,
			Headers:   headers,
			Metadata:  metadata,
			Timestamp: time.Now().UnixNano(),
		}

		// Set TTL if provided
		if msg.TTL != nil {
			portaskMsg.TTL = *msg.TTL
		}

		// Set transaction ID if provided
		if req.TransactionID != "" {
			portaskMsg.Metadata["transaction_id"] = req.TransactionID
		}

		batch = append(batch, portaskMsg)

		// Prepare result (will be updated after storage)
		results[i] = PublishResult{
			Index:     i,
			MessageID: string(messageID),
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    0, // TODO: Get from storage
			Success:   true,
		}
	}

	// Write batch to storage
	if len(batch) > 0 {
		ctx := c.Context()
		messageBatch := &types.MessageBatch{
			Messages:  batch,
			BatchID:   fmt.Sprintf("batch-%d", time.Now().UnixNano()),
			CreatedAt: time.Now().Unix(),
		}

		if err := s.storage.StoreBatch(ctx, messageBatch); err != nil {
			log.Printf("[Native API] Failed to store batch: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   fmt.Sprintf("Failed to store batch: %v", err),
			})
		}

		published = len(batch)
		log.Printf("[Native API] Published batch to storage: %d messages", published)
	}

	duration := time.Since(startTime)

	return c.Status(201).JSON(BatchPublishResponse{
		Published: published,
		Failed:    failed,
		Results:   results,
		Duration:  duration.String(),
	})
}

// handleBatchPublishAsync publishes messages asynchronously (fire-and-forget)
// POST /api/v1/messages/batch/publish/async
func (s *FiberServer) handleBatchPublishAsync(c *fiber.Ctx) error {
	var req BatchPublishRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate
	if len(req.Messages) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "At least one message is required",
		})
	}

	// Process asynchronously
	go func() {
		// TODO: Process batch in background
		log.Printf("[Native API] Async publishing %d messages", len(req.Messages))
	}()

	return c.Status(202).JSON(fiber.Map{
		"success":  true,
		"accepted": len(req.Messages),
		"message":  "Batch accepted for async processing",
	})
}

// handleBatchFetch fetches multiple messages
// POST /api/v1/messages/batch/fetch
func (s *FiberServer) handleBatchFetch(c *fiber.Ctx) error {
	startTime := time.Now()

	var req BatchFetchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Set defaults
	if req.MaxMessages == 0 {
		req.MaxMessages = 100
	}
	if req.MaxWaitMs == 0 {
		req.MaxWaitMs = 1000
	}
	if req.IsolationLevel == "" {
		req.IsolationLevel = "read_uncommitted"
	}

	// Cap max messages
	if req.MaxMessages > 1000 {
		req.MaxMessages = 1000
	}

	topicResponses := make([]TopicFetchResponse, 0, len(req.Topics))
	totalMessages := 0

	for _, topicReq := range req.Topics {
		partitionResponses := make([]PartitionFetchResponse, 0, len(topicReq.Partitions))

		for _, partReq := range topicReq.Partitions {
			// Fetch messages from storage
			ctx := c.Context()
			limit := req.MaxMessages - totalMessages
			if limit > 100 {
				limit = 100 // Cap per partition
			}

			storedMessages, err := s.storage.Fetch(ctx, types.TopicName(topicReq.Topic), partReq.Partition, partReq.FetchOffset, limit)
			if err != nil {
				log.Printf("[Native API] Failed to fetch from topic %s partition %d: %v", topicReq.Topic, partReq.Partition, err)
				// Continue to next partition even if one fails
				continue
			}

			// Convert to API format
			messages := make([]FetchedMessage, 0, len(storedMessages))
			for _, msg := range storedMessages {
				var value interface{}
				if err := json.Unmarshal(msg.Payload, &value); err != nil {
					value = string(msg.Payload) // Fallback to string
				}

				// Convert Metadata map[string]string to map[string]interface{}
				headers := make(map[string]interface{})
				for k, v := range msg.Metadata {
					headers[k] = v
				}

				messages = append(messages, FetchedMessage{
					MessageID: string(msg.ID),
					Offset:    msg.Offset,
					Key:       msg.Key,
					Value:     value,
					Headers:   headers,
					Timestamp: time.Unix(0, msg.Timestamp).Format(time.RFC3339),
					Size:      len(msg.Payload),
				})
			}

			totalMessages += len(messages)

			partitionResponses = append(partitionResponses, PartitionFetchResponse{
				Partition:     partReq.Partition,
				HighWaterMark: partReq.FetchOffset + int64(len(messages)),
				Messages:      messages,
			})
		}

		topicResponses = append(topicResponses, TopicFetchResponse{
			Topic:      topicReq.Topic,
			Partitions: partitionResponses,
		})
	}

	duration := time.Since(startTime)

	log.Printf("[Native API] Fetched %d messages from %d topics", totalMessages, len(req.Topics))

	return c.JSON(BatchFetchResponse{
		Topics:   topicResponses,
		Total:    totalMessages,
		Duration: duration.String(),
	})
}

// handleBatchFetchPoll long-polling fetch (waits until messages available or timeout)
// POST /api/v1/messages/batch/fetch/poll
func (s *FiberServer) handleBatchFetchPoll(c *fiber.Ctx) error {
	// Similar to handleBatchFetch but with long-polling
	// TODO: Implement long-polling logic
	return s.handleBatchFetch(c)
}

// handleBatchAck acknowledges multiple messages
// POST /api/v1/messages/batch/ack
func (s *FiberServer) handleBatchAck(c *fiber.Ctx) error {
	var req BatchAckRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate
	if len(req.MessageIDs) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "At least one message ID is required",
		})
	}

	if len(req.MessageIDs) > 1000 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Max 1000 message IDs per batch",
		})
	}

	// TODO: Acknowledge messages
	acknowledged := len(req.MessageIDs)
	log.Printf("[Native API] Acknowledged %d messages (group: %s)", acknowledged, req.GroupID)

	return c.JSON(BatchAckResponse{
		Acknowledged: acknowledged,
		Failed:       0,
		Errors:       []string{},
	})
}

// handleBatchNack negative acknowledges multiple messages
// POST /api/v1/messages/batch/nack
func (s *FiberServer) handleBatchNack(c *fiber.Ctx) error {
	var req BatchNackRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate
	if len(req.MessageIDs) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "At least one message ID is required",
		})
	}

	// TODO: Nack messages (requeue or send to DLQ)
	log.Printf("[Native API] Nacked %d messages (requeue: %v, reason: %s)", len(req.MessageIDs), req.Requeue, req.Reason)

	return c.JSON(fiber.Map{
		"success":  true,
		"nacked":   len(req.MessageIDs),
		"requeued": req.Requeue,
		"group_id": req.GroupID,
	})
}
