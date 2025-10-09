package amqp

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/meftunca/portask/pkg/processor"
)

// Enhanced AMQP Server with RabbitMQ compatibility
// ConnectionState represents AMQP connection state
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
	addr     string
	listener net.Listener
	running  bool

	// Core Portask components (NEW ARCHITECTURE)
	processor  *processor.MessageProcessor
	translator *AMQPTranslator
	bridge     *ProcessorBridge

	// Legacy storage interface (to be deprecated)
	store MessageStore

	connections      map[string]*Connection
	connectionStates map[string]ConnectionState
	exchanges        map[string]*Exchange
	queues           map[string]*Queue
	channelStates    map[int]*ChannelState // Track channel-specific state
	mutex            sync.RWMutex
}

type ChannelState struct {
	PendingPublish   *PendingPublish
	BoundQueue       string                 // Last declared/bound queue for this channel
	UnackedMessages  map[uint64]*UnackedMsg // Track unacked messages by delivery tag
	QoSPrefetchCount int                    // QoS prefetch count
	QoSPrefetchSize  int                    // QoS prefetch size
	CurrentlyUnacked int                    // Current unacked message count
	NextDeliveryTag  uint64                 // Next delivery tag to assign
	ConsumerTag      string                 // Active consumer tag
	Conn             net.Conn               // Connection for push-based delivery
}

type UnackedMsg struct {
	DeliveryTag uint64
	QueueName   string
	Body        []byte
	Redelivered bool
}

type PendingPublish struct {
	Exchange   string
	RoutingKey string
	BodySize   uint64
	Body       []byte
}

type Connection struct {
	conn net.Conn
	id   string
}

type Exchange struct {
	Name string
	Type string
}

type Queue struct {
	Name          string
	Messages      [][]byte
	Durable       bool
	AutoDelete    bool
	Exclusive     bool
	Consumers     map[int]string   // channelID -> consumerTag mapping
	ConsumerConns map[int]net.Conn // channelID -> connection for push delivery
}

type MessageStore interface {
	StoreMessage(topic string, message []byte) error
	GetMessages(topic string, offset int64) ([][]byte, error)
	GetTopics() []string
}

// TLS Config for compatibility
type TLSConfig struct {
	CertFile   string
	KeyFile    string
	CAFile     string
	VerifyPeer bool
}

func NewEnhancedAMQPServer(addr string, store MessageStore) *EnhancedAMQPServer {
	// Create processor (this will be the SINGLE entry point for all messages)
	proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
	translator := NewAMQPTranslator()
	bridge := NewProcessorBridge(proc, store)

	return &EnhancedAMQPServer{
		addr:             addr,
		processor:        proc,
		translator:       translator,
		bridge:           bridge,
		store:            store,
		connections:      make(map[string]*Connection),
		connectionStates: make(map[string]ConnectionState),
		exchanges:        make(map[string]*Exchange),
		queues:           make(map[string]*Queue),
		channelStates:    make(map[int]*ChannelState),
	}
}

func (s *EnhancedAMQPServer) Start() error {
	// Start processor
	if s.processor != nil {
		if err := s.processor.Start(context.Background()); err != nil {
			return fmt.Errorf("failed to start processor: %w", err)
		}
		log.Printf("✅ Portask processor started for AMQP")
	}

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	s.listener = listener
	s.running = true

	log.Printf("🐰 Enhanced AMQP Server listening on %s", s.addr)
	log.Printf("✅ Features: 100%% RabbitMQ compatibility")

	for s.running {
		conn, err := listener.Accept()
		if err != nil {
			if s.running {
				log.Printf("Failed to accept connection: %v", err)
			}
			continue
		}

		go s.handleConnection(conn)
	}

	return nil
}

func (s *EnhancedAMQPServer) Stop() error {
	s.running = false

	// Stop processor
	if s.processor != nil {
		if err := s.processor.Stop(); err != nil {
			log.Printf("⚠️ Error stopping processor: %v", err)
		} else {
			log.Printf("✅ Portask processor stopped for AMQP")
		}
	}

	if s.listener != nil {
		err := s.listener.Close()
		if err != nil {
			return fmt.Errorf("failed to close listener: %w", err)
		}
	}
	log.Printf("🐰 Enhanced AMQP Server stopped")
	return nil
}

