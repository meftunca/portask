package amqp_test

import (
	"log"
	"testing"
	"time"

	"github.com/streadway/amqp"
)

func TestPortaskAMQP_BasicPublishConsume(t *testing.T) {
	t.Skip("Skipping AMQP test - requires RabbitMQ server")
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"test-queue", false, false, false, false, nil,
	)
	if err != nil {
		t.Fatalf("QueueDeclare failed: %v", err)
	}

	body := "hello portask"
	err = ch.Publish(
		"", q.Name, false, false,
		amqp.Publishing{ContentType: "text/plain", Body: []byte(body)},
	)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		t.Fatalf("Consume failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	select {
	case msg := <-msgs:
		if string(msg.Body) != body {
			t.Errorf("Body mismatch: got %q want %q", msg.Body, body)
		}
		if err := msg.Ack(false); err != nil {
			t.Errorf("Ack failed: %v", err)
		}
		log.Printf("[TEST] Message received and acked: %s", msg.Body)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}
