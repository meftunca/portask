package api

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/meftunca/portask/pkg/types"
)

// ==================== UNIFIED NATIVE TYPES ====================
// These types work for BOTH Kafka and AMQP translators

// NativeConsumerGroup = ConsumerGroup (type alias for unified API)
type NativeConsumerGroup = ConsumerGroup

// NativeGroupMember = ConsumerGroupMember (type alias)
type NativeGroupMember = ConsumerGroupMember

// NativePartitionAssignment represents partition assignment
type NativePartitionAssignment struct {
	Topic      string  `json:"topic"`
	Partitions []int32 `json:"partitions"`
}

// ==================== REQUEST/RESPONSE TYPES ====================

// CreateGroupRequest for creating a consumer group
type CreateGroupRequest struct {
	Name         string   `json:"name" validate:"required"`
	Protocol     string   `json:"protocol"`      // Default: "range"
	ProtocolType string   `json:"protocol_type"` // Default: "consumer"
	Topics       []string `json:"topics" validate:"required"`
}

// JoinGroupRequest for joining a consumer group
type JoinGroupRequest struct {
	MemberID       string `json:"member_id"` // Empty on first join
	ClientID       string `json:"client_id" validate:"required"`
	ClientHost     string `json:"client_host"`        // Optional
	SessionTimeout int32  `json:"session_timeout_ms"` // Default: 10000
}

// JoinGroupResponse after joining a group
type JoinGroupResponse struct {
	MemberID   string                      `json:"member_id"`
	Generation int32                       `json:"generation"`
	Leader     bool                        `json:"is_leader"`
	Members    []NativeGroupMember         `json:"members,omitempty"` // Only for leader
	Assignment []NativePartitionAssignment `json:"assignment"`
}

// LeaveGroupRequest for leaving a group
type LeaveGroupRequest struct {
	MemberID string `json:"member_id" validate:"required"`
}

// HeartbeatRequest for sending heartbeat
type HeartbeatRequest struct {
	MemberID   string `json:"member_id" validate:"required"`
	Generation int32  `json:"generation" validate:"required"`
}

// CommitOffsetRequest for committing an offset
type CommitOffsetRequest struct {
	Topic     string `json:"topic" validate:"required"`
	Partition int32  `json:"partition" validate:"min=0"`
	Offset    int64  `json:"offset" validate:"min=0"`
	Metadata  string `json:"metadata"`
}

// CommitOffsetsRequest for batch commit
type CommitOffsetsRequest struct {
	Offsets []CommitOffsetRequest `json:"offsets" validate:"required,min=1"`
}

// FetchOffsetsResponse for fetching committed offsets
type FetchOffsetsResponse struct {
	Offsets map[string]map[int32]OffsetMetadata `json:"offsets"` // topic -> partition -> offset+metadata
}

// OffsetMetadata contains offset and metadata
type OffsetMetadata struct {
	Offset   int64  `json:"offset"`
	Metadata string `json:"metadata"`
}

// ResetOffsetsRequest for resetting offsets
type ResetOffsetsRequest struct {
	Topics   []string `json:"topics"`                       // Empty = all topics
	Position string   `json:"position" validate:"required"` // "earliest" or "latest"
}

// GroupLagInfo contains lag information for a partition
type GroupLagInfo struct {
	Topic         string `json:"topic"`
	Partition     int32  `json:"partition"`
	CurrentOffset int64  `json:"current_offset"`
	LogEndOffset  int64  `json:"log_end_offset"`
	Lag           int64  `json:"lag"`
}

// GroupLagResponse contains lag information for a group
type GroupLagResponse struct {
	GroupID  string         `json:"group_id"`
	TotalLag int64          `json:"total_lag"`
	Lags     []GroupLagInfo `json:"partitions"`
}

// ==================== API HANDLERS ====================

