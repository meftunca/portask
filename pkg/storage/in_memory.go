package storage

import (
	"sync"
)

// InMemoryStorage provides a non-persistent, in-memory implementation of MessageStore.
type InMemoryStorage struct {
	messages map[string][]byte
	mutex    sync.RWMutex
}

// NewInMemoryStorage creates a new instance of InMemoryStorage.
func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		messages: make(map[string][]byte),
	}
}

// StoreMessage stores a message in memory.
func (s *InMemoryStorage) StoreMessage(topic string, message []byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// Simple key for demo purposes
	key := topic + "_" + string(rune(len(s.messages)))
	s.messages[key] = message
	return nil
}

// GetMessages retrieves messages from memory.
func (s *InMemoryStorage) GetMessages(topic string, offset int64) ([][]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var results [][]byte
	// This is a simplified implementation. A real one would handle offsets properly.
	for _, msg := range s.messages {
		results = append(results, msg)
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
