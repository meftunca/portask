package portask

import (
	"context"
	"time"
)

// Consumer handles message consumption
type Consumer struct {
	client *Client
}

// ConsumeOptions configures message consumption
type ConsumeOptions struct {
	Topic       string
	Partition   *int32
	StartOffset *int64
	MaxMessages int
	MaxWaitMs   int
	GroupID     string // Optional: for consumer groups
	AutoCommit  bool
}

// FetchedMessage represents a consumed message
type FetchedMessage struct {
	MessageID string                 `json:"message_id"`
	Topic     string                 `json:"topic"`
	Partition int32                  `json:"partition"`
	Offset    int64                  `json:"offset"`
	Key       string                 `json:"key"`
	Value     interface{}            `json:"value"`
	Headers   map[string]interface{} `json:"headers"`
	Timestamp time.Time              `json:"timestamp"`
	Size      int                    `json:"size_bytes"`
}

// Fetch fetches messages from a topic
func (c *Consumer) Fetch(ctx context.Context, opts ConsumeOptions) ([]FetchedMessage, error) {
	// Set defaults
	if opts.MaxMessages == 0 {
		opts.MaxMessages = 100
	}
	if opts.MaxWaitMs == 0 {
		opts.MaxWaitMs = 1000
	}

	// Build partition fetch request
	partitions := []map[string]interface{}{
		{
			"partition":    opts.Partition,
			"fetch_offset": opts.StartOffset,
		},
	}

	req := map[string]interface{}{
		"topics": []map[string]interface{}{
			{
				"topic":      opts.Topic,
				"partitions": partitions,
			},
		},
		"max_messages": opts.MaxMessages,
		"max_wait_ms":  opts.MaxWaitMs,
	}

	var response struct {
		Success bool `json:"success"`
		Topics  []struct {
			Topic      string `json:"topic"`
			Partitions []struct {
				Partition     int32            `json:"partition"`
				HighWaterMark int64            `json:"high_water_mark"`
				Messages      []FetchedMessage `json:"messages"`
			} `json:"partitions"`
		} `json:"topics"`
	}

	err := c.client.post(ctx, "/api/v1/messages/batch/fetch", req, &response)
	if err != nil {
		return nil, err
	}

	// Flatten messages
	var messages []FetchedMessage
	for _, topic := range response.Topics {
		for _, partition := range topic.Partitions {
			messages = append(messages, partition.Messages...)
		}
	}

	return messages, nil
}

// FetchPoll performs long-polling fetch (waits until messages available or timeout)
func (c *Consumer) FetchPoll(ctx context.Context, opts ConsumeOptions) ([]FetchedMessage, error) {
	// Set defaults
	if opts.MaxMessages == 0 {
		opts.MaxMessages = 100
	}
	if opts.MaxWaitMs == 0 {
		opts.MaxWaitMs = 5000 // 5 seconds for long-polling
	}

	// Build partition fetch request
	partitions := []map[string]interface{}{
		{
			"partition":    opts.Partition,
			"fetch_offset": opts.StartOffset,
		},
	}

	req := map[string]interface{}{
		"topics": []map[string]interface{}{
			{
				"topic":      opts.Topic,
				"partitions": partitions,
			},
		},
		"max_messages": opts.MaxMessages,
		"max_wait_ms":  opts.MaxWaitMs,
	}

	var response struct {
		Success bool `json:"success"`
		Topics  []struct {
			Topic      string `json:"topic"`
			Partitions []struct {
				Partition     int32            `json:"partition"`
				HighWaterMark int64            `json:"high_water_mark"`
				Messages      []FetchedMessage `json:"messages"`
			} `json:"partitions"`
		} `json:"topics"`
	}

	err := c.client.post(ctx, "/api/v1/messages/batch/fetch/poll", req, &response)
	if err != nil {
		return nil, err
	}

	// Flatten messages
	var messages []FetchedMessage
	for _, topic := range response.Topics {
		for _, partition := range topic.Partitions {
			messages = append(messages, partition.Messages...)
		}
	}

	return messages, nil
}

// Acknowledge acknowledges a message
func (c *Consumer) Acknowledge(ctx context.Context, messageID string, groupID string) error {
	req := map[string]interface{}{
		"message_ids": []string{messageID},
	}

	if groupID != "" {
		req["group_id"] = groupID
	}

	return c.client.post(ctx, "/api/v1/messages/batch/ack", req, nil)
}

// AcknowledgeBatch acknowledges multiple messages
func (c *Consumer) AcknowledgeBatch(ctx context.Context, messageIDs []string, groupID string) error {
	req := map[string]interface{}{
		"message_ids": messageIDs,
	}

	if groupID != "" {
		req["group_id"] = groupID
	}

	return c.client.post(ctx, "/api/v1/messages/batch/ack", req, nil)
}

// NegativeAcknowledge negatively acknowledges a message (requeue or send to DLQ)
func (c *Consumer) NegativeAcknowledge(ctx context.Context, messageID string, reason string, requeue bool, groupID string) error {
	req := map[string]interface{}{
		"message_ids": []string{messageID},
		"reason":      reason,
		"requeue":     requeue,
	}

	if groupID != "" {
		req["group_id"] = groupID
	}

	return c.client.post(ctx, "/api/v1/messages/batch/nack", req, nil)
}

