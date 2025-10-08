package kafka

import (
	"fmt"
	"strconv"
	"time"

	"github.com/meftunca/portask/pkg/common"
	"github.com/meftunca/portask/pkg/memory"
	"github.com/meftunca/portask/pkg/types"
)

// KafkaTranslator converts Kafka wire protocol to Portask protocol
// This is a THIN layer - no business logic, just translation
type KafkaTranslator struct {
	// No storage, no processing, just translation!
}

// NewKafkaTranslator creates a new Kafka protocol translator
func NewKafkaTranslator() *KafkaTranslator {
	return &KafkaTranslator{}
}

// TranslateProduce converts Kafka Produce request to Portask message
// Optimized to minimize allocations
func (t *KafkaTranslator) TranslateProduce(
	topic string,
	partition int32,
	key []byte,
	value []byte,
) (*types.PortaskMessage, error) {

	if topic == "" {
		return nil, fmt.Errorf("topic cannot be empty")
	}

	// Get message from pool
	msg := memory.GetMessage()
	
	// Ensure maps are initialized (pool might return partially reset message)
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]string, 8)
	}
	if msg.Headers == nil {
		msg.Headers = make(types.MessageHeaders, 4)
	}
	
	// Generate ID without allocation (no fmt.Sprintf!)
	msgID := common.NextKafkaID()
	msg.ID = types.MessageID(strconv.FormatUint(msgID, 10))
	
	// Intern topic to reuse string
	msg.Topic = types.TopicName(memory.InternTopic(topic))
	msg.Partition = partition
	
	// Keep key as string (required by type)
	if len(key) > 0 {
		msg.Key = string(key) // Unavoidable allocation
	} else {
		msg.Key = ""
	}
	
	// Reuse payload buffer if possible
	if cap(msg.Payload) >= len(value) {
		msg.Payload = msg.Payload[:len(value)]
		copy(msg.Payload, value)
	} else {
		msg.Payload = append(msg.Payload[:0], value...)
	}
	
	msg.Timestamp = time.Now().UnixNano()
	msg.TTL = 0 // Use default from config
	
	// Reuse metadata map from pool (already cleared in Reset)
	msg.Metadata["source"] = "kafka"
	msg.Metadata["protocol"] = "kafka-wire"
	msg.Metadata["version"] = "2.0"

	return msg, nil
}

// TranslateFetch converts Kafka Fetch request to Portask fetch request
func (t *KafkaTranslator) TranslateFetch(
	topic string,
	partition int32,
	offset int64,
	maxBytes int32,
) (*types.FetchRequest, error) {

	if topic == "" {
		return nil, fmt.Errorf("topic cannot be empty")
	}

	// Estimate message count from maxBytes (assume ~1KB per message)
	estimatedLimit := int(maxBytes / 1024)
	if estimatedLimit <= 0 {
		estimatedLimit = 100 // Default
	}

	return &types.FetchRequest{
		Topic:     types.TopicName(topic),
		Partition: partition,
		Offset:    offset,
		Limit:     estimatedLimit,
	}, nil
}

// TranslateProduceResponse converts Portask response to Kafka Produce response
func (t *KafkaTranslator) TranslateProduceResponse(
	offset int64,
	err error,
) *KafkaProduceResponse {

	if err != nil {
		return &KafkaProduceResponse{
			ErrorCode: -1, // Unknown error
			Offset:    -1,
			Message:   err.Error(),
		}
	}

	return &KafkaProduceResponse{
		ErrorCode: NoError,
		Offset:    offset,
		Message:   "success",
	}
}

// TranslateFetchResponse converts Portask messages to Kafka Fetch response
func (t *KafkaTranslator) TranslateFetchResponse(
	messages []*types.PortaskMessage,
	err error,
) *KafkaFetchResponse {

	if err != nil {
		return &KafkaFetchResponse{
			ErrorCode: -1, // Unknown error
			Messages:  nil,
		}
	}

	// Convert Portask messages to Kafka messages
	kafkaMessages := make([]*Message, 0, len(messages))
	for _, msg := range messages {
		kafkaMessages = append(kafkaMessages, &Message{
			Offset: msg.Timestamp, // Use timestamp as offset for now
			Key:    []byte(msg.Key),
			Value:  msg.Payload,
		})
	}

	return &KafkaFetchResponse{
		ErrorCode: NoError,
		Messages:  kafkaMessages,
	}
}

// KafkaProduceResponse represents Kafka produce response
type KafkaProduceResponse struct {
	ErrorCode int16
	Offset    int64
	Message   string
}

// KafkaFetchResponse represents Kafka fetch response
type KafkaFetchResponse struct {
	ErrorCode int16
	Messages  []*Message
}