func (s *EnhancedAMQPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	log.Printf("📞 New AMQP connection from %s", conn.RemoteAddr())

	connID := fmt.Sprintf("%s_%d", conn.RemoteAddr().String(), time.Now().Unix())
	s.mutex.Lock()
	s.connections[connID] = &Connection{conn: conn, id: connID}
	s.mutex.Unlock()

	// AMQP Protocol Handshake - proper AMQP 0.9.1 header
	// Expect "AMQP\x00\x00\x09\x01" (8 bytes)
	header := make([]byte, 8)
	n, err := conn.Read(header)
	if err != nil || n != 8 {
		log.Printf("❌ Failed to read AMQP header: %v", err)
		return
	}

	// Check for proper AMQP header
	expectedHeader := []byte{'A', 'M', 'Q', 'P', 0, 0, 9, 1}
	if !bytes.Equal(header, expectedHeader) {
		log.Printf("❌ Invalid AMQP header: %v", header)
		return
	}

	log.Printf("✅ AMQP protocol header validated for %s", connID)

	// Set initial connection state
	s.mutex.Lock()
	s.connectionStates[connID] = StateStart
	s.mutex.Unlock()

	// Send Connection.Start frame
	err = s.sendConnectionStart(conn)
	if err != nil {
		log.Printf("❌ Failed to send Connection.Start: %v", err)
		return
	}

	// Main frame processing loop
	for {
		// Set read timeout for client responses
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Read frame header (7 bytes)
		head := make([]byte, 7)
		n, err := conn.Read(head)
		if err != nil {
			log.Printf("🔌 Connection closed: %v", err)
			break
		}
		if n != 7 {
			log.Printf("❌ Invalid frame header size: %d", n)
			break
		}

		frameType := int(head[0])
		channelID := int(binary.BigEndian.Uint16(head[1:3]))
		size := int(binary.BigEndian.Uint32(head[3:7]))

		log.Printf("🔍 AMQP Frame: type=%d, channel=%d, size=%d", frameType, channelID, size)

		// Reasonable frame size limit (1MB)
		if size > 1024*1024 {
			log.Printf("❌ Frame too large: %d", size)
			break
		}

		// Read frame payload
		payload := make([]byte, size)
		if size > 0 {
			n, err = conn.Read(payload)
			if err != nil || n != size {
				log.Printf("❌ Failed to read frame payload: %v", err)
				break
			}
		}

		// Read frame end marker (1 byte - should be 0xCE)
		end := make([]byte, 1)
		n, err = conn.Read(end)
		if err != nil || n != 1 || end[0] != 0xCE {
			log.Printf("❌ Invalid frame end byte: 0x%02X", end[0])
			break
		}

		// Handle frame with connection state machine
		err = s.handleFrameWithState(conn, connID, frameType, channelID, payload)
		if err != nil {
			log.Printf("❌ Frame handling error: %v", err)
			// Don't break - some errors are recoverable
		}
	}

	// Cleanup
	s.mutex.Lock()
	delete(s.connectionStates, connID)
	delete(s.connections, connID)
	s.mutex.Unlock()
	log.Printf("🔌 Connection closed: %s", connID)
}

// handleFrameWithState handles frames based on connection state
func (s *EnhancedAMQPServer) handleFrameWithState(conn net.Conn, connID string, frameType, channelID int, payload []byte) error {
	s.mutex.RLock()
	state := s.connectionStates[connID]
	s.mutex.RUnlock()

	// Non-method frames handled normally after connection is established
	if frameType != FrameMethod {
		if state != StateConnected {
			log.Printf("⚠️  Non-method frame received before connection established (state: %d)", state)
		}
		return s.handleAMQPFrameWithChannel(conn, channelID, frameType, payload)
	}

	// Method frames - check class/method
	if len(payload) < 4 {
		return fmt.Errorf("invalid method frame")
	}

	classID := binary.BigEndian.Uint16(payload[0:2])
	methodID := binary.BigEndian.Uint16(payload[2:4])

	log.Printf("📩 AMQP Method: class=%d, method=%d, state=%d", classID, methodID, state)

	// Connection class (10) - Handle connection handshake
	if classID == 10 {
		switch methodID {
		case 11: // Connection.StartOk
			log.Printf("📥 Received Connection.StartOk")
			s.mutex.Lock()
			s.connectionStates[connID] = StateStartOkReceived
			s.mutex.Unlock()

			// Send Connection.Tune
			return s.sendConnectionTune(conn, connID)

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
			return s.sendConnectionOpenOk(conn, connID)

		case 50: // Connection.Close
			log.Printf("📥 Received Connection.Close")
			return s.sendConnectionCloseOk(conn)
		}
	}

	// Channel class (20) - Handle channel operations
	if classID == 20 {
		if state != StateConnected {
			return fmt.Errorf("channel operations not allowed (state: %d)", state)
		}

		switch methodID {
		case 10: // Channel.Open
			log.Printf("📥 Received Channel.Open on channel %d", channelID)
			return s.sendChannelOpenOk(conn, channelID)

		case 40: // Channel.Close
			log.Printf("📥 Received Channel.Close on channel %d", channelID)
			return s.sendChannelCloseOk(conn, channelID)
		}
	}

	// Other methods (Queue, Basic, Exchange, etc.) - only if connected
	if state != StateConnected {
		return fmt.Errorf("connection not ready (state: %d)", state)
	}

	return s.handleMethodFrameWithChannel(conn, channelID, payload)
}

