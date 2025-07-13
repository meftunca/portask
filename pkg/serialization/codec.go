package serialization

import (
	"encoding/json"
	"sync"

	"github.com/fxamacker/cbor/v2"
	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/types"
	"github.com/vmihailenco/msgpack/v5"
)

// Codec defines the interface for message serialization/deserialization
type Codec interface {
	// Encode serializes a message to bytes
	Encode(message *types.PortaskMessage) ([]byte, error)

	// Decode deserializes bytes to a message
	Decode(data []byte) (*types.PortaskMessage, error)

	// EncodeBatch serializes a batch of messages
	EncodeBatch(batch *types.MessageBatch) ([]byte, error)

	// DecodeBatch deserializes bytes to a message batch
	DecodeBatch(data []byte) (*types.MessageBatch, error)

	// Name returns the codec name
	Name() string

	// ContentType returns the MIME content type
	ContentType() string
}

// CodecFactory creates codecs based on configuration
type CodecFactory struct {
	codecs map[config.SerializationType]Codec
	mutex  sync.RWMutex
}

// NewCodecFactory creates a new codec factory
func NewCodecFactory() *CodecFactory {
	return &CodecFactory{
		codecs: make(map[config.SerializationType]Codec),
	}
}

// RegisterCodec registers a codec for a serialization type
func (f *CodecFactory) RegisterCodec(serType config.SerializationType, codec Codec) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.codecs[serType] = codec
}

// GetCodec returns a codec for the specified serialization type
func (f *CodecFactory) GetCodec(serType config.SerializationType) (Codec, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	codec, exists := f.codecs[serType]
	if !exists {
		return nil, types.NewPortaskError(types.ErrCodeSerializationError, "unsupported serialization type").
			WithDetail("type", serType)
	}

	return codec, nil
}

// GetAvailableCodecs returns all available codec types
func (f *CodecFactory) GetAvailableCodecs() []config.SerializationType {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	types := make([]config.SerializationType, 0, len(f.codecs))
	for serType := range f.codecs {
		types = append(types, serType)
	}

	return types
}

// InitializeDefaultCodecs initializes all default codecs
func (f *CodecFactory) InitializeDefaultCodecs(cfg *config.Config) error {
	// CBOR Codec
	cborCodec, err := NewCBORCodec(cfg.Serialization.CBORConfig)
	if err != nil {
		return err
	}
	f.RegisterCodec(config.SerializationCBOR, cborCodec)

	// JSON Codec
	jsonCodec := NewJSONCodec(cfg.Serialization.JSONConfig)
	f.RegisterCodec(config.SerializationJSON, jsonCodec)

	// MessagePack Codec
	msgpackCodec := NewMsgPackCodec(cfg.Serialization.MsgPackConfig)
	f.RegisterCodec(config.SerializationMsgPack, msgpackCodec)

	return nil
}

// CBORCodec implements CBOR serialization
type CBORCodec struct {
	encMode cbor.EncMode
	decMode cbor.DecMode
}

// NewCBORCodec creates a new CBOR codec
func NewCBORCodec(cfg config.CBORConfig) (*CBORCodec, error) {
	// Configure CBOR encoding options for optimal performance
	encOpts := cbor.EncOptions{
		Time:        cbor.TimeMode(cfg.TimeMode),
		TimeTag:     cbor.EncTagNone, // No tag for better performance
		IndefLength: cbor.IndefLengthForbidden,
		TagsMd:      cbor.TagsAllowed,
	}

	if cfg.DeterministicMode {
		encOpts.Sort = cbor.SortCanonical
	} else {
		encOpts.Sort = cbor.SortNone // Faster
	}

	decOpts := cbor.DecOptions{
		TimeTag:           cbor.DecTagIgnored,
		IndefLength:       cbor.IndefLengthForbidden,
		TagsMd:            cbor.TagsAllowed,
		IntDec:            cbor.IntDecConvertNone,
		MapKeyByteString:  cbor.MapKeyByteStringAllowed,
		ExtraReturnErrors: cbor.ExtraDecErrorUnknownField,
	}

	encMode, err := encOpts.EncMode()
	if err != nil {
		return nil, types.ErrSerializationError("cbor", err)
	}

	decMode, err := decOpts.DecMode()
	if err != nil {
		return nil, types.ErrSerializationError("cbor", err)
	}

	return &CBORCodec{
		encMode: encMode,
		decMode: decMode,
	}, nil
}

