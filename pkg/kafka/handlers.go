package kafka

import (
	"bytes"
	"encoding/binary"
	"log"
)

// handleApiVersions handles API_VERSIONS requests
func (h *KafkaProtocolHandler) handleApiVersions(request *KafkaRequest) []byte {
	var buf bytes.Buffer

	// Error code (no error)
	binary.Write(&buf, binary.BigEndian, int16(NoError))

	// Number of API versions
	supportedAPIs := []struct {
		apiKey     int16
		minVersion int16
		maxVersion int16
	}{
		{ApiVersionsAPI, 0, 3},
		{MetadataAPI, 0, 9},
		{ProduceAPI, 0, 8},
		{FetchAPI, 0, 11},
		{ListOffsetsAPI, 0, 5},
		{CreateTopicsAPI, 0, 5},
		{DeleteTopicsAPI, 0, 4},
		{SaslHandshakeAPI, 0, 1},
		{SaslAuthenticateAPI, 0, 2},
	}

	binary.Write(&buf, binary.BigEndian, int32(len(supportedAPIs)))

	for _, api := range supportedAPIs {
		binary.Write(&buf, binary.BigEndian, api.apiKey)
		binary.Write(&buf, binary.BigEndian, api.minVersion)
		binary.Write(&buf, binary.BigEndian, api.maxVersion)
	}

	// Throttle time (0 ms)
	binary.Write(&buf, binary.BigEndian, int32(0))

	return buf.Bytes()
}

// handleMetadata handles METADATA requests
func (h *KafkaProtocolHandler) handleMetadata(request *KafkaRequest) []byte {
	var buf bytes.Buffer

	// Parse request
	reqBuf := bytes.NewReader(request.Body)

	// Read topics array
	var topicCount int32
	binary.Read(reqBuf, binary.BigEndian, &topicCount)

	var requestedTopics []string
	if topicCount > 0 {
		for i := int32(0); i < topicCount; i++ {
			topic, _ := h.readString(reqBuf)
			requestedTopics = append(requestedTopics, topic)
		}
	}

	// Get metadata from store
	metadata, err := h.messageStore.GetTopicMetadata(requestedTopics)
	if err != nil {
		// Return error response
		binary.Write(&buf, binary.BigEndian, int32(0))  // throttle time
		binary.Write(&buf, binary.BigEndian, int32(0))  // broker count
		binary.Write(&buf, binary.BigEndian, int32(-1)) // cluster id (null)
		binary.Write(&buf, binary.BigEndian, int32(0))  // controller id
		binary.Write(&buf, binary.BigEndian, int32(0))  // topic count
		return buf.Bytes()
	}

	// Build response

	// Throttle time
	binary.Write(&buf, binary.BigEndian, int32(0))

	// Brokers array
	binary.Write(&buf, binary.BigEndian, int32(1))    // broker count
	binary.Write(&buf, binary.BigEndian, int32(0))    // broker id
	h.writeString(&buf, "localhost")                  // host
	binary.Write(&buf, binary.BigEndian, int32(9092)) // port
	h.writeString(&buf, "")                           // rack (null)

	// Cluster ID (null)
	h.writeString(&buf, "")

	// Controller ID
	binary.Write(&buf, binary.BigEndian, int32(0))

	// Topics array
	if metadata != nil {
		binary.Write(&buf, binary.BigEndian, int32(1)) // topic count

		// Topic metadata
		binary.Write(&buf, binary.BigEndian, metadata.Error)
		h.writeString(&buf, metadata.Name)
		binary.Write(&buf, binary.BigEndian, int8(0)) // is_internal

		// Partitions array
		binary.Write(&buf, binary.BigEndian, int32(len(metadata.Partitions)))
		for _, partition := range metadata.Partitions {
			binary.Write(&buf, binary.BigEndian, partition.Error)
			binary.Write(&buf, binary.BigEndian, partition.ID)
			binary.Write(&buf, binary.BigEndian, partition.Leader)

			// Replica array
			binary.Write(&buf, binary.BigEndian, int32(len(partition.Replicas)))
			for _, replica := range partition.Replicas {
				binary.Write(&buf, binary.BigEndian, replica)
			}

			// ISR array
			binary.Write(&buf, binary.BigEndian, int32(len(partition.ISR)))
			for _, isr := range partition.ISR {
				binary.Write(&buf, binary.BigEndian, isr)
			}

			// Offline replicas array
			binary.Write(&buf, binary.BigEndian, int32(len(partition.OfflineReplicas)))
			for _, offline := range partition.OfflineReplicas {
				binary.Write(&buf, binary.BigEndian, offline)
			}
		}
	} else {
		binary.Write(&buf, binary.BigEndian, int32(0)) // topic count
	}

	return buf.Bytes()
}

