package portask

import "time"

// ==================== Health & Status ====================

// HealthStatus represents server health status
type HealthStatus struct {
	Status      string                 `json:"status"`
	Version     string                 `json:"version"`
	Uptime      time.Duration          `json:"uptime"`
	Connections int                    `json:"connections"`
	Memory      map[string]interface{} `json:"memory"`
	Storage     map[string]interface{} `json:"storage"`
}

// ==================== Consumer Group ====================

// ConsumerGroup represents a consumer group
type ConsumerGroup struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	State         string                  `json:"state"`
	Protocol      string                  `json:"protocol"`
	ProtocolType  string                  `json:"protocol_type"`
	Leader        string                  `json:"leader"`
	Generation    int32                   `json:"generation"`
	Members       []GroupMember           `json:"members"`
	Subscriptions []string                `json:"subscriptions"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
}

// GroupMember represents a member of a consumer group
type GroupMember struct {
	ID             string                `json:"id"`
	ClientID       string                `json:"client_id"`
	ClientHost     string                `json:"client_host"`
	SessionTimeout int32                 `json:"session_timeout_ms"`
	Assignment     []PartitionAssignment `json:"assignment"`
	JoinedAt       time.Time             `json:"joined_at"`
	LastHeartbeat  time.Time             `json:"last_heartbeat"`
}

// PartitionAssignment represents partition assignment for a member
type PartitionAssignment struct {
	Topic      string  `json:"topic"`
	Partitions []int32 `json:"partitions"`
}

// JoinGroupResponse after joining a group
type JoinGroupResponse struct {
	MemberID   string                `json:"member_id"`
	Generation int32                 `json:"generation"`
	Leader     bool                  `json:"is_leader"`
	Members    []GroupMember         `json:"members,omitempty"`
	Assignment []PartitionAssignment `json:"assignment"`
}

// GroupLag represents consumer lag information
type GroupLag struct {
	GroupID  string         `json:"group_id"`
	TotalLag int64          `json:"total_lag"`
	Lags     []PartitionLag `json:"partitions"`
}

// PartitionLag represents lag for a single partition
type PartitionLag struct {
	Topic         string `json:"topic"`
	Partition     int32  `json:"partition"`
	CurrentOffset int64  `json:"current_offset"`
	LogEndOffset  int64  `json:"log_end_offset"`
	Lag           int64  `json:"lag"`
}

// ==================== Transaction ====================

// Transaction represents an active transaction
type Transaction struct {
	ID         string    `json:"id"`
	State      string    `json:"state"`
	Topics     []string  `json:"topics"`
	Messages   int       `json:"messages_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	TimeoutMs  int64     `json:"timeout_ms"`
}

// TransactionStatus represents transaction status
type TransactionStatus struct {
	Transaction Transaction `json:"transaction"`
	Healthy     bool        `json:"healthy"`
	CanCommit   bool        `json:"can_commit"`
}

// ==================== Topic ====================

// Topic represents a unified topic
type Topic struct {
	Name              string      `json:"name"`
	Partitions        int         `json:"partitions"`
	ReplicationFactor int         `json:"replication_factor"`
	Config            TopicConfig `json:"config"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
	MessageCount      int64       `json:"message_count"`
	TotalBytes        int64       `json:"total_bytes"`
}

// TopicConfig represents topic configuration
type TopicConfig struct {
	RetentionMs       int64  `json:"retention_ms"`
	CompressionType   string `json:"compression_type"`
	MaxMessageBytes   int64  `json:"max_message_bytes"`
	MinInSyncReplicas int    `json:"min_insync_replicas"`
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
	ISR          int    `json:"in_sync_replicas"`
}

// ==================== Error ====================

// ErrorResponse represents an error response from the server
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