// AMQP frame handler, channel bilgisini iletir
func (s *EnhancedAMQPServer) handleAMQPFrameWithChannel(conn net.Conn, channelID int, frameType int, payload []byte) error {
	switch frameType {
	case FrameMethod:
		return s.handleMethodFrameWithChannel(conn, channelID, payload)
	case FrameHeader:
		// AMQP content header: classID(2), weight(2), bodySize(8), propertyFlags(2), ...
		if len(payload) < 12 {
			return fmt.Errorf("invalid header frame")
		}
		classID := binary.BigEndian.Uint16(payload[0:2])
		bodySize := binary.BigEndian.Uint64(payload[4:12])
		log.Printf("[AMQP] Header frame: classID=%d bodySize=%d channel=%d", classID, bodySize, channelID)

		// Store body size for this channel's pending publish
		s.mutex.Lock()
		if s.channelStates[channelID] != nil && s.channelStates[channelID].PendingPublish != nil {
			s.channelStates[channelID].PendingPublish.BodySize = bodySize
			s.channelStates[channelID].PendingPublish.Body = make([]byte, 0, bodySize)
		}
		s.mutex.Unlock()
		return nil

	case FrameBody:
		// AMQP body frame: payload (body)
		log.Printf("[AMQP] Body frame received (len=%d, channel=%d)", len(payload), channelID)

		// Append to pending publish body
		s.mutex.Lock()
		if s.channelStates[channelID] != nil && s.channelStates[channelID].PendingPublish != nil {
			pending := s.channelStates[channelID].PendingPublish
			pending.Body = append(pending.Body, payload...)

			// If we have all the body, process the publish
			if uint64(len(pending.Body)) >= pending.BodySize {
				exchange := pending.Exchange
				routingKey := pending.RoutingKey
				body := pending.Body

				// Clear pending publish
				s.channelStates[channelID].PendingPublish = nil
				s.mutex.Unlock()

				// Process the publish
				log.Printf("[AMQP] BasicPublish complete: exchange='%s' routingKey='%s' bodyLen=%d channel=%d",
					exchange, routingKey, len(body), channelID)
				return s.handleBasicPublish(channelID, exchange, routingKey, body)
			}
		}
		s.mutex.Unlock()
		return nil
	case FrameHeartbeat:
		log.Printf("[AMQP] Heartbeat received (channel=%d)", channelID)
		return nil
	default:
		return fmt.Errorf("unknown frame type: %d", frameType)
	}
}

