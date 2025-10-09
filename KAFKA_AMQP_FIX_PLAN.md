# 🔧 KAFKA & AMQP CLIENT LIBRARY UYUMLULUK FİX PLANI

**Tarih:** 9 Ekim 2025  
**Hedef:** kafka-go ve amqp client library'lerini %100 uyumlu hale getirmek

---

## 🐛 TESPİT EDİLEN SORUNLAR

### 1️⃣ Kafka Produce API - Response Format Hatası

**Sorun:**
```
❌ kafka-go client: "unexpected EOF"
```

**Root Cause:**
`pkg/kafka/handlers.go:160` - `handleProduce()` fonksiyonunda response format eksik:

```go
// ❌ MEVCUT KOD:
binary.Write(&buf, binary.BigEndian, int32(topicCount)) // YANLIŞ!

// ✅ OLMASI GEREKEN:
// Throttle time must be FIRST in response
binary.Write(&buf, binary.BigEndian, int32(0)) // throttle_time_ms
binary.Write(&buf, binary.BigEndian, int32(topicCount)) // topic responses
```

**Kafka Produce Response Format (v0-v8):**
```
Produce Response => 
  throttle_time_ms: int32         // ÖNCE BU!
  responses: [TopicProduceResponse]
```

---

### 2️⃣ Kafka Fetch API - Response Format Hatası

**Sorun:**
```
❌ kafka-go consumer: "context deadline exceeded" (no messages)
```

**Root Cause:**
`pkg/kafka/handlers.go:267` - `handleFetch()` incomplete implementation

**Kafka Fetch Response Format:**
```
Fetch Response =>
  throttle_time_ms: int32
  error_code: int16
  session_id: int32
  responses: [TopicFetchResponse]
    topic: string
    partitions: [PartitionFetchResponse]
      partition_index: int32
      error_code: int16
      high_watermark: int64
      last_stable_offset: int64
      log_start_offset: int64
      record_batches: RecordBatch    // MİSSİNG!
```

---

### 3️⃣ AMQP Handshake - Sequence Incomplete

**Sorun:**
```
❌ amqp client: "Exception (501) Reason: EOF"
```

**Root Cause:**
`pkg/amqp/server.go:452` - `sendConnectionStart()` sadece minimal frame gönderiyor

**AMQP 0.9.1 Connection Handshake Sequence:**
```
Client → Server: Protocol Header (AMQP\x00\x00\x09\x01)
Server → Client: Connection.Start        ✅ MEVCUT
Client → Server: Connection.StartOk      ❌ HANDLE EKSİK
Server → Client: Connection.Tune         ❌ EKSİK
Client → Server: Connection.TuneOk       ❌ HANDLE EKSİK
Client → Server: Connection.Open         ❌ HANDLE EKSİK
Server → Client: Connection.OpenOk       ❌ EKSİK
```

**Current State:**
- Server sadece Connection.Start gönderiyor
- Client'ın StartOk cevabını handle etmiyor
- Tune/Open sequence tamamen eksik

---

## 🔧 FİX PLANI

### Sprint 1: Kafka Produce/Fetch Fix (2-3 gün)

#### Fix 1.1: Kafka Produce Response Format ✅

**Dosya:** `pkg/kafka/handlers.go:160-263`

