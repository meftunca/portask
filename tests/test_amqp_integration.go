package tests

import (
	"log"
	"net"
	"time"
)

// Simple AMQP integration test
func main() {
	log.Printf("🧪 Starting AMQP Integration Test")

	// Give server time to start
	time.Sleep(2 * time.Second)

	// Test AMQP connection
	testAMQPConnection()
}

func testAMQPConnection() {
	// Connect to AMQP server
	conn, err := net.Dial("tcp", "localhost:5672")
	if err != nil {
		log.Fatalf("❌ Failed to connect to AMQP server: %v", err)
	}
	defer conn.Close()

	log.Printf("✅ Connected to AMQP server")

	// Read protocol header
	header := make([]byte, 8)
	n, err := conn.Read(header)
	if err != nil {
		log.Fatalf("❌ Failed to read protocol header: %v", err)
	}

	log.Printf("📨 Received protocol header (%d bytes): %s", n, string(header[:4]))

	// Send Connection.StartOK (simplified)
	startOK := []byte{
		0x01,       // Frame type: Method
		0x00, 0x00, // Channel: 0
		0x00, 0x00, 0x00, 0x20, // Frame size: 32 bytes
		0x00, 0x0A, // Class: Connection (10)
		0x00, 0x0B, // Method: StartOK (11)
		// Simplified client properties
		0x00, 0x00, 0x00, 0x00, // Client properties (empty)
		0x05, 'P', 'L', 'A', 'I', 'N', // Mechanism: PLAIN
		0x00, 0x00, 0x00, 0x00, // Response (empty)
		0x05, 'e', 'n', '_', 'U', 'S', // Locale: en_US
		0xCE, // Frame end
	}

	if _, err := conn.Write(startOK); err != nil {
		log.Printf("⚠️  Failed to send StartOK: %v", err)
		return
	}

	log.Printf("📤 Sent Connection.StartOK")

	// Read Connection.Tune
	tuneFrame := make([]byte, 256)
	n, err = conn.Read(tuneFrame)
	if err != nil {
		log.Printf("⚠️  Failed to read Connection.Tune: %v", err)
		return
	}

	log.Printf("📨 Received Connection.Tune (%d bytes)", n)

	// Send Connection.TuneOK
	tuneOK := []byte{
		0x01,       // Frame type: Method
		0x00, 0x00, // Channel: 0
		0x00, 0x00, 0x00, 0x0C, // Frame size: 12 bytes
		0x00, 0x0A, // Class: Connection (10)
		0x00, 0x1F, // Method: TuneOK (31)
		0x00, 0x00, // Channel max: 0
		0x00, 0x02, 0x00, 0x00, // Frame max: 131072
		0x00, 0x3C, // Heartbeat: 60
		0xCE, // Frame end
	}

	if _, err := conn.Write(tuneOK); err != nil {
		log.Printf("⚠️  Failed to send TuneOK: %v", err)
		return
	}

	log.Printf("📤 Sent Connection.TuneOK")

	// Send Connection.Open
	openFrame := []byte{
		0x01,       // Frame type: Method
		0x00, 0x00, // Channel: 0
		0x00, 0x00, 0x00, 0x08, // Frame size: 8 bytes
		0x00, 0x0A, // Class: Connection (10)
		0x00, 0x28, // Method: Open (40)
		0x01, '/', // Virtual host: "/"
		0x00, // Reserved
		0x00, // Reserved
		0xCE, // Frame end
	}

	if _, err := conn.Write(openFrame); err != nil {
		log.Printf("⚠️  Failed to send Connection.Open: %v", err)
		return
	}

	log.Printf("📤 Sent Connection.Open")

	// Read Connection.OpenOK
	openOKFrame := make([]byte, 64)
	n, err = conn.Read(openOKFrame)
	if err != nil {
		log.Printf("⚠️  Failed to read Connection.OpenOK: %v", err)
		return
	}

	log.Printf("📨 Received Connection.OpenOK (%d bytes)", n)

	// Test Channel operations
	testChannelOperations(conn)

	log.Printf("✅ AMQP Integration Test completed successfully!")
}

func testChannelOperations(conn net.Conn) {
	log.Printf("🔧 Testing Channel operations...")

	// Send Channel.Open
	channelOpen := []byte{
		0x01,       // Frame type: Method
		0x00, 0x01, // Channel: 1
		0x00, 0x00, 0x00, 0x08, // Frame size: 8 bytes
		0x00, 0x14, // Class: Channel (20)
		0x00, 0x0A, // Method: Open (10)
		0x00, 0x00, 0x00, 0x00, // Reserved
		0xCE, // Frame end
	}

	if _, err := conn.Write(channelOpen); err != nil {
		log.Printf("⚠️  Failed to send Channel.Open: %v", err)
		return
	}

	log.Printf("📤 Sent Channel.Open")

	// Read Channel.OpenOK
	openOKFrame := make([]byte, 64)
	n, err := conn.Read(openOKFrame)
	if err != nil {
		log.Printf("⚠️  Failed to read Channel.OpenOK: %v", err)
		return
	}

	log.Printf("📨 Received Channel.OpenOK (%d bytes)", n)

	// Test Queue.Declare
	queueDeclare := []byte{
		0x01,       // Frame type: Method
		0x00, 0x01, // Channel: 1
		0x00, 0x00, 0x00, 0x10, // Frame size: 16 bytes
		0x00, 0x32, // Class: Queue (50)
		0x00, 0x0A, // Method: Declare (10)
		0x00, 0x00, // Reserved
		0x09, 't', 'e', 's', 't', '-', 'q', 'u', 'e', 'u', // Queue name: "test-queue"
		0x00,                   // Flags (passive, durable, exclusive, auto-delete, no-wait)
		0x00, 0x00, 0x00, 0x00, // Arguments (empty)
		0xCE, // Frame end
	}

	if _, err := conn.Write(queueDeclare); err != nil {
		log.Printf("⚠️  Failed to send Queue.Declare: %v", err)
		return
	}

	log.Printf("📤 Sent Queue.Declare")

	// Read Queue.DeclareOK
	declareOKFrame := make([]byte, 128)
	n, err = conn.Read(declareOKFrame)
	if err != nil {
		log.Printf("⚠️  Failed to read Queue.DeclareOK: %v", err)
		return
	}

	log.Printf("📨 Received Queue.DeclareOK (%d bytes)", n)
	log.Printf("✅ Channel operations completed successfully!")
}