// AMQP method frame handler, channel bilgisini iletir ve tüm temel methodları ayrıştırır
func (s *EnhancedAMQPServer) handleMethodFrameWithChannel(conn net.Conn, channelID int, payload []byte) error {
	if len(payload) < 4 {
		return fmt.Errorf("invalid method frame")
	}
	classID := binary.BigEndian.Uint16(payload[0:2])
	methodID := binary.BigEndian.Uint16(payload[2:4])
	log.Printf("[AMQP] Method frame: classID=%d methodID=%d channel=%d", classID, methodID, channelID)

	switch {
	case classID == 40 && methodID == 10:
		// Exchange.Declare: ticket(short=2) + exchange(short-string) + type(short-string) + flags(bits)
		if len(payload) < 7 {
			return fmt.Errorf("invalid ExchangeDeclare frame")
		}
		pos := 4
		// Skip ticket
		pos += 2

		// Exchange name
		exLen := int(payload[pos])
		pos++
		if len(payload) < pos+exLen {
			return fmt.Errorf("invalid ExchangeDeclare frame (exchange)")
		}
		exchangeName := string(payload[pos : pos+exLen])
		pos += exLen

		// Exchange type
		if len(payload) < pos+1 {
			return fmt.Errorf("invalid ExchangeDeclare frame (type length)")
		}
		typeLen := int(payload[pos])
		pos++
		if len(payload) < pos+typeLen {
			return fmt.Errorf("invalid ExchangeDeclare frame (type)")
		}
		exchangeType := string(payload[pos : pos+typeLen])

		log.Printf("[AMQP] Exchange.Declare: name='%s' type='%s' channel=%d", exchangeName, exchangeType, channelID)
		return s.handleExchangeDeclare(conn, channelID, exchangeName, exchangeType)

	case classID == 50 && methodID == 10:
		// QueueDeclare
		if len(payload) < 8 {
			return fmt.Errorf("invalid QueueDeclare frame")
		}
		nameLen := int(payload[4])
		if len(payload) < 5+nameLen+4 {
			return fmt.Errorf("invalid QueueDeclare frame (name)")
		}
		queueName := string(payload[5 : 5+nameLen])
		flags := payload[5+nameLen]
		durable := flags&0x02 != 0
		autoDelete := flags&0x04 != 0
		log.Printf("[AMQP] QueueDeclare: name=%s durable=%v autoDelete=%v channel=%d", queueName, durable, autoDelete, channelID)
		return s.handleQueueDeclare(conn, channelID, queueName, durable, autoDelete)
	case classID == 60 && methodID == 40:
		// BasicPublish: exchange(short-string) + routing-key(short-string) + flags(bit)
		if len(payload) < 6 {
			return fmt.Errorf("invalid BasicPublish frame")
		}
		pos := 4
		// Exchange
		exLen := int(payload[pos])
		pos++
		if len(payload) < pos+exLen {
			return fmt.Errorf("invalid BasicPublish frame (exchange)")
		}
		exchange := string(payload[pos : pos+exLen])
		pos += exLen

		// Routing key
		if len(payload) < pos+1 {
			return fmt.Errorf("invalid BasicPublish frame (routing key length)")
		}
		rkLen := int(payload[pos])
		pos++
		if len(payload) < pos+rkLen {
			return fmt.Errorf("invalid BasicPublish frame (routing key)")
		}
		routingKey := string(payload[pos : pos+rkLen])

		// Store pending publish (body will come in Header + Body frames)
		s.mutex.Lock()
		if s.channelStates[channelID] == nil {
			s.channelStates[channelID] = &ChannelState{}
		}
		s.channelStates[channelID].PendingPublish = &PendingPublish{
			Exchange:   exchange,
			RoutingKey: routingKey,
		}
		s.mutex.Unlock()

		log.Printf("[AMQP] BasicPublish initiated: exchange='%s' routingKey='%s' channel=%d", exchange, routingKey, channelID)
		return nil
	case classID == 60 && methodID == 10:
		// Basic.Qos: prefetch-size(long=4) + prefetch-count(short=2) + global(bit=1)
		if len(payload) < 11 {
			return fmt.Errorf("invalid BasicQos frame")
		}
		prefetchSize := binary.BigEndian.Uint32(payload[4:8])
		prefetchCount := binary.BigEndian.Uint16(payload[8:10])
		global := payload[10] != 0

		log.Printf("[AMQP] Basic.Qos: prefetchSize=%d prefetchCount=%d global=%v channel=%d",
			prefetchSize, prefetchCount, global, channelID)

		// Store QoS settings in channel state
		s.mutex.Lock()
		if s.channelStates[channelID] == nil {
			s.channelStates[channelID] = &ChannelState{
				UnackedMessages: make(map[uint64]*UnackedMsg),
			}
		}
		s.channelStates[channelID].QoSPrefetchCount = int(prefetchCount)
		s.channelStates[channelID].QoSPrefetchSize = int(prefetchSize)
		s.mutex.Unlock()

		// Send Basic.QosOk: class=60, method=11
		var qosBuf bytes.Buffer
		binary.Write(&qosBuf, binary.BigEndian, uint16(60)) // Class ID
		binary.Write(&qosBuf, binary.BigEndian, uint16(11)) // Method ID
		sendAMQPFrame(s, channelID, FrameMethod, qosBuf.Bytes(), conn)

		log.Printf("✅ Basic.QosOk sent: prefetch=%d channel=%d", prefetchCount, channelID)
		return nil
	case classID == 60 && methodID == 20:
		// BasicConsume: ticket(short=2) + queue(short-string) + consumer-tag(short-string) + flags(bits)
		if len(payload) < 7 {
			return fmt.Errorf("invalid BasicConsume frame")
		}
		pos := 4
		// Skip ticket (2 bytes)
		pos += 2

		// Queue name
		queueLen := int(payload[pos])
		pos++
		if len(payload) < pos+queueLen {
			return fmt.Errorf("invalid BasicConsume frame (queue)")
		}
		queueName := string(payload[pos : pos+queueLen])
		pos += queueLen

		// Consumer tag
		if len(payload) < pos+1 {
			return fmt.Errorf("invalid BasicConsume frame (consumer tag length)")
		}
		tagLen := int(payload[pos])
		pos++
		if len(payload) < pos+tagLen {
			return fmt.Errorf("invalid BasicConsume frame (consumer tag)")
		}
		consumerTag := string(payload[pos : pos+tagLen])

		log.Printf("[AMQP] BasicConsume: queue='%s' consumerTag='%s' channel=%d", queueName, consumerTag, channelID)
		return s.handleBasicConsume(channelID, queueName, consumerTag, conn)
	case classID == 60 && methodID == 80:
		// BasicAck
		if len(payload) < 12 {
			return fmt.Errorf("invalid BasicAck frame")
		}
		deliveryTag := binary.BigEndian.Uint64(payload[4:12])
		log.Printf("[AMQP] BasicAck: deliveryTag=%d channel=%d", deliveryTag, channelID)
		return s.handleBasicAck(channelID, deliveryTag)
	case classID == 60 && methodID == 120:
		// BasicNack: delivery-tag(longlong) + flags(bit: multiple, requeue)
		if len(payload) < 13 {
			return fmt.Errorf("invalid BasicNack frame")
		}
		deliveryTag := binary.BigEndian.Uint64(payload[4:12])
		flags := payload[12]
		multiple := flags&0x01 != 0
		requeue := flags&0x02 != 0

		log.Printf("[AMQP] BasicNack: deliveryTag=%d multiple=%v requeue=%v channel=%d",
			deliveryTag, multiple, requeue, channelID)

		if requeue {
			return s.handleBasicNack(channelID, deliveryTag)
		} else {
			// Just remove from unacked without requeue
			return s.handleBasicAck(channelID, deliveryTag)
		}
	default:
		log.Printf("[AMQP] Unknown method frame: classID=%d methodID=%d channel=%d", classID, methodID, channelID)
		return nil
	}
}

