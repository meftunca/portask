package amqp_test

import (
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/amqp"
	"github.com/meftunca/portask/pkg/storage"
)

func TestEnhancedAMQPServer_StartAndStop(t *testing.T) {
	store := storage.NewInMemoryStorage()
	server := amqp.NewEnhancedAMQPServer(":5673", store)

	go func() {
		if err := server.Start(); err != nil {
			if err.Error() != "accept tcp 127.0.0.1:5673: use of closed network connection" {
				t.Errorf("Server Start() failed: %v", err)
			}
		}
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the server
	if err := server.Stop(); err != nil {
		t.Errorf("Server Stop() failed: %v", err)
	}
}
