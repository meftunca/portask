package common

import (
	"sync/atomic"
	"time"
)

// IDGenerator generates unique IDs without allocations
type IDGenerator struct {
	counter atomic.Uint64
	prefix  uint64
}

// NewIDGenerator creates a new ID generator
func NewIDGenerator(prefix string) *IDGenerator {
	// Use timestamp as prefix to ensure uniqueness across restarts
	var prefixNum uint64
	switch prefix {
	case "kafka":
		prefixNum = 1
	case "amqp":
		prefixNum = 2
	default:
		prefixNum = 0
	}
	
	return &IDGenerator{
		prefix: (prefixNum << 56) | uint64(time.Now().Unix()<<24),
	}
}

// Next generates the next unique ID
// Returns a uint64 that can be converted to MessageID without allocation
func (g *IDGenerator) Next() uint64 {
	counter := g.counter.Add(1)
	return g.prefix | (counter & 0xFFFFFF) // 24 bits for counter
}

// Global generators
var (
	kafkaIDGen = NewIDGenerator("kafka")
	amqpIDGen  = NewIDGenerator("amqp")
)

// NextKafkaID generates next Kafka message ID
func NextKafkaID() uint64 {
	return kafkaIDGen.Next()
}

// NextAMQPID generates next AMQP message ID
func NextAMQPID() uint64 {
	return amqpIDGen.Next()
}

// FormatID formats an ID as a string (use sparingly, allocates)
func FormatID(prefix string, id uint64) string {
	return prefix + "-" + itoa(id)
}

// itoa converts uint64 to string without fmt (faster)
func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	
	// Reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	
	return string(buf)
}

// Static metadata maps (reusable, no allocations per message)
var (
	KafkaMetadata = map[string]string{
		"source":   "kafka",
		"protocol": "kafka-wire",
		"version":  "2.0",
	}
	
	AMQPMetadata = map[string]string{
		"source":   "amqp",
		"protocol": "amqp-0.9.1",
	}
)

// CloneMetadata creates a shallow copy of metadata
func CloneMetadata(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

