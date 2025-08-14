package main

import (
	"context"
	"fmt"
	"time"

	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

type InMemoryStore struct {
	messages map[string]*types.PortaskMessage
	topics   map[string]*types.TopicInfo
	offsets  map[string]*types.ConsumerOffset
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		messages: make(map[string]*types.PortaskMessage),
		topics:   make(map[string]*types.TopicInfo),
		offsets:  make(map[string]*types.ConsumerOffset),
	}
}

// Implement required MessageStore methods for InMemoryStore
func (s *InMemoryStore) Store(ctx context.Context, message *types.PortaskMessage) error {
	s.messages[string(message.ID)] = message
	return nil
}

func (s *InMemoryStore) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	for _, msg := range batch.Messages {
		s.messages[string(msg.ID)] = msg
	}
	return nil
}

func (s *InMemoryStore) Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error) {
	var messages []*types.PortaskMessage
	for _, msg := range s.messages {
		if msg.Topic == topic {
			messages = append(messages, msg)
			if len(messages) >= limit {
				break
			}
		}
	}
	return messages, nil
}

func (s *InMemoryStore) FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error) {
	if msg, ok := s.messages[string(messageID)]; ok {
		return msg, nil
	}
	return nil, fmt.Errorf("message not found: %s", messageID)
}

func (s *InMemoryStore) Delete(ctx context.Context, messageID types.MessageID) error {
	delete(s.messages, string(messageID))
	return nil
}

func (s *InMemoryStore) DeleteBatch(ctx context.Context, messageIDs []types.MessageID) error {
	for _, id := range messageIDs {
		delete(s.messages, string(id))
	}
	return nil
}

func (s *InMemoryStore) CreateTopic(ctx context.Context, topicInfo *types.TopicInfo) error {
	s.topics[string(topicInfo.Name)] = topicInfo
	return nil
}

func (s *InMemoryStore) DeleteTopic(ctx context.Context, topic types.TopicName) error {
	delete(s.topics, string(topic))
	return nil
}

func (s *InMemoryStore) GetTopicInfo(ctx context.Context, topic types.TopicName) (*types.TopicInfo, error) {
	if info, ok := s.topics[string(topic)]; ok {
		return info, nil
	}
	return nil, fmt.Errorf("topic not found: %s", topic)
}

func (s *InMemoryStore) ListTopics(ctx context.Context) ([]*types.TopicInfo, error) {
	topics := make([]*types.TopicInfo, 0, len(s.topics))
	for _, topic := range s.topics {
		topics = append(topics, topic)
	}
	return topics, nil
}

func (s *InMemoryStore) TopicExists(ctx context.Context, topic types.TopicName) (bool, error) {
	_, exists := s.topics[string(topic)]
	return exists, nil
}

func (s *InMemoryStore) GetPartitionInfo(ctx context.Context, topic types.TopicName, partition int32) (*types.PartitionInfo, error) {
	return &types.PartitionInfo{
		Topic:     topic,
		Partition: partition,
		Leader:    "node-0",
		Replicas:  []string{"node-0"},
		ISR:       []string{"node-0"},
	}, nil
}

func (s *InMemoryStore) GetPartitionCount(ctx context.Context, topic types.TopicName) (int32, error) {
	return 1, nil
}

func (s *InMemoryStore) GetLatestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	count := 0
	for _, msg := range s.messages {
		if msg.Topic == topic {
			count++
		}
	}
	return int64(count), nil
}

func (s *InMemoryStore) GetEarliestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}

func (s *InMemoryStore) CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error {
	key := fmt.Sprintf("%s_%s_%d", offset.ConsumerID, offset.Topic, offset.Partition)
	s.offsets[key] = offset
	return nil
}

func (s *InMemoryStore) CommitOffsetBatch(ctx context.Context, offsets []*types.ConsumerOffset) error {
	for _, offset := range offsets {
		key := fmt.Sprintf("%s_%s_%d", offset.ConsumerID, offset.Topic, offset.Partition)
		s.offsets[key] = offset
	}
	return nil
}

func (s *InMemoryStore) GetOffset(ctx context.Context, consumerID types.ConsumerID, topic types.TopicName, partition int32) (*types.ConsumerOffset, error) {
	key := fmt.Sprintf("%s_%s_%d", consumerID, topic, partition)
	if offset, ok := s.offsets[key]; ok {
		return offset, nil
	}
	return &types.ConsumerOffset{
		ConsumerID: consumerID,
		Topic:      topic,
		Partition:  partition,
		Offset:     0,
	}, nil
}

func (s *InMemoryStore) GetConsumerOffsets(ctx context.Context, consumerID types.ConsumerID) ([]*types.ConsumerOffset, error) {
	var offsets []*types.ConsumerOffset
	for _, offset := range s.offsets {
		if offset.ConsumerID == consumerID {
			offsets = append(offsets, offset)
		}
	}
	return offsets, nil
}

func (s *InMemoryStore) ListConsumers(ctx context.Context, topic types.TopicName) ([]types.ConsumerID, error) {
	consumerMap := make(map[types.ConsumerID]bool)
	for _, offset := range s.offsets {
		if offset.Topic == topic {
			consumerMap[offset.ConsumerID] = true
		}
	}

	consumers := make([]types.ConsumerID, 0, len(consumerMap))
	for consumerID := range consumerMap {
		consumers = append(consumers, consumerID)
	}
	return consumers, nil
}

func (s *InMemoryStore) Ping(ctx context.Context) error {
	return nil
}

func (s *InMemoryStore) Stats(ctx context.Context) (*storage.StorageStats, error) {
	return &storage.StorageStats{
		MessageCount:    int64(len(s.messages)),
		TopicCount:      int64(len(s.topics)),
		Status:          "connected",
		LastHealthCheck: time.Now(),
	}, nil
}

func (s *InMemoryStore) Cleanup(ctx context.Context, retentionPolicy *storage.RetentionPolicy) error {
	// No cleanup needed for in-memory, but method signature matches interface
	return nil
}

func (s *InMemoryStore) Close() error {
	return nil
}
