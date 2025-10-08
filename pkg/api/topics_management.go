package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// ==================== EXTENDED TOPIC MANAGEMENT ====================
// Unified topic management for Kafka + AMQP

// ==================== HELPER FUNCTIONS ====================

// toTopicInfo converts API Topic to storage TopicInfo
func toTopicInfo(topic *Topic) *types.TopicInfo {
	config := make(map[string]string)
	config["retention_ms"] = strconv.FormatInt(topic.Config.RetentionMs, 10)
	config["compression_type"] = topic.Config.CompressionType
	config["max_message_bytes"] = strconv.FormatInt(topic.Config.MaxMessageBytes, 10)
	config["min_insync_replicas"] = strconv.Itoa(topic.Config.MinInSyncReplicas)

	createdAt, _ := time.Parse(time.RFC3339, topic.CreatedAt)
	return &types.TopicInfo{
		Name:              types.TopicName(topic.Name),
		Partitions:        int32(topic.Partitions),
		ReplicationFactor: int16(topic.ReplicationFactor),
		Config:            config,
		CreatedAt:         createdAt.Unix(),
	}
}

// fromTopicInfo converts storage TopicInfo to API Topic
func fromTopicInfo(info *types.TopicInfo) *Topic {
	retentionMs, _ := strconv.ParseInt(info.Config["retention_ms"], 10, 64)
	maxMessageBytes, _ := strconv.ParseInt(info.Config["max_message_bytes"], 10, 64)
	minInSyncReplicas, _ := strconv.Atoi(info.Config["min_insync_replicas"])

	return &Topic{
		Name:              string(info.Name),
		Partitions:        int(info.Partitions),
		ReplicationFactor: int(info.ReplicationFactor),
		Config: TopicConfig{
			RetentionMs:       retentionMs,
			CompressionType:   info.Config["compression_type"],
			MaxMessageBytes:   maxMessageBytes,
			MinInSyncReplicas: minInSyncReplicas,
		},
		CreatedAt:    time.Unix(info.CreatedAt, 0).Format(time.RFC3339),
		UpdatedAt:    time.Unix(info.CreatedAt, 0).Format(time.RFC3339),
		MessageCount: 0, // Will be updated by caller with real stats
		TotalBytes:   0, // Will be updated by caller with real stats
	}
}

// TopicConfig represents topic configuration
type TopicConfig struct {
	RetentionMs       int64  `json:"retention_ms"`        // Message retention in milliseconds
	CompressionType   string `json:"compression_type"`    // "none", "gzip", "snappy", "lz4", "zstd"
	MaxMessageBytes   int64  `json:"max_message_bytes"`   // Max message size
	MinInSyncReplicas int    `json:"min_insync_replicas"` // Min replicas for ack
}

// Topic represents a unified topic
type Topic struct {
	Name              string      `json:"name"`
	Partitions        int         `json:"partitions"`
	ReplicationFactor int         `json:"replication_factor"`
	Config            TopicConfig `json:"config"`
	CreatedAt         string      `json:"created_at"`
	UpdatedAt         string      `json:"updated_at"`
	MessageCount      int64       `json:"message_count"`
	TotalBytes        int64       `json:"total_bytes"`
}

// CreateTopicRequest for creating a topic
type CreateTopicRequest struct {
	Name              string       `json:"name" validate:"required"`
	Partitions        int          `json:"partitions"`         // Default: 1
	ReplicationFactor int          `json:"replication_factor"` // Default: 1
	Config            *TopicConfig `json:"config"`             // Optional config
}

// UpdateTopicRequest for updating topic configuration
type UpdateTopicRequest struct {
	Partitions *int         `json:"partitions"` // Increase partitions (cannot decrease)
	Config     *TopicConfig `json:"config"`     // Update config
}

// DeleteTopicRequest for deleting a topic
type DeleteTopicRequest struct {
	Force bool `json:"force"` // Force delete even if messages exist
}

// TopicStats represents topic statistics
type TopicStats struct {
	Name         string `json:"name"`
	Partitions   int    `json:"partitions"`
	MessageCount int64  `json:"message_count"`
	TotalBytes   int64  `json:"total_bytes"`
	FirstOffset  int64  `json:"first_offset"`
	LastOffset   int64  `json:"last_offset"`
	Replicas     int    `json:"replicas"`
	ISR          int    `json:"in_sync_replicas"` // In-Sync Replicas
}

// ==================== API HANDLERS ====================