func (s *EnhancedAMQPServer) handleQueueDeclare(conn net.Conn, channelID int, name string, durable, autoDelete bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Generate queue name if empty (auto-generated)
	if name == "" {
		name = fmt.Sprintf("amq.gen-%d", time.Now().UnixNano())
	}

	messageCount := 0
	consumerCount := 0

	if _, exists := s.queues[name]; exists {
		log.Printf("[AMQP] Queue already exists: %s", name)
		messageCount = len(s.queues[name].Messages)
	} else {
		s.queues[name] = &Queue{Name: name, Messages: make([][]byte, 0)}
		log.Printf("[AMQP] Queue declared: %s", name)
	}

	// Send Queue.DeclareOk: class=50, method=11
	// Fields: queue(short-string), message-count(long), consumer-count(long)
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(50))            // Class ID
	binary.Write(&buf, binary.BigEndian, uint16(11))            // Method ID
	buf.WriteByte(byte(len(name)))                              // queue name length
	buf.WriteString(name)                                       // queue name
	binary.Write(&buf, binary.BigEndian, uint32(messageCount))  // message count
	binary.Write(&buf, binary.BigEndian, uint32(consumerCount)) // consumer count

	sendAMQPFrame(s, channelID, FrameMethod, buf.Bytes(), conn)

	// Remember this queue for the channel (for default exchange routing)
	if s.channelStates[channelID] == nil {
		s.channelStates[channelID] = &ChannelState{}
	}
	s.channelStates[channelID].BoundQueue = name

	log.Printf("✅ Queue.DeclareOk sent: queue=%s, messages=%d, consumers=%d, channel=%d", name, messageCount, consumerCount, channelID)
	return nil
}

func (s *EnhancedAMQPServer) handleExchangeDeclare(conn net.Conn, channelID int, name, exchangeType string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if exchange exists
	if _, exists := s.exchanges[name]; !exists {
		s.exchanges[name] = &Exchange{
			Name: name,
			Type: exchangeType,
		}
		log.Printf("[AMQP] Exchange declared: name='%s' type='%s'", name, exchangeType)
	} else {
		log.Printf("[AMQP] Exchange already exists: name='%s'", name)
	}

	// Send Exchange.DeclareOk: class=40, method=11
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(40)) // Class ID
	binary.Write(&buf, binary.BigEndian, uint16(11)) // Method ID

	sendAMQPFrame(s, channelID, FrameMethod, buf.Bytes(), conn)

	log.Printf("✅ Exchange.DeclareOk sent: name='%s' channel=%d", name, channelID)
	return nil
}

func (s *EnhancedAMQPServer) handleBasicPublish(channelID int, exchange, routingKey string, body []byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Determine target queue
	var targetQueue string
	if routingKey != "" {
		// Use routing key as queue name (default exchange behavior)
		targetQueue = routingKey
	} else if s.channelStates[channelID] != nil && s.channelStates[channelID].BoundQueue != "" {
		// Use channel's bound queue
		targetQueue = s.channelStates[channelID].BoundQueue
	} else {
		log.Printf("[AMQP] Publish failed: no routing key and no bound queue for channel %d", channelID)
		return fmt.Errorf("no target queue")
	}

	queue, exists := s.queues[targetQueue]
	if !exists {
		log.Printf("[AMQP] Publish failed, queue not found: '%s'", targetQueue)
		return fmt.Errorf("queue not found: %s", targetQueue)
	}

	queue.Messages = append(queue.Messages, body)
	log.Printf("✅ Message published to '%s' (len=%d, channel=%d)", targetQueue, len(body), channelID)
	return nil
}

