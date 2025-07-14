package json

import (
	"bytes"
	"encoding/json"
	"io"
)

// StandardEncoder implements Encoder using Go's standard encoding/json
type StandardEncoder struct {
	config Config
}

// StandardStreamEncoder wraps json.Encoder
type StandardStreamEncoder struct {
	encoder *json.Encoder
}

// StandardStreamDecoder wraps json.Decoder
type StandardStreamDecoder struct {
	decoder *json.Decoder
}

// NewStandardEncoder creates a new standard JSON encoder
func NewStandardEncoder(config Config) *StandardEncoder {
	return &StandardEncoder{config: config}
}

// Marshal encodes v as JSON
func (e *StandardEncoder) Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(e.config.EscapeHTML)

	if e.config.Compact {
		encoder.SetIndent("", "")
	}

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	// Remove trailing newline added by Encode
	data := buf.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	return data, nil
}

// Unmarshal decodes JSON data into v
func (e *StandardEncoder) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// NewEncoder creates a new streaming encoder
func (e *StandardEncoder) NewEncoder(w io.Writer) StreamEncoder {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(e.config.EscapeHTML)

	if e.config.Compact {
		encoder.SetIndent("", "")
	}

	return &StandardStreamEncoder{encoder: encoder}
}

// NewDecoder creates a new streaming decoder
func (e *StandardEncoder) NewDecoder(r io.Reader) StreamDecoder {
	return &StandardStreamDecoder{decoder: json.NewDecoder(r)}
}

// Encode implements StreamEncoder
func (e *StandardStreamEncoder) Encode(v interface{}) error {
	return e.encoder.Encode(v)
}

// Decode implements StreamDecoder
func (d *StandardStreamDecoder) Decode(v interface{}) error {
	return d.decoder.Decode(v)
}
