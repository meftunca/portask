package benchmarks

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func BenchmarkRabbitMQPublish(b *testing.B) {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		b.Skip("RabbitMQ not available")
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		b.Skip("RabbitMQ channel error")
	}
	defer ch.Close()
	ch.ExchangeDeclare("bench_exchange", "topic", true, false, false, false, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ch.Publish("bench_exchange", "bench.key", false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("bench-message"),
		})
	}
}