```go
func (h *KafkaProtocolHandler) handleProduce(request *KafkaRequest) []byte {
    var buf bytes.Buffer
    reqBuf := bytes.NewReader(request.Body)

    // Parse request
    var requiredAcks int16
    binary.Read(reqBuf, binary.BigEndian, &requiredAcks)
    
    var timeout int32
    binary.Read(reqBuf, binary.BigEndian, &timeout)
    
    var topicCount int32
    binary.Read(reqBuf, binary.BigEndian, &topicCount)

    // ✅ FIX: Throttle time MUST be first!
    binary.Write(&buf, binary.BigEndian, int32(0)) // throttle_time_ms

    // Build topic responses
    binary.Write(&buf, binary.BigEndian, int32(topicCount))

    for i := int32(0); i < topicCount; i++ {
        topic, _ := h.readString(reqBuf)
        h.writeString(&buf, topic)

        var partitionCount int32
        binary.Read(reqBuf, binary.BigEndian, &partitionCount)
        binary.Write(&buf, binary.BigEndian, int32(partitionCount))

        for j := int32(0); j < partitionCount; j++ {
            var partition int32
            binary.Read(reqBuf, binary.BigEndian, &partition)

            var messageSetSize int32
            binary.Read(reqBuf, binary.BigEndian, &messageSetSize)

            messageSet := make([]byte, messageSetSize)
            reqBuf.Read(messageSet)

            // Store message
            offset, err := h.messageStore.ProduceMessage(topic, partition, nil, messageSet)

            // Write partition response
            binary.Write(&buf, binary.BigEndian, partition)
            
            if err != nil {
                binary.Write(&buf, binary.BigEndian, int16(UnknownTopicOrPartition))
                binary.Write(&buf, binary.BigEndian, int64(-1))
            } else {
                binary.Write(&buf, binary.BigEndian, int16(NoError))
                binary.Write(&buf, binary.BigEndian, offset)
            }
            
            // ✅ FIX: Add missing fields
            binary.Write(&buf, binary.BigEndian, int64(-1)) // log_append_time
            binary.Write(&buf, binary.BigEndian, int64(-1)) // log_start_offset
        }
    }

    return buf.Bytes()
}
```

---

#### Fix 1.2: Kafka Fetch Response with RecordBatch ✅

**Dosya:** `pkg/kafka/handlers.go:267-523`