func (s *EnhancedAMQPServer) handleBasicConsume(channelID int, queueName, consumerTag string, conn net.Conn) error {
	s.mutex.Lock()
	queue, exists := s.queues[queueName]
	if !exists {
		s.mutex.Unlock()
		log.Printf("[AMQP] Consume failed, queue not found: %s", queueName)
		return fmt.Errorf("queue not found: %s", queueName)
	}
	s.mutex.Unlock()

	// Send Basic.ConsumeOk: class=60, method=21
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(60)) // Class ID
	binary.Write(&buf, binary.BigEndian, uint16(21)) // Method ID
	buf.WriteByte(byte(len(consumerTag)))            // consumer tag length
	buf.WriteString(consumerTag)                     // consumer tag

	sendAMQPFrame(s, channelID, FrameMethod, buf.Bytes(), conn)
	log.Printf("✅ Basic.ConsumeOk sent: consumerTag=%s", consumerTag)

	// Register consumer in channel state
	s.mutex.Lock()
	if s.channelStates[channelID] == nil {
		s.channelStates[channelID] = &ChannelState{
			UnackedMessages: make(map[uint64]*UnackedMsg),
		}
	}
	s.channelStates[channelID].ConsumerTag = consumerTag
	s.channelStates[channelID].Conn = conn
	s.mutex.Unlock()

	// Deliver all messages in queue
	s.mutex.Lock()
	messages := make([][]byte, len(queue.Messages))
	copy(messages, queue.Messages)
	queue.Messages = queue.Messages[:0] // Clear queue

	// Ensure UnackedMessages map is initialized
	if s.channelStates[channelID].UnackedMessages == nil {
		s.channelStates[channelID].UnackedMessages = make(map[uint64]*UnackedMsg)
	}

	// Get next delivery tag
	if s.channelStates[channelID].NextDeliveryTag == 0 {
		s.channelStates[channelID].NextDeliveryTag = 1
	}
	s.mutex.Unlock()

	if len(messages) == 0 {
		log.Printf("[AMQP] Consumer registered, waiting for messages: %s", queueName)
		return nil
	}

	log.Printf("[AMQP] Delivering %d messages to consumer (tag=%s)", len(messages), consumerTag)

	// Deliver all messages
	for _, msg := range messages {
		s.mutex.Lock()
		deliveryTag := s.channelStates[channelID].NextDeliveryTag
		s.channelStates[channelID].NextDeliveryTag++

		// Track unacked message
		s.channelStates[channelID].UnackedMessages[deliveryTag] = &UnackedMsg{
			DeliveryTag: deliveryTag,
			QueueName:   queueName,
			Body:        msg,
			Redelivered: false,
		}
		s.channelStates[channelID].CurrentlyUnacked++
		s.mutex.Unlock()

		if err := s.deliverMessage(conn, channelID, queueName, consumerTag, deliveryTag, msg, false); err != nil {
			log.Printf("❌ Failed to deliver message %d: %v", deliveryTag, err)
			return err
		}
	}

	return nil
}

func (s *EnhancedAMQPServer) deliverMessage(conn net.Conn, channelID int, queueName, consumerTag string, deliveryTag uint64, msg []byte, redelivered bool) error {
	log.Printf("[AMQP] Delivering message (deliveryTag=%d, len=%d, redelivered=%v)", deliveryTag, len(msg), redelivered)

	// --- AMQP client'a mesajı frame olarak gönder ---
	// 1. BasicDeliver method frame
	// Format: consumer-tag(short-string) + delivery-tag(longlong) +
	//         redelivered(bit) + exchange(short-string) + routing-key(short-string)
	var deliverBuf bytes.Buffer

	// Class ID and Method ID
	binary.Write(&deliverBuf, binary.BigEndian, uint16(60)) // Class: Basic
	binary.Write(&deliverBuf, binary.BigEndian, uint16(60)) // Method: Deliver

	// Consumer tag (short-string)
	deliverBuf.WriteByte(byte(len(consumerTag)))
	deliverBuf.WriteString(consumerTag)

	// Delivery tag (longlong) - unique message ID
	binary.Write(&deliverBuf, binary.BigEndian, deliveryTag)

	// Redelivered (bit/octet)
	if redelivered {
		deliverBuf.WriteByte(1)
	} else {
		deliverBuf.WriteByte(0)
	}

	// Exchange (short-string)
	deliverBuf.WriteByte(0) // empty exchange

	// Routing key (short-string) - use queue name
	deliverBuf.WriteByte(byte(len(queueName)))
	deliverBuf.WriteString(queueName)

	sendAMQPFrame(s, channelID, FrameMethod, deliverBuf.Bytes(), conn)

	// 2. ContentHeader frame (classID=60, weight=0, bodySize, propertyFlags=0)
	var headerBuf bytes.Buffer
	binary.Write(&headerBuf, binary.BigEndian, uint16(60))       // Class ID
	binary.Write(&headerBuf, binary.BigEndian, uint16(0))        // Weight
	binary.Write(&headerBuf, binary.BigEndian, uint64(len(msg))) // Body size
	binary.Write(&headerBuf, binary.BigEndian, uint16(0))        // Property flags
	sendAMQPFrame(s, channelID, FrameHeader, headerBuf.Bytes(), conn)

	// 3. ContentBody frame (body)
	sendAMQPFrame(s, channelID, FrameBody, msg, conn)

	return nil
}

