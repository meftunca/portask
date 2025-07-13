package types

import (
	"time"
)

// MessageID represents a unique message identifier
type MessageID string

// TopicName represents a topic/queue name
type TopicName string

// ConsumerID represents a consumer identifier
type ConsumerID string

// MessagePriority defines message priority levels
type MessagePriority uint8

const (
	PriorityLow    MessagePriority = 1
	PriorityNormal MessagePriority = 5
	PriorityHigh   MessagePriority = 9
)

// MessageStatus represents the current status of a message
type MessageStatus uint8

const (
	StatusPending MessageStatus = iota
	StatusProcessing
	StatusAcknowledged
	StatusFailed
	StatusExpired
)

// MessageHeaders represents custom message headers
type MessageHeaders map[string]interface{}

// PortaskMessage represents the core message structure optimized for performance
type PortaskMessage struct {
	// Core fields (most frequently accessed)
	ID        MessageID `cbor:"i" json:"id" msgpack:"i"`
	Topic     TopicName `cbor:"t" json:"topic" msgpack:"t"`
	Payload   []byte    `cbor:"p" json:"payload" msgpack:"p"`
	Timestamp int64     `cbor:"ts" json:"timestamp" msgpack:"ts"` // Unix nanoseconds for precision

	// Optional fields (less frequently accessed)
	Headers  MessageHeaders  `cbor:"h,omitempty" json:"headers,omitempty" msgpack:"h,omitempty"`
	Priority MessagePriority `cbor:"pr,omitempty" json:"priority,omitempty" msgpack:"pr,omitempty"`
	Status   MessageStatus   `cbor:"st,omitempty" json:"status,omitempty" msgpack:"st,omitempty"`

	// Routing and delivery
	PartitionKey string    `cbor:"pk,omitempty" json:"partition_key,omitempty" msgpack:"pk,omitempty"`
	ReplyTo      TopicName `cbor:"rt,omitempty" json:"reply_to,omitempty" msgpack:"rt,omitempty"`

	// Expiration and retry
	ExpiresAt int64 `cbor:"ex,omitempty" json:"expires_at,omitempty" msgpack:"ex,omitempty"` // Unix nanoseconds
	TTL       int64 `cbor:"ttl,omitempty" json:"ttl,omitempty" msgpack:"ttl,omitempty"`      // Duration in nanoseconds
	Attempts  uint8 `cbor:"att,omitempty" json:"attempts,omitempty" msgpack:"att,omitempty"`
	MaxRetry  uint8 `cbor:"mr,omitempty" json:"max_retry,omitempty" msgpack:"mr,omitempty"`

	// Tracking and correlation
	CorrelationID string `cbor:"cid,omitempty" json:"correlation_id,omitempty" msgpack:"cid,omitempty"`
	TraceID       string `cbor:"tid,omitempty" json:"trace_id,omitempty" msgpack:"tid,omitempty"`

	// Internal fields (not serialized in external protocols)
	_internal struct {
		ReceivedAt     time.Time
		ProcessedAt    time.Time
		AcknowledgedAt time.Time
		Size           int
		CompressedSize int
		SerializedData []byte
	} `cbor:"-" json:"-" msgpack:"-"`
}

// NewMessage creates a new PortaskMessage with default values
func NewMessage(topic TopicName, payload []byte) *PortaskMessage {
	now := time.Now().UnixNano()
	return &PortaskMessage{
		ID:        MessageID(generateMessageID()),
		Topic:     topic,
		Payload:   payload,
		Timestamp: now,
		Priority:  PriorityNormal,
		Status:    StatusPending,
		Headers:   make(MessageHeaders),
		TTL:       int64(24 * time.Hour), // Default 24 hour TTL
	}
}

// WithHeader adds a header to the message
func (m *PortaskMessage) WithHeader(key string, value interface{}) *PortaskMessage {
	if m.Headers == nil {
		m.Headers = make(MessageHeaders)
	}
	m.Headers[key] = value
	return m
}

// WithPriority sets the message priority
func (m *PortaskMessage) WithPriority(priority MessagePriority) *PortaskMessage {
	m.Priority = priority
	return m
}

// WithTTL sets the message TTL
func (m *PortaskMessage) WithTTL(ttl time.Duration) *PortaskMessage {
	m.TTL = int64(ttl)
	m.ExpiresAt = m.Timestamp + int64(ttl)
	return m
}

