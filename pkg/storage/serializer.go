package storage

import (
	"encoding/json"

	"github.com/meftunca/portask/pkg/types"
)

// Serializer defines interface for message serialization
type Serializer interface {
	Serialize(message *types.PortaskMessage) ([]byte, error)
	Deserialize(data []byte) (*types.PortaskMessage, error)
}

// Compressor defines interface for data compression
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

// JSONSerializer implements Serializer using JSON
type JSONSerializer struct{}

func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

func (j *JSONSerializer) Serialize(message *types.PortaskMessage) ([]byte, error) {
	return json.Marshal(message)
}

func (j *JSONSerializer) Deserialize(data []byte) (*types.PortaskMessage, error) {
	var message types.PortaskMessage
	err := json.Unmarshal(data, &message)
	if err != nil {
		return nil, err
	}
	return &message, nil
}

// NoOpCompressor implements Compressor with no compression
type NoOpCompressor struct{}

func NewNoOpCompressor() *NoOpCompressor {
	return &NoOpCompressor{}
}

func (n *NoOpCompressor) Compress(data []byte) ([]byte, error) {
	return data, nil
}

func (n *NoOpCompressor) Decompress(data []byte) ([]byte, error) {
	return data, nil
}

// ZstdCompressor implements Compressor using Zstd
type ZstdCompressor struct {
	level int
}

func NewZstdCompressor(level int) *ZstdCompressor {
	return &ZstdCompressor{level: level}
}

func (z *ZstdCompressor) Compress(data []byte) ([]byte, error) {
	// Implementation would use zstd library
	// For now, placeholder
	return data, nil
}

func (z *ZstdCompressor) Decompress(data []byte) ([]byte, error) {
	// Implementation would use zstd library
	// For now, placeholder
	return data, nil
}

// DragonflyConfig holds Dragonfly-specific configuration
type DragonflyConfig struct {
	Addresses         []string `yaml:"addresses" json:"addresses"`
	Username          string   `yaml:"username" json:"username"`
	Password          string   `yaml:"password" json:"password"`
	DB                int      `yaml:"db" json:"db"`
	EnableCluster     bool     `yaml:"enable_cluster" json:"enable_cluster"`
	KeyPrefix         string   `yaml:"key_prefix" json:"key_prefix"`
	EnableCompression bool     `yaml:"enable_compression" json:"enable_compression"`
	CompressionLevel  int      `yaml:"compression_level" json:"compression_level"`
}
