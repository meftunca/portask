package benchmarks

import (
	"testing"

	"github.com/meftunca/portask/pkg/network"
)

func BenchmarkKafkaBridgePublish(b *testing.B) {
	config := network.DefaultKafkaConfig(network.KafkaClientKafkaGo)
	bridge, err := network.NewKafkaPortaskBridge(config, "localhost:9000", "bench-topic")
	if err != nil {
		b.Skip("Kafka bridge not available")
	}
	defer bridge.Stop()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bridge.PublishToKafka("bench-topic", "", []byte("bench-message"))
	}
}