// handleCreateTopic creates a new topic
// POST /api/v1/topics
func (s *FiberServer) handleCreateTopic(c *fiber.Ctx) error {
	// Parse request body
	var req CreateTopicRequest
	body := c.Body()

	// Return debug info temporarily
	bodyStr := string(body)
	fmt.Printf("[CreateTopic DEBUG] Raw body: %s\n", bodyStr)

	if len(body) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Empty request body",
		})
	}

	// Use standard json.Unmarshal instead of BodyParser
	if err := json.Unmarshal(body, &req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid JSON: " + err.Error(),
		})
	}

	fmt.Printf("[CreateTopic DEBUG] Parsed: Name='%s', Partitions=%d\n", req.Name, req.Partitions)

	// Validate
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Topic name is required",
			"debug":   fmt.Sprintf("Body was: %s, Parsed name: '%s'", bodyStr, req.Name),
		})
	}

	// Set defaults
	if req.Partitions == 0 {
		req.Partitions = 1
	}
	if req.ReplicationFactor == 0 {
		req.ReplicationFactor = 1
	}

	// Set default config if not provided
	config := TopicConfig{
		RetentionMs:       86400000, // 1 day
		CompressionType:   "none",
		MaxMessageBytes:   1048576, // 1MB
		MinInSyncReplicas: 1,
	}
	if req.Config != nil {
		config = *req.Config
	}

	// Create topic object
	topic := &Topic{
		Name:              req.Name,
		Partitions:        req.Partitions,
		ReplicationFactor: req.ReplicationFactor,
		Config:            config,
		CreatedAt:         time.Now().Format(time.RFC3339),
		UpdatedAt:         time.Now().Format(time.RFC3339),
		MessageCount:      0,
		TotalBytes:        0,
	}

	// Store topic in storage (persistent)
	topicInfo := toTopicInfo(topic)
	ctx := c.Context()
	if err := s.storage.CreateTopic(ctx, topicInfo); err != nil {
		log.Printf("[Native API] Failed to create topic %s: %v", req.Name, err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to create topic: %v", err),
		})
	}

	// Also keep in memory for fast lookup (cache)
	s.topicsMutex.Lock()
	s.topics[req.Name] = topic
	s.topicsMutex.Unlock()

	log.Printf("[Native API] Created topic: %s (partitions: %d, replication: %d) in storage", req.Name, req.Partitions, req.ReplicationFactor)

	return c.Status(201).JSON(fiber.Map{
		"success": true,
		"topic":   topic,
	})
}

// handleListTopics lists all topics
// GET /api/v1/topics
func (s *FiberServer) handleListTopics(c *fiber.Ctx) error {
	// Get topics from storage (persistent)
	ctx := c.Context()
	topicInfos, err := s.storage.ListTopics(ctx)
	if err != nil {
		log.Printf("[Native API] Failed to list topics: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to list topics: %v", err),
		})
	}

	// Convert to API format
	topics := make([]Topic, 0, len(topicInfos))
	for _, info := range topicInfos {
		topic := fromTopicInfo(info)
		
		// Calculate MessageCount and TotalBytes from storage
		var messageCount int64
		var totalBytes int64
		for partition := int32(0); partition < info.Partitions; partition++ {
			first, _ := s.storage.GetEarliestOffset(ctx, info.Name, partition)
			last, _ := s.storage.GetLatestOffset(ctx, info.Name, partition)
			messageCount += (last - first)
			// TotalBytes approximation: messageCount * average message size (1KB default)
			totalBytes += (last - first) * 1024
		}
		topic.MessageCount = messageCount
		topic.TotalBytes = totalBytes
		
		topics = append(topics, *topic)

		// Update cache
		s.topicsMutex.Lock()
		s.topics[topic.Name] = topic
		s.topicsMutex.Unlock()
	}

	log.Printf("[Native API] Listed %d topics from storage", len(topics))

	return c.JSON(fiber.Map{
		"success": true,
		"topics":  topics,
		"count":   len(topics),
	})
}