```go
func (h *KafkaProtocolHandler) handleFetch(request *KafkaRequest) []byte {
    var buf bytes.Buffer
    reqBuf := bytes.NewReader(request.Body)

    // Parse request
    var replicaId, maxWaitTime, minBytes, maxBytes int32
    binary.Read(reqBuf, binary.BigEndian, &replicaId)
    binary.Read(reqBuf, binary.BigEndian, &maxWaitTime)
    binary.Read(reqBuf, binary.BigEndian, &minBytes)
    if request.Header.APIVersion >= 3 {
        binary.Read(reqBuf, binary.BigEndian, &maxBytes)
    }

    var topicCount int32
    binary.Read(reqBuf, binary.BigEndian, &topicCount)

    // ✅ FIX: Proper response header
    binary.Write(&buf, binary.BigEndian, int32(0))  // throttle_time_ms
    binary.Write(&buf, binary.BigEndian, int16(0))  // error_code
    binary.Write(&buf, binary.BigEndian, int32(0))  // session_id (v7+)
    
    binary.Write(&buf, binary.BigEndian, int32(topicCount))

    for i := int32(0); i < topicCount; i++ {
        topic, _ := h.readString(reqBuf)
        h.writeString(&buf, topic)

        var partitionCount int32
        binary.Read(reqBuf, binary.BigEndian, &partitionCount)
        binary.Write(&buf, binary.BigEndian, int32(partitionCount))

        for j := int32(0); j < partitionCount; j++ {
            var partition, fetchOffset int32
            var maxPartitionBytes int32 = 1024 * 1024
            
            binary.Read(reqBuf, binary.BigEndian, &partition)
            binary.Read(reqBuf, binary.BigEndian, &fetchOffset)
            binary.Read(reqBuf, binary.BigEndian, &maxPartitionBytes)

            // Fetch messages
            messages, err := h.messageStore.FetchMessages(topic, partition, int64(fetchOffset), 10)

            // Write partition response
            binary.Write(&buf, binary.BigEndian, partition)
            
            if err != nil {
                binary.Write(&buf, binary.BigEndian, int16(UnknownTopicOrPartition))
                binary.Write(&buf, binary.BigEndian, int64(0))    // high_watermark
                binary.Write(&buf, binary.BigEndian, int64(-1))   // last_stable_offset
                binary.Write(&buf, binary.BigEndian, int64(0))    // log_start_offset
                binary.Write(&buf, binary.BigEndian, int32(0))    // aborted_transactions
                binary.Write(&buf, binary.BigEndian, int32(0))    // record_batch_size
            } else {
                binary.Write(&buf, binary.BigEndian, int16(NoError))
                binary.Write(&buf, binary.BigEndian, int64(len(messages))) // high_watermark
                binary.Write(&buf, binary.BigEndian, int64(-1))            // last_stable_offset
                binary.Write(&buf, binary.BigEndian, int64(0))             // log_start_offset
                binary.Write(&buf, binary.BigEndian, int32(0))             // aborted_transactions

                // ✅ FIX: Build proper RecordBatch
                recordBatch := h.buildRecordBatch(messages, int64(fetchOffset))
                binary.Write(&buf, binary.BigEndian, int32(len(recordBatch))) // record_batch_size
                buf.Write(recordBatch)
            }
        }
    }

    return buf.Bytes()
}

// ✅ NEW: Build Kafka RecordBatch format
func (h *KafkaProtocolHandler) buildRecordBatch(messages [][]byte, baseOffset int64) []byte {
    var batch bytes.Buffer

    for i, msg := range messages {
        offset := baseOffset + int64(i)
        
        // RecordBatch header
        binary.Write(&batch, binary.BigEndian, offset)         // base_offset
        binary.Write(&batch, binary.BigEndian, int32(len(msg)+14)) // batch_length
        binary.Write(&batch, binary.BigEndian, int32(0))       // partition_leader_epoch
        binary.Write(&batch, binary.BigEndian, int8(2))        // magic (v2)
        binary.Write(&batch, binary.BigEndian, int32(0))       // crc
        binary.Write(&batch, binary.BigEndian, int16(0))       // attributes
        binary.Write(&batch, binary.BigEndian, int32(0))       // last_offset_delta
        binary.Write(&batch, binary.BigEndian, int64(time.Now().UnixMilli())) // first_timestamp
        binary.Write(&batch, binary.BigEndian, int64(time.Now().UnixMilli())) // max_timestamp
        binary.Write(&batch, binary.BigEndian, int64(-1))      // producer_id
        binary.Write(&batch, binary.BigEndian, int16(-1))      // producer_epoch
        binary.Write(&batch, binary.BigEndian, int32(-1))      // base_sequence
        binary.Write(&batch, binary.BigEndian, int32(1))       // record_count

        // Record
        batch.Write(msg)
    }

    return batch.Bytes()
}
```

---

### Sprint 2: AMQP Full Handshake (2-3 gün)

#### Fix 2.1: AMQP Connection State Machine ✅

**Dosya:** `pkg/amqp/server.go`

