package kafka

import (
	"bytes"
	"encoding/binary"
)

// writeString writes a string to the buffer in Kafka format
func (h *KafkaProtocolHandler) writeString(buf *bytes.Buffer, s string) {
	binary.Write(buf, binary.BigEndian, int16(len(s)))
	buf.Write([]byte(s))
}
