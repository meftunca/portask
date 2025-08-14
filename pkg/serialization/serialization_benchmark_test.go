package serialization

import (
	"testing"

	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/types"
)

// Encode işlemi için benchmark
func BenchmarkCodecEncode(b *testing.B) {
	cfg := config.DefaultConfig()
	codec, _ := NewCodecManager(cfg)
	msg := &types.PortaskMessage{ID: "bench", Topic: "bench", Payload: []byte("payload")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Encode(msg)
	}
}

// Decode işlemi için benchmark
func BenchmarkCodecDecode(b *testing.B) {
	cfg := config.DefaultConfig()
	codec, _ := NewCodecManager(cfg)
	msg := &types.PortaskMessage{ID: "bench", Topic: "bench", Payload: []byte("payload")}
	encoded, _ := codec.Encode(msg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		codec.Decode(encoded)
	}
}
