package api

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ==================== UNIFIED NATIVE TYPES ====================
// These types work for BOTH Kafka and AMQP translators

// NativeConsumerGroup represents a unified consumer group
type NativeConsumerGroup struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	State         string                  `json:"state"` // Stable, Rebalancing, Dead, Empty
	Protocol      string                  `json:"protocol"`
	ProtocolType  string                  `json:"protocol_type"`
	Leader        string                  `json:"leader"`
	Generation    int32                   `json:"generation"`
	Members       []NativeGroupMember     `json:"members"`
	Subscriptions []string                `json:"subscriptions"` // Topics
	CreatedAt     string                  `json:"created_at"`
	UpdatedAt     string                  `json:"updated_at"`
}

// NativeGroupMember represents a consumer group member
type NativeGroupMember struct {
	ID             string                      `json:"id"`
	ClientID       string                      `json:"client_id"`
	ClientHost     string                      `json:"client_host"`
	SessionTimeout int32                       `json:"session_timeout_ms"`
	Assignment     []NativePartitionAssignment `json:"assignment"`
	JoinedAt       string                      `json:"joined_at"`
	LastHeartbeat  string                      `json:"last_heartbeat"`
}

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
	MemberID       string `json:"member_id"`   // Empty on first join
	ClientID       string `json:"client_id" validate:"required"`
	ClientHost     string `json:"client_host"` // Optional
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
	Topics   []string `json:"topics"` // Empty = all topics
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

	// TODO: Get Kafka coordinator (or implement native consumer group manager)
	// For now, return placeholder response
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

	log.Printf("[Native API] Created consumer group: %s (topics: %v)", req.Name, req.Topics)

	return c.Status(201).JSON(fiber.Map{
		"success": true,
		"group":   group,
	})
}

// handleListConsumerGroups lists all consumer groups
// GET /api/v1/consumer-groups
func (s *FiberServer) handleListConsumerGroups(c *fiber.Ctx) error {
	// TODO: Get real groups from coordinator
	// For now, return placeholder
	groups := []NativeConsumerGroup{
		{
			ID:            "sample-group-1",
			Name:          "sample-group-1",
			State:         "Stable",
			Protocol:      "range",
			ProtocolType:  "consumer",
			Leader:        "member-1",
			Generation:    1,
			Members:       []NativeGroupMember{},
			Subscriptions: []string{"orders", "payments"},
			CreatedAt:     time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			UpdatedAt:     time.Now().Format(time.RFC3339),
		},
	}

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

	// TODO: Get real group from coordinator
	group := NativeConsumerGroup{
		ID:           groupID,
		Name:         groupID,
		State:        "Stable",
		Protocol:     "range",
		ProtocolType: "consumer",
		Leader:       "member-1",
		Generation:   1,
		Members: []NativeGroupMember{
			{
				ID:             "member-1",
				ClientID:       "client-1",
				ClientHost:     "127.0.0.1",
				SessionTimeout: 10000,
				Assignment: []NativePartitionAssignment{
					{Topic: "orders", Partitions: []int32{0, 1}},
				},
				JoinedAt:      time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
				LastHeartbeat: time.Now().Format(time.RFC3339),
			},
		},
		Subscriptions: []string{"orders"},
		CreatedAt:     time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		UpdatedAt:     time.Now().Format(time.RFC3339),
	}

	log.Printf("[Native API] Got consumer group: %s", groupID)

	return c.JSON(fiber.Map{
		"success": true,
		"group":   group,
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

	// TODO: Delete group from coordinator
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

	// TODO: Update group topics in coordinator
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

	// TODO: Join group via coordinator
	response := JoinGroupResponse{
		MemberID:   memberID,
		Generation: 1,
		Leader:     false,
		Assignment: []NativePartitionAssignment{
			{Topic: "orders", Partitions: []int32{0}},
		},
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

	// TODO: Leave group via coordinator
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

	// TODO: Send heartbeat via coordinator
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

	// TODO: Commit offsets via offset manager
	log.Printf("[Native API] Committed %d offsets for group %s", len(req.Offsets), groupID)

	return c.JSON(fiber.Map{
		"success":        true,
		"committed":      len(req.Offsets),
		"group_id":       groupID,
	})
}

// handleFetchOffsets fetches committed offsets for a consumer group
// GET /api/v1/consumer-groups/:id/offsets
func (s *FiberServer) handleFetchOffsets(c *fiber.Ctx) error {
	groupID := c.Params("id")

	// TODO: Fetch offsets from offset manager
	offsets := map[string]map[int32]OffsetMetadata{
		"orders": {
			0: {Offset: 100, Metadata: ""},
			1: {Offset: 150, Metadata: ""},
		},
		"payments": {
			0: {Offset: 50, Metadata: ""},
		},
	}

	log.Printf("[Native API] Fetched offsets for group %s", groupID)

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

	// TODO: Reset offsets via offset manager
	topics := req.Topics
	if len(topics) == 0 {
		topics = []string{"all"}
	}

	log.Printf("[Native API] Reset offsets for group %s to %s (topics: %v)", groupID, req.Position, topics)

	return c.JSON(fiber.Map{
		"success":  true,
		"message":  fmt.Sprintf("Offsets reset to %s", req.Position),
		"group_id": groupID,
		"topics":   topics,
	})
}

// handleGetGroupLag gets consumer lag for a consumer group
// GET /api/v1/consumer-groups/:id/lag
func (s *FiberServer) handleGetGroupLag(c *fiber.Ctx) error {
	groupID := c.Params("id")

	// TODO: Calculate real lag from storage
	lags := []GroupLagInfo{
		{
			Topic:         "orders",
			Partition:     0,
			CurrentOffset: 100,
			LogEndOffset:  105,
			Lag:           5,
		},
		{
			Topic:         "orders",
			Partition:     1,
			CurrentOffset: 150,
			LogEndOffset:  160,
			Lag:           10,
		},
	}

	totalLag := int64(0)
	for _, lag := range lags {
		totalLag += lag.Lag
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

	// TODO: Get real members from coordinator
	members := []NativeGroupMember{
		{
			ID:             "member-1",
			ClientID:       "client-1",
			ClientHost:     "127.0.0.1",
			SessionTimeout: 10000,
			Assignment: []NativePartitionAssignment{
				{Topic: "orders", Partitions: []int32{0, 1}},
			},
			JoinedAt:      time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
			LastHeartbeat: time.Now().Format(time.RFC3339),
		},
	}

	log.Printf("[Native API] Listed members for group %s: %d members", groupID, len(members))

	return c.JSON(fiber.Map{
		"success": true,
		"members": members,
		"count":   len(members),
	})
}

// handleGetGroupState gets the state of a consumer group
// GET /api/v1/consumer-groups/:id/state
func (s *FiberServer) handleGetGroupState(c *fiber.Ctx) error {
	groupID := c.Params("id")

	// TODO: Get real state from coordinator
	state := fiber.Map{
		"group_id":   groupID,
		"state":      "Stable",
		"generation": 1,
		"leader":     "member-1",
		"members":    1,
	}

	log.Printf("[Native API] Got state for group %s: %s", groupID, state["state"])

	return c.JSON(fiber.Map{
		"success": true,
		"state":   state,
	})
}