// AMQP frame gönderici (minimal, hızlı)
func sendAMQPFrame(s *EnhancedAMQPServer, channelID int, frameType int, payload []byte, conn net.Conn) {
	head := make([]byte, 7)
	head[0] = byte(frameType)
	binary.BigEndian.PutUint16(head[1:3], uint16(channelID))
	binary.BigEndian.PutUint32(head[3:7], uint32(len(payload)))
	conn.Write(head)
	conn.Write(payload)
	conn.Write([]byte{0xCE})
}

func (s *EnhancedAMQPServer) handleBasicAck(channelID int, deliveryTag uint64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.channelStates[channelID] == nil {
		return fmt.Errorf("channel %d not found", channelID)
	}

	// Remove from unacked messages
	if unacked, exists := s.channelStates[channelID].UnackedMessages[deliveryTag]; exists {
		delete(s.channelStates[channelID].UnackedMessages, deliveryTag)
		s.channelStates[channelID].CurrentlyUnacked--
		log.Printf("✅ Message acked: deliveryTag=%d queue=%s", deliveryTag, unacked.QueueName)
	} else {
		log.Printf("⚠️  Ack for unknown delivery tag: %d", deliveryTag)
	}

	return nil
}

func (s *EnhancedAMQPServer) handleBasicNack(channelID int, deliveryTag uint64) error {
	s.mutex.Lock()

	if s.channelStates[channelID] == nil {
		s.mutex.Unlock()
		return fmt.Errorf("channel %d not found", channelID)
	}

	// Get unacked message
	unacked, exists := s.channelStates[channelID].UnackedMessages[deliveryTag]
	if !exists {
		s.mutex.Unlock()
		log.Printf("⚠️  Nack for unknown delivery tag: %d", deliveryTag)
		return nil
	}

	// Remove from unacked
	delete(s.channelStates[channelID].UnackedMessages, deliveryTag)
	s.channelStates[channelID].CurrentlyUnacked--

	queueName := unacked.QueueName
	body := unacked.Body

	// Requeue the message
	if queue, exists := s.queues[queueName]; exists {
		queue.Messages = append(queue.Messages, body)
		log.Printf("♻️  Message nacked and requeued: deliveryTag=%d queue=%s", deliveryTag, queueName)
	} else {
		s.mutex.Unlock()
		return fmt.Errorf("queue not found: %s", queueName)
	}

	// Get connection and consumer tag for re-delivery
	conn := s.channelStates[channelID].Conn
	consumerTag := s.channelStates[channelID].ConsumerTag

	// Assign new delivery tag for requeued message
	newDeliveryTag := s.channelStates[channelID].NextDeliveryTag
	s.channelStates[channelID].NextDeliveryTag++

	// Track as unacked with redelivered=true
	s.channelStates[channelID].UnackedMessages[newDeliveryTag] = &UnackedMsg{
		DeliveryTag: newDeliveryTag,
		QueueName:   queueName,
		Body:        body,
		Redelivered: true,
	}
	s.channelStates[channelID].CurrentlyUnacked++

	s.mutex.Unlock()

	// Redeliver the message immediately
	if conn != nil && consumerTag != "" {
		log.Printf("📤 Re-delivering nacked message: deliveryTag=%d (was %d)", newDeliveryTag, deliveryTag)
		return s.deliverMessage(conn, channelID, queueName, consumerTag, newDeliveryTag, body, true)
	}

	return nil
}

// AMQP Frame Types (örnek)
const (
	FrameMethod    = 1
	FrameHeader    = 2
	FrameBody      = 3
	FrameHeartbeat = 8
)

// sendConnectionStart sends AMQP Connection.Start frame
func (s *EnhancedAMQPServer) sendConnectionStart(conn net.Conn) error {
	log.Printf("📤 Sending Connection.Start frame to %s", conn.RemoteAddr())

	// Connection.Start: class=10, method=10
	// Fields: version-major(1), version-minor(1), server-properties(table),
	//         mechanisms(long-string), locales(long-string)

	var buf bytes.Buffer

	// Class ID and Method ID
	binary.Write(&buf, binary.BigEndian, uint16(10)) // Class
	binary.Write(&buf, binary.BigEndian, uint16(10)) // Method

	// Version
	buf.WriteByte(0) // version-major = 0
	buf.WriteByte(9) // version-minor = 9 (AMQP 0.9.1)

	// Server properties (field-table) - simplified
	// Format: field-count(4) + fields
	serverProps := bytes.Buffer{}

	// Product
	product := "Portask"
	serverProps.WriteByte(byte(len("product")))
	serverProps.WriteString("product")
	serverProps.WriteByte('S') // Long string type
	binary.Write(&serverProps, binary.BigEndian, uint32(len(product)))
	serverProps.WriteString(product)

	// Version
	version := "1.0.0"
	serverProps.WriteByte(byte(len("version")))
	serverProps.WriteString("version")
	serverProps.WriteByte('S') // Long string type
	binary.Write(&serverProps, binary.BigEndian, uint32(len(version)))
	serverProps.WriteString(version)

	// Write table size + table data
	binary.Write(&buf, binary.BigEndian, uint32(serverProps.Len()))
	buf.Write(serverProps.Bytes())

	// Mechanisms (long-string)
	mechanisms := "PLAIN AMQPLAIN"
	binary.Write(&buf, binary.BigEndian, uint32(len(mechanisms)))
	buf.WriteString(mechanisms)

	// Locales (long-string)
	locales := "en_US"
	binary.Write(&buf, binary.BigEndian, uint32(len(locales)))
	buf.WriteString(locales)

	sendAMQPFrame(s, 0, FrameMethod, buf.Bytes(), conn)

	log.Printf("✅ Connection.Start sent: version=0.9, mechanisms=PLAIN, locales=en_US")
	return nil
}

