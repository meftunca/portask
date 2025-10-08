package portask

import "context"

// Producer handles message production
type Producer struct {
	client *Client
}

// Message represents a message to publish
type Message struct {
	Topic     string
	Key       string
	Value     interface{}
	Headers   map[string]interface{}
	Partition *int32 // Optional: specific partition
	TTL       *int64 // Optional: TTL in milliseconds
}

// ProduceResult contains the result of a publish operation
type ProduceResult struct {
	MessageID string `json:"message_id"`
	Topic     string `json:"topic"`
	Partition int32  `json:"partition"`
	Offset    int64  `json:"offset"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// Publish publishes a single message
func (p *Producer) Publish(ctx context.Context, msg Message) (*ProduceResult, error) {
	req := map[string]interface{}{
		"topic": msg.Topic,
		"value": msg.Value,
	}

	if msg.Key != "" {
		req["key"] = msg.Key
	}
	if msg.Headers != nil {
		req["headers"] = msg.Headers
	}
	if msg.Partition != nil {
		req["partition"] = *msg.Partition
	}
	if msg.TTL != nil {
		req["ttl_ms"] = *msg.TTL
	}

	var result ProduceResult
	err := p.client.post(ctx, "/api/v1/messages/publish", req, &result)
	return &result, err
}

// PublishBatch publishes multiple messages in a single request
func (p *Producer) PublishBatch(ctx context.Context, messages []Message) ([]ProduceResult, error) {
	// Convert messages to request format
	msgReqs := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		msgReq := map[string]interface{}{
			"topic": msg.Topic,
			"value": msg.Value,
		}

		if msg.Key != "" {
			msgReq["key"] = msg.Key
		}
		if msg.Headers != nil {
			msgReq["headers"] = msg.Headers
		}
		if msg.Partition != nil {
			msgReq["partition"] = *msg.Partition
		}
		if msg.TTL != nil {
			msgReq["ttl_ms"] = *msg.TTL
		}

		msgReqs[i] = msgReq
	}

	req := map[string]interface{}{
		"messages": msgReqs,
	}

	var response struct {
		Success   bool            `json:"success"`
		Published int             `json:"published"`
		Failed    int             `json:"failed"`
		Results   []ProduceResult `json:"results"`
	}

	err := p.client.post(ctx, "/api/v1/messages/batch/publish", req, &response)
	return response.Results, err
}

// PublishAsync publishes messages asynchronously (fire-and-forget)
func (p *Producer) PublishAsync(ctx context.Context, messages []Message) error {
	// Convert messages to request format
	msgReqs := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		msgReq := map[string]interface{}{
			"topic": msg.Topic,
			"value": msg.Value,
		}

		if msg.Key != "" {
			msgReq["key"] = msg.Key
		}
		if msg.Headers != nil {
			msgReq["headers"] = msg.Headers
		}
		if msg.Partition != nil {
			msgReq["partition"] = *msg.Partition
		}
		if msg.TTL != nil {
			msgReq["ttl_ms"] = *msg.TTL
		}

		msgReqs[i] = msgReq
	}

	req := map[string]interface{}{
		"messages": msgReqs,
	}

	return p.client.post(ctx, "/api/v1/messages/batch/publish/async", req, nil)
}

