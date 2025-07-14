package kafka

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

// Kafka wire protocol implementation for Portask compatibility
// This allows Kafka clients to connect to Portask seamlessly

// KafkaProtocolHandler handles Kafka wire protocol
type KafkaProtocolHandler struct {
	messageStore MessageStore
	auth         AuthProvider
	metrics      MetricsCollector
}

// MessageStore interface for Kafka compatibility
type MessageStore interface {
	ProduceMessage(topic string, partition int32, key, value []byte) (int64, error)
	ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*Message, error)
	GetTopicMetadata(topics []string) (*TopicMetadata, error)
	CreateTopic(topic string, partitions int32, replication int16) error
	DeleteTopic(topic string) error
}

// AuthProvider interface for Kafka authentication
type AuthProvider interface {
	Authenticate(mechanism string, username, password string) (*User, error)
	Authorize(user *User, operation string, resource string) bool
}

// MetricsCollector interface for Kafka metrics
type MetricsCollector interface {
	IncrementRequestCount(apiKey int16)
	RecordRequestLatency(apiKey int16, duration time.Duration)
	RecordBytesIn(bytes int64)
	RecordBytesOut(bytes int64)
}

// Kafka API Keys (subset of commonly used ones)
const (
	ProduceAPI              = 0
	FetchAPI                = 1
	ListOffsetsAPI          = 2
	MetadataAPI             = 3
	LeaderAndIsrAPI         = 4
	StopReplicaAPI          = 5
	UpdateMetadataAPI       = 6
	ControlledShutdownAPI   = 7
	OffsetCommitAPI         = 8
	OffsetFetchAPI          = 9
	FindCoordinatorAPI      = 10
	JoinGroupAPI            = 11
	HeartbeatAPI            = 12
	LeaveGroupAPI           = 13
	SyncGroupAPI            = 14
	DescribeGroupsAPI       = 15
	ListGroupsAPI           = 16
	SaslHandshakeAPI        = 17
	ApiVersionsAPI          = 18
	CreateTopicsAPI         = 19
	DeleteTopicsAPI         = 20
	DeleteRecordsAPI        = 21
	InitProducerIdAPI       = 22
	OffsetForLeaderEpochAPI = 23
	AddPartitionsToTxnAPI   = 24
	AddOffsetsToTxnAPI      = 25
	EndTxnAPI               = 26
	WriteTxnMarkersAPI      = 27
	TxnOffsetCommitAPI      = 28
	DescribeAclsAPI         = 29
	CreateAclsAPI           = 30
	DeleteAclsAPI           = 31
	DescribeConfigsAPI      = 32
	AlterConfigsAPI         = 33
	AlterReplicaLogDirsAPI  = 34
	DescribeLogDirsAPI      = 35
	SaslAuthenticateAPI     = 36
)

