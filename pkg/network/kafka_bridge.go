package network

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/segmentio/kafka-go"
)

// KafkaClientType defines which Go client to use
// "sarama" or "kafka-go"
type KafkaClientType string

const (
	KafkaClientSarama  KafkaClientType = "sarama"
	KafkaClientKafkaGo KafkaClientType = "kafka-go"
)

// KafkaIntegrationConfig holds Kafka test configuration
type KafkaIntegrationConfig struct {
	ClientType        KafkaClientType
	BrokerAddresses   []string
	TopicPrefix       string
	ConsumerGroup     string
	PartitionCount    int32
	ReplicationFactor int16
}

// DefaultKafkaConfig returns default Kafka test configuration
func DefaultKafkaConfig(clientType KafkaClientType) *KafkaIntegrationConfig {
	return &KafkaIntegrationConfig{
		ClientType:        clientType,
		BrokerAddresses:   []string{"localhost:9092"},
		TopicPrefix:       "portask_test",
		ConsumerGroup:     "portask_test_group",
		PartitionCount:    3,
		ReplicationFactor: 1,
	}
}

// KafkaPortaskBridge is a generic bridge for both Sarama and kafka-go
// Only one of saramaBridge or kafkaGoBridge will be non-nil
type KafkaPortaskBridge struct {
	saramaBridge  *SaramaKafkaBridge
	kafkaGoBridge *KafkaGoBridge
	clientType    KafkaClientType
}

// NewKafkaPortaskBridge creates a new bridge based on config.ClientType
func NewKafkaPortaskBridge(config *KafkaIntegrationConfig, portaskAddr string, topic string) (*KafkaPortaskBridge, error) {
	switch config.ClientType {
	case KafkaClientSarama:
		b, err := NewSaramaKafkaBridge(config, portaskAddr, topic)
		if err != nil {
			return nil, err
		}
		return &KafkaPortaskBridge{saramaBridge: b, clientType: KafkaClientSarama}, nil
	case KafkaClientKafkaGo:
		b, err := NewKafkaGoBridge(config, portaskAddr, topic)
		if err != nil {
			return nil, err
		}
		return &KafkaPortaskBridge{kafkaGoBridge: b, clientType: KafkaClientKafkaGo}, nil
	default:
		return nil, fmt.Errorf("unknown Kafka client type: %s", config.ClientType)
	}
}

// --- Sarama implementation ---
type SaramaKafkaBridge struct {
	producer    sarama.SyncProducer
	consumer    sarama.ConsumerGroup
	portaskConn net.Conn
	config      *KafkaIntegrationConfig
	isRunning   bool
	stopChan    chan struct{}
	mu          sync.RWMutex
	// ...
}

func NewSaramaKafkaBridge(config *KafkaIntegrationConfig, portaskAddr, topic string) (*SaramaKafkaBridge, error) {
	producerConfig := sarama.NewConfig()
	producerConfig.Producer.Return.Successes = true
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producerConfig.Producer.Retry.Max = 3

	producer, err := sarama.NewSyncProducer(config.BrokerAddresses, producerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Sarama producer: %v", err)
	}

	// Consumer group setup (not started yet)
	consumer, err := sarama.NewConsumerGroup(config.BrokerAddresses, config.ConsumerGroup, producerConfig)
	if err != nil {
		producer.Close()
		return nil, fmt.Errorf("failed to create Sarama consumer group: %v", err)
	}

	portaskConn, err := net.DialTimeout("tcp", portaskAddr, 5*time.Second)
	if err != nil {
		producer.Close()
		consumer.Close()
		return nil, fmt.Errorf("failed to connect to Portask: %v", err)
	}

	return &SaramaKafkaBridge{
		producer:    producer,
		consumer:    consumer,
		portaskConn: portaskConn,
		config:      config,
		stopChan:    make(chan struct{}),
	}, nil
}

func (b *SaramaKafkaBridge) PublishToKafka(topic string, message []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(message),
	}
	_, _, err := b.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to publish to Kafka (Sarama): %v", err)
	}
	return nil
}

// Sarama consume loop (Portask'a forward)
func (b *SaramaKafkaBridge) ConsumeFromKafka(topics []string) error {
	b.mu.Lock()
	b.isRunning = true
	b.mu.Unlock()
	go func() {
		h := &saramaConsumerGroupHandler{bridge: b}
		for {
			if err := b.consumer.Consume(context.Background(), topics, h); err != nil {
				return
			}
			if !b.isRunning {
				return
			}
		}
	}()
	return nil
}

type saramaConsumerGroupHandler struct{ bridge *SaramaKafkaBridge }

