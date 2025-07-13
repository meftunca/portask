package types

import (
	"fmt"
	"runtime"
	"time"
)

// ErrorCode represents standardized error codes
type ErrorCode string

const (
	// Message related errors
	ErrCodeInvalidMessage   ErrorCode = "INVALID_MESSAGE"
	ErrCodeMessageExpired   ErrorCode = "MESSAGE_EXPIRED"
	ErrCodeMessageTooLarge  ErrorCode = "MESSAGE_TOO_LARGE"
	ErrCodeMessageNotFound  ErrorCode = "MESSAGE_NOT_FOUND"
	ErrCodeDuplicateMessage ErrorCode = "DUPLICATE_MESSAGE"

	// Topic/Queue related errors
	ErrCodeTopicNotFound     ErrorCode = "TOPIC_NOT_FOUND"
	ErrCodeTopicExists       ErrorCode = "TOPIC_EXISTS"
	ErrCodeInvalidTopic      ErrorCode = "INVALID_TOPIC"
	ErrCodePartitionNotFound ErrorCode = "PARTITION_NOT_FOUND"

	// Consumer related errors
	ErrCodeConsumerNotFound ErrorCode = "CONSUMER_NOT_FOUND"
	ErrCodeConsumerExists   ErrorCode = "CONSUMER_EXISTS"
	ErrCodeInvalidConsumer  ErrorCode = "INVALID_CONSUMER"
	ErrCodeConsumerTimeout  ErrorCode = "CONSUMER_TIMEOUT"

	// Storage related errors
	ErrCodeStorageError     ErrorCode = "STORAGE_ERROR"
	ErrCodeStorageTimeout   ErrorCode = "STORAGE_TIMEOUT"
	ErrCodeStorageFull      ErrorCode = "STORAGE_FULL"
	ErrCodeStorageCorrupted ErrorCode = "STORAGE_CORRUPTED"

	// Network related errors
	ErrCodeNetworkError    ErrorCode = "NETWORK_ERROR"
	ErrCodeConnectionLost  ErrorCode = "CONNECTION_LOST"
	ErrCodeTimeout         ErrorCode = "TIMEOUT"
	ErrCodeInvalidProtocol ErrorCode = "INVALID_PROTOCOL"

	// Serialization related errors
	ErrCodeSerializationError   ErrorCode = "SERIALIZATION_ERROR"
	ErrCodeDeserializationError ErrorCode = "DESERIALIZATION_ERROR"
	ErrCodeCompressionError     ErrorCode = "COMPRESSION_ERROR"
	ErrCodeDecompressionError   ErrorCode = "DECOMPRESSION_ERROR"

	// Configuration related errors
	ErrCodeInvalidConfig  ErrorCode = "INVALID_CONFIG"
	ErrCodeConfigNotFound ErrorCode = "CONFIG_NOT_FOUND"

	// Authentication related errors
	ErrCodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden          ErrorCode = "FORBIDDEN"
	ErrCodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"

	// System related errors
	ErrCodeSystemError       ErrorCode = "SYSTEM_ERROR"
	ErrCodeMemoryError       ErrorCode = "MEMORY_ERROR"
	ErrCodeResourceExhausted ErrorCode = "RESOURCE_EXHAUSTED"

	// ID related errors
	ErrCodeInvalidULID ErrorCode = "INVALID_ULID"
	ErrCodeInvalidID   ErrorCode = "INVALID_ID"
)

