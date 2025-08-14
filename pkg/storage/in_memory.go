package storage

import (
	"context"
	"sync"

	"github.com/meftunca/portask/pkg/types"
)

// InMemoryStorage provides a non-persistent, in-memory implementation of MessageStore.
type InMemoryStorage struct {
	messages   map[string]*types.PortaskMessage
	mutex      sync.RWMutex
	nextOffset int64
}

// NewInMemoryStorage creates a new instance of InMemoryStorage.
func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		messages:   make(map[string]*types.PortaskMessage),
		nextOffset: 0,
	}
}

// StoreMessage stores a message in memory (legacy, for demo/testing).
func (s *InMemoryStorage) StoreMessage(topic string, message []byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	msg := &types.PortaskMessage{
		ID:      types.MessageID(topic + "_" + string(rune(len(s.messages)))),
		Topic:   types.TopicName(topic),
		Payload: message,
	}
	s.messages[string(msg.ID)] = msg
	return nil
}

// GetMessages retrieves messages from memory (legacy, for demo/testing).
func (s *InMemoryStorage) GetMessages(topic string, offset int64) ([][]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var results [][]byte
	for _, msg := range s.messages {
		if string(msg.Topic) == topic {
			results = append(results, msg.Payload)
		}
	}
	return results, nil
}

// GetTopics returns a list of topics.
func (s *InMemoryStorage) GetTopics() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	topics := make(map[string]bool)
	// Simplified topic extraction
	for key := range s.messages {
		topics[key] = true
	}
	var topicList []string
	for topic := range topics {
		topicList = append(topicList, topic)
	}
	return topicList
}

// Cleanup implements the MessageStore interface (no-op for in-memory)
func (s *InMemoryStorage) Cleanup(ctx context.Context, retentionPolicy *RetentionPolicy) error {
	return nil
}

// Close implements the MessageStore interface (no-op for in-memory)
func (s *InMemoryStorage) Close() error {
	return nil
}

// MessageStore interface'inin tüm metotlarını ekle
func (s *InMemoryStorage) Store(ctx context.Context, message *types.PortaskMessage) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	message.Offset = s.nextOffset
	s.nextOffset++
	s.messages[string(message.ID)] = message
	return nil
}

func (s *InMemoryStorage) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, msg := range batch.Messages {
		s.messages[string(msg.ID)] = msg
	}
	return nil
}

func (s *InMemoryStorage) Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var result []*types.PortaskMessage
	for _, msg := range s.messages {
		if msg.Topic == topic && (partition < 0 || msg.Partition == partition) && msg.Offset >= offset {
			result = append(result, msg)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *InMemoryStorage) FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	msg, ok := s.messages[string(messageID)]
	if !ok {
		return nil, nil
	}
	return msg, nil
}

func (s *InMemoryStorage) Delete(ctx context.Context, messageID types.MessageID) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.messages, string(messageID))
	return nil
}

func (s *InMemoryStorage) DeleteBatch(ctx context.Context, messageIDs []types.MessageID) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, id := range messageIDs {
		delete(s.messages, string(id))
	}
	return nil
}

func (s *InMemoryStorage) CreateTopic(ctx context.Context, topicInfo *types.TopicInfo) error {
	return nil
}

func (s *InMemoryStorage) DeleteTopic(ctx context.Context, topic types.TopicName) error {
	return nil
}

func (s *InMemoryStorage) GetTopicInfo(ctx context.Context, topic types.TopicName) (*types.TopicInfo, error) {
	return nil, nil
}

func (s *InMemoryStorage) ListTopics(ctx context.Context) ([]*types.TopicInfo, error) {
	return nil, nil
}

func (s *InMemoryStorage) TopicExists(ctx context.Context, topic types.TopicName) (bool, error) {
	return false, nil
}

func (s *InMemoryStorage) GetPartitionInfo(ctx context.Context, topic types.TopicName, partition int32) (*types.PartitionInfo, error) {
	return nil, nil
}

func (s *InMemoryStorage) GetPartitionCount(ctx context.Context, topic types.TopicName) (int32, error) {
	return 0, nil
}

func (s *InMemoryStorage) GetLatestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}

func (s *InMemoryStorage) GetEarliestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}

func (s *InMemoryStorage) CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error {
	return nil
}

func (s *InMemoryStorage) CommitOffsetBatch(ctx context.Context, offsets []*types.ConsumerOffset) error {
	return nil
}

func (s *InMemoryStorage) GetOffset(ctx context.Context, consumerID types.ConsumerID, topic types.TopicName, partition int32) (*types.ConsumerOffset, error) {
	return nil, nil
}

func (s *InMemoryStorage) GetConsumerOffsets(ctx context.Context, consumerID types.ConsumerID) ([]*types.ConsumerOffset, error) {
	return nil, nil
}

func (s *InMemoryStorage) ListConsumers(ctx context.Context, topic types.TopicName) ([]types.ConsumerID, error) {
	return nil, nil
}

func (s *InMemoryStorage) Ping(ctx context.Context) error {
	return nil
}

func (s *InMemoryStorage) Stats(ctx context.Context) (*StorageStats, error) {
	return nil, nil
}
