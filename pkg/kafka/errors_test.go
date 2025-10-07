package kafka

import (
	"testing"
)

func TestKafkaError_Error(t *testing.T) {
	err := &KafkaError{
		Code:    ErrorCodeUnknownTopicOrPartition,
		Message: "Topic not found",
	}

	expected := "Kafka error 3: Topic not found"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestKafkaError_IsRetriable(t *testing.T) {
	tests := []struct {
		name     string
		err      *KafkaError
		expected bool
	}{
		{
			name:     "LeaderNotAvailable is retriable",
			err:      &KafkaError{Code: ErrorCodeLeaderNotAvailable},
			expected: true,
		},
		{
			name:     "RequestTimedOut is retriable",
			err:      &KafkaError{Code: ErrorCodeRequestTimedOut},
			expected: true,
		},
		{
			name:     "RebalanceInProgress is retriable",
			err:      &KafkaError{Code: ErrorCodeRebalanceInProgress},
			expected: true,
		},
		{
			name:     "InvalidRequest is not retriable",
			err:      &KafkaError{Code: ErrorCodeInvalidRequest},
			expected: false,
		},
		{
			name:     "TopicAuthorizationFailed is not retriable",
			err:      &KafkaError{Code: ErrorCodeTopicAuthorizationFailed},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.IsRetriable() != tt.expected {
				t.Errorf("Expected IsRetriable() to return %v for %s", tt.expected, tt.name)
			}
		})
	}
}

func TestKafkaError_IsFatal(t *testing.T) {
	tests := []struct {
		name     string
		err      *KafkaError
		expected bool
	}{
		{
			name:     "TopicAuthorizationFailed is fatal",
			err:      &KafkaError{Code: ErrorCodeTopicAuthorizationFailed},
			expected: true,
		},
		{
			name:     "InvalidRequest is fatal",
			err:      &KafkaError{Code: ErrorCodeInvalidRequest},
			expected: true,
		},
		{
			name:     "UnsupportedVersion is fatal",
			err:      &KafkaError{Code: ErrorCodeUnsupportedVersion},
			expected: true,
		},
		{
			name:     "RequestTimedOut is not fatal",
			err:      &KafkaError{Code: ErrorCodeRequestTimedOut},
			expected: false,
		},
		{
			name:     "RebalanceInProgress is not fatal",
			err:      &KafkaError{Code: ErrorCodeRebalanceInProgress},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.IsFatal() != tt.expected {
				t.Errorf("Expected IsFatal() to return %v for %s", tt.expected, tt.name)
			}
		})
	}
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name         string
		constructor  func() *KafkaError
		expectedCode int16
	}{
		{"NewNoError", NewNoError, ErrorCodeNoError},
		{"NewOffsetOutOfRange", NewOffsetOutOfRange, ErrorCodeOffsetOutOfRange},
		{"NewRebalanceInProgress", NewRebalanceInProgress, ErrorCodeRebalanceInProgress},
		{"NewInvalidSessionTimeout", NewInvalidSessionTimeout, ErrorCodeInvalidSessionTimeout},
		{"NewCoordinatorNotAvailable", NewCoordinatorNotAvailable, ErrorCodeCoordinatorNotAvailable},
		{"NewNotCoordinator", NewNotCoordinator, ErrorCodeNotCoordinator},
		{"NewRequestTimedOut", NewRequestTimedOut, ErrorCodeRequestTimedOut},
		{"NewCorruptMessage", NewCorruptMessage, ErrorCodeCorruptMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor()
			if err.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, err.Code)
			}
			if err.Message == "" {
				t.Error("Expected non-empty error message")
			}
		})
	}
}