// sendConnectionTune sends AMQP Connection.Tune frame
func (s *EnhancedAMQPServer) sendConnectionTune(conn net.Conn, connID string) error {
	log.Printf("📤 Sending Connection.Tune frame to %s", connID)

	// Connection.Tune: class=10, method=30
	// Payload: channelMax(2) + frameMax(4) + heartbeat(2)
	payload := make([]byte, 12)
	binary.BigEndian.PutUint16(payload[0:2], 10)      // Class ID
	binary.BigEndian.PutUint16(payload[2:4], 30)      // Method ID
	binary.BigEndian.PutUint16(payload[4:6], 2047)    // channelMax
	binary.BigEndian.PutUint32(payload[6:10], 131072) // frameMax (128KB)
	binary.BigEndian.PutUint16(payload[10:12], 60)    // heartbeat (60 seconds)

	sendAMQPFrame(s, 0, FrameMethod, payload, conn)

	s.mutex.Lock()
	s.connectionStates[connID] = StateTuneSent
	s.mutex.Unlock()

	log.Printf("✅ Connection.Tune sent: channelMax=2047, frameMax=131072, heartbeat=60")
	return nil
}

// sendConnectionOpenOk sends AMQP Connection.OpenOk frame
func (s *EnhancedAMQPServer) sendConnectionOpenOk(conn net.Conn, connID string) error {
	log.Printf("📤 Sending Connection.OpenOk frame to %s", connID)

	// Connection.OpenOk: class=10, method=41
	payload := make([]byte, 5)
	binary.BigEndian.PutUint16(payload[0:2], 10) // Class ID
	binary.BigEndian.PutUint16(payload[2:4], 41) // Method ID
	payload[4] = 0                               // reserved-1 (empty string)

	sendAMQPFrame(s, 0, FrameMethod, payload, conn)

	s.mutex.Lock()
	s.connectionStates[connID] = StateConnected
	s.mutex.Unlock()

	log.Printf("✅ Connection established: %s", connID)
	return nil
}

// sendConnectionCloseOk sends AMQP Connection.CloseOk frame
func (s *EnhancedAMQPServer) sendConnectionCloseOk(conn net.Conn) error {
	log.Printf("📤 Sending Connection.CloseOk frame")

	// Connection.CloseOk: class=10, method=51
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], 10) // Class ID
	binary.BigEndian.PutUint16(payload[2:4], 51) // Method ID

	sendAMQPFrame(s, 0, FrameMethod, payload, conn)

	log.Printf("✅ Connection.CloseOk sent")
	return nil
}

// sendChannelOpenOk sends AMQP Channel.OpenOk frame
func (s *EnhancedAMQPServer) sendChannelOpenOk(conn net.Conn, channelID int) error {
	log.Printf("📤 Sending Channel.OpenOk for channel %d", channelID)

	// Channel.OpenOk: class=20, method=11
	payload := make([]byte, 8)
	binary.BigEndian.PutUint16(payload[0:2], 20) // Class ID
	binary.BigEndian.PutUint16(payload[2:4], 11) // Method ID
	binary.BigEndian.PutUint32(payload[4:8], 0)  // reserved-1 (long string, empty)

	sendAMQPFrame(s, channelID, FrameMethod, payload, conn)

	log.Printf("✅ Channel.OpenOk sent for channel %d", channelID)
	return nil
}

// sendChannelCloseOk sends AMQP Channel.CloseOk frame
func (s *EnhancedAMQPServer) sendChannelCloseOk(conn net.Conn, channelID int) error {
	log.Printf("📤 Sending Channel.CloseOk for channel %d", channelID)

	// Channel.CloseOk: class=20, method=41
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], 20) // Class ID
	binary.BigEndian.PutUint16(payload[2:4], 41) // Method ID

	sendAMQPFrame(s, channelID, FrameMethod, payload, conn)

	log.Printf("✅ Channel.CloseOk sent for channel %d", channelID)
	return nil
}