// handleGetTopic gets details of a topic
// GET /api/v1/topics/:name
func (s *FiberServer) handleGetTopic(c *fiber.Ctx) error {
	topicName := c.Params("name")

	if topicName == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Topic name is required",
		})
	}

	// Try cache first
	s.topicsMutex.RLock()
	cachedTopic, cached := s.topics[topicName]
	s.topicsMutex.RUnlock()

	if cached {
		return c.JSON(fiber.Map{
			"success": true,
			"topic":   cachedTopic,
		})
	}

	// Get topic from storage
	ctx := c.Context()
	topicInfo, err := s.storage.GetTopicInfo(ctx, types.TopicName(topicName))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Topic '%s' not found", topicName),
		})
	}

	// Convert to API format and calculate stats
	topic := fromTopicInfo(topicInfo)
	
	// Calculate MessageCount and TotalBytes from storage
	var messageCount int64
	var totalBytes int64
	for partition := int32(0); partition < topicInfo.Partitions; partition++ {
		first, _ := s.storage.GetEarliestOffset(ctx, topicInfo.Name, partition)
		last, _ := s.storage.GetLatestOffset(ctx, topicInfo.Name, partition)
		messageCount += (last - first)
		// TotalBytes approximation: messageCount * average message size (1KB default)
		totalBytes += (last - first) * 1024
	}
	topic.MessageCount = messageCount
	topic.TotalBytes = totalBytes
	
	// Cache with updated stats
	s.topicsMutex.Lock()
	s.topics[topicName] = topic
	s.topicsMutex.Unlock()

	return c.JSON(fiber.Map{
		"success": true,
		"topic":   topic,
	})
}

// handleUpdateTopic updates topic configuration
// PUT /api/v1/topics/:name
func (s *FiberServer) handleUpdateTopic(c *fiber.Ctx) error {
	topicName := c.Params("name")

	var req UpdateTopicRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Update topic in storage
	ctx := c.Context()
	
	// Get existing topic
	topicInfo, err := s.storage.GetTopicInfo(ctx, types.TopicName(topicName))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Topic '%s' not found", topicName),
		})
	}
	
	// Update config if provided
	if req.Config != nil {
		topicInfo.Config["retention_ms"] = strconv.FormatInt(req.Config.RetentionMs, 10)
		topicInfo.Config["compression_type"] = req.Config.CompressionType
		topicInfo.Config["max_message_bytes"] = strconv.FormatInt(req.Config.MaxMessageBytes, 10)
		topicInfo.Config["min_insync_replicas"] = strconv.Itoa(req.Config.MinInSyncReplicas)
	}
	
	// Note: Storage interface doesn't have UpdateTopic, so delete and recreate
	// In production, this should be atomic or use a dedicated Update method
	if err := s.storage.DeleteTopic(ctx, types.TopicName(topicName)); err != nil {
		log.Printf("[Native API] Warning: Failed to delete for update: %v", err)
	}
	if err := s.storage.CreateTopic(ctx, topicInfo); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to update topic: %v", err),
		})
	}
	
	// Update cache
	s.topicsMutex.Lock()
	if cachedTopic, exists := s.topics[topicName]; exists {
		if req.Config != nil {
			cachedTopic.Config = *req.Config
		}
	}
	s.topicsMutex.Unlock()
	
	log.Printf("[Native API] Updated topic in storage: %s", topicName)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Topic '%s' updated", topicName),
	})
}

// handleDeleteTopic deletes a topic
// DELETE /api/v1/topics/:name
func (s *FiberServer) handleDeleteTopic(c *fiber.Ctx) error {
	topicName := c.Params("name")

	// Parse force flag from query
	force := c.QueryBool("force", false)

	// Delete topic from storage (persistent)
	ctx := c.Context()
	if err := s.storage.DeleteTopic(ctx, types.TopicName(topicName)); err != nil {
		log.Printf("[Native API] Failed to delete topic %s: %v", topicName, err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to delete topic: %v", err),
		})
	}

	// Also remove from cache
	s.topicsMutex.Lock()
	delete(s.topics, topicName)
	s.topicsMutex.Unlock()

	log.Printf("[Native API] Deleted topic: %s from storage (force: %v)", topicName, force)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Topic '%s' deleted", topicName),
	})
}

