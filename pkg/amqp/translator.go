package amqp

import (
	"fmt"
	"strconv"
	"time"

	"github.com/meftunca/portask/pkg/common"
	"github.com/meftunca/portask/pkg/memory"
	"github.com/meftunca/portask/pkg/types"
)

// AMQPTranslator converts AMQP wire protocol to Portask protocol
// This is a THIN layer - no business logic, just translation
type AMQPTranslator struct {
	// No storage, no processing, just translation!
}

// NewAMQPTranslator creates a new AMQP protocol translator
func NewAMQPTranslator() *AMQPTranslator {
	return &AMQPTranslator{}
}

// TranslatePublish converts AMQP Publish to Portask message
// Uses object pooling to reduce allocations
func (t *AMQPTranslator) TranslatePublish(
	exchange string,
	routingKey string,
	body []byte,
	properties *MessageProperties,
) (*types.PortaskMessage, error) {

	if routingKey == "" {
		return nil, fmt.Errorf("routing key cannot be empty")
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
	
	// Use routing key as topic
	topic := routingKey
	if exchange != "" {
		topic = exchange + "." + routingKey // String concat instead of fmt.Sprintf
	}
	
	// Generate ID without allocation
	msgID := common.NextAMQPID()
	msg.ID = types.MessageID(strconv.FormatUint(msgID, 10))
	
	msg.Topic = types.TopicName(memory.InternTopic(topic)) // Intern topic string
	msg.Partition = 0                                      // AMQP doesn't have partitions

	// Reuse payload buffer
	if cap(msg.Payload) >= len(body) {
		msg.Payload = msg.Payload[:len(body)]
		copy(msg.Payload, body)
	} else {
		msg.Payload = append(msg.Payload[:0], body...)
	}

	msg.Timestamp = time.Now().UnixNano()
	msg.TTL = 0 // Use default from config

	// Reuse metadata map
	msg.Metadata["source"] = "amqp"
	msg.Metadata["protocol"] = "amqp-0.9.1"
	msg.Metadata["exchange"] = exchange
	msg.Metadata["routing_key"] = routingKey

	// Add AMQP properties to metadata
	if properties != nil {
		if properties.ContentType != "" {
			msg.Metadata["content_type"] = properties.ContentType
		}
		if properties.ContentEncoding != "" {
			msg.Metadata["content_encoding"] = properties.ContentEncoding
		}
		if properties.CorrelationID != "" {
			msg.Metadata["correlation_id"] = properties.CorrelationID
		}
		if properties.ReplyTo != "" {
			msg.Metadata["reply_to"] = properties.ReplyTo
		}
		if properties.MessageID != "" {
			msg.Metadata["message_id"] = properties.MessageID
		}
		if properties.AppID != "" {
			msg.Metadata["app_id"] = properties.AppID
		}
		if properties.UserID != "" {
			msg.Metadata["user_id"] = properties.UserID
		}
		if properties.Priority > 0 {
			msg.Metadata["priority"] = fmt.Sprintf("%d", properties.Priority)
		}
		if properties.DeliveryMode > 0 {
			msg.Metadata["delivery_mode"] = fmt.Sprintf("%d", properties.DeliveryMode)
		}
	}

	return msg, nil
}

// TranslateConsume converts AMQP Consume request to Portask fetch request
func (t *AMQPTranslator) TranslateConsume(
	queue string,
	consumerTag string,
	noAck bool,
	exclusive bool,
	noLocal bool,
	noWait bool,
) (*types.FetchRequest, error) {

	if queue == "" {
		return nil, fmt.Errorf("queue name cannot be empty")
	}

	return &types.FetchRequest{
		Topic:     types.TopicName(queue),
		Partition: 0,   // AMQP doesn't have partitions
		Offset:    0,   // Start from beginning
		Limit:     100, // Default batch size
	}, nil
}

// TranslatePublishResponse converts Portask response to AMQP response
func (t *AMQPTranslator) TranslatePublishResponse(
	offset int64,
	err error,
) *AMQPPublishResponse {

	if err != nil {
		return &AMQPPublishResponse{
			Success: false,
			Error:   err.Error(),
		}
	}

	return &AMQPPublishResponse{
		Success: true,
		Offset:  offset,
	}
}

// TranslateConsumeResponse converts Portask messages to AMQP Deliver
func (t *AMQPTranslator) TranslateConsumeResponse(
	messages []*types.PortaskMessage,
	consumerTag string,
	err error,
) ([]*AMQPDeliver, error) {

	if err != nil {
		return nil, err
	}

	// Convert Portask messages to AMQP Deliver messages
	delivers := make([]*AMQPDeliver, 0, len(messages))
	for i, msg := range messages {
		delivers = append(delivers, &AMQPDeliver{
			ConsumerTag: consumerTag,
			DeliveryTag: uint64(i + 1),
			Redelivered: false,
			Exchange:    msg.Metadata["exchange"],
			RoutingKey:  msg.Metadata["routing_key"],
			Body:        msg.Payload,
			Properties:  t.extractProperties(msg),
		})
	}

	return delivers, nil
}

// extractProperties extracts AMQP properties from Portask message metadata
func (t *AMQPTranslator) extractProperties(msg *types.PortaskMessage) *MessageProperties {
	props := &MessageProperties{}

	if msg.Metadata != nil {
		props.ContentType = msg.Metadata["content_type"]
		props.ContentEncoding = msg.Metadata["content_encoding"]
		props.CorrelationID = msg.Metadata["correlation_id"]
		props.ReplyTo = msg.Metadata["reply_to"]
		props.MessageID = msg.Metadata["message_id"]
		props.AppID = msg.Metadata["app_id"]
		props.UserID = msg.Metadata["user_id"]

		// Parse numeric values
		if priority, ok := msg.Metadata["priority"]; ok {
			fmt.Sscanf(priority, "%d", &props.Priority)
		}
		if deliveryMode, ok := msg.Metadata["delivery_mode"]; ok {
			fmt.Sscanf(deliveryMode, "%d", &props.DeliveryMode)
		}
	}

	return props
}

// MessageProperties represents AMQP message properties
type MessageProperties struct {
	ContentType     string
	ContentEncoding string
	Headers         map[string]interface{}
	DeliveryMode    uint8 // 1 = non-persistent, 2 = persistent
	Priority        uint8 // 0-9
	CorrelationID   string
	ReplyTo         string
	Expiration      string
	MessageID       string
	Timestamp       time.Time
	Type            string
	UserID          string
	AppID           string
	ClusterID       string
}

// AMQPPublishResponse represents AMQP publish response
type AMQPPublishResponse struct {
	Success bool
	Offset  int64
	Error   string
}

// AMQPDeliver represents AMQP message delivery
type AMQPDeliver struct {
	ConsumerTag string
	DeliveryTag uint64
	Redelivered bool
	Exchange    string
	RoutingKey  string
	Body        []byte
	Properties  *MessageProperties
}
