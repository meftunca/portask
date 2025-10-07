package kafka

import "fmt"

// KafkaError represents a Kafka protocol error
type KafkaError struct {
	Code    int16
	Message string
}

// Error implements the error interface
func (e *KafkaError) Error() string {
	return fmt.Sprintf("Kafka error %d: %s", e.Code, e.Message)
}

// Kafka Error Codes (as per Kafka protocol)
const (
	ErrorCodeNoError                           int16 = 0
	ErrorCodeOffsetOutOfRange                  int16 = 1
	ErrorCodeCorruptMessage                    int16 = 2
	ErrorCodeUnknownTopicOrPartition           int16 = 3
	ErrorCodeInvalidFetchSize                  int16 = 4
	ErrorCodeLeaderNotAvailable                int16 = 5
	ErrorCodeNotLeaderForPartition             int16 = 6
	ErrorCodeRequestTimedOut                   int16 = 7
	ErrorCodeBrokerNotAvailable                int16 = 8
	ErrorCodeReplicaNotAvailable               int16 = 9
	ErrorCodeMessageTooLarge                   int16 = 10
	ErrorCodeStaleControllerEpoch              int16 = 11
	ErrorCodeOffsetMetadataTooLarge            int16 = 12
	ErrorCodeNetworkException                  int16 = 13
	ErrorCodeCoordinatorLoadInProgress         int16 = 14
	ErrorCodeCoordinatorNotAvailable           int16 = 15
	ErrorCodeNotCoordinator                    int16 = 16
	ErrorCodeInvalidTopicException             int16 = 17
	ErrorCodeRecordListTooLarge                int16 = 18
	ErrorCodeNotEnoughReplicas                 int16 = 19
	ErrorCodeNotEnoughReplicasAfterAppend      int16 = 20
	ErrorCodeInvalidRequiredAcks               int16 = 21
	ErrorCodeIllegalGeneration                 int16 = 22
	ErrorCodeInconsistentGroupProtocol         int16 = 23
	ErrorCodeInvalidGroupId                    int16 = 24
	ErrorCodeUnknownMemberId                   int16 = 25
	ErrorCodeInvalidSessionTimeout             int16 = 26
	ErrorCodeRebalanceInProgress               int16 = 27
	ErrorCodeInvalidCommitOffsetSize           int16 = 28
	ErrorCodeTopicAuthorizationFailed          int16 = 29
	ErrorCodeGroupAuthorizationFailed          int16 = 30
	ErrorCodeClusterAuthorizationFailed        int16 = 31
	ErrorCodeInvalidTimestamp                  int16 = 32
	ErrorCodeUnsupportedSaslMechanism          int16 = 33
	ErrorCodeIllegalSaslState                  int16 = 34
	ErrorCodeUnsupportedVersion                int16 = 35
	ErrorCodeTopicAlreadyExists                int16 = 36
	ErrorCodeInvalidPartitions                 int16 = 37
	ErrorCodeInvalidReplicationFactor          int16 = 38
	ErrorCodeInvalidReplicaAssignment          int16 = 39
	ErrorCodeInvalidConfig                     int16 = 40
	ErrorCodeNotController                     int16 = 41
	ErrorCodeInvalidRequest                    int16 = 42
	ErrorCodeUnsupportedForMessageFormat       int16 = 43
	ErrorCodePolicyViolation                   int16 = 44
	ErrorCodeOutOfOrderSequenceNumber          int16 = 45
	ErrorCodeDuplicateSequenceNumber           int16 = 46
	ErrorCodeInvalidProducerEpoch              int16 = 47
	ErrorCodeInvalidTxnState                   int16 = 48
	ErrorCodeInvalidProducerIdMapping          int16 = 49
	ErrorCodeInvalidTransactionTimeout         int16 = 50
	ErrorCodeConcurrentTransactions            int16 = 51
	ErrorCodeTransactionCoordinatorFenced      int16 = 52
	ErrorCodeTransactionalIdAuthorizationFailed int16 = 53
	ErrorCodeSecurityDisabled                  int16 = 54
	ErrorCodeOperationNotAttempted             int16 = 55
	ErrorCodeKafkaStorageError                 int16 = 56
	ErrorCodeLogDirNotFound                    int16 = 57
	ErrorCodeSaslAuthenticationFailed          int16 = 58
	ErrorCodeUnknownProducerId                 int16 = 59
	ErrorCodeReassignmentInProgress            int16 = 60
	ErrorCodeDelegationTokenAuthDisabled       int16 = 61
	ErrorCodeDelegationTokenNotFound           int16 = 62
	ErrorCodeDelegationTokenOwnerMismatch      int16 = 63
	ErrorCodeDelegationTokenRequestNotAllowed  int16 = 64
	ErrorCodeDelegationTokenAuthorizationFailed int16 = 65
	ErrorCodeDelegationTokenExpired            int16 = 66
	ErrorCodeInvalidPrincipalType              int16 = 67
	ErrorCodeNonEmptyGroup                     int16 = 68
	ErrorCodeGroupIdNotFound                   int16 = 69
	ErrorCodeFetchSessionIdNotFound            int16 = 70
	ErrorCodeInvalidFetchSessionEpoch          int16 = 71
	ErrorCodeListenerNotFound                  int16 = 72
	ErrorCodeTopicDeletionDisabled             int16 = 73
	ErrorCodeFencedLeaderEpoch                 int16 = 74
	ErrorCodeUnknownLeaderEpoch                int16 = 75
	ErrorCodeUnsupportedCompressionType        int16 = 76
	ErrorCodeStaleBrokerEpoch                  int16 = 77
	ErrorCodeOffsetNotAvailable                int16 = 78
	ErrorCodeMemberIdRequired                  int16 = 79
	ErrorCodePreferredLeaderNotAvailable       int16 = 80
	ErrorCodeGroupMaxSizeReached               int16 = 81
	ErrorCodeFencedInstanceId                  int16 = 82
	ErrorCodeEligibleLeadersNotAvailable       int16 = 83
	ErrorCodeElectionNotNeeded                 int16 = 84
	ErrorCodeNoReassignmentInProgress          int16 = 85
	ErrorCodeGroupSubscribedToTopic            int16 = 86
	ErrorCodeInvalidRecord                     int16 = 87
	ErrorCodeUnstableOffsetCommit              int16 = 88
)