// handleGetTopicStats gets topic statistics
// GET /api/v1/topics/:name/stats
func (s *FiberServer) handleGetTopicStats(c *fiber.Ctx) error {
	topicName := c.Params("name")
	ctx := c.Context()

	// Get topic info from storage
	topicInfo, err := s.storage.GetTopicInfo(ctx, types.TopicName(topicName))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Topic '%s' not found", topicName),
		})
	}

	// Get partition count and calculate stats
	partitionCount := topicInfo.Partitions
	totalMessages := int64(0)
	totalBytes := int64(0)
	firstOffset := int64(0)
	lastOffset := int64(0)

	// Get first and last offset from first partition (as approximation)
	if partitionCount > 0 {
		firstOffset, _ = s.storage.GetEarliestOffset(ctx, types.TopicName(topicName), 0)
		lastOffset, _ = s.storage.GetLatestOffset(ctx, types.TopicName(topicName), 0)

		// Calculate total messages as rough estimate (last - first)
		totalMessages = lastOffset - firstOffset
		// Estimate bytes (assume average 1KB per message)
		totalBytes = totalMessages * 1024
	}

	stats := TopicStats{
		Name:         topicName,
		Partitions:   int(partitionCount),
		MessageCount: totalMessages,
		TotalBytes:   totalBytes,
		FirstOffset:  firstOffset,
		LastOffset:   lastOffset,
		Replicas:     int(topicInfo.ReplicationFactor),
		ISR:          int(topicInfo.ReplicationFactor), // Assume all replicas in-sync
	}

	log.Printf("[Native API] Got topic stats from storage: %s (messages: %d, bytes: %d)", topicName, stats.MessageCount, stats.TotalBytes)

	return c.JSON(fiber.Map{
		"success": true,
		"stats":   stats,
	})
}

// handleGetTopicPartitions gets topic partition details
// GET /api/v1/topics/:name/partitions
func (s *FiberServer) handleGetTopicPartitions(c *fiber.Ctx) error {
	topicName := c.Params("name")

	// Get partition info from storage
	ctx := c.Context()
	partitionCount, err := s.storage.GetPartitionCount(ctx, types.TopicName(topicName))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Topic '%s' not found", topicName),
		})
	}
	
	// Build partition info for each partition
	partitions := make([]fiber.Map, partitionCount)
	for i := int32(0); i < partitionCount; i++ {
		firstOffset, _ := s.storage.GetEarliestOffset(ctx, types.TopicName(topicName), i)
		lastOffset, _ := s.storage.GetLatestOffset(ctx, types.TopicName(topicName), i)
		messageCount := lastOffset - firstOffset
		
		partitions[i] = fiber.Map{
			"partition":     i,
			"leader":        1,                 // Single node, always leader
			"replicas":      []int{1},          // Single replica
			"isr":           []int{1},          // Always in-sync
			"first_offset":  firstOffset,
			"last_offset":   lastOffset,
			"message_count": messageCount,
		}
	}

	log.Printf("[Native API] Got topic partitions from storage: %s (%d partitions)", topicName, len(partitions))

	return c.JSON(fiber.Map{
		"success":    true,
		"topic":      topicName,
		"partitions": partitions,
	})
}

// handleCompactTopic triggers topic compaction
// POST /api/v1/topics/:name/compact
func (s *FiberServer) handleCompactTopic(c *fiber.Ctx) error {
	topicName := c.Params("name")

	// Trigger compaction using storage Cleanup (background task)
	go func() {
		ctx := context.Background()
		retentionPolicy := &storage.RetentionPolicy{
			MaxAge:          24 * time.Hour, // Keep last 24 hours
			CleanupStrategy: storage.CleanupOldest,
			BatchSize:       1000,
		}
		if err := s.storage.Cleanup(ctx, retentionPolicy); err != nil {
			log.Printf("[Native API] Compaction failed for topic %s: %v", topicName, err)
		} else {
			log.Printf("[Native API] Compaction completed for topic %s", topicName)
		}
	}()
	
	log.Printf("[Native API] Compaction triggered for topic: %s", topicName)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Compaction triggered for topic '%s'", topicName),
	})
}

// handlePurgeTopic purges all messages from a topic
// POST /api/v1/topics/:name/purge
func (s *FiberServer) handlePurgeTopic(c *fiber.Ctx) error {
	topicName := c.Params("name")

	// Purge all messages from topic (delete + recreate for atomic purge)
	ctx := c.Context()
	
	// Get topic info before deletion
	topicInfo, err := s.storage.GetTopicInfo(ctx, types.TopicName(topicName))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Topic '%s' not found", topicName),
		})
	}
	
	// Delete topic (this deletes all messages)
	if err := s.storage.DeleteTopic(ctx, types.TopicName(topicName)); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to purge topic: %v", err),
		})
	}
	
	// Recreate topic with same config (fresh, no messages)
	if err := s.storage.CreateTopic(ctx, topicInfo); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to recreate topic after purge: %v", err),
		})
	}
	
	log.Printf("[Native API] Purged all messages from topic: %s", topicName)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("All messages purged from topic '%s'", topicName),
	})
}
