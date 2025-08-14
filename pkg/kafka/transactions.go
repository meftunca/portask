package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TransactionManager handles Kafka transactions for exactly-once semantics
type TransactionManager struct {
	transactionalID string
	producerID      int64
	producerEpoch   int16
	timeout         time.Duration
	state           TransactionState
	coordinator     *TransactionCoordinator
	mutex           sync.RWMutex
}

// TransactionCoordinator manages transaction coordination
type TransactionCoordinator struct {
	brokerID     int32
	host         string
	port         int32
	transactions map[string]*TransactionMetadata
	mutex        sync.RWMutex
}

// TransactionMetadata contains transaction metadata
type TransactionMetadata struct {
	TransactionalID   string
	ProducerID        int64
	ProducerEpoch     int16
	State             TransactionState
	TopicPartitions   map[string][]int32
	TxnStartTimestamp time.Time
	TxnTimeoutMs      int32
	LastUpdateTime    time.Time
}

// ProducerBatch represents a batch of messages in a transaction
type ProducerBatch struct {
	TopicPartition TopicPartition
	Messages       []TransactionalMessage
	FirstSequence  int32
	LastSequence   int32
	ProducerID     int64
	ProducerEpoch  int16
}

// TransactionalMessage represents a message within a transaction
type TransactionalMessage struct {
	Key       []byte
	Value     []byte
	Headers   map[string][]byte
	Timestamp time.Time
	Sequence  int32
}

// TransactionalProducer provides exactly-once delivery semantics
type TransactionalProducer struct {
	transactionManager *TransactionManager
	batches            map[TopicPartition]*ProducerBatch
	sequenceNumbers    map[TopicPartition]int32
	mutex              sync.RWMutex
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(transactionalID string, timeout time.Duration) *TransactionManager {
	return &TransactionManager{
		transactionalID: transactionalID,
		timeout:         timeout,
		state:           TransactionStateEmpty,
		coordinator:     NewTransactionCoordinator(),
	}
}

// NewTransactionCoordinator creates a new transaction coordinator
func NewTransactionCoordinator() *TransactionCoordinator {
	return &TransactionCoordinator{
		transactions: make(map[string]*TransactionMetadata),
	}
}

// NewTransactionalProducer creates a new transactional producer
func NewTransactionalProducer(transactionalID string, timeout time.Duration) *TransactionalProducer {
	return &TransactionalProducer{
		transactionManager: NewTransactionManager(transactionalID, timeout),
		batches:            make(map[TopicPartition]*ProducerBatch),
		sequenceNumbers:    make(map[TopicPartition]int32),
	}
}

// InitTransactions initializes the transaction manager
func (tm *TransactionManager) InitTransactions(ctx context.Context) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.state != TransactionStateEmpty {
		return fmt.Errorf("transaction manager already initialized")
	}

	// Request producer ID and epoch from coordinator
	producerID, epoch, err := tm.coordinator.InitProducerID(ctx, tm.transactionalID)
	if err != nil {
		return fmt.Errorf("failed to initialize producer ID: %w", err)
	}

	tm.producerID = producerID
	tm.producerEpoch = epoch
	tm.state = TransactionStateEmpty

	return nil
}

// BeginTransaction starts a new transaction
func (tm *TransactionManager) BeginTransaction(ctx context.Context) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.state != TransactionStateEmpty {
		return fmt.Errorf("transaction already in progress, state: %v", tm.state)
	}

	// Create transaction metadata
	metadata := &TransactionMetadata{
		TransactionalID:   tm.transactionalID,
		ProducerID:        tm.producerID,
		ProducerEpoch:     tm.producerEpoch,
		State:             TransactionStateOngoing,
		TopicPartitions:   make(map[string][]int32),
		TxnStartTimestamp: time.Now(),
		TxnTimeoutMs:      int32(tm.timeout.Milliseconds()),
		LastUpdateTime:    time.Now(),
	}

	if err := tm.coordinator.AddTransaction(metadata); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	tm.state = TransactionStateOngoing
	return nil
}

// AddPartitionsToTxn adds partitions to the current transaction
func (tm *TransactionManager) AddPartitionsToTxn(ctx context.Context, partitions map[string][]int32) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.state != TransactionStateOngoing {
		return fmt.Errorf("no ongoing transaction, state: %v", tm.state)
	}

	return tm.coordinator.AddPartitionsToTransaction(ctx, tm.transactionalID, partitions)
}

// CommitTransaction commits the current transaction
func (tm *TransactionManager) CommitTransaction(ctx context.Context) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.state != TransactionStateOngoing {
		return fmt.Errorf("no ongoing transaction to commit, state: %v", tm.state)
	}

	// Set state to prepare commit
	tm.state = TransactionStatePrepareCommit

	if err := tm.coordinator.EndTransaction(ctx, tm.transactionalID, true); err != nil {
		tm.state = TransactionStateOngoing // Rollback state on error
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	tm.state = TransactionStateCompleteCommit

	// Reset for next transaction
	tm.state = TransactionStateEmpty
	return nil
}

// AbortTransaction aborts the current transaction
func (tm *TransactionManager) AbortTransaction(ctx context.Context) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.state != TransactionStateOngoing && tm.state != TransactionStatePrepareCommit {
		return fmt.Errorf("no transaction to abort, state: %v", tm.state)
	}

	// Set state to prepare abort
	tm.state = TransactionStatePrepareAbort

	if err := tm.coordinator.EndTransaction(ctx, tm.transactionalID, false); err != nil {
		return fmt.Errorf("failed to abort transaction: %w", err)
	}

	tm.state = TransactionStateCompleteAbort

	// Reset for next transaction
	tm.state = TransactionStateEmpty
	return nil
}