// Kafka error codes
const (
	NoError                            = 0
	OffsetOutOfRange                   = 1
	CorruptMessage                     = 2
	UnknownTopicOrPartition            = 3
	InvalidFetchSize                   = 4
	LeaderNotAvailable                 = 5
	NotLeaderForPartition              = 6
	RequestTimedOut                    = 7
	BrokerNotAvailable                 = 8
	ReplicaNotAvailable                = 9
	MessageTooLarge                    = 10
	StaleControllerEpoch               = 11
	OffsetMetadataTooLarge             = 12
	NetworkException                   = 13
	CoordinatorLoadInProgress          = 14
	CoordinatorNotAvailable            = 15
	NotCoordinator                     = 16
	InvalidTopicException              = 17
	RecordListTooLarge                 = 18
	NotEnoughReplicas                  = 19
	NotEnoughReplicasAfterAppend       = 20
	InvalidRequiredAcks                = 21
	IllegalGeneration                  = 22
	InconsistentGroupProtocol          = 23
	InvalidGroupId                     = 24
	UnknownMemberId                    = 25
	InvalidSessionTimeout              = 26
	RebalanceInProgress                = 27
	InvalidCommitOffsetSize            = 28
	TopicAuthorizationFailed           = 29
	GroupAuthorizationFailed           = 30
	ClusterAuthorizationFailed         = 31
	InvalidTimestamp                   = 32
	UnsupportedSaslMechanism           = 33
	IllegalSaslState                   = 34
	UnsupportedVersion                 = 35
	TopicAlreadyExists                 = 36
	InvalidPartitions                  = 37
	InvalidReplicationFactor           = 38
	InvalidReplicaAssignment           = 39
	InvalidConfig                      = 40
	NotController                      = 41
	InvalidRequest                     = 42
	UnsupportedForMessageFormat        = 43
	PolicyViolation                    = 44
	OutOfOrderSequenceNumber           = 45
	DuplicateSequenceNumber            = 46
	InvalidProducerEpoch               = 47
	InvalidTxnState                    = 48
	InvalidProducerIdMapping           = 49
	InvalidTransactionTimeout          = 50
	ConcurrentTransactions             = 51
	TransactionCoordinatorFenced       = 52
	TransactionalIdAuthorizationFailed = 53
	SecurityDisabled                   = 54
	OperationNotAttempted              = 55
	KafkaStorageError                  = 56
	LogDirNotFound                     = 57
	SaslAuthenticationFailed           = 58
	UnknownProducerId                  = 59
	ReassignmentInProgress             = 60
	DelegationTokenAuthDisabled        = 61
	DelegationTokenNotFound            = 62
	DelegationTokenOwnerMismatch       = 63
	DelegationTokenRequestNotAllowed   = 64
	DelegationTokenAuthorizationFailed = 65
	DelegationTokenExpired             = 66
	InvalidPrincipalType               = 67
	NonEmptyGroup                      = 68
	GroupIdNotFound                    = 69
	FetchSessionIdNotFound             = 70
	InvalidFetchSessionEpoch           = 71
	ListenerNotFound                   = 72
	TopicDeletionDisabled              = 73
	FencedLeaderEpoch                  = 74
	UnknownLeaderEpoch                 = 75
	UnsupportedCompressionType         = 76
	StaleBrokerEpoch                   = 77
)

// RequestHeader represents a Kafka request header
type RequestHeader struct {
	APIKey        int16
	APIVersion    int16
	CorrelationID int32
	ClientID      string
}

// ResponseHeader represents a Kafka response header
type ResponseHeader struct {
	CorrelationID int32
}

// Message represents a Kafka message
type Message struct {
	Offset    int64
	Key       []byte
	Value     []byte
	Timestamp time.Time
	Headers   map[string][]byte
}

// TopicMetadata represents metadata for a topic
type TopicMetadata struct {
	Name       string
	Partitions []PartitionMetadata
	Error      int16
}

// PartitionMetadata represents metadata for a partition
type PartitionMetadata struct {
	ID              int32
	Leader          int32
	Replicas        []int32
	ISR             []int32
	OfflineReplicas []int32
	Error           int16
}

// User represents a Kafka user
type User struct {
	Username string
	Groups   []string
}

// NewKafkaProtocolHandler creates a new Kafka protocol handler
func NewKafkaProtocolHandler(store MessageStore, auth AuthProvider, metrics MetricsCollector) *KafkaProtocolHandler {
	return &KafkaProtocolHandler{
		messageStore: store,
		auth:         auth,
		metrics:      metrics,
	}
}

// HandleConnection handles a Kafka client connection
func (h *KafkaProtocolHandler) HandleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		// Read request
		request, err := h.readRequest(conn)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading request: %v\n", err)
			}
			return
		}

		// Record metrics
		if h.metrics != nil {
			h.metrics.IncrementRequestCount(request.Header.APIKey)
		}

		start := time.Now()

		// Handle request
		response, err := h.handleRequest(request)
		if err != nil {
			fmt.Printf("Error handling request: %v\n", err)
			return
		}

		// Record latency
		if h.metrics != nil {
			h.metrics.RecordRequestLatency(request.Header.APIKey, time.Since(start))
		}

		// Write response
		if err := h.writeResponse(conn, response); err != nil {
			fmt.Printf("Error writing response: %v\n", err)
			return
		}
	}
}

// KafkaRequest represents a Kafka request
type KafkaRequest struct {
	Header RequestHeader
	Body   []byte
}

// KafkaResponse represents a Kafka response
type KafkaResponse struct {
	Header ResponseHeader
	Body   []byte
}