// Encode serializes a message using CBOR
func (c *CBORCodec) Encode(message *types.PortaskMessage) ([]byte, error) {
	data, err := c.encMode.Marshal(message)
	if err != nil {
		return nil, types.ErrSerializationError("cbor", err)
	}
	return data, nil
}

// Decode deserializes CBOR data to a message
func (c *CBORCodec) Decode(data []byte) (*types.PortaskMessage, error) {
	var message types.PortaskMessage
	if err := c.decMode.Unmarshal(data, &message); err != nil {
		return nil, types.ErrDeserializationError("cbor", err)
	}
	return &message, nil
}

// EncodeBatch serializes a message batch using CBOR
func (c *CBORCodec) EncodeBatch(batch *types.MessageBatch) ([]byte, error) {
	data, err := c.encMode.Marshal(batch)
	if err != nil {
		return nil, types.ErrSerializationError("cbor", err)
	}
	return data, nil
}

// DecodeBatch deserializes CBOR data to a message batch
func (c *CBORCodec) DecodeBatch(data []byte) (*types.MessageBatch, error) {
	var batch types.MessageBatch
	if err := c.decMode.Unmarshal(data, &batch); err != nil {
		return nil, types.ErrDeserializationError("cbor", err)
	}
	return &batch, nil
}

// Name returns the codec name
func (c *CBORCodec) Name() string {
	return "cbor"
}

// ContentType returns the MIME content type
func (c *CBORCodec) ContentType() string {
	return "application/cbor"
}

// JSONCodec implements JSON serialization
type JSONCodec struct {
	compact    bool
	escapeHTML bool
}

// NewJSONCodec creates a new JSON codec
func NewJSONCodec(cfg config.JSONConfig) *JSONCodec {
	return &JSONCodec{
		compact:    cfg.Compact,
		escapeHTML: cfg.EscapeHTML,
	}
}

// Encode serializes a message using JSON
func (j *JSONCodec) Encode(message *types.PortaskMessage) ([]byte, error) {
	if j.compact {
		data, err := json.Marshal(message)
		if err != nil {
			return nil, types.ErrSerializationError("json", err)
		}
		return data, nil
	}

	data, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		return nil, types.ErrSerializationError("json", err)
	}
	return data, nil
}

// Decode deserializes JSON data to a message
func (j *JSONCodec) Decode(data []byte) (*types.PortaskMessage, error) {
	var message types.PortaskMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, types.ErrDeserializationError("json", err)
	}
	return &message, nil
}

// EncodeBatch serializes a message batch using JSON
func (j *JSONCodec) EncodeBatch(batch *types.MessageBatch) ([]byte, error) {
	if j.compact {
		data, err := json.Marshal(batch)
		if err != nil {
			return nil, types.ErrSerializationError("json", err)
		}
		return data, nil
	}

	data, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return nil, types.ErrSerializationError("json", err)
	}
	return data, nil
}

// DecodeBatch deserializes JSON data to a message batch
func (j *JSONCodec) DecodeBatch(data []byte) (*types.MessageBatch, error) {
	var batch types.MessageBatch
	if err := json.Unmarshal(data, &batch); err != nil {
		return nil, types.ErrDeserializationError("json", err)
	}
	return &batch, nil
}

// Name returns the codec name
func (j *JSONCodec) Name() string {
	return "json"
}

// ContentType returns the MIME content type
func (j *JSONCodec) ContentType() string {
	if j.escapeHTML {
		return "application/json"
	}
	return "application/json; charset=utf-8"
}

// MsgPackCodec implements MessagePack serialization
type MsgPackCodec struct {
	useArrayEncoded bool
}

// NewMsgPackCodec creates a new MessagePack codec
func NewMsgPackCodec(cfg config.MsgPackConfig) *MsgPackCodec {
	return &MsgPackCodec{
		useArrayEncoded: cfg.UseArrayEncodedStructs,
	}
}