// SendToTransaction sends a message within a transaction
func (tp *TransactionalProducer) SendToTransaction(ctx context.Context, topic string, partition int32, message TransactionalMessage) error {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	if tp.transactionManager.state != TransactionStateOngoing {
		return fmt.Errorf("no ongoing transaction")
	}

	topicPartition := TopicPartition{
		Topic:     topic,
		Partition: partition,
	}

	// Get or create batch for this topic-partition
	batch, exists := tp.batches[topicPartition]
	if !exists {
		sequence := tp.sequenceNumbers[topicPartition]
		batch = &ProducerBatch{
			TopicPartition: topicPartition,
			Messages:       make([]TransactionalMessage, 0),
			FirstSequence:  sequence,
			ProducerID:     tp.transactionManager.producerID,
			ProducerEpoch:  tp.transactionManager.producerEpoch,
		}
		tp.batches[topicPartition] = batch
	}

	// Add sequence number to message
	message.Sequence = tp.sequenceNumbers[topicPartition]
	tp.sequenceNumbers[topicPartition]++

	// Add message to batch
	batch.Messages = append(batch.Messages, message)
	batch.LastSequence = message.Sequence

	// Add partition to transaction if not already added
	partitions := map[string][]int32{
		topic: {partition},
	}

	return tp.transactionManager.AddPartitionsToTxn(ctx, partitions)
}

// FlushTransaction flushes all pending batches in the transaction
func (tp *TransactionalProducer) FlushTransaction(ctx context.Context) error {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	// Send all batches
	for topicPartition, batch := range tp.batches {
		if err := tp.sendBatch(ctx, topicPartition, batch); err != nil {
			return fmt.Errorf("failed to send batch for %v: %w", topicPartition, err)
		}
	}

	// Clear batches after successful send
	tp.batches = make(map[TopicPartition]*ProducerBatch)

	return nil
}

// Transaction coordinator methods

// InitProducerID initializes a producer ID for transactional producer
func (tc *TransactionCoordinator) InitProducerID(ctx context.Context, transactionalID string) (int64, int16, error) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	// In a real implementation, this would coordinate with Kafka brokers
	// For now, we generate a simple producer ID
	producerID := time.Now().UnixNano()
	epoch := int16(0)

	// Check if there's an existing transaction for this ID
	if existing, exists := tc.transactions[transactionalID]; exists {
		epoch = existing.ProducerEpoch + 1
	}

	return producerID, epoch, nil
}

// AddTransaction adds a new transaction
func (tc *TransactionCoordinator) AddTransaction(metadata *TransactionMetadata) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	tc.transactions[metadata.TransactionalID] = metadata
	return nil
}

// AddPartitionsToTransaction adds partitions to an existing transaction
func (tc *TransactionCoordinator) AddPartitionsToTransaction(ctx context.Context, transactionalID string, partitions map[string][]int32) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	metadata, exists := tc.transactions[transactionalID]
	if !exists {
		return fmt.Errorf("transaction not found: %s", transactionalID)
	}

	// Merge partitions
	for topic, partitionList := range partitions {
		if existing, exists := metadata.TopicPartitions[topic]; exists {
			// Merge partition lists (avoid duplicates)
			merged := append(existing, partitionList...)
			uniquePartitions := make([]int32, 0, len(merged))
			seen := make(map[int32]bool)

			for _, p := range merged {
				if !seen[p] {
					uniquePartitions = append(uniquePartitions, p)
					seen[p] = true
				}
			}
			metadata.TopicPartitions[topic] = uniquePartitions
		} else {
			metadata.TopicPartitions[topic] = partitionList
		}
	}

	metadata.LastUpdateTime = time.Now()
	return nil
}

// EndTransaction ends a transaction (commit or abort)
func (tc *TransactionCoordinator) EndTransaction(ctx context.Context, transactionalID string, commit bool) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	metadata, exists := tc.transactions[transactionalID]
	if !exists {
		return fmt.Errorf("transaction not found: %s", transactionalID)
	}

	if commit {
		metadata.State = TransactionStateCompleteCommit
	} else {
		metadata.State = TransactionStateCompleteAbort
	}

	metadata.LastUpdateTime = time.Now()

	// In a real implementation, this would coordinate with all brokers
	// to either commit or abort the transaction

	return nil
}

// GetTransactionState returns the current state of a transaction
func (tc *TransactionCoordinator) GetTransactionState(transactionalID string) (TransactionState, error) {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	metadata, exists := tc.transactions[transactionalID]
	if !exists {
		return TransactionStateDead, fmt.Errorf("transaction not found: %s", transactionalID)
	}

	return metadata.State, nil
}

// CleanupExpiredTransactions removes expired transactions
func (tc *TransactionCoordinator) CleanupExpiredTransactions(maxAge time.Duration) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	now := time.Now()
	for transactionalID, metadata := range tc.transactions {
		if now.Sub(metadata.LastUpdateTime) > maxAge {
			delete(tc.transactions, transactionalID)
		}
	}
}

// Helper methods

func (tp *TransactionalProducer) sendBatch(ctx context.Context, topicPartition TopicPartition, batch *ProducerBatch) error {
	// Implementation for sending batch to Kafka
	// This would serialize the batch and send it to the appropriate broker
	return nil
}

// GetState returns the current transaction state
func (tm *TransactionManager) GetState() TransactionState {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.state
}

// GetProducerID returns the producer ID
func (tm *TransactionManager) GetProducerID() int64 {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.producerID
}

// GetProducerEpoch returns the producer epoch
func (tm *TransactionManager) GetProducerEpoch() int16 {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.producerEpoch
}
