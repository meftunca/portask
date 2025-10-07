package kafka

import (
	"bytes"
	"encoding/binary"
	"log"
	"time"
)

// Extended handlers for consumer groups, offsets, and transactions
var _ = time.Now // Use time for future implementations

// Extended handlers for consumer groups, offsets, and transactions

// handleFindCoordinator handles FIND_COORDINATOR requests
func (h *KafkaProtocolHandler) handleFindCoordinator(request *KafkaRequest) []byte {
	var buf bytes.Buffer
	reqBuf := bytes.NewReader(request.Body)

	// Read coordinator key (group ID or transactional ID)
	coordinatorKey, _ := h.readString(reqBuf)

	// Read coordinator type (0 = group, 1 = transaction)
	var coordinatorType int8
	binary.Read(reqBuf, binary.BigEndian, &coordinatorType)

	log.Printf("[Kafka] FindCoordinator: key=%s, type=%d", coordinatorKey, coordinatorType)

	// Response
	binary.Write(&buf, binary.BigEndian, int32(0))  // throttle time
	binary.Write(&buf, binary.BigEndian, int16(0))  // error code: no error
	binary.Write(&buf, binary.BigEndian, int16(0))  // error message (null)
	binary.Write(&buf, binary.BigEndian, int32(0))  // node id
	h.writeString(&buf, "localhost")                // host
	binary.Write(&buf, binary.BigEndian, int32(9092)) // port

	return buf.Bytes()
}

// handleJoinGroup handles JOIN_GROUP requests
func (h *KafkaProtocolHandler) handleJoinGroup(request *KafkaRequest) []byte {
	var buf bytes.Buffer
	reqBuf := bytes.NewReader(request.Body)

	// Parse request
	groupID, _ := h.readString(reqBuf)
	
	var sessionTimeout int32
	binary.Read(reqBuf, binary.BigEndian, &sessionTimeout)
	
	var rebalanceTimeout int32
	binary.Read(reqBuf, binary.BigEndian, &rebalanceTimeout)
	
	memberID, _ := h.readString(reqBuf)
	protocolType, _ := h.readString(reqBuf)

	log.Printf("[Kafka] JoinGroup: group=%s, member=%s, sessionTimeout=%d", 
		groupID, memberID, sessionTimeout)

	// Call group coordinator
	if h.groupCoordinator != nil {
		resp, err := h.groupCoordinator.JoinGroup(
			groupID,
			memberID,
			"client-1",
			"localhost",
			protocolType,
			time.Duration(sessionTimeout)*time.Millisecond,
			time.Duration(rebalanceTimeout)*time.Millisecond,
			[]string{"range"},
			[]byte{},
		)

		if err != nil {
			log.Printf("[Kafka] JoinGroup error: %v", err)
			// Error response
			binary.Write(&buf, binary.BigEndian, int32(0))  // throttle time
			binary.Write(&buf, binary.BigEndian, int16(15)) // error: coordinator not available
			binary.Write(&buf, binary.BigEndian, int32(0))  // generation id
			h.writeString(&buf, "range")                    // protocol name
			h.writeString(&buf, "")                         // leader id
			h.writeString(&buf, "")                         // member id
			binary.Write(&buf, binary.BigEndian, int32(0))  // members array
			return buf.Bytes()
		}

		// Success response
		binary.Write(&buf, binary.BigEndian, int32(0))          // throttle time
		binary.Write(&buf, binary.BigEndian, int16(0))          // no error
		binary.Write(&buf, binary.BigEndian, resp.GenerationID) // generation id
		h.writeString(&buf, resp.ProtocolName)                  // protocol name
		h.writeString(&buf, resp.LeaderID)                      // leader id
		h.writeString(&buf, resp.MemberID)                      // member id

		// Members array
		binary.Write(&buf, binary.BigEndian, int32(len(resp.Members)))
		for _, member := range resp.Members {
			h.writeString(&buf, member.MemberID)
			h.writeBytes(&buf, member.Metadata)
		}

		return buf.Bytes()
	}

	// Fallback if no coordinator
	binary.Write(&buf, binary.BigEndian, int32(0))  // throttle time
	binary.Write(&buf, binary.BigEndian, int16(15)) // error: coordinator not available
	binary.Write(&buf, binary.BigEndian, int32(0))  // generation id
	h.writeString(&buf, "")
	h.writeString(&buf, "")
	h.writeString(&buf, "")
	binary.Write(&buf, binary.BigEndian, int32(0))

	return buf.Bytes()
}