// handleProduce handles PRODUCE requests
func (h *KafkaProtocolHandler) handleProduce(request *KafkaRequest) []byte {
	log.Printf("🔍 Handling Produce API request (API 0) - body size: %d bytes", len(request.Body))
	
	var buf bytes.Buffer

	// Parse request
	reqBuf := bytes.NewReader(request.Body)

	// Read required acks
	var requiredAcks int16
	binary.Read(reqBuf, binary.BigEndian, &requiredAcks)
	log.Printf("🔍 Produce request - requiredAcks: %d", requiredAcks)

	// Read timeout
	var timeout int32
	binary.Read(reqBuf, binary.BigEndian, &timeout)
	log.Printf("🔍 Produce request - timeout: %d", timeout)

	// Read topic data
	var topicCount int32
	binary.Read(reqBuf, binary.BigEndian, &topicCount)
	log.Printf("🔍 Produce request - topicCount: %d", topicCount)

	// Build response
	binary.Write(&buf, binary.BigEndian, int32(topicCount)) // topic count

	for i := int32(0); i < topicCount; i++ {
		// Read topic name
		topic, _ := h.readString(reqBuf)
		log.Printf("🔍 Produce request - topic[%d]: %s", i, topic)
		h.writeString(&buf, topic)

		// Read partition data
		var partitionCount int32
		binary.Read(reqBuf, binary.BigEndian, &partitionCount)
		log.Printf("🔍 Produce request - partitionCount for topic %s: %d", topic, partitionCount)
		binary.Write(&buf, binary.BigEndian, int32(partitionCount))

		for j := int32(0); j < partitionCount; j++ {
			// Read partition
			var partition int32
			binary.Read(reqBuf, binary.BigEndian, &partition)

			// Read message set size
			var messageSetSize int32
			binary.Read(reqBuf, binary.BigEndian, &messageSetSize)
			log.Printf("🔍 Produce request - partition %d, messageSetSize: %d", partition, messageSetSize)

			// Read messages (simplified - just skip for now)
			messageSet := make([]byte, messageSetSize)
			reqBuf.Read(messageSet)

			// For demo, just return success
			offset, err := h.messageStore.ProduceMessage(topic, partition, nil, messageSet)
			if err != nil {
				log.Printf("❌ Failed to produce message to %s[%d]: %v", topic, partition, err)
			} else {
				log.Printf("✅ Produced message to %s[%d] at offset %d", topic, partition, offset)
			}

			// Write partition response
			binary.Write(&buf, binary.BigEndian, partition)
			if err != nil {
				binary.Write(&buf, binary.BigEndian, int16(UnknownTopicOrPartition))
				binary.Write(&buf, binary.BigEndian, int64(-1)) // base offset
			} else {
				binary.Write(&buf, binary.BigEndian, int16(NoError))
				binary.Write(&buf, binary.BigEndian, offset) // base offset
			}
			binary.Write(&buf, binary.BigEndian, int64(-1)) // log append time
			binary.Write(&buf, binary.BigEndian, int64(-1)) // log start offset
		}
	}

	// Throttle time
	binary.Write(&buf, binary.BigEndian, int32(0))

	log.Printf("✅ Produce API response created - response size: %d bytes", buf.Len())

	return buf.Bytes()
}