func (h *saramaConsumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *saramaConsumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *saramaConsumerGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		_ = h.bridge.SendToPortask("publish", msg.Topic, string(msg.Value))
		sess.MarkMessage(msg, "")
	}
	return nil
}

func (b *SaramaKafkaBridge) SendToPortask(msgType, topic, data string) error {
	msg := fmt.Sprintf(`{"type":"%s","topic":"%s","data":%q}`, msgType, topic, data)
	_, err := b.portaskConn.Write([]byte(msg + "\n"))
	return err
}

func (b *SaramaKafkaBridge) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.isRunning {
		close(b.stopChan)
		b.isRunning = false
	}
	var errs []error
	if err := b.producer.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := b.consumer.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := b.portaskConn.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}
	return nil
}

// --- kafka-go implementation ---
type KafkaGoBridge struct {
	writer      *kafka.Writer
	reader      *kafka.Reader
	portaskConn net.Conn
	config      *KafkaIntegrationConfig
	isRunning   bool
	stopChan    chan struct{}
	mu          sync.RWMutex
	// ...
}

func NewKafkaGoBridge(config *KafkaIntegrationConfig, portaskAddr, topic string) (*KafkaGoBridge, error) {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(config.BrokerAddresses...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  config.BrokerAddresses,
		GroupID:  config.ConsumerGroup,
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	portaskConn, err := net.DialTimeout("tcp", portaskAddr, 5*time.Second)
	if err != nil {
		writer.Close()
		reader.Close()
		return nil, fmt.Errorf("failed to connect to Portask: %v", err)
	}
	return &KafkaGoBridge{
		writer:      writer,
		reader:      reader,
		portaskConn: portaskConn,
		config:      config,
		stopChan:    make(chan struct{}),
	}, nil
}

func (b *KafkaGoBridge) PublishToKafka(topic string, message []byte) error {
	return b.writer.WriteMessages(context.Background(), kafka.Message{
		Topic: topic,
		Value: message,
	})
}

// kafka-go consume loop (Portask'a forward)
func (b *KafkaGoBridge) ConsumeFromKafka() error {
	b.mu.Lock()
	b.isRunning = true
	b.mu.Unlock()
	go func() {
		for b.isRunning {
			m, err := b.reader.ReadMessage(context.Background())
			if err != nil {
				return
			}
			_ = b.SendToPortask("publish", m.Topic, string(m.Value))
		}
	}()
	return nil
}

func (b *KafkaGoBridge) SendToPortask(msgType, topic, data string) error {
	msg := fmt.Sprintf(`{"type":"%s","topic":"%s","data":%q}`, msgType, topic, data)
	_, err := b.portaskConn.Write([]byte(msg + "\n"))
	return err
}

func (b *KafkaGoBridge) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.isRunning {
		close(b.stopChan)
		b.isRunning = false
	}
	var errs []error
	if err := b.writer.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := b.reader.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := b.portaskConn.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}
	return nil
}

// Wrapper methods for KafkaPortaskBridge
func (b *KafkaPortaskBridge) Stop() error {
	switch b.clientType {
	case KafkaClientSarama:
		return b.saramaBridge.Stop()
	case KafkaClientKafkaGo:
		return b.kafkaGoBridge.Stop()
	default:
		return fmt.Errorf("unknown client type: %s", b.clientType)
	}
}

func (b *KafkaPortaskBridge) PublishToKafka(topic string, key string, message []byte) error {
	switch b.clientType {
	case KafkaClientSarama:
		return b.saramaBridge.PublishToKafka(topic, message)
	case KafkaClientKafkaGo:
		return b.kafkaGoBridge.PublishToKafka(topic, message)
	default:
		return fmt.Errorf("unknown client type: %s", b.clientType)
	}
}

func (b *KafkaPortaskBridge) ForwardFromKafkaToPortask(topics []string) error {
	// Production-grade: Start consuming and forward to Portask
	switch b.clientType {
	case KafkaClientSarama:
		return b.saramaBridge.ConsumeFromKafka(topics)
	case KafkaClientKafkaGo:
		return b.kafkaGoBridge.ConsumeFromKafka()
	default:
		return fmt.Errorf("unknown client type: %s", b.clientType)
	}
}

// Edge-case support: Kafka header mapping, partition, offset, batch, priority, encryption
// SaramaKafkaBridge: PublishToKafka, ConsumeFromKafka, SendToPortask fonksiyonlarında header, partition, offset, batch, priority, encryption desteği eklenmeli
// KafkaGoBridge: PublishToKafka, ConsumeFromKafka, SendToPortask fonksiyonlarında header, partition, offset, batch, priority, encryption desteği eklenmeli