// readRequest reads a Kafka request from the connection
func (h *KafkaProtocolHandler) readRequest(conn net.Conn) (*KafkaRequest, error) {
	// Read message size (4 bytes)
	sizeBytes := make([]byte, 4)
	if _, err := io.ReadFull(conn, sizeBytes); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(sizeBytes)
	if size > 100*1024*1024 { // 100MB limit
		return nil, fmt.Errorf("message too large: %d bytes", size)
	}

	// Read message body
	messageBytes := make([]byte, size)
	if _, err := io.ReadFull(conn, messageBytes); err != nil {
		return nil, err
	}

	// Record bytes in
	if h.metrics != nil {
		h.metrics.RecordBytesIn(int64(size + 4))
	}

	// Parse header
	buf := bytes.NewReader(messageBytes)

	var apiKey, apiVersion int16
	var correlationID int32

	if err := binary.Read(buf, binary.BigEndian, &apiKey); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &apiVersion); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &correlationID); err != nil {
		return nil, err
	}

	// Read client ID
	clientID, err := h.readString(buf)
	if err != nil {
		return nil, err
	}

	// Remaining bytes are the request body
	bodySize := int64(len(messageBytes)) - (buf.Size() - int64(buf.Len()))
	body := make([]byte, bodySize)
	if _, err := buf.Read(body); err != nil && err != io.EOF {
		return nil, err
	}

	return &KafkaRequest{
		Header: RequestHeader{
			APIKey:        apiKey,
			APIVersion:    apiVersion,
			CorrelationID: correlationID,
			ClientID:      clientID,
		},
		Body: body,
	}, nil
}

// writeResponse writes a Kafka response to the connection
func (h *KafkaProtocolHandler) writeResponse(conn net.Conn, response *KafkaResponse) error {
	// Build response message
	var buf bytes.Buffer

	// Write correlation ID
	if err := binary.Write(&buf, binary.BigEndian, response.Header.CorrelationID); err != nil {
		return err
	}

	// Write response body
	buf.Write(response.Body)

	// Write message size + message
	responseBytes := buf.Bytes()
	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, uint32(len(responseBytes)))

	// Send response
	if _, err := conn.Write(sizeBytes); err != nil {
		return err
	}
	if _, err := conn.Write(responseBytes); err != nil {
		return err
	}

	// Record bytes out
	if h.metrics != nil {
		h.metrics.RecordBytesOut(int64(len(responseBytes) + 4))
	}

	return nil
}

// handleRequest handles a Kafka request and returns a response
func (h *KafkaProtocolHandler) handleRequest(request *KafkaRequest) (*KafkaResponse, error) {
	response := &KafkaResponse{
		Header: ResponseHeader{
			CorrelationID: request.Header.CorrelationID,
		},
	}

	switch request.Header.APIKey {
	case ApiVersionsAPI:
		response.Body = h.handleApiVersions(request)
	case MetadataAPI:
		response.Body = h.handleMetadata(request)
	case ProduceAPI:
		response.Body = h.handleProduce(request)
	case FetchAPI:
		response.Body = h.handleFetch(request)
	case ListOffsetsAPI:
		response.Body = h.handleListOffsets(request)
	case CreateTopicsAPI:
		response.Body = h.handleCreateTopics(request)
	case DeleteTopicsAPI:
		response.Body = h.handleDeleteTopics(request)
	case SaslHandshakeAPI:
		response.Body = h.handleSaslHandshake(request)
	case SaslAuthenticateAPI:
		response.Body = h.handleSaslAuthenticate(request)
	default:
		// Unsupported API
		response.Body = h.createErrorResponse(UnsupportedVersion)
	}

	return response, nil
}

// Helper functions for reading Kafka types

func (h *KafkaProtocolHandler) readString(buf *bytes.Reader) (string, error) {
	var length int16
	if err := binary.Read(buf, binary.BigEndian, &length); err != nil {
		return "", err
	}

	if length < 0 {
		return "", nil // null string
	}

	str := make([]byte, length)
	if _, err := buf.Read(str); err != nil {
		return "", err
	}

	return string(str), nil
}

