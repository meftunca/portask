package kafka

import (
	"fmt"
	"time"

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
func (t *KafkaTranslator) TranslateProduce(
	topic string,
	partition int32,
	key []byte,
	value []byte,
) (*types.PortaskMessage, error) {

	if topic == "" {
		return nil, fmt.Errorf("topic cannot be empty")
	}

	return &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("kafka-%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Partition: partition,
		Key:       string(key),
		Payload:   value,
		Timestamp: time.Now().UnixNano(),
		TTL:       0, // Use default from config
		Metadata: map[string]string{
			"source":   "kafka",
			"protocol": "kafka-wire",
			"version":  "2.0",
		},
	}, nil
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
