package kafka

import (
	"testing"
	"time"
)

func TestOffsetManager_CommitAndFetch(t *testing.T) {
	om := NewOffsetManager()

	// Test committing offset
	err := om.CommitOffset("group1", "topic1", 0, 100)
	if err != nil {
		t.Fatalf("Failed to commit offset: %v", err)
	}

	// Test fetching offset
	offset, err := om.FetchOffset("group1", "topic1", 0)
	if err != nil {
		t.Fatalf("Failed to fetch offset: %v", err)
	}

	if offset != 100 {
		t.Errorf("Expected offset 100, got %d", offset)
	}
}

func TestOffsetManager_FetchNonExistent(t *testing.T) {
	om := NewOffsetManager()

	// Test fetching non-existent group
	_, err := om.FetchOffset("nonexistent", "topic1", 0)
	if err == nil {
		t.Error("Expected error for non-existent group")
	}
}

func TestOffsetManager_FetchAllOffsets(t *testing.T) {
	om := NewOffsetManager()

	// Commit multiple offsets
	om.CommitOffset("group1", "topic1", 0, 100)
	om.CommitOffset("group1", "topic1", 1, 200)
	om.CommitOffset("group1", "topic2", 0, 300)

	// Fetch all offsets
	offsets, err := om.FetchAllOffsets("group1")
	if err != nil {
		t.Fatalf("Failed to fetch all offsets: %v", err)
	}

	if len(offsets) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(offsets))
	}

	if offsets["topic1"][0] != 100 {
		t.Errorf("Expected topic1 partition 0 offset 100, got %d", offsets["topic1"][0])
	}

	if offsets["topic1"][1] != 200 {
		t.Errorf("Expected topic1 partition 1 offset 200, got %d", offsets["topic1"][1])
	}

	if offsets["topic2"][0] != 300 {
		t.Errorf("Expected topic2 partition 0 offset 300, got %d", offsets["topic2"][0])
	}
}

func TestOffsetManager_DeleteGroup(t *testing.T) {
	om := NewOffsetManager()

	// Commit offsets
	om.CommitOffset("group1", "topic1", 0, 100)
	om.CommitOffset("group1", "topic1", 1, 200)

	// Delete group
	err := om.DeleteGroup("group1")
	if err != nil {
		t.Fatalf("Failed to delete group: %v", err)
	}

	// Verify group is deleted
	_, err = om.FetchAllOffsets("group1")
	if err == nil {
		t.Error("Expected error after deleting group")
	}
}

func TestOffsetManager_ListGroups(t *testing.T) {
	om := NewOffsetManager()

	// Commit offsets for multiple groups
	om.CommitOffset("group1", "topic1", 0, 100)
	om.CommitOffset("group2", "topic1", 0, 200)
	om.CommitOffset("group3", "topic1", 0, 300)

	// List groups
	groups := om.ListGroups()

	if len(groups) != 3 {
		t.Errorf("Expected 3 groups, got %d", len(groups))
	}

	// Verify all groups are present
	groupMap := make(map[string]bool)
	for _, group := range groups {
		groupMap[group] = true
	}

	if !groupMap["group1"] || !groupMap["group2"] || !groupMap["group3"] {
		t.Error("Missing expected groups")
	}
}

func TestOffsetManager_GetGroupTopics(t *testing.T) {
	om := NewOffsetManager()

	// Commit offsets for multiple topics
	om.CommitOffset("group1", "topic1", 0, 100)
	om.CommitOffset("group1", "topic2", 0, 200)
	om.CommitOffset("group1", "topic3", 0, 300)

	// Get group topics
	topics, err := om.GetGroupTopics("group1")
	if err != nil {
		t.Fatalf("Failed to get group topics: %v", err)
	}

	if len(topics) != 3 {
		t.Errorf("Expected 3 topics, got %d", len(topics))
	}

	// Verify all topics are present
	topicMap := make(map[string]bool)
	for _, topic := range topics {
		topicMap[topic] = true
	}

	if !topicMap["topic1"] || !topicMap["topic2"] || !topicMap["topic3"] {
		t.Error("Missing expected topics")
	}
}

