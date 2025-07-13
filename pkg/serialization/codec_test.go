package serialization

import (
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/types"
)

func TestCodecFactory(t *testing.T) {
	t.Run("NewCodecFactory", func(t *testing.T) {
		factory := NewCodecFactory()
		if factory == nil {
			t.Fatal("Expected factory to be created")
		}
		if factory.codecs == nil {
			t.Fatal("Expected codecs map to be initialized")
		}
	})

	t.Run("InitializeDefaultCodecs", func(t *testing.T) {
		factory := NewCodecFactory()
		cfg := config.DefaultConfig()

		err := factory.InitializeDefaultCodecs(cfg)
		if err != nil {
			t.Fatalf("Failed to initialize default codecs: %v", err)
		}

		// Check that all expected codecs are registered
		availableCodecs := factory.GetAvailableCodecs()
		expectedCodecs := []config.SerializationType{
			config.SerializationCBOR,
			config.SerializationJSON,
			config.SerializationMsgPack,
		}

		if len(availableCodecs) != len(expectedCodecs) {
			t.Fatalf("Expected %d codecs, got %d", len(expectedCodecs), len(availableCodecs))
		}

		for _, expected := range expectedCodecs {
			found := false
			for _, available := range availableCodecs {
				if available == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected codec %s not found", expected)
			}
		}
	})

	t.Run("GetCodec", func(t *testing.T) {
		factory := NewCodecFactory()
		cfg := config.DefaultConfig()
		factory.InitializeDefaultCodecs(cfg)

		// Test getting existing codec
		codec, err := factory.GetCodec(config.SerializationCBOR)
		if err != nil {
			t.Fatalf("Failed to get CBOR codec: %v", err)
		}
		if codec == nil {
			t.Fatal("Expected codec to be returned")
		}

		// Test getting non-existing codec
		_, err = factory.GetCodec("nonexistent")
		if err == nil {
			t.Fatal("Expected error for non-existing codec")
		}
	})
}

func TestCBORCodec(t *testing.T) {
	codec, err := NewCBORCodec(config.CBORConfig{
		DeterministicMode: true,
		TimeMode:          1,
	})
	if err != nil {
		t.Fatalf("Failed to create CBOR codec: %v", err)
	}

	testMessage := &types.PortaskMessage{
		ID:        "test-message-cbor",
		Topic:     "test-topic",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("Test CBOR serialization"),
	}

	t.Run("Encode", func(t *testing.T) {
		data, err := codec.Encode(testMessage)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("Expected encoded data to have content")
		}
	})

	t.Run("Decode", func(t *testing.T) {
		data, err := codec.Encode(testMessage)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}

		decoded, err := codec.Decode(data)
		if err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}

		if decoded.ID != testMessage.ID {
			t.Errorf("Expected ID %s, got %s", testMessage.ID, decoded.ID)
		}
		if decoded.Topic != testMessage.Topic {
			t.Errorf("Expected Topic %s, got %s", testMessage.Topic, decoded.Topic)
		}
		if string(decoded.Payload) != string(testMessage.Payload) {
			t.Errorf("Expected Payload %s, got %s", testMessage.Payload, decoded.Payload)
		}
	})

	t.Run("EncodeBatch", func(t *testing.T) {
		batch := &types.MessageBatch{
			Messages: []*types.PortaskMessage{testMessage},
			BatchID:  "test-batch",
		}

		data, err := codec.EncodeBatch(batch)
		if err != nil {
			t.Fatalf("Failed to encode batch: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("Expected encoded batch to have content")
		}
	})

	t.Run("DecodeBatch", func(t *testing.T) {
		batch := &types.MessageBatch{
			Messages: []*types.PortaskMessage{testMessage},
			BatchID:  "test-batch",
		}

		data, err := codec.EncodeBatch(batch)
		if err != nil {
			t.Fatalf("Failed to encode batch: %v", err)
		}

		decoded, err := codec.DecodeBatch(data)
		if err != nil {
			t.Fatalf("Failed to decode batch: %v", err)
		}

		if decoded.BatchID != batch.BatchID {
			t.Errorf("Expected BatchID %s, got %s", batch.BatchID, decoded.BatchID)
		}
		if len(decoded.Messages) != len(batch.Messages) {
			t.Errorf("Expected %d messages, got %d", len(batch.Messages), len(decoded.Messages))
		}
	})

	t.Run("Properties", func(t *testing.T) {
		if codec.Name() != "cbor" {
			t.Errorf("Expected name 'cbor', got %s", codec.Name())
		}
		if codec.ContentType() != "application/cbor" {
			t.Errorf("Expected content type 'application/cbor', got %s", codec.ContentType())
		}
	})
}