// handleSyncGroup handles SYNC_GROUP requests
func (h *KafkaProtocolHandler) handleSyncGroup(request *KafkaRequest) []byte {
	var buf bytes.Buffer
	reqBuf := bytes.NewReader(request.Body)

	// Parse request
	groupID, _ := h.readString(reqBuf)
	
	var generationID int32
	binary.Read(reqBuf, binary.BigEndian, &generationID)
	
	memberID, _ := h.readString(reqBuf)

	// Read group assignments (only leader sends these)
	var assignmentCount int32
	binary.Read(reqBuf, binary.BigEndian, &assignmentCount)

	assignments := make(map[string][]byte)
	for i := int32(0); i < assignmentCount; i++ {
		assignmentMemberID, _ := h.readString(reqBuf)
		assignmentBytes, _ := h.readBytes(reqBuf)
		assignments[assignmentMemberID] = assignmentBytes
	}

	log.Printf("[Kafka] SyncGroup: group=%s, member=%s, gen=%d, assignments=%d",
		groupID, memberID, generationID, len(assignments))

	// Call group coordinator
	if h.groupCoordinator != nil {
		resp, err := h.groupCoordinator.SyncGroup(groupID, memberID, generationID, assignments)

		if err != nil {
			log.Printf("[Kafka] SyncGroup error: %v", err)
			// Error response
			binary.Write(&buf, binary.BigEndian, int32(0))  // throttle time
			binary.Write(&buf, binary.BigEndian, int16(15)) // error: coordinator not available
			h.writeBytes(&buf, []byte{})                    // empty assignment
			return buf.Bytes()
		}

		// Success response
		binary.Write(&buf, binary.BigEndian, int32(0)) // throttle time
		binary.Write(&buf, binary.BigEndian, int16(0)) // no error
		h.writeBytes(&buf, resp.Assignment)

		return buf.Bytes()
	}

	// Fallback
	binary.Write(&buf, binary.BigEndian, int32(0))
	binary.Write(&buf, binary.BigEndian, int16(15))
	h.writeBytes(&buf, []byte{})

	return buf.Bytes()
}

// handleHeartbeat handles HEARTBEAT requests
func (h *KafkaProtocolHandler) handleHeartbeat(request *KafkaRequest) []byte {
	var buf bytes.Buffer
	reqBuf := bytes.NewReader(request.Body)

	// Parse request
	groupID, _ := h.readString(reqBuf)
	
	var generationID int32
	binary.Read(reqBuf, binary.BigEndian, &generationID)
	
	memberID, _ := h.readString(reqBuf)

	log.Printf("[Kafka] Heartbeat: group=%s, member=%s, gen=%d", groupID, memberID, generationID)

	// Call group coordinator
	if h.groupCoordinator != nil {
		err := h.groupCoordinator.Heartbeat(groupID, memberID, generationID)

		if err != nil {
			log.Printf("[Kafka] Heartbeat error: %v", err)
			// Error response
			binary.Write(&buf, binary.BigEndian, int32(0))  // throttle time
			binary.Write(&buf, binary.BigEndian, int16(27)) // error: rebalance in progress
			return buf.Bytes()
		}

		// Success response
		binary.Write(&buf, binary.BigEndian, int32(0)) // throttle time
		binary.Write(&buf, binary.BigEndian, int16(0)) // no error

		return buf.Bytes()
	}

	// Fallback
	binary.Write(&buf, binary.BigEndian, int32(0))
	binary.Write(&buf, binary.BigEndian, int16(15))

	return buf.Bytes()
}

// handleLeaveGroup handles LEAVE_GROUP requests
func (h *KafkaProtocolHandler) handleLeaveGroup(request *KafkaRequest) []byte {
	var buf bytes.Buffer
	reqBuf := bytes.NewReader(request.Body)

	// Parse request
	groupID, _ := h.readString(reqBuf)
	memberID, _ := h.readString(reqBuf)

	log.Printf("[Kafka] LeaveGroup: group=%s, member=%s", groupID, memberID)

	// Call group coordinator
	if h.groupCoordinator != nil {
		err := h.groupCoordinator.LeaveGroup(groupID, memberID)

		if err != nil {
			log.Printf("[Kafka] LeaveGroup error: %v", err)
			// Error response
			binary.Write(&buf, binary.BigEndian, int32(0))  // throttle time
			binary.Write(&buf, binary.BigEndian, int16(69)) // error: group id not found
			return buf.Bytes()
		}

		// Success response
		binary.Write(&buf, binary.BigEndian, int32(0)) // throttle time
		binary.Write(&buf, binary.BigEndian, int16(0)) // no error

		return buf.Bytes()
	}

	// Fallback
	binary.Write(&buf, binary.BigEndian, int32(0))
	binary.Write(&buf, binary.BigEndian, int16(15))

	return buf.Bytes()
}