// handleFetch handles FETCH requests
func (h *KafkaProtocolHandler) handleFetch(request *KafkaRequest) []byte {
	var buf bytes.Buffer

	// Parse request
	reqBuf := bytes.NewReader(request.Body)

	// Read replica id
	var replicaId int32
	binary.Read(reqBuf, binary.BigEndian, &replicaId)

	// Read max wait time
	var maxWaitTime int32
	binary.Read(reqBuf, binary.BigEndian, &maxWaitTime)

	// Read min bytes
	var minBytes int32
	binary.Read(reqBuf, binary.BigEndian, &minBytes)

	// Read max bytes (if version >= 3)
	var maxBytes int32 = 1024 * 1024 // default 1MB
	if request.Header.APIVersion >= 3 {
		binary.Read(reqBuf, binary.BigEndian, &maxBytes)
	}

	// Read topics
	var topicCount int32
	binary.Read(reqBuf, binary.BigEndian, &topicCount)

	// Build response
	binary.Write(&buf, binary.BigEndian, int32(0))          // throttle time
	binary.Write(&buf, binary.BigEndian, int32(topicCount)) // topic count

	for i := int32(0); i < topicCount; i++ {
		// Read topic name
		topic, _ := h.readString(reqBuf)
		h.writeString(&buf, topic)

		// Read partitions
		var partitionCount int32
		binary.Read(reqBuf, binary.BigEndian, &partitionCount)
		binary.Write(&buf, binary.BigEndian, int32(partitionCount))

		for j := int32(0); j < partitionCount; j++ {
			// Read partition request
			var partition int32
			var fetchOffset int64
			var partitionMaxBytes int32

			binary.Read(reqBuf, binary.BigEndian, &partition)
			binary.Read(reqBuf, binary.BigEndian, &fetchOffset)
			binary.Read(reqBuf, binary.BigEndian, &partitionMaxBytes)

			// Fetch messages
			messages, err := h.messageStore.ConsumeMessages(topic, partition, fetchOffset, partitionMaxBytes)

			// Write partition response
			binary.Write(&buf, binary.BigEndian, partition)
			if err != nil {
				binary.Write(&buf, binary.BigEndian, int16(UnknownTopicOrPartition))
				binary.Write(&buf, binary.BigEndian, int64(-1)) // high watermark
				binary.Write(&buf, binary.BigEndian, int64(-1)) // last stable offset
				binary.Write(&buf, binary.BigEndian, int64(-1)) // log start offset
				binary.Write(&buf, binary.BigEndian, int32(0))  // aborted transactions count
				binary.Write(&buf, binary.BigEndian, int32(0))  // record set size
			} else {
				binary.Write(&buf, binary.BigEndian, int16(NoError))
				binary.Write(&buf, binary.BigEndian, int64(1000)) // high watermark
				binary.Write(&buf, binary.BigEndian, int64(-1))   // last stable offset
				binary.Write(&buf, binary.BigEndian, int64(0))    // log start offset
				binary.Write(&buf, binary.BigEndian, int32(0))    // aborted transactions count

				// Build record set (simplified)
				var recordBuf bytes.Buffer
				for _, msg := range messages {
					// Write simplified record
					binary.Write(&recordBuf, binary.BigEndian, msg.Offset)
					h.writeBytes(&recordBuf, msg.Key)
					h.writeBytes(&recordBuf, msg.Value)
				}

				binary.Write(&buf, binary.BigEndian, int32(recordBuf.Len()))
				buf.Write(recordBuf.Bytes())
			}
		}
	}

	return buf.Bytes()
}

// handleListOffsets handles LIST_OFFSETS requests
func (h *KafkaProtocolHandler) handleListOffsets(request *KafkaRequest) []byte {
	var buf bytes.Buffer

	// For simplicity, return empty response
	binary.Write(&buf, binary.BigEndian, int32(0)) // throttle time
	binary.Write(&buf, binary.BigEndian, int32(0)) // topic count

	return buf.Bytes()
}

