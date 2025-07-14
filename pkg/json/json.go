package json

import (
	"io"
)

// JSONLibrary defines which JSON library to use
type JSONLibrary string

const (
	JSONLibraryStandard JSONLibrary = "standard" // encoding/json
	JSONLibrarySonic    JSONLibrary = "sonic"    // bytedance/sonic
)

// Encoder interface for JSON encoding
type Encoder interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
	NewEncoder(w io.Writer) StreamEncoder
	NewDecoder(r io.Reader) StreamDecoder
}

// StreamEncoder interface for streaming JSON encoding
type StreamEncoder interface {
	Encode(v interface{}) error
}

// StreamDecoder interface for streaming JSON decoding
type StreamDecoder interface {
	Decode(v interface{}) error
}

// Config holds JSON configuration
type Config struct {
	Library    JSONLibrary `mapstructure:"library" yaml:"library" json:"library"`
	Compact    bool        `mapstructure:"compact" yaml:"compact" json:"compact"`
	EscapeHTML bool        `mapstructure:"escape_html" yaml:"escape_html" json:"escape_html"`
}

// DefaultConfig returns default JSON configuration
func DefaultConfig() Config {
	return Config{
		Library:    JSONLibraryStandard,
		Compact:    false,
		EscapeHTML: true,
	}
}

var (
	// Global JSON encoder instance
	globalEncoder Encoder
)

// SetEncoder sets the global JSON encoder
func SetEncoder(encoder Encoder) {
	globalEncoder = encoder
}

// GetEncoder returns the current JSON encoder
func GetEncoder() Encoder {
	if globalEncoder == nil {
		globalEncoder = NewStandardEncoder(DefaultConfig())
	}
	return globalEncoder
}

// Marshal encodes v as JSON using the configured encoder
func Marshal(v interface{}) ([]byte, error) {
	return GetEncoder().Marshal(v)
}

// Unmarshal decodes JSON data into v using the configured encoder
func Unmarshal(data []byte, v interface{}) error {
	return GetEncoder().Unmarshal(data, v)
}

// NewEncoder creates a new JSON stream encoder
func NewEncoder(w io.Writer) StreamEncoder {
	return GetEncoder().NewEncoder(w)
}

// NewDecoder creates a new JSON stream decoder
func NewDecoder(r io.Reader) StreamDecoder {
	return GetEncoder().NewDecoder(r)
}

// InitializeFromConfig initializes the JSON library from configuration
func InitializeFromConfig(config Config) error {
	var encoder Encoder
	var err error

	switch config.Library {
	case JSONLibrarySonic:
		encoder, err = NewSonicEncoder(config)
	case JSONLibraryStandard:
		fallthrough
	default:
		encoder = NewStandardEncoder(config)
	}

	if err != nil {
		return err
	}

	SetEncoder(encoder)
	return nil
}