// handleCreateConsumerGroup creates a new consumer group
// POST /api/v1/consumer-groups
func (s *FiberServer) handleCreateConsumerGroup(c *fiber.Ctx) error {
	var req CreateGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Group name is required",
		})
	}

	if len(req.Topics) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "At least one topic is required",
		})
	}

	// Set defaults
	if req.Protocol == "" {
		req.Protocol = "range"
	}
	if req.ProtocolType == "" {
		req.ProtocolType = "consumer"
	}

	// Create consumer group (storage-backed)
	group := NativeConsumerGroup{
		ID:            req.Name,
		Name:          req.Name,
		State:         "Empty",
		Protocol:      req.Protocol,
		ProtocolType:  req.ProtocolType,
		Leader:        "",
		Generation:    0,
		Members:       []NativeGroupMember{},
		Subscriptions: req.Topics,
		CreatedAt:     time.Now().Format(time.RFC3339),
		UpdatedAt:     time.Now().Format(time.RFC3339),
	}
	
	// Store in memory (in-memory coordinator)
	s.groupsMutex.Lock()
	s.consumerGroups[req.Name] = &group
	s.groupsMutex.Unlock()

	log.Printf("[Native API] Created consumer group: %s (topics: %v)", req.Name, req.Topics)

	return c.Status(201).JSON(fiber.Map{
		"success": true,
		"group":   group,
	})
}

// handleListConsumerGroups lists all consumer groups
// GET /api/v1/consumer-groups
func (s *FiberServer) handleListConsumerGroups(c *fiber.Ctx) error {
	// Get real groups from in-memory coordinator
	s.groupsMutex.RLock()
	groups := make([]NativeConsumerGroup, 0, len(s.consumerGroups))
	for _, group := range s.consumerGroups {
		groups = append(groups, *group)
	}
	s.groupsMutex.RUnlock()

	log.Printf("[Native API] Listed consumer groups: %d groups", len(groups))

	return c.JSON(fiber.Map{
		"success": true,
		"groups":  groups,
		"count":   len(groups),
	})
}

// handleGetConsumerGroup gets details of a consumer group
// GET /api/v1/consumer-groups/:id
func (s *FiberServer) handleGetConsumerGroup(c *fiber.Ctx) error {
	groupID := c.Params("id")

	if groupID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Group ID is required",
		})
	}

	// Get real group from in-memory coordinator
	s.groupsMutex.RLock()
	group, exists := s.consumerGroups[groupID]
	s.groupsMutex.RUnlock()
	
	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Consumer group '%s' not found", groupID),
		})
	}

	log.Printf("[Native API] Got consumer group: %s", groupID)

	return c.JSON(fiber.Map{
		"success": true,
		"group":   *group,
	})
}

// handleDeleteConsumerGroup deletes a consumer group
// DELETE /api/v1/consumer-groups/:id
func (s *FiberServer) handleDeleteConsumerGroup(c *fiber.Ctx) error {
	groupID := c.Params("id")

	if groupID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Group ID is required",
		})
	}

	// Delete group from in-memory coordinator
	s.groupsMutex.Lock()
	delete(s.consumerGroups, groupID)
	s.groupsMutex.Unlock()
	
	log.Printf("[Native API] Deleted consumer group: %s", groupID)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Consumer group '%s' deleted", groupID),
	})
}