// Common error constructors

func NewNoError() *KafkaError {
	return &KafkaError{Code: ErrorCodeNoError, Message: "No error"}
}

func NewOffsetOutOfRange() *KafkaError {
	return &KafkaError{Code: ErrorCodeOffsetOutOfRange, Message: "The requested offset is not within the range of offsets maintained by the server"}
}

func NewUnknownTopicOrPartition(topic string) *KafkaError {
	return &KafkaError{Code: ErrorCodeUnknownTopicOrPartition, Message: fmt.Sprintf("Unknown topic or partition: %s", topic)}
}

func NewInvalidRequest(msg string) *KafkaError {
	return &KafkaError{Code: ErrorCodeInvalidRequest, Message: msg}
}

func NewUnknownMemberId(memberID string) *KafkaError {
	return &KafkaError{Code: ErrorCodeUnknownMemberId, Message: fmt.Sprintf("Unknown member ID: %s", memberID)}
}

func NewIllegalGeneration(expected, actual int32) *KafkaError {
	return &KafkaError{
		Code:    ErrorCodeIllegalGeneration,
		Message: fmt.Sprintf("Illegal generation: expected %d, got %d", expected, actual),
	}
}

func NewRebalanceInProgress() *KafkaError {
	return &KafkaError{Code: ErrorCodeRebalanceInProgress, Message: "The group is rebalancing, so a rejoin is needed"}
}

func NewInvalidGroupId(groupID string) *KafkaError {
	return &KafkaError{Code: ErrorCodeInvalidGroupId, Message: fmt.Sprintf("Invalid group ID: %s", groupID)}
}

func NewGroupIdNotFound(groupID string) *KafkaError {
	return &KafkaError{Code: ErrorCodeGroupIdNotFound, Message: fmt.Sprintf("Group not found: %s", groupID)}
}

func NewInvalidSessionTimeout() *KafkaError {
	return &KafkaError{Code: ErrorCodeInvalidSessionTimeout, Message: "The session timeout is not within the range allowed by the broker"}
}

func NewCoordinatorNotAvailable() *KafkaError {
	return &KafkaError{Code: ErrorCodeCoordinatorNotAvailable, Message: "The coordinator is not available"}
}

func NewNotCoordinator() *KafkaError {
	return &KafkaError{Code: ErrorCodeNotCoordinator, Message: "This server is not the coordinator"}
}