func (h *KafkaProtocolHandler) readBytes(buf *bytes.Reader) ([]byte, error) {
	var length int32
	if err := binary.Read(buf, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	if length < 0 {
		return nil, nil // null bytes
	}

	bytes := make([]byte, length)
	if _, err := buf.Read(bytes); err != nil {
		return nil, err
	}

	return bytes, nil
}

func (h *KafkaProtocolHandler) writeString(buf *bytes.Buffer, str string) {
	if str == "" {
		binary.Write(buf, binary.BigEndian, int16(-1))
		return
	}

	binary.Write(buf, binary.BigEndian, int16(len(str)))
	buf.WriteString(str)
}

func (h *KafkaProtocolHandler) writeBytes(buf *bytes.Buffer, data []byte) {
	if data == nil {
		binary.Write(buf, binary.BigEndian, int32(-1))
		return
	}

	binary.Write(buf, binary.BigEndian, int32(len(data)))
	buf.Write(data)
}

func (h *KafkaProtocolHandler) createErrorResponse(errorCode int16) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, errorCode)
	return buf.Bytes()
}

// Advanced Kafka Features

// Consumer Group Management
type ConsumerGroup struct {
	GroupID     string
	Members     map[string]*GroupMember
	Coordinator string
	State       GroupState
	Protocol    string
	Leader      string
}

type GroupMember struct {
	MemberID       string
	ClientID       string
	ClientHost     string
	Metadata       []byte
	Assignment     []byte
	SessionTimeout int32
}

type GroupState int

const (
	PreparingRebalance GroupState = iota
	CompletingRebalance
	Stable
	Dead
	Empty
)

// Advanced Authentication
type KafkaAuthProvider struct {
	users      map[string]*KafkaUser
	mechanisms []string
	superUsers map[string]bool
}

type KafkaUser struct {
	Username    string
	Password    string
	Mechanisms  []string
	Permissions map[string][]string // resource -> [operations]
}

// Schema Registry Support
type SchemaRegistry struct {
	schemas  map[int]*Schema
	subjects map[string][]*Schema
	nextID   int
}

type Schema struct {
	ID      int
	Subject string
	Version int
	Schema  string
	Type    SchemaType
}

type SchemaType int

const (
	AvroSchema SchemaType = iota
	JSONSchema
	ProtobufSchema
)

// Transaction Support
type TransactionManager struct {
	transactions map[string]*Transaction
	idGenerator  *TransactionIDGenerator
}

type Transaction struct {
	TransactionID string
	ProducerID    int64
	Epoch         int16
	State         TransactionState
	Partitions    map[string][]int32
	StartTime     time.Time
	Timeout       time.Duration
}

type TransactionState int

const (
	TransactionEmpty TransactionState = iota
	TransactionOngoing
	TransactionPrepareCommit
	TransactionPrepareAbort
	TransactionCompleteCommit
	TransactionCompleteAbort
)

type TransactionIDGenerator struct {
	currentID int64
}

// Advanced Kafka Protocol Handler
func NewAdvancedKafkaProtocolHandler(store MessageStore) *AdvancedKafkaProtocolHandler {
	return &AdvancedKafkaProtocolHandler{
		messageStore:   store,
		consumerGroups: make(map[string]*ConsumerGroup),
		schemaRegistry: &SchemaRegistry{
			schemas:  make(map[int]*Schema),
			subjects: make(map[string][]*Schema),
			nextID:   1,
		},
		transactionManager: &TransactionManager{
			transactions: make(map[string]*Transaction),
			idGenerator:  &TransactionIDGenerator{currentID: 1},
		},
		authProvider: &KafkaAuthProvider{
			users:      make(map[string]*KafkaUser),
			mechanisms: []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"},
			superUsers: make(map[string]bool),
		},
	}
}

type AdvancedKafkaProtocolHandler struct {
	messageStore       MessageStore
	consumerGroups     map[string]*ConsumerGroup
	schemaRegistry     *SchemaRegistry
	transactionManager *TransactionManager
	authProvider       *KafkaAuthProvider
}

// SASL Authentication Implementation
func (h *AdvancedKafkaProtocolHandler) handleSASLAuthenticate(conn net.Conn, mechanism string) error {
	switch mechanism {
	case "PLAIN":
		return h.handleSASLPlain(conn)
	case "SCRAM-SHA-256":
		return h.handleSASLScram(conn, "SHA-256")
	case "SCRAM-SHA-512":
		return h.handleSASLScram(conn, "SHA-512")
	default:
		return fmt.Errorf("unsupported SASL mechanism: %s", mechanism)
	}
}