func TestJSONCodec(t *testing.T) {
	codec := NewJSONCodec(config.JSONConfig{
		Compact:    true,
		EscapeHTML: false,
	})

	testMessage := &types.PortaskMessage{
		ID:        "test-message-json",
		Topic:     "test-topic",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("Test JSON serialization"),
	}

	t.Run("Encode", func(t *testing.T) {
		data, err := codec.Encode(testMessage)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("Expected encoded data to have content")
		}
	})

	t.Run("Decode", func(t *testing.T) {
		data, err := codec.Encode(testMessage)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}

		decoded, err := codec.Decode(data)
		if err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}

		if decoded.ID != testMessage.ID {
			t.Errorf("Expected ID %s, got %s", testMessage.ID, decoded.ID)
		}
		if decoded.Topic != testMessage.Topic {
			t.Errorf("Expected Topic %s, got %s", testMessage.Topic, decoded.Topic)
		}
		if string(decoded.Payload) != string(testMessage.Payload) {
			t.Errorf("Expected Payload %s, got %s", testMessage.Payload, decoded.Payload)
		}
	})

	t.Run("Properties", func(t *testing.T) {
		if codec.Name() != "json" {
			t.Errorf("Expected name 'json', got %s", codec.Name())
		}
		expectedContentType := "application/json; charset=utf-8"
		if codec.ContentType() != expectedContentType {
			t.Errorf("Expected content type '%s', got %s", expectedContentType, codec.ContentType())
		}
	})
}

func TestMsgPackCodec(t *testing.T) {
	codec := NewMsgPackCodec(config.MsgPackConfig{
		UseArrayEncodedStructs: true,
	})

	testMessage := &types.PortaskMessage{
		ID:        "test-message-msgpack",
		Topic:     "test-topic",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("Test MessagePack serialization"),
	}

	t.Run("Encode", func(t *testing.T) {
		data, err := codec.Encode(testMessage)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("Expected encoded data to have content")
		}
	})

	t.Run("Decode", func(t *testing.T) {
		data, err := codec.Encode(testMessage)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}

		decoded, err := codec.Decode(data)
		if err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}

		if decoded.ID != testMessage.ID {
			t.Errorf("Expected ID %s, got %s", testMessage.ID, decoded.ID)
		}
		if decoded.Topic != testMessage.Topic {
			t.Errorf("Expected Topic %s, got %s", testMessage.Topic, decoded.Topic)
		}
		if string(decoded.Payload) != string(testMessage.Payload) {
			t.Errorf("Expected Payload %s, got %s", testMessage.Payload, decoded.Payload)
		}
	})

	t.Run("Properties", func(t *testing.T) {
		if codec.Name() != "msgpack" {
			t.Errorf("Expected name 'msgpack', got %s", codec.Name())
		}
		if codec.ContentType() != "application/msgpack" {
			t.Errorf("Expected content type 'application/msgpack', got %s", codec.ContentType())
		}
	})
}