// handleCreateTopics handles CREATE_TOPICS requests
func (h *KafkaProtocolHandler) handleCreateTopics(request *KafkaRequest) []byte {
	var buf bytes.Buffer

	// Parse request
	reqBuf := bytes.NewReader(request.Body)

	// Read topics
	var topicCount int32
	binary.Read(reqBuf, binary.BigEndian, &topicCount)

	// Build response
	binary.Write(&buf, binary.BigEndian, int32(0)) // throttle time
	binary.Write(&buf, binary.BigEndian, int32(topicCount))

	for i := int32(0); i < topicCount; i++ {
		// Read topic data
		topic, _ := h.readString(reqBuf)
		var numPartitions int32
		var replicationFactor int16
		binary.Read(reqBuf, binary.BigEndian, &numPartitions)
		binary.Read(reqBuf, binary.BigEndian, &replicationFactor)

		// Skip replica assignments and configs arrays
		var assignmentCount int32
		binary.Read(reqBuf, binary.BigEndian, &assignmentCount)
		// Skip assignments...

		var configCount int32
		binary.Read(reqBuf, binary.BigEndian, &configCount)
		// Skip configs...

		// Create topic
		err := h.messageStore.CreateTopic(topic, numPartitions, replicationFactor)

		// Write topic response
		h.writeString(&buf, topic)
		if err != nil {
			binary.Write(&buf, binary.BigEndian, int16(TopicAlreadyExists))
		} else {
			binary.Write(&buf, binary.BigEndian, int16(NoError))
		}
		h.writeString(&buf, "") // error message (null)
	}

	return buf.Bytes()
}

// handleDeleteTopics handles DELETE_TOPICS requests
func (h *KafkaProtocolHandler) handleDeleteTopics(request *KafkaRequest) []byte {
	var buf bytes.Buffer

	// Parse request
	reqBuf := bytes.NewReader(request.Body)

	// Read topics
	var topicCount int32
	binary.Read(reqBuf, binary.BigEndian, &topicCount)

	// Build response
	binary.Write(&buf, binary.BigEndian, int32(0)) // throttle time
	binary.Write(&buf, binary.BigEndian, int32(topicCount))

	for i := int32(0); i < topicCount; i++ {
		topic, _ := h.readString(reqBuf)

		// Delete topic
		err := h.messageStore.DeleteTopic(topic)

		// Write topic response
		h.writeString(&buf, topic)
		if err != nil {
			binary.Write(&buf, binary.BigEndian, int16(UnknownTopicOrPartition))
		} else {
			binary.Write(&buf, binary.BigEndian, int16(NoError))
		}
	}

	return buf.Bytes()
}

// handleSaslHandshake handles SASL_HANDSHAKE requests
func (h *KafkaProtocolHandler) handleSaslHandshake(request *KafkaRequest) []byte {
	var buf bytes.Buffer

	// Error code
	binary.Write(&buf, binary.BigEndian, int16(NoError))

	// Supported mechanisms
	mechanisms := []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"}
	binary.Write(&buf, binary.BigEndian, int32(len(mechanisms)))

	for _, mechanism := range mechanisms {
		h.writeString(&buf, mechanism)
	}

	return buf.Bytes()
}

// handleSaslAuthenticate handles SASL_AUTHENTICATE requests
func (h *KafkaProtocolHandler) handleSaslAuthenticate(request *KafkaRequest) []byte {
	var buf bytes.Buffer

	// For demo purposes, always succeed
	binary.Write(&buf, binary.BigEndian, int16(NoError)) // error code
	h.writeString(&buf, "")                              // error message (null)
	h.writeBytes(&buf, []byte("auth-success"))           // auth bytes
	binary.Write(&buf, binary.BigEndian, int64(0))       // session lifetime

	return buf.Bytes()
}

// Eksik handler fonksiyonları
// DescribeGroups
// TODO: handleDescribeGroups fonksiyonu protocol.go'da eklendi
// OffsetCommit
// TODO: handleOffsetCommit fonksiyonu protocol.go'da eklendi
// OffsetFetch
// TODO: handleOffsetFetch fonksiyonu protocol.go'da eklendi
// FindCoordinator
// TODO: handleFindCoordinator fonksiyonu protocol.go'da eklendi