```go
type ConnectionState int

const (
    StateStart ConnectionState = iota
    StateStartOkReceived
    StateTuneSent
    StateTuneOkReceived
    StateOpenReceived
    StateConnected
)

type EnhancedAMQPServer struct {
    // ... existing fields ...
    connectionStates map[string]ConnectionState
}

func (s *EnhancedAMQPServer) handleConnection(conn net.Conn) {
    connID := conn.RemoteAddr().String()
    
    // Set initial state
    s.mutex.Lock()
    s.connectionStates[connID] = StateStart
    s.mutex.Unlock()

    // Read protocol header
    header := make([]byte, 8)
    n, err := conn.Read(header)
    if err != nil || n != 8 {
        return
    }

    expectedHeader := []byte{'A', 'M', 'Q', 'P', 0, 0, 9, 1}
    if !bytes.Equal(header, expectedHeader) {
        log.Printf("❌ Invalid AMQP header")
        return
    }

    // Send Connection.Start
    if err := s.sendConnectionStart(conn); err != nil {
        return
    }

    // Main frame processing loop
    for {
        conn.SetReadDeadline(time.Now().Add(30 * time.Second))

        // Read frame
        frameType, channelID, payload, err := s.readFrame(conn)
        if err != nil {
            break
        }

        // Handle frame based on connection state
        if err := s.handleFrameWithState(conn, connID, frameType, channelID, payload); err != nil {
            log.Printf("❌ Frame handling error: %v", err)
            break
        }
    }

    // Cleanup
    s.mutex.Lock()
    delete(s.connectionStates, connID)
    delete(s.connections, connID)
    s.mutex.Unlock()
}

func (s *EnhancedAMQPServer) handleFrameWithState(conn net.Conn, connID string, frameType, channelID int, payload []byte) error {
    s.mutex.RLock()
    state := s.connectionStates[connID]
    s.mutex.RUnlock()

    if frameType != FrameMethod {
        // Non-method frames handled normally
        return s.handleAMQPFrameWithChannel(conn, channelID, frameType, payload)
    }

    // Method frames - check class/method
    if len(payload) < 4 {
        return fmt.Errorf("invalid method frame")
    }
    
    classID := binary.BigEndian.Uint16(payload[0:2])
    methodID := binary.BigEndian.Uint16(payload[2:4])

    // Connection class (10)
    if classID == 10 {
        switch methodID {
        case 11: // Connection.StartOk
            log.Printf("📥 Received Connection.StartOk")
            s.mutex.Lock()
            s.connectionStates[connID] = StateStartOkReceived
            s.mutex.Unlock()
            
            // Send Connection.Tune
            return s.sendConnectionTune(conn)

        case 31: // Connection.TuneOk
            log.Printf("📥 Received Connection.TuneOk")
            s.mutex.Lock()
            s.connectionStates[connID] = StateTuneOkReceived
            s.mutex.Unlock()
            return nil

        case 40: // Connection.Open
            log.Printf("📥 Received Connection.Open")
            s.mutex.Lock()
            s.connectionStates[connID] = StateOpenReceived
            s.mutex.Unlock()
            
            // Send Connection.OpenOk
            return s.sendConnectionOpenOk(conn)
        }
    }

    // Other methods (Queue, Basic, etc.) - only if connected
    if state != StateConnected {
        return fmt.Errorf("connection not ready (state: %d)", state)
    }

    return s.handleMethodFrameWithChannel(conn, channelID, payload)
}

// ✅ NEW: Send Connection.Tune
func (s *EnhancedAMQPServer) sendConnectionTune(conn net.Conn) error {
    log.Printf("📤 Sending Connection.Tune")

    var payload bytes.Buffer
    binary.Write(&payload, binary.BigEndian, uint16(10))  // class: Connection
    binary.Write(&payload, binary.BigEndian, uint16(30))  // method: Tune
    binary.Write(&payload, binary.BigEndian, uint16(0))   // channel-max
    binary.Write(&payload, binary.BigEndian, uint32(131072)) // frame-max (128KB)
    binary.Write(&payload, binary.BigEndian, uint16(60))  // heartbeat (60s)

    sendAMQPFrame(s, 0, FrameMethod, payload.Bytes(), conn)
    
    s.mutex.Lock()
    // Update state after sending
    for connID, c := range s.connections {
        if c.conn == conn {
            s.connectionStates[connID] = StateTuneSent
            break
        }
    }
    s.mutex.Unlock()

    log.Printf("✅ Connection.Tune sent")
    return nil
}

// ✅ NEW: Send Connection.OpenOk
func (s *EnhancedAMQPServer) sendConnectionOpenOk(conn net.Conn) error {
    log.Printf("📤 Sending Connection.OpenOk")

    var payload bytes.Buffer
    binary.Write(&payload, binary.BigEndian, uint16(10))  // class: Connection
    binary.Write(&payload, binary.BigEndian, uint16(41))  // method: OpenOk
    payload.WriteByte(0) // reserved (empty string)

    sendAMQPFrame(s, 0, FrameMethod, payload.Bytes(), conn)
    
    s.mutex.Lock()
    for connID, c := range s.connections {
        if c.conn == conn {
            s.connectionStates[connID] = StateConnected
            log.Printf("✅ Connection %s is now CONNECTED", connID)
            break
        }
    }
    s.mutex.Unlock()

    return nil
}

// ✅ UPDATE: Connection.Start with proper fields
func (s *EnhancedAMQPServer) sendConnectionStart(conn net.Conn) error {
    log.Printf("📤 Sending Connection.Start")

    var payload bytes.Buffer
    binary.Write(&payload, binary.BigEndian, uint16(10))  // class: Connection
    binary.Write(&payload, binary.BigEndian, uint16(10))  // method: Start
    
    // version-major, version-minor
    payload.WriteByte(0)
    payload.WriteByte(9)
    
    // server-properties (empty table for simplicity)
    binary.Write(&payload, binary.BigEndian, uint32(0))
    
    // mechanisms (PLAIN)
    mechanism := "PLAIN"
    binary.Write(&payload, binary.BigEndian, uint32(len(mechanism)))
    payload.WriteString(mechanism)
    
    // locales (en_US)
    locale := "en_US"
    binary.Write(&payload, binary.BigEndian, uint32(len(locale)))
    payload.WriteString(locale)

    sendAMQPFrame(s, 0, FrameMethod, payload.Bytes(), conn)
    log.Printf("✅ Connection.Start sent")
    return nil
}

// ✅ NEW: Helper to read frame
func (s *EnhancedAMQPServer) readFrame(conn net.Conn) (int, int, []byte, error) {
    head := make([]byte, 7)
    if _, err := conn.Read(head); err != nil {
        return 0, 0, nil, err
    }

    frameType := int(head[0])
    channelID := int(binary.BigEndian.Uint16(head[1:3]))
    size := int(binary.BigEndian.Uint32(head[3:7]))

    payload := make([]byte, size)
    if size > 0 {
        if _, err := conn.Read(payload); err != nil {
            return 0, 0, nil, err
        }
    }

    end := make([]byte, 1)
    if _, err := conn.Read(end); err != nil || end[0] != 0xCE {
        return 0, 0, nil, fmt.Errorf("invalid frame end")
    }

    return frameType, channelID, payload, nil
}
```

