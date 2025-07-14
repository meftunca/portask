package json

import (
	"io"

	"github.com/bytedance/sonic"
)

// SonicEncoder implements Encoder using bytedance/sonic
type SonicEncoder struct {
	config Config
	api    sonic.API
}

// SonicStreamEncoder wraps sonic for streaming
type SonicStreamEncoder struct {
	writer io.Writer
	api    sonic.API
}

// SonicStreamDecoder wraps sonic for streaming
type SonicStreamDecoder struct {
	reader io.Reader
	api    sonic.API
}

// NewSonicEncoder creates a new Sonic JSON encoder
func NewSonicEncoder(config Config) (*SonicEncoder, error) {
	// Configure Sonic API options
	apiConfig := sonic.Config{
		EscapeHTML: config.EscapeHTML,
		// Sonic is compact by default, no specific compact setting needed
	}

	api := apiConfig.Froze()

	return &SonicEncoder{
		config: config,
		api:    api,
	}, nil
}

// Marshal encodes v as JSON using Sonic
func (e *SonicEncoder) Marshal(v interface{}) ([]byte, error) {
	return e.api.Marshal(v)
}

// Unmarshal decodes JSON data into v using Sonic
func (e *SonicEncoder) Unmarshal(data []byte, v interface{}) error {
	return e.api.Unmarshal(data, v)
}

// NewEncoder creates a new streaming encoder with Sonic
func (e *SonicEncoder) NewEncoder(w io.Writer) StreamEncoder {
	return &SonicStreamEncoder{
		writer: w,
		api:    e.api,
	}
}

// NewDecoder creates a new streaming decoder with Sonic
func (e *SonicEncoder) NewDecoder(r io.Reader) StreamDecoder {
	return &SonicStreamDecoder{
		reader: r,
		api:    e.api,
	}
}

// Encode implements StreamEncoder using Sonic
func (e *SonicStreamEncoder) Encode(v interface{}) error {
	data, err := e.api.Marshal(v)
	if err != nil {
		return err
	}

	// Add newline for consistency with standard library
	data = append(data, '\n')

	_, err = e.writer.Write(data)
	return err
}

// Decode implements StreamDecoder using Sonic
func (d *SonicStreamDecoder) Decode(v interface{}) error {
	// Read all data from reader
	data, err := io.ReadAll(d.reader)
	if err != nil {
		return err
	}

	return d.api.Unmarshal(data, v)
}