// PortaskError represents a structured error in Portask
type PortaskError struct {
	Code      ErrorCode              `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Stack     string                 `json:"stack,omitempty"`
	Cause     error                  `json:"cause,omitempty"`
}

// NewPortaskError creates a new PortaskError
func NewPortaskError(code ErrorCode, message string) *PortaskError {
	return &PortaskError{
		Code:      code,
		Message:   message,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// NewPortaskErrorWithCause creates a new PortaskError with a cause
func NewPortaskErrorWithCause(code ErrorCode, message string, cause error) *PortaskError {
	err := NewPortaskError(code, message)
	err.Cause = cause
	return err
}

// WithDetail adds a detail to the error
func (e *PortaskError) WithDetail(key string, value interface{}) *PortaskError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithStack captures the current stack trace
func (e *PortaskError) WithStack() *PortaskError {
	e.Stack = captureStack()
	return e
}

// Error implements the error interface
func (e *PortaskError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause (for Go 1.13+ error wrapping)
func (e *PortaskError) Unwrap() error {
	return e.Cause
}

// IsCode checks if this error is of a specific code
func (e *PortaskError) IsCode(code ErrorCode) bool {
	return e.Code == code
}

// Is implements the Go 1.13+ error interface for error comparison
func (e *PortaskError) Is(target error) bool {
	if pe, ok := target.(*PortaskError); ok {
		return e.Code == pe.Code
	}
	return false
}

// IsRetryable returns true if the error is retryable
func (e *PortaskError) IsRetryable() bool {
	switch e.Code {
	case ErrCodeStorageTimeout, ErrCodeNetworkError, ErrCodeConnectionLost, ErrCodeTimeout:
		return true
	default:
		return false
	}
}

// IsPermanent returns true if the error is permanent and should not be retried
func (e *PortaskError) IsPermanent() bool {
	switch e.Code {
	case ErrCodeInvalidMessage, ErrCodeInvalidTopic, ErrCodeInvalidConsumer,
		ErrCodeInvalidProtocol, ErrCodeInvalidConfig, ErrCodeUnauthorized,
		ErrCodeForbidden, ErrCodeInvalidCredentials, ErrCodeInvalidULID, ErrCodeInvalidID:
		return true
	default:
		return false
	}
}

// captureStack captures the current stack trace
func captureStack() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// Common error constructors for frequently used errors

// ErrInvalidMessage creates an invalid message error
func ErrInvalidMessage(details string) *PortaskError {
	return NewPortaskError(ErrCodeInvalidMessage, "invalid message").WithDetail("details", details)
}

// ErrMessageExpired creates a message expired error
func ErrMessageExpired(messageID MessageID) *PortaskError {
	return NewPortaskError(ErrCodeMessageExpired, "message has expired").WithDetail("message_id", messageID)
}

// ErrMessageTooLarge creates a message too large error
func ErrMessageTooLarge(size, maxSize int) *PortaskError {
	return NewPortaskError(ErrCodeMessageTooLarge, "message size exceeds limit").
		WithDetail("size", size).
		WithDetail("max_size", maxSize)
}

// ErrTopicNotFound creates a topic not found error
func ErrTopicNotFound(topic TopicName) *PortaskError {
	return NewPortaskError(ErrCodeTopicNotFound, "topic not found").WithDetail("topic", topic)
}

// ErrConsumerNotFound creates a consumer not found error
func ErrConsumerNotFound(consumerID ConsumerID) *PortaskError {
	return NewPortaskError(ErrCodeConsumerNotFound, "consumer not found").WithDetail("consumer_id", consumerID)
}

// ErrStorageError creates a storage error
func ErrStorageError(operation string, cause error) *PortaskError {
	return NewPortaskErrorWithCause(ErrCodeStorageError, fmt.Sprintf("storage operation failed: %s", operation), cause)
}

// ErrSerializationError creates a serialization error
func ErrSerializationError(format string, cause error) *PortaskError {
	return NewPortaskErrorWithCause(ErrCodeSerializationError, fmt.Sprintf("serialization failed for format: %s", format), cause).
		WithDetail("format", format)
}

// ErrDeserializationError creates a deserialization error
func ErrDeserializationError(format string, cause error) *PortaskError {
	return NewPortaskErrorWithCause(ErrCodeDeserializationError, fmt.Sprintf("deserialization failed for format: %s", format), cause).
		WithDetail("format", format)
}

// ErrCompressionError creates a compression error
func ErrCompressionError(algorithm string, cause error) *PortaskError {
	return NewPortaskErrorWithCause(ErrCodeCompressionError, fmt.Sprintf("compression failed for algorithm: %s", algorithm), cause).
		WithDetail("algorithm", algorithm)
}

// ErrDecompressionError creates a decompression error
func ErrDecompressionError(algorithm string, cause error) *PortaskError {
	return NewPortaskErrorWithCause(ErrCodeDecompressionError, fmt.Sprintf("decompression failed for algorithm: %s", algorithm), cause).
		WithDetail("algorithm", algorithm)
}

// ErrNetworkError creates a network error
func ErrNetworkError(operation string, cause error) *PortaskError {
	return NewPortaskErrorWithCause(ErrCodeNetworkError, fmt.Sprintf("network operation failed: %s", operation), cause).
		WithDetail("operation", operation)
}

// ErrTimeout creates a timeout error
func ErrTimeout(operation string, timeout time.Duration) *PortaskError {
	return NewPortaskError(ErrCodeTimeout, fmt.Sprintf("operation timed out: %s", operation)).
		WithDetail("operation", operation).
		WithDetail("timeout", timeout.String())
}

// ErrUnauthorized creates an unauthorized error
func ErrUnauthorized(resource string) *PortaskError {
	return NewPortaskError(ErrCodeUnauthorized, "access denied").WithDetail("resource", resource)
}

// ErrResourceExhausted creates a resource exhausted error
func ErrResourceExhausted(resource string, limit interface{}) *PortaskError {
	return NewPortaskError(ErrCodeResourceExhausted, fmt.Sprintf("resource exhausted: %s", resource)).
		WithDetail("resource", resource).
		WithDetail("limit", limit)
}

// ErrorCollector collects multiple errors
type ErrorCollector struct {
	Errors []error
}

// NewErrorCollector creates a new error collector
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		Errors: make([]error, 0),
	}
}

// Add adds an error to the collector
func (ec *ErrorCollector) Add(err error) {
	if err != nil {
		ec.Errors = append(ec.Errors, err)
	}
}

// HasErrors returns true if there are any errors
func (ec *ErrorCollector) HasErrors() bool {
	return len(ec.Errors) > 0
}

// Error returns a combined error message
func (ec *ErrorCollector) Error() string {
	if !ec.HasErrors() {
		return ""
	}

	if len(ec.Errors) == 1 {
		return ec.Errors[0].Error()
	}

	result := fmt.Sprintf("multiple errors (%d):", len(ec.Errors))
	for i, err := range ec.Errors {
		result += fmt.Sprintf("\n  %d: %v", i+1, err)
	}

	return result
}

// ToError returns the collected errors as a single error, or nil if no errors
func (ec *ErrorCollector) ToError() error {
	if !ec.HasErrors() {
		return nil
	}
	return ec
}

// Clear clears all collected errors
func (ec *ErrorCollector) Clear() {
	ec.Errors = ec.Errors[:0]
}