func TestCodecManager(t *testing.T) {
	cfg := config.DefaultConfig()
	manager, err := NewCodecManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create codec manager: %v", err)
	}

	testMessage := &types.PortaskMessage{
		ID:        "test-message-manager",
		Topic:     "test-topic",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("Test codec manager"),
	}

	t.Run("GetActiveCodec", func(t *testing.T) {
		codec := manager.GetActiveCodec()
		if codec == nil {
			t.Fatal("Expected active codec to be returned")
		}
	})

	t.Run("Encode", func(t *testing.T) {
		data, err := manager.Encode(testMessage)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("Expected encoded data to have content")
		}
	})

	t.Run("Decode", func(t *testing.T) {
		data, err := manager.Encode(testMessage)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}

		decoded, err := manager.Decode(data)
		if err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}

		if decoded.ID != testMessage.ID {
			t.Errorf("Expected ID %s, got %s", testMessage.ID, decoded.ID)
		}
	})

	t.Run("SwitchCodec", func(t *testing.T) {
		err := manager.SwitchCodec(config.SerializationJSON)
		if err != nil {
			t.Fatalf("Failed to switch codec: %v", err)
		}

		codec := manager.GetActiveCodec()
		if codec.Name() != "json" {
			t.Errorf("Expected active codec to be 'json', got %s", codec.Name())
		}
	})

	t.Run("EncodeWithCodec", func(t *testing.T) {
		data, err := manager.EncodeWithCodec(testMessage, config.SerializationCBOR)
		if err != nil {
			t.Fatalf("Failed to encode with specific codec: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("Expected encoded data to have content")
		}
	})

	t.Run("DecodeWithCodec", func(t *testing.T) {
		data, err := manager.EncodeWithCodec(testMessage, config.SerializationCBOR)
		if err != nil {
			t.Fatalf("Failed to encode with specific codec: %v", err)
		}

		decoded, err := manager.DecodeWithCodec(data, config.SerializationCBOR)
		if err != nil {
			t.Fatalf("Failed to decode with specific codec: %v", err)
		}

		if decoded.ID != testMessage.ID {
			t.Errorf("Expected ID %s, got %s", testMessage.ID, decoded.ID)
		}
	})
}

// Benchmark tests
func BenchmarkCBOREncode(b *testing.B) {
	codec, _ := NewCBORCodec(config.CBORConfig{
		DeterministicMode: true,
		TimeMode:          1,
	})

	message := &types.PortaskMessage{
		ID:        "bench-message",
		Topic:     "benchmark",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("Benchmark message payload for CBOR encoding"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := codec.Encode(message)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCBORDecode(b *testing.B) {
	codec, _ := NewCBORCodec(config.CBORConfig{
		DeterministicMode: true,
		TimeMode:          1,
	})

	message := &types.PortaskMessage{
		ID:        "bench-message",
		Topic:     "benchmark",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("Benchmark message payload for CBOR decoding"),
	}

	data, _ := codec.Encode(message)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := codec.Decode(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONEncode(b *testing.B) {
	codec := NewJSONCodec(config.JSONConfig{
		Compact:    true,
		EscapeHTML: false,
	})

	message := &types.PortaskMessage{
		ID:        "bench-message",
		Topic:     "benchmark",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("Benchmark message payload for JSON encoding"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := codec.Encode(message)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCodecComparison(b *testing.B) {
	message := &types.PortaskMessage{
		ID:        "bench-message",
		Topic:     "benchmark",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("Benchmark message payload for codec comparison testing"),
	}

	b.Run("CBOR", func(b *testing.B) {
		codec, _ := NewCBORCodec(config.CBORConfig{
			DeterministicMode: true,
			TimeMode:          1,
		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, _ := codec.Encode(message)
			_, _ = codec.Decode(data)
		}
	})

	b.Run("JSON", func(b *testing.B) {
		codec := NewJSONCodec(config.JSONConfig{
			Compact:    true,
			EscapeHTML: false,
		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, _ := codec.Encode(message)
			_, _ = codec.Decode(data)
		}
	})

	b.Run("MessagePack", func(b *testing.B) {
		codec := NewMsgPackCodec(config.MsgPackConfig{
			UseArrayEncodedStructs: true,
		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, _ := codec.Encode(message)
			_, _ = codec.Decode(data)
		}
	})
}