func TestOffsetManagerWithMetadata(t *testing.T) {
	omm := NewOffsetManagerWithMetadata()

	// Commit offset with metadata
	err := omm.CommitOffsetWithMetadata("group1", "topic1", 0, 100, "test-metadata")
	if err != nil {
		t.Fatalf("Failed to commit offset with metadata: %v", err)
	}

	// Fetch offset metadata
	meta, err := omm.FetchOffsetMetadata("group1", "topic1", 0)
	if err != nil {
		t.Fatalf("Failed to fetch offset metadata: %v", err)
	}

	if meta.Offset != 100 {
		t.Errorf("Expected offset 100, got %d", meta.Offset)
	}

	if meta.Metadata != "test-metadata" {
		t.Errorf("Expected metadata 'test-metadata', got '%s'", meta.Metadata)
	}
}

func TestOffsetManagerWithMetadata_CleanupExpired(t *testing.T) {
	omm := NewOffsetManagerWithMetadata()

	// Commit old offset
	omm.CommitOffsetWithMetadata("group1", "topic1", 0, 100, "old")
	
	// Manually set old timestamp
	omm.metaMux.Lock()
	omm.metadata["group1"]["topic1"][0].Timestamp = time.Now().Add(-2 * time.Hour)
	omm.metaMux.Unlock()

	// Commit new offset
	omm.CommitOffsetWithMetadata("group1", "topic2", 0, 200, "new")

	// Cleanup with 1 hour retention
	cleaned := omm.CleanupExpiredOffsets(1 * time.Hour)

	if cleaned != 1 {
		t.Errorf("Expected 1 cleaned offset, got %d", cleaned)
	}

	// Verify old offset is gone
	_, err := omm.FetchOffsetMetadata("group1", "topic1", 0)
	if err == nil {
		t.Error("Expected error for expired offset")
	}

	// Verify new offset is still there
	_, err = omm.FetchOffsetMetadata("group1", "topic2", 0)
	if err != nil {
		t.Error("New offset should still be present")
	}
}

func TestOffsetManagerWithMetadata_Stats(t *testing.T) {
	omm := NewOffsetManagerWithMetadata()

	// Commit multiple offsets
	omm.CommitOffsetWithMetadata("group1", "topic1", 0, 100, "")
	omm.CommitOffsetWithMetadata("group1", "topic1", 1, 200, "")
	omm.CommitOffsetWithMetadata("group2", "topic2", 0, 300, "")

	// Get stats
	stats := omm.Stats()

	totalGroups, ok := stats["total_groups"].(int)
	if !ok || totalGroups != 2 {
		t.Errorf("Expected 2 total groups, got %v", stats["total_groups"])
	}

	totalOffsets, ok := stats["total_offsets"].(int)
	if !ok || totalOffsets != 3 {
		t.Errorf("Expected 3 total offsets, got %v", stats["total_offsets"])
	}
}

// Benchmarks

func BenchmarkOffsetManager_CommitOffset(b *testing.B) {
	om := NewOffsetManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		om.CommitOffset("group1", "topic1", 0, int64(i))
	}
}

func BenchmarkOffsetManager_FetchOffset(b *testing.B) {
	om := NewOffsetManager()
	om.CommitOffset("group1", "topic1", 0, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		om.FetchOffset("group1", "topic1", 0)
	}
}

func BenchmarkOffsetManagerWithMetadata_CommitOffset(b *testing.B) {
	omm := NewOffsetManagerWithMetadata()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		omm.CommitOffsetWithMetadata("group1", "topic1", 0, int64(i), "metadata")
	}
}

func BenchmarkOffsetManagerWithMetadata_FetchMetadata(b *testing.B) {
	omm := NewOffsetManagerWithMetadata()
	omm.CommitOffsetWithMetadata("group1", "topic1", 0, 100, "metadata")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		omm.FetchOffsetMetadata("group1", "topic1", 0)
	}
}