// WithPartitionKey sets the partition key for routing
func (m *PortaskMessage) WithPartitionKey(key string) *PortaskMessage {
	m.PartitionKey = key
	return m
}

// WithReplyTo sets the reply-to topic
func (m *PortaskMessage) WithReplyTo(topic TopicName) *PortaskMessage {
	m.ReplyTo = topic
	return m
}

// WithCorrelationID sets the correlation ID
func (m *PortaskMessage) WithCorrelationID(id string) *PortaskMessage {
	m.CorrelationID = id
	return m
}

// WithTraceID sets the trace ID for distributed tracing
func (m *PortaskMessage) WithTraceID(id string) *PortaskMessage {
	m.TraceID = id
	return m
}

// IsExpired checks if the message has expired
func (m *PortaskMessage) IsExpired() bool {
	if m.ExpiresAt == 0 {
		return false
	}
	return time.Now().UnixNano() > m.ExpiresAt
}

// CanRetry checks if the message can be retried
func (m *PortaskMessage) CanRetry() bool {
	return m.Attempts < m.MaxRetry
}

// IncrementAttempts increments the retry attempt counter
func (m *PortaskMessage) IncrementAttempts() {
	m.Attempts++
}

// GetHeader retrieves a header value
func (m *PortaskMessage) GetHeader(key string) (interface{}, bool) {
	if m.Headers == nil {
		return nil, false
	}
	value, exists := m.Headers[key]
	return value, exists
}