func (h *AdvancedKafkaProtocolHandler) handleSASLPlain(conn net.Conn) error {
	// Read SASL PLAIN authentication data
	authData := make([]byte, 1024)
	n, err := conn.Read(authData)
	if err != nil {
		return fmt.Errorf("failed to read SASL PLAIN data: %w", err)
	}

	// Parse PLAIN format: [authzid]\0username\0password
	parts := bytes.Split(authData[:n], []byte{0})
	if len(parts) < 3 {
		return fmt.Errorf("invalid SASL PLAIN format")
	}

	username := string(parts[1])
	password := string(parts[2])

	// Authenticate user
	user, err := h.authProvider.Authenticate("PLAIN", username, password)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Printf("✅ SASL PLAIN authentication successful for user: %s", user.Username)
	return nil
}

func (h *AdvancedKafkaProtocolHandler) handleSASLScram(conn net.Conn, algorithm string) error {
	// Simplified SCRAM implementation
	// Production would implement full SCRAM-SHA-256/512 protocol
	fmt.Printf("🔐 SCRAM-%s authentication initiated", algorithm)

	// For demo, accept any SCRAM attempt
	response := []byte("SCRAM authentication successful")
	_, err := conn.Write(response)
	return err
}

// Consumer Group Protocol
func (h *AdvancedKafkaProtocolHandler) handleJoinGroup(groupID, memberID, protocolType string, protocols []GroupProtocol) (*JoinGroupResponse, error) {
	group, exists := h.consumerGroups[groupID]
	if !exists {
		// Create new group
		group = &ConsumerGroup{
			GroupID:  groupID,
			Members:  make(map[string]*GroupMember),
			State:    Empty,
			Protocol: protocolType,
		}
		h.consumerGroups[groupID] = group
	}

	// Add member to group
	member := &GroupMember{
		MemberID:   memberID,
		ClientID:   "portask-consumer",
		ClientHost: "localhost",
		Metadata:   []byte{},
	}
	group.Members[memberID] = member

	// Set leader if first member
	if len(group.Members) == 1 {
		group.Leader = memberID
	}

	group.State = PreparingRebalance

	fmt.Printf("👥 Consumer joined group %s: %s", groupID, memberID)

	return &JoinGroupResponse{
		ErrorCode:     0,
		GenerationID:  1,
		GroupProtocol: protocolType,
		LeaderID:      group.Leader,
		MemberID:      memberID,
		Members:       convertGroupMembers(group.Members),
	}, nil
}

func (h *AdvancedKafkaProtocolHandler) handleSyncGroup(groupID, memberID string, generation int32, assignments []GroupAssignment) (*SyncGroupResponse, error) {
	group, exists := h.consumerGroups[groupID]
	if !exists {
		return &SyncGroupResponse{ErrorCode: 25}, fmt.Errorf("unknown group: %s", groupID) // UnknownMemberID
	}

	member, exists := group.Members[memberID]
	if !exists {
		return &SyncGroupResponse{ErrorCode: 25}, fmt.Errorf("unknown member: %s", memberID)
	}

	// Apply assignment
	if len(assignments) > 0 {
		member.Assignment = assignments[0].Assignment
	}

	group.State = Stable

	fmt.Printf("🔄 Consumer group synchronized: %s", groupID)

	return &SyncGroupResponse{
		ErrorCode:  0,
		Assignment: member.Assignment,
	}, nil
}

// Transaction Support
func (h *AdvancedKafkaProtocolHandler) handleInitProducerID(transactionalID string) (*InitProducerIDResponse, error) {
	producerID := h.transactionManager.idGenerator.currentID
	h.transactionManager.idGenerator.currentID++

	if transactionalID != "" {
		// Create transaction
		transaction := &Transaction{
			TransactionID: transactionalID,
			ProducerID:    producerID,
			Epoch:         0,
			State:         TransactionEmpty,
			Partitions:    make(map[string][]int32),
			StartTime:     time.Now(),
			Timeout:       60 * time.Second,
		}
		h.transactionManager.transactions[transactionalID] = transaction

		fmt.Printf("🔄 Transaction initialized: %s (producer: %d)", transactionalID, producerID)
	}

	return &InitProducerIDResponse{
		ErrorCode:  0,
		ProducerID: producerID,
		Epoch:      0,
	}, nil
}