---

## ✅ EXPECTED RESULTS AFTER FIX

### Kafka Client Tests:
```bash
cd tests/client-tests
go run kafka_consumer_group.go

# Expected output:
✅ Test 1: Single Consumer - PASSED (5/5 messages)
✅ Test 2: Consumer Group - PASSED (15/15 messages)
✅ Test 3: Manual Commit - PASSED (5/5 messages)
✅ Test 4: Seek to Offset - PASSED
✅ Test 5: Consumer Lag - PASSED
```

### RabbitMQ Client Tests:
```bash
go run rabbitmq_consumer.go

# Expected output:
✅ Test 1: Basic Consumer - PASSED (5/5 messages)
✅ Test 2: Manual Ack - PASSED (5/5 messages)
✅ Test 3: Nack + Requeue - PASSED
✅ Test 4: QoS Prefetch - PASSED (5/5 messages)
✅ Test 5: Multiple Consumers - PASSED (10/10 messages)
✅ Test 6: Exchange Types - PASSED
✅ Test 7: Priority Queue - PASSED (5/5 messages)
```

---

## 📅 TIMELINE

**Day 1:** Kafka Produce response format fix  
**Day 2:** Kafka Fetch + RecordBatch implementation  
**Day 3:** Test kafka-go client compatibility  
**Day 4:** AMQP handshake sequence (Tune, TuneOk, Open, OpenOk)  
**Day 5:** AMQP connection state machine  
**Day 6:** Test amqp client compatibility  
**Day 7:** Integration tests + documentation

**Total:** 7 gün (1 hafta)

---

**Hazırlayan:** AI Assistant  
**Tarih:** 9 Ekim 2025  
**Durum:** Ready to implement