// Encode serializes a message using MessagePack
func (m *MsgPackCodec) Encode(message *types.PortaskMessage) ([]byte, error) {
	data, err := msgpack.Marshal(message)
	if err != nil {
		return nil, types.ErrSerializationError("msgpack", err)
	}
	return data, nil
}

// Decode deserializes MessagePack data to a message
func (m *MsgPackCodec) Decode(data []byte) (*types.PortaskMessage, error) {
	var message types.PortaskMessage
	if err := msgpack.Unmarshal(data, &message); err != nil {
		return nil, types.ErrDeserializationError("msgpack", err)
	}
	return &message, nil
}

// EncodeBatch serializes a message batch using MessagePack
func (m *MsgPackCodec) EncodeBatch(batch *types.MessageBatch) ([]byte, error) {
	data, err := msgpack.Marshal(batch)
	if err != nil {
		return nil, types.ErrSerializationError("msgpack", err)
	}
	return data, nil
}

// DecodeBatch deserializes MessagePack data to a message batch
func (m *MsgPackCodec) DecodeBatch(data []byte) (*types.MessageBatch, error) {
	var batch types.MessageBatch
	if err := msgpack.Unmarshal(data, &batch); err != nil {
		return nil, types.ErrDeserializationError("msgpack", err)
	}
	return &batch, nil
}

// Name returns the codec name
func (m *MsgPackCodec) Name() string {
	return "msgpack"
}

// ContentType returns the MIME content type
func (m *MsgPackCodec) ContentType() string {
	return "application/msgpack"
}

// CodecManager manages serialization codecs with caching and pooling
type CodecManager struct {
	factory     *CodecFactory
	activeCodec Codec
	config      *config.Config

	// Codec pools for reuse
	encodePools map[string]*sync.Pool
	decodePools map[string]*sync.Pool
	mutex       sync.RWMutex
}

// NewCodecManager creates a new codec manager
func NewCodecManager(cfg *config.Config) (*CodecManager, error) {
	factory := NewCodecFactory()
	if err := factory.InitializeDefaultCodecs(cfg); err != nil {
		return nil, err
	}

	activeCodec, err := factory.GetCodec(cfg.Serialization.Type)
	if err != nil {
		return nil, err
	}

	return &CodecManager{
		factory:     factory,
		activeCodec: activeCodec,
		config:      cfg,
		encodePools: make(map[string]*sync.Pool),
		decodePools: make(map[string]*sync.Pool),
	}, nil
}

// GetActiveCodec returns the currently active codec
func (cm *CodecManager) GetActiveCodec() Codec {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.activeCodec
}

// SwitchCodec switches to a different codec
func (cm *CodecManager) SwitchCodec(serType config.SerializationType) error {
	codec, err := cm.factory.GetCodec(serType)
	if err != nil {
		return err
	}

	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.activeCodec = codec

	return nil
}

// Encode encodes a message using the active codec
func (cm *CodecManager) Encode(message *types.PortaskMessage) ([]byte, error) {
	return cm.GetActiveCodec().Encode(message)
}

// Decode decodes data using the active codec
func (cm *CodecManager) Decode(data []byte) (*types.PortaskMessage, error) {
	return cm.GetActiveCodec().Decode(data)
}

// EncodeBatch encodes a batch using the active codec
func (cm *CodecManager) EncodeBatch(batch *types.MessageBatch) ([]byte, error) {
	return cm.GetActiveCodec().EncodeBatch(batch)
}

// DecodeBatch decodes a batch using the active codec
func (cm *CodecManager) DecodeBatch(data []byte) (*types.MessageBatch, error) {
	return cm.GetActiveCodec().DecodeBatch(data)
}

// EncodeWithCodec encodes using a specific codec
func (cm *CodecManager) EncodeWithCodec(message *types.PortaskMessage, serType config.SerializationType) ([]byte, error) {
	codec, err := cm.factory.GetCodec(serType)
	if err != nil {
		return nil, err
	}
	return codec.Encode(message)
}

// DecodeWithCodec decodes using a specific codec
func (cm *CodecManager) DecodeWithCodec(data []byte, serType config.SerializationType) (*types.PortaskMessage, error) {
	codec, err := cm.factory.GetCodec(serType)
	if err != nil {
		return nil, err
	}
	return codec.Decode(data)
}
