package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// InMemoryAuditLogger implements AuditLogger interface with in-memory storage
type InMemoryAuditLogger struct {
	events []AuditEvent
	mu     sync.RWMutex
}

// NewInMemoryAuditLogger creates a new in-memory audit logger
func NewInMemoryAuditLogger() *InMemoryAuditLogger {
	return &InMemoryAuditLogger{
		events: make([]AuditEvent, 0),
	}
}

// LogEvent logs an audit event
func (l *InMemoryAuditLogger) LogEvent(event *AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, *event)
	return nil
}

// GetEvents retrieves audit events with optional filters
func (l *InMemoryAuditLogger) GetEvents(filters map[string]interface{}, limit int) ([]*AuditEvent, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var filteredEvents []*AuditEvent

	for i := range l.events {
		event := &l.events[i]

		// Apply filters
		if userID, ok := filters["user_id"].(string); ok && event.UserID != userID {
			continue
		}
		if eventType, ok := filters["event_type"].(string); ok && event.EventType != eventType {
			continue
		}
		if resource, ok := filters["resource"].(string); ok && event.Resource != resource {
			continue
		}
		if action, ok := filters["action"].(string); ok && event.Action != action {
			continue
		}
		if result, ok := filters["result"].(string); ok && event.Result != result {
			continue
		}

		// Apply time range filters
		if afterTime, ok := filters["after"].(time.Time); ok && event.Timestamp.Before(afterTime) {
			continue
		}
		if beforeTime, ok := filters["before"].(time.Time); ok && event.Timestamp.After(beforeTime) {
			continue
		}

		filteredEvents = append(filteredEvents, event)

		// Apply limit
		if limit > 0 && len(filteredEvents) >= limit {
			break
		}
	}

	return filteredEvents, nil
}

// FileAuditLogger implements AuditLogger interface with file-based storage
type FileAuditLogger struct {
	filePath string
	mu       sync.Mutex
}

// NewFileAuditLogger creates a new file-based audit logger
func NewFileAuditLogger(filePath string) *FileAuditLogger {
	return &FileAuditLogger{
		filePath: filePath,
	}
}

// LogEvent logs an audit event to file
func (l *FileAuditLogger) LogEvent(event *AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Open file in append mode
	file, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	// Marshal event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Write event to file
	_, err = file.Write(append(eventJSON, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	return nil
}

// GetEvents retrieves audit events from file (simplified implementation)
func (l *FileAuditLogger) GetEvents(filters map[string]interface{}, limit int) ([]*AuditEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Read file
	data, err := os.ReadFile(l.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*AuditEvent{}, nil
		}
		return nil, fmt.Errorf("failed to read audit log file: %w", err)
	}

	// Parse events
	var events []*AuditEvent
	lines := string(data)

	for _, line := range strings.Split(lines, "\n") {
		if line == "" {
			continue
		}

		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip malformed lines
		}

		// Apply filters (simplified)
		if userID, ok := filters["user_id"].(string); ok && event.UserID != userID {
			continue
		}
		if eventType, ok := filters["event_type"].(string); ok && event.EventType != eventType {
			continue
		}

		events = append(events, &event)

		// Apply limit
		if limit > 0 && len(events) >= limit {
			break
		}
	}

	return events, nil
}