// handleUpdateConsumerGroup updates consumer group topics
// PUT /api/v1/consumer-groups/:id
func (s *FiberServer) handleUpdateConsumerGroup(c *fiber.Ctx) error {
	groupID := c.Params("id")

	var req struct {
		Topics []string `json:"topics" validate:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Update group topics in in-memory coordinator
	s.groupsMutex.Lock()
	if group, exists := s.consumerGroups[groupID]; exists {
		group.Subscriptions = req.Topics
		group.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	s.groupsMutex.Unlock()
	
	log.Printf("[Native API] Updated consumer group %s topics: %v", groupID, req.Topics)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Consumer group '%s' updated", groupID),
		"topics":  req.Topics,
	})
}

// handleJoinConsumerGroup joins a consumer group
// POST /api/v1/consumer-groups/:id/join
func (s *FiberServer) handleJoinConsumerGroup(c *fiber.Ctx) error {
	groupID := c.Params("id")

	var req JoinGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate
	if req.ClientID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Client ID is required",
		})
	}

	// Set defaults
	if req.SessionTimeout == 0 {
		req.SessionTimeout = 10000 // 10 seconds
	}
	if req.ClientHost == "" {
		req.ClientHost = c.IP()
	}

	// Generate member ID if not provided
	memberID := req.MemberID
	if memberID == "" {
		memberID = fmt.Sprintf("%s-%d", req.ClientID, time.Now().UnixNano())
	}

	// Join group via in-memory coordinator
	s.groupsMutex.Lock()
	group, exists := s.consumerGroups[groupID]
	if !exists {
		s.groupsMutex.Unlock()
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Consumer group '%s' not found", groupID),
		})
	}
	
	// Add member
	member := NativeGroupMember{
		ID:             memberID,
		ClientID:       req.ClientID,
		ClientHost:     req.ClientHost,
		SessionTimeout: int(req.SessionTimeout), // int32 -> int
		JoinedAt:       time.Now().Format(time.RFC3339),
		LastHeartbeat:  time.Now().Format(time.RFC3339),
	}
	group.Members = append(group.Members, member)
	group.State = "Stable"
	group.Generation++
	group.UpdatedAt = time.Now().Format(time.RFC3339)
	s.groupsMutex.Unlock()
	
	response := JoinGroupResponse{
		MemberID:   memberID,
		Generation: int32(group.Generation), // int -> int32
		Leader:     len(group.Members) == 1, // First member is leader
		Assignment: []NativePartitionAssignment{},
	}

	log.Printf("[Native API] Member %s joined group %s", memberID, groupID)

	return c.JSON(fiber.Map{
		"success":  true,
		"response": response,
	})
}

// handleLeaveConsumerGroup leaves a consumer group
// POST /api/v1/consumer-groups/:id/leave
func (s *FiberServer) handleLeaveConsumerGroup(c *fiber.Ctx) error {
	groupID := c.Params("id")

	var req LeaveGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate
	if req.MemberID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Member ID is required",
		})
	}

	// Leave group via in-memory coordinator
	s.groupsMutex.Lock()
	if group, exists := s.consumerGroups[groupID]; exists {
		// Remove member
		for i, member := range group.Members {
			if member.ID == req.MemberID {
				group.Members = append(group.Members[:i], group.Members[i+1:]...)
				break
			}
		}
		if len(group.Members) == 0 {
			group.State = "Empty"
		}
		group.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	s.groupsMutex.Unlock()
	
	log.Printf("[Native API] Member %s left group %s", req.MemberID, groupID)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Member '%s' left group '%s'", req.MemberID, groupID),
	})
}

// handleHeartbeat sends heartbeat to consumer group
// POST /api/v1/consumer-groups/:id/heartbeat
func (s *FiberServer) handleHeartbeat(c *fiber.Ctx) error {
	groupID := c.Params("id")

	var req HeartbeatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate
	if req.MemberID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Member ID is required",
		})
	}

	// Send heartbeat via in-memory coordinator
	s.groupsMutex.Lock()
	if group, exists := s.consumerGroups[groupID]; exists {
		// Update member's last heartbeat
		for i, member := range group.Members {
			if member.ID == req.MemberID {
				group.Members[i].LastHeartbeat = time.Now().Format(time.RFC3339)
				break
			}
		}
	}
	s.groupsMutex.Unlock()
	
	log.Printf("[Native API] Heartbeat from member %s in group %s (gen: %d)", req.MemberID, groupID, req.Generation)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Heartbeat received",
	})
}

// handleCommitOffsets commits offsets for a consumer group
// POST /api/v1/consumer-groups/:id/offsets/commit
func (s *FiberServer) handleCommitOffsets(c *fiber.Ctx) error {
	groupID := c.Params("id")

	var req CommitOffsetsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate
	if len(req.Offsets) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "At least one offset is required",
		})
	}

	// Convert to ConsumerOffset and commit to storage
	ctx := c.Context()
	consumerOffsets := make([]*types.ConsumerOffset, 0, len(req.Offsets))

	for _, offsetReq := range req.Offsets {
		consumerOffsets = append(consumerOffsets, &types.ConsumerOffset{
			ConsumerID: types.ConsumerID(groupID),
			Topic:      types.TopicName(offsetReq.Topic),
			Partition:  offsetReq.Partition,
			Offset:     offsetReq.Offset,
			Timestamp:  time.Now().UnixNano(),
			Metadata:   offsetReq.Metadata,
		})
	}

	// Commit offsets to storage
	if err := s.storage.CommitOffsetBatch(ctx, consumerOffsets); err != nil {
		log.Printf("[Native API] Failed to commit offsets for group %s: %v", groupID, err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to commit offsets: %v", err),
		})
	}

	log.Printf("[Native API] Committed %d offsets to storage for group %s", len(req.Offsets), groupID)

	return c.JSON(fiber.Map{
		"success":   true,
		"committed": len(consumerOffsets),
		"group_id":  groupID,
	})
}

// handleFetchOffsets fetches committed offsets for a consumer group
// GET /api/v1/consumer-groups/:id/offsets
func (s *FiberServer) handleFetchOffsets(c *fiber.Ctx) error {
	groupID := c.Params("id")

	// Fetch offsets from storage
	ctx := c.Context()
	consumerOffsets, err := s.storage.GetConsumerOffsets(ctx, types.ConsumerID(groupID))
	if err != nil {
		log.Printf("[Native API] Failed to fetch offsets for group %s: %v", groupID, err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to fetch offsets: %v", err),
		})
	}

	// Convert to API format: map[topic]map[partition]OffsetMetadata
	offsets := make(map[string]map[int32]OffsetMetadata)
	for _, offset := range consumerOffsets {
		topicName := string(offset.Topic)
		if offsets[topicName] == nil {
			offsets[topicName] = make(map[int32]OffsetMetadata)
		}
		offsets[topicName][offset.Partition] = OffsetMetadata{
			Offset:   offset.Offset,
			Metadata: offset.Metadata,
		}
	}

	log.Printf("[Native API] Fetched %d offsets from storage for group %s", len(consumerOffsets), groupID)

	return c.JSON(fiber.Map{
		"success": true,
		"offsets": offsets,
	})
}

// handleResetOffsets resets offsets for a consumer group
// POST /api/v1/consumer-groups/:id/offsets/reset
func (s *FiberServer) handleResetOffsets(c *fiber.Ctx) error {
	groupID := c.Params("id")

	var req ResetOffsetsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate
	if req.Position != "earliest" && req.Position != "latest" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Position must be 'earliest' or 'latest'",
		})
	}

	// Reset offsets in storage
	ctx := c.Context()
	topics := req.Topics
	resetCount := 0

	// If no topics specified, get all topics for this consumer
	if len(topics) == 0 {
		consumerOffsets, err := s.storage.GetConsumerOffsets(ctx, types.ConsumerID(groupID))
		if err == nil {
			topicSet := make(map[string]bool)
			for _, offset := range consumerOffsets {
				topicSet[string(offset.Topic)] = true
			}
			for topic := range topicSet {
				topics = append(topics, topic)
			}
		}
	}

	// Reset offsets for each topic
	for _, topic := range topics {
		// Get partition count for topic
		partitionCount, err := s.storage.GetPartitionCount(ctx, types.TopicName(topic))
		if err != nil {
			log.Printf("[Native API] Failed to get partition count for topic %s: %v", topic, err)
			continue
		}

		// Reset each partition
		for partition := int32(0); partition < partitionCount; partition++ {
			var newOffset int64
			if req.Position == "earliest" {
				newOffset, err = s.storage.GetEarliestOffset(ctx, types.TopicName(topic), partition)
			} else {
				newOffset, err = s.storage.GetLatestOffset(ctx, types.TopicName(topic), partition)
			}

			if err != nil {
				log.Printf("[Native API] Failed to get %s offset for %s[%d]: %v", req.Position, topic, partition, err)
				continue
			}

			// Commit the reset offset
			consumerOffset := &types.ConsumerOffset{
				ConsumerID: types.ConsumerID(groupID),
				Topic:      types.TopicName(topic),
				Partition:  partition,
				Offset:     newOffset,
				Timestamp:  time.Now().UnixNano(),
				Metadata:   fmt.Sprintf("reset to %s", req.Position),
			}

			if err := s.storage.CommitOffset(ctx, consumerOffset); err != nil {
				log.Printf("[Native API] Failed to commit reset offset for %s[%d]: %v", topic, partition, err)
				continue
			}

			resetCount++
		}
	}

	log.Printf("[Native API] Reset %d offsets to %s for group %s (topics: %v)", resetCount, req.Position, groupID, topics)

	return c.JSON(fiber.Map{
		"success":  true,
		"message":  fmt.Sprintf("Reset %d offsets to %s", resetCount, req.Position),
		"group_id": groupID,
		"count":    resetCount,
		"topics":   topics,
	})
}

// handleGetGroupLag gets consumer lag for a consumer group
// GET /api/v1/consumer-groups/:id/lag
func (s *FiberServer) handleGetGroupLag(c *fiber.Ctx) error {
	groupID := c.Params("id")

	// Calculate real lag from storage
	ctx := c.Context()
	consumerOffsets, err := s.storage.GetConsumerOffsets(ctx, types.ConsumerID(groupID))
	if err != nil {
		log.Printf("[Native API] Failed to get offsets for group %s: %v", groupID, err)
		consumerOffsets = []*types.ConsumerOffset{}
	}

	lags := []GroupLagInfo{}
	totalLag := int64(0)
	
	for _, offset := range consumerOffsets {
		// Get latest offset for this topic/partition
		latestOffset, err := s.storage.GetLatestOffset(ctx, offset.Topic, offset.Partition)
		if err != nil {
			continue
		}
		
		lag := latestOffset - offset.Offset
		if lag < 0 {
			lag = 0
		}
		
		lags = append(lags, GroupLagInfo{
			Topic:         string(offset.Topic),
			Partition:     offset.Partition,
			CurrentOffset: offset.Offset,
			LogEndOffset:  latestOffset,
			Lag:           lag,
		})
		
		totalLag += lag
	}

	log.Printf("[Native API] Got lag for group %s: total=%d", groupID, totalLag)

	return c.JSON(fiber.Map{
		"success": true,
		"lag": GroupLagResponse{
			GroupID:  groupID,
			TotalLag: totalLag,
			Lags:     lags,
		},
	})
}

// handleListGroupMembers lists active members of a consumer group
// GET /api/v1/consumer-groups/:id/members
func (s *FiberServer) handleListGroupMembers(c *fiber.Ctx) error {
	groupID := c.Params("id")

	// Get real members from in-memory coordinator
	s.groupsMutex.RLock()
	group, exists := s.consumerGroups[groupID]
	s.groupsMutex.RUnlock()
	
	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Consumer group '%s' not found", groupID),
		})
	}

	log.Printf("[Native API] Listed members for group %s: %d members", groupID, len(group.Members))

	return c.JSON(fiber.Map{
		"success": true,
		"members": group.Members,
		"count":   len(group.Members),
	})
}

// handleGetGroupState gets the state of a consumer group
// GET /api/v1/consumer-groups/:id/state
func (s *FiberServer) handleGetGroupState(c *fiber.Ctx) error {
	groupID := c.Params("id")

	// Get real state from in-memory coordinator
	s.groupsMutex.RLock()
	group, exists := s.consumerGroups[groupID]
	s.groupsMutex.RUnlock()
	
	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Consumer group '%s' not found", groupID),
		})
	}
	
	leader := ""
	if len(group.Members) > 0 {
		leader = group.Members[0].ID
	}
	
	state := fiber.Map{
		"group_id":   groupID,
		"state":      group.State,
		"generation": group.Generation,
		"leader":     leader,
		"members":    len(group.Members),
		"protocol":   group.Protocol,
		"created_at": group.CreatedAt,
		"updated_at": group.UpdatedAt,
	}

	log.Printf("[Native API] Got state for group %s: %s", groupID, state["state"])

	return c.JSON(fiber.Map{
		"success": true,
		"state":   state,
	})
}