func NewTopicAuthorizationFailed(topic string) *KafkaError {
	return &KafkaError{Code: ErrorCodeTopicAuthorizationFailed, Message: fmt.Sprintf("Topic authorization failed: %s", topic)}
}

func NewGroupAuthorizationFailed(groupID string) *KafkaError {
	return &KafkaError{Code: ErrorCodeGroupAuthorizationFailed, Message: fmt.Sprintf("Group authorization failed: %s", groupID)}
}

func NewMessageTooLarge(size, max int64) *KafkaError {
	return &KafkaError{
		Code:    ErrorCodeMessageTooLarge,
		Message: fmt.Sprintf("Message size %d exceeds maximum %d", size, max),
	}
}

func NewRequestTimedOut() *KafkaError {
	return &KafkaError{Code: ErrorCodeRequestTimedOut, Message: "The request timed out"}
}

func NewUnsupportedVersion(version int16) *KafkaError {
	return &KafkaError{Code: ErrorCodeUnsupportedVersion, Message: fmt.Sprintf("Unsupported version: %d", version)}
}

func NewTopicAlreadyExists(topic string) *KafkaError {
	return &KafkaError{Code: ErrorCodeTopicAlreadyExists, Message: fmt.Sprintf("Topic already exists: %s", topic)}
}

func NewInvalidPartitions(partitions int32) *KafkaError {
	return &KafkaError{Code: ErrorCodeInvalidPartitions, Message: fmt.Sprintf("Invalid number of partitions: %d", partitions)}
}

func NewInvalidReplicationFactor(factor int16) *KafkaError {
	return &KafkaError{Code: ErrorCodeInvalidReplicationFactor, Message: fmt.Sprintf("Invalid replication factor: %d", factor)}
}

func NewInvalidConfig(config string) *KafkaError {
	return &KafkaError{Code: ErrorCodeInvalidConfig, Message: fmt.Sprintf("Invalid configuration: %s", config)}
}

func NewNetworkException(msg string) *KafkaError {
	return &KafkaError{Code: ErrorCodeNetworkException, Message: msg}
}

func NewCorruptMessage() *KafkaError {
	return &KafkaError{Code: ErrorCodeCorruptMessage, Message: "Message is corrupt (crc mismatch)"}
}

// IsRetriable returns whether the error is retriable
func (e *KafkaError) IsRetriable() bool {
	switch e.Code {
	case ErrorCodeLeaderNotAvailable,
		ErrorCodeNotLeaderForPartition,
		ErrorCodeRequestTimedOut,
		ErrorCodeNetworkException,
		ErrorCodeCoordinatorLoadInProgress,
		ErrorCodeCoordinatorNotAvailable,
		ErrorCodeNotCoordinator,
		ErrorCodeRebalanceInProgress:
		return true
	default:
		return false
	}
}

// IsFatal returns whether the error is fatal
func (e *KafkaError) IsFatal() bool {
	switch e.Code {
	case ErrorCodeTopicAuthorizationFailed,
		ErrorCodeGroupAuthorizationFailed,
		ErrorCodeClusterAuthorizationFailed,
		ErrorCodeInvalidGroupId,
		ErrorCodeInvalidTopicException,
		ErrorCodeUnsupportedVersion,
		ErrorCodeInvalidRequest:
		return true
	default:
		return false
	}
}

// GetErrorForCode returns a KafkaError for a given error code
func GetErrorForCode(code int16) *KafkaError {
	switch code {
	case ErrorCodeNoError:
		return NewNoError()
	case ErrorCodeOffsetOutOfRange:
		return NewOffsetOutOfRange()
	case ErrorCodeUnknownMemberId:
		return NewUnknownMemberId("")
	case ErrorCodeIllegalGeneration:
		return NewIllegalGeneration(0, 0)
	case ErrorCodeRebalanceInProgress:
		return NewRebalanceInProgress()
	case ErrorCodeInvalidSessionTimeout:
		return NewInvalidSessionTimeout()
	case ErrorCodeRequestTimedOut:
		return NewRequestTimedOut()
	case ErrorCodeCoordinatorNotAvailable:
		return NewCoordinatorNotAvailable()
	case ErrorCodeNotCoordinator:
		return NewNotCoordinator()
	default:
		return &KafkaError{Code: code, Message: "Unknown error"}
	}
}