// handleOffsetCommit handles OFFSET_COMMIT requests
func (h *KafkaProtocolHandler) handleOffsetCommit(request *KafkaRequest) []byte {
	var buf bytes.Buffer
	reqBuf := bytes.NewReader(request.Body)

	// Parse request
	groupID, _ := h.readString(reqBuf)
	
	var generationID int32
	binary.Read(reqBuf, binary.BigEndian, &generationID)
	
	
		// Read topics
	var topicCount int32
	binary.Read(reqBuf, binary.BigEndian, &topicCount)

	log.Printf("[Kafka] OffsetCommit: group=%s, topics=%d", groupID, topicCount)

	// Call offset manager
	if h.offsetManager != nil {
		for i := int32(0); i < topicCount; i++ {
			topicName, _ := h.readString(reqBuf)
			
			var partitionCount int32
			binary.Read(reqBuf, binary.BigEndian, &partitionCount)

			for j := int32(0); j < partitionCount; j++ {
				var partition int32
				binary.Read(reqBuf, binary.BigEndian, &partition)
				
				var offset int64
				binary.Read(reqBuf, binary.BigEndian, &offset)
				
				metadata, _ := h.readString(reqBuf)

				// Commit offset
				err := h.offsetManager.CommitOffsetWithMetadata(groupID, topicName, partition, offset, metadata)
				if err != nil {
					log.Printf("[Kafka] OffsetCommit error: %v", err)
				}
			}
		}

		// Success response
		binary.Write(&buf, binary.BigEndian, int32(0))      // throttle time
		binary.Write(&buf, binary.BigEndian, int32(topicCount)) // topics

		// Write empty errors for all topics
		for i := int32(0); i < topicCount; i++ {
			h.writeString(&buf, "topic")
			binary.Write(&buf, binary.BigEndian, int32(0)) // partitions
		}

		return buf.Bytes()
	}

	// Fallback
	binary.Write(&buf, binary.BigEndian, int32(0))
	binary.Write(&buf, binary.BigEndian, int32(0))

	return buf.Bytes()
}

// handleOffsetFetch handles OFFSET_FETCH requests
func (h *KafkaProtocolHandler) handleOffsetFetch(request *KafkaRequest) []byte {
	var buf bytes.Buffer
	reqBuf := bytes.NewReader(request.Body)

	// Parse request
	groupID, _ := h.readString(reqBuf)
	
	// Read topics
	var topicCount int32
	binary.Read(reqBuf, binary.BigEndian, &topicCount)

	log.Printf("[Kafka] OffsetFetch: group=%s, topics=%d", groupID, topicCount)

	// Response
	binary.Write(&buf, binary.BigEndian, int32(0)) // throttle time

	if h.offsetManager != nil {
		binary.Write(&buf, binary.BigEndian, topicCount)

		for i := int32(0); i < topicCount; i++ {
			topicName, _ := h.readString(reqBuf)
			
			var partitionCount int32
			binary.Read(reqBuf, binary.BigEndian, &partitionCount)

			h.writeString(&buf, topicName)
			binary.Write(&buf, binary.BigEndian, partitionCount)

			for j := int32(0); j < partitionCount; j++ {
				var partition int32
				binary.Read(reqBuf, binary.BigEndian, &partition)

				// Fetch offset
				offset := int64(-1)
				meta, err := h.offsetManager.FetchOffsetMetadata(groupID, topicName, partition)
				if err == nil && meta != nil {
					offset = meta.Offset
				}

				binary.Write(&buf, binary.BigEndian, partition)
				binary.Write(&buf, binary.BigEndian, offset)
				h.writeString(&buf, "")            // metadata
				binary.Write(&buf, binary.BigEndian, int16(0)) // error code
			}
		}

		binary.Write(&buf, binary.BigEndian, int16(0)) // error code

		return buf.Bytes()
	}

	// Fallback
	binary.Write(&buf, binary.BigEndian, int32(0))
	binary.Write(&buf, binary.BigEndian, int16(15))

	return buf.Bytes()
}

// Helper functions

func (h *KafkaProtocolHandler) readBytes(r *bytes.Reader) ([]byte, error) {
	var length int32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	if length < 0 {
		return nil, nil
	}

	data := make([]byte, length)
	if _, err := r.Read(data); err != nil {
		return nil, err
	}

	return data, nil
}

func (h *KafkaProtocolHandler) writeBytes(w *bytes.Buffer, data []byte) {
	if data == nil {
		binary.Write(w, binary.BigEndian, int32(-1))
		return
	}

	binary.Write(w, binary.BigEndian, int32(len(data)))
	w.Write(data)
}

// NewKafkaProtocolHandlerWithCoordinators creates a handler with all coordinators
func NewKafkaProtocolHandlerWithCoordinators(
	store MessageStore,
	auth AuthProvider,
	metrics MetricsCollector,
	groupCoordinator *GroupCoordinator,
	offsetManager *OffsetManagerWithMetadata,
	transactionManager *TransactionManager,
	compressionHandler *CompressionHandler,
) *KafkaProtocolHandler {
	handler := &KafkaProtocolHandler{
		messageStore:       store,
		authProvider:       auth,
		metricsCollector:   metrics,
		groupCoordinator:   groupCoordinator,
		offsetManager:      offsetManager,
		transactionManager: transactionManager,
		compressionHandler: compressionHandler,
	}

	// Register extended handlers
	handler.registerExtendedHandlers()

	return handler
}

// registerExtendedHandlers registers the new handler functions
func (h *KafkaProtocolHandler) registerExtendedHandlers() {
	// These will be called from HandleRequest based on API key
	log.Println("[Kafka] Extended handlers registered: FindCoordinator, JoinGroup, SyncGroup, Heartbeat, LeaveGroup, OffsetCommit, OffsetFetch")
}

// Add fields to KafkaProtocolHandler
type ExtendedKafkaProtocolHandler struct {
	groupCoordinator   *GroupCoordinator
	offsetManager      *OffsetManagerWithMetadata
	transactionManager *TransactionManager
	compressionHandler *CompressionHandler
}