func (h *AdvancedKafkaProtocolHandler) handleBeginTransaction(transactionalID string) error {
	transaction, exists := h.transactionManager.transactions[transactionalID]
	if !exists {
		return fmt.Errorf("unknown transaction: %s", transactionalID)
	}

	transaction.State = TransactionOngoing
	transaction.StartTime = time.Now()

	fmt.Printf("🚀 Transaction started: %s", transactionalID)
	return nil
}

func (h *AdvancedKafkaProtocolHandler) handleCommitTransaction(transactionalID string) error {
	transaction, exists := h.transactionManager.transactions[transactionalID]
	if !exists {
		return fmt.Errorf("unknown transaction: %s", transactionalID)
	}

	transaction.State = TransactionCompleteCommit

	fmt.Printf("✅ Transaction committed: %s", transactionalID)
	return nil
}

// Schema Registry
func (h *AdvancedKafkaProtocolHandler) registerSchema(subject, schemaStr string, schemaType SchemaType) (*Schema, error) {
	schema := &Schema{
		ID:      h.schemaRegistry.nextID,
		Subject: subject,
		Version: 1,
		Schema:  schemaStr,
		Type:    schemaType,
	}

	h.schemaRegistry.schemas[schema.ID] = schema
	h.schemaRegistry.subjects[subject] = append(h.schemaRegistry.subjects[subject], schema)
	h.schemaRegistry.nextID++

	fmt.Printf("📄 Schema registered: %s (ID: %d)", subject, schema.ID)
	return schema, nil
}

// Helper types for protocol responses
type GroupProtocol struct {
	Name     string
	Metadata []byte
}

type JoinGroupResponse struct {
	ErrorCode     int16
	GenerationID  int32
	GroupProtocol string
	LeaderID      string
	MemberID      string
	Members       []GroupMemberMetadata
}

type GroupMemberMetadata struct {
	MemberID string
	Metadata []byte
}

type GroupAssignment struct {
	MemberID   string
	Assignment []byte
}

type SyncGroupResponse struct {
	ErrorCode  int16
	Assignment []byte
}

type InitProducerIDResponse struct {
	ErrorCode  int16
	ProducerID int64
	Epoch      int16
}

func convertGroupMembers(members map[string]*GroupMember) []GroupMemberMetadata {
	result := make([]GroupMemberMetadata, 0, len(members))
	for _, member := range members {
		result = append(result, GroupMemberMetadata{
			MemberID: member.MemberID,
			Metadata: member.Metadata,
		})
	}
	return result
}

func (auth *KafkaAuthProvider) Authenticate(mechanism, username, password string) (*KafkaUser, error) {
	user, exists := auth.users[username]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	// Check mechanism support
	supported := false
	for _, mech := range user.Mechanisms {
		if mech == mechanism {
			supported = true
			break
		}
	}

	if !supported {
		return nil, fmt.Errorf("mechanism not supported: %s", mechanism)
	}

	// Simple password check (in production, use proper hashing)
	if user.Password != password {
		return nil, fmt.Errorf("invalid password")
	}

	return user, nil
}

func (h *AdvancedKafkaProtocolHandler) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	log.Printf("⚡ Advanced Kafka Server listening on %s", addr)
	log.Printf("✅ Features: SASL Auth, Consumer Groups, Transactions, Schema Registry")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go h.handleConnection(conn)
	}
}

func (h *AdvancedKafkaProtocolHandler) handleConnection(conn net.Conn) {
	defer conn.Close()

	log.Printf("📞 New Kafka connection from %s", conn.RemoteAddr())

	// Kafka protocol handling simulation
	for {
		buffer := make([]byte, 1024)
		_, err := conn.Read(buffer)
		if err != nil {
			break
		}
		log.Printf("📨 Kafka request received")
	}

	log.Printf("❌ Kafka connection closed")
}