// GetStringHeader retrieves a header value as string
func (m *PortaskMessage) GetStringHeader(key string) (string, bool) {
	if value, exists := m.GetHeader(key); exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetSize returns the estimated size of the message in bytes
func (m *PortaskMessage) GetSize() int {
	if m._internal.Size > 0 {
		return m._internal.Size
	}

	// Estimate size if not calculated
	size := len(m.Payload)
	size += len(string(m.ID))
	size += len(string(m.Topic))
	size += len(m.PartitionKey)
	size += len(string(m.ReplyTo))
	size += len(m.CorrelationID)
	size += len(m.TraceID)

	// Estimate headers size
	for k, v := range m.Headers {
		size += len(k) + estimateValueSize(v)
	}

	// Add overhead for struct fields
	size += 128 // Estimated overhead

	m._internal.Size = size
	return size
}

// GetCompressedSize returns the compressed size if available
func (m *PortaskMessage) GetCompressedSize() int {
	return m._internal.CompressedSize
}

// SetCompressedSize sets the compressed size
func (m *PortaskMessage) SetCompressedSize(size int) {
	m._internal.CompressedSize = size
}

// SetSerializedData caches the serialized data
func (m *PortaskMessage) SetSerializedData(data []byte) {
	m._internal.SerializedData = data
}

// GetSerializedData returns cached serialized data
func (m *PortaskMessage) GetSerializedData() []byte {
	return m._internal.SerializedData
}

// ClearSerializedData clears cached serialized data to save memory
func (m *PortaskMessage) ClearSerializedData() {
	m._internal.SerializedData = nil
}

// markReceived marks the message as received
func (m *PortaskMessage) markReceived() {
	m._internal.ReceivedAt = time.Now()
}

// markProcessed marks the message as processed
func (m *PortaskMessage) markProcessed() {
	m._internal.ProcessedAt = time.Now()
	m.Status = StatusProcessing
}

// markAcknowledged marks the message as acknowledged
func (m *PortaskMessage) markAcknowledged() {
	m._internal.AcknowledgedAt = time.Now()
	m.Status = StatusAcknowledged
}

// markFailed marks the message as failed
func (m *PortaskMessage) markFailed() {
	m.Status = StatusFailed
}

// GetProcessingTime returns the time taken to process the message
func (m *PortaskMessage) GetProcessingTime() time.Duration {
	if m._internal.ReceivedAt.IsZero() || m._internal.ProcessedAt.IsZero() {
		return 0
	}
	return m._internal.ProcessedAt.Sub(m._internal.ReceivedAt)
}

// GetTotalTime returns the total time from received to acknowledged
func (m *PortaskMessage) GetTotalTime() time.Duration {
	if m._internal.ReceivedAt.IsZero() || m._internal.AcknowledgedAt.IsZero() {
		return 0
	}
	return m._internal.AcknowledgedAt.Sub(m._internal.ReceivedAt)
}

// estimateValueSize estimates the byte size of an interface{} value
func estimateValueSize(v interface{}) int {
	switch val := v.(type) {
	case string:
		return len(val)
	case []byte:
		return len(val)
	case int, int8, int16, int32, int64:
		return 8
	case uint, uint8, uint16, uint32, uint64:
		return 8
	case float32:
		return 4
	case float64:
		return 8
	case bool:
		return 1
	default:
		return 16 // Default estimate
	}
}

// generateMessageID generates a unique message ID
// This is a simple implementation - in production, use a more sophisticated approach
func generateMessageID() string {
	return generateULID()
}

// MessageBatch represents a batch of messages for bulk operations
type MessageBatch struct {
	Messages  []*PortaskMessage `cbor:"m" json:"messages" msgpack:"m"`
	BatchID   string            `cbor:"bid" json:"batch_id" msgpack:"bid"`
	CreatedAt int64             `cbor:"ca" json:"created_at" msgpack:"ca"`

	_internal struct {
		TotalSize      int
		CompressedSize int
		SerializedData []byte
	} `cbor:"-" json:"-" msgpack:"-"`
}

// NewMessageBatch creates a new message batch
func NewMessageBatch(messages []*PortaskMessage) *MessageBatch {
	return &MessageBatch{
		Messages:  messages,
		BatchID:   generateULID(),
		CreatedAt: time.Now().UnixNano(),
	}
}

// Add adds a message to the batch
func (b *MessageBatch) Add(message *PortaskMessage) {
	b.Messages = append(b.Messages, message)
	b._internal.TotalSize += message.GetSize()
}

// Len returns the number of messages in the batch
func (b *MessageBatch) Len() int {
	return len(b.Messages)
}

// GetTotalSize returns the total size of all messages in the batch
func (b *MessageBatch) GetTotalSize() int {
	if b._internal.TotalSize > 0 {
		return b._internal.TotalSize
	}

	total := 0
	for _, msg := range b.Messages {
		total += msg.GetSize()
	}
	b._internal.TotalSize = total
	return total
}

// GetCompressedSize returns the compressed size of the batch
func (b *MessageBatch) GetCompressedSize() int {
	return b._internal.CompressedSize
}

// SetCompressedSize sets the compressed size of the batch
func (b *MessageBatch) SetCompressedSize(size int) {
	b._internal.CompressedSize = size
}

// ConsumerOffset represents consumer offset information
type ConsumerOffset struct {
	ConsumerID ConsumerID `cbor:"cid" json:"consumer_id" msgpack:"cid"`
	Topic      TopicName  `cbor:"t" json:"topic" msgpack:"t"`
	Partition  int32      `cbor:"p" json:"partition" msgpack:"p"`
	Offset     int64      `cbor:"o" json:"offset" msgpack:"o"`
	Timestamp  int64      `cbor:"ts" json:"timestamp" msgpack:"ts"`
	Metadata   string     `cbor:"m,omitempty" json:"metadata,omitempty" msgpack:"m,omitempty"`
}

// TopicInfo represents topic metadata
type TopicInfo struct {
	Name              TopicName         `cbor:"n" json:"name" msgpack:"n"`
	Partitions        int32             `cbor:"p" json:"partitions" msgpack:"p"`
	ReplicationFactor int16             `cbor:"rf" json:"replication_factor" msgpack:"rf"`
	Config            map[string]string `cbor:"c,omitempty" json:"config,omitempty" msgpack:"c,omitempty"`
	CreatedAt         int64             `cbor:"ca" json:"created_at" msgpack:"ca"`
}

// PartitionInfo represents partition metadata
type PartitionInfo struct {
	Topic     TopicName `cbor:"t" json:"topic" msgpack:"t"`
	Partition int32     `cbor:"p" json:"partition" msgpack:"p"`
	Leader    string    `cbor:"l" json:"leader" msgpack:"l"`
	Replicas  []string  `cbor:"r" json:"replicas" msgpack:"r"`
	ISR       []string  `cbor:"isr" json:"isr" msgpack:"isr"` // In-Sync Replicas
}