func TestErrorConstructorsWithParams(t *testing.T) {
	tests := []struct {
		name         string
		constructor  func() *KafkaError
		expectedCode int16
		expectedMsg  string
	}{
		{
			name: "NewUnknownTopicOrPartition",
			constructor: func() *KafkaError {
				return NewUnknownTopicOrPartition("test-topic")
			},
			expectedCode: ErrorCodeUnknownTopicOrPartition,
			expectedMsg:  "Unknown topic or partition: test-topic",
		},
		{
			name: "NewUnknownMemberId",
			constructor: func() *KafkaError {
				return NewUnknownMemberId("member-123")
			},
			expectedCode: ErrorCodeUnknownMemberId,
			expectedMsg:  "Unknown member ID: member-123",
		},
		{
			name: "NewIllegalGeneration",
			constructor: func() *KafkaError {
				return NewIllegalGeneration(5, 3)
			},
			expectedCode: ErrorCodeIllegalGeneration,
			expectedMsg:  "Illegal generation: expected 5, got 3",
		},
		{
			name: "NewInvalidGroupId",
			constructor: func() *KafkaError {
				return NewInvalidGroupId("invalid-group")
			},
			expectedCode: ErrorCodeInvalidGroupId,
			expectedMsg:  "Invalid group ID: invalid-group",
		},
		{
			name: "NewGroupIdNotFound",
			constructor: func() *KafkaError {
				return NewGroupIdNotFound("missing-group")
			},
			expectedCode: ErrorCodeGroupIdNotFound,
			expectedMsg:  "Group not found: missing-group",
		},
		{
			name: "NewTopicAuthorizationFailed",
			constructor: func() *KafkaError {
				return NewTopicAuthorizationFailed("secure-topic")
			},
			expectedCode: ErrorCodeTopicAuthorizationFailed,
			expectedMsg:  "Topic authorization failed: secure-topic",
		},
		{
			name: "NewGroupAuthorizationFailed",
			constructor: func() *KafkaError {
				return NewGroupAuthorizationFailed("secure-group")
			},
			expectedCode: ErrorCodeGroupAuthorizationFailed,
			expectedMsg:  "Group authorization failed: secure-group",
		},
		{
			name: "NewMessageTooLarge",
			constructor: func() *KafkaError {
				return NewMessageTooLarge(2000000, 1000000)
			},
			expectedCode: ErrorCodeMessageTooLarge,
			expectedMsg:  "Message size 2000000 exceeds maximum 1000000",
		},
		{
			name: "NewUnsupportedVersion",
			constructor: func() *KafkaError {
				return NewUnsupportedVersion(99)
			},
			expectedCode: ErrorCodeUnsupportedVersion,
			expectedMsg:  "Unsupported version: 99",
		},
		{
			name: "NewTopicAlreadyExists",
			constructor: func() *KafkaError {
				return NewTopicAlreadyExists("existing-topic")
			},
			expectedCode: ErrorCodeTopicAlreadyExists,
			expectedMsg:  "Topic already exists: existing-topic",
		},
		{
			name: "NewInvalidPartitions",
			constructor: func() *KafkaError {
				return NewInvalidPartitions(-1)
			},
			expectedCode: ErrorCodeInvalidPartitions,
			expectedMsg:  "Invalid number of partitions: -1",
		},
		{
			name: "NewInvalidReplicationFactor",
			constructor: func() *KafkaError {
				return NewInvalidReplicationFactor(0)
			},
			expectedCode: ErrorCodeInvalidReplicationFactor,
			expectedMsg:  "Invalid replication factor: 0",
		},
		{
			name: "NewInvalidConfig",
			constructor: func() *KafkaError {
				return NewInvalidConfig("bad.config=value")
			},
			expectedCode: ErrorCodeInvalidConfig,
			expectedMsg:  "Invalid configuration: bad.config=value",
		},
		{
			name: "NewNetworkException",
			constructor: func() *KafkaError {
				return NewNetworkException("Connection reset")
			},
			expectedCode: ErrorCodeNetworkException,
			expectedMsg:  "Connection reset",
		},
		{
			name: "NewInvalidRequest",
			constructor: func() *KafkaError {
				return NewInvalidRequest("Missing required field")
			},
			expectedCode: ErrorCodeInvalidRequest,
			expectedMsg:  "Missing required field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor()
			if err.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, err.Code)
			}
			if err.Message != tt.expectedMsg {
				t.Errorf("Expected error message '%s', got '%s'", tt.expectedMsg, err.Message)
			}
		})
	}
}

func TestGetErrorForCode(t *testing.T) {
	tests := []struct {
		code         int16
		expectedCode int16
	}{
		{ErrorCodeNoError, ErrorCodeNoError},
		{ErrorCodeOffsetOutOfRange, ErrorCodeOffsetOutOfRange},
		{ErrorCodeUnknownMemberId, ErrorCodeUnknownMemberId},
		{ErrorCodeRebalanceInProgress, ErrorCodeRebalanceInProgress},
		{ErrorCodeRequestTimedOut, ErrorCodeRequestTimedOut},
		{9999, 9999}, // Unknown error code
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := GetErrorForCode(tt.code)
			if err.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, err.Code)
			}
		})
	}
}

// Benchmarks

func BenchmarkKafkaError_Error(b *testing.B) {
	err := NewUnknownTopicOrPartition("test-topic")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

func BenchmarkKafkaError_IsRetriable(b *testing.B) {
	err := NewRebalanceInProgress()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.IsRetriable()
	}
}

func BenchmarkKafkaError_IsFatal(b *testing.B) {
	err := NewTopicAuthorizationFailed("test-topic")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.IsFatal()
	}
}

func BenchmarkNewUnknownTopicOrPartition(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewUnknownTopicOrPartition("test-topic")
	}
}

func BenchmarkGetErrorForCode(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetErrorForCode(ErrorCodeRebalanceInProgress)
	}
}

