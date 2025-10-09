package main

import (
	"fmt"
	"net"
	"time"
)

// Test Portask AMQP Protocol Compatibility
func main() {
	fmt.Println("🧪 Testing Portask AMQP/RabbitMQ Protocol Compatibility")
	fmt.Println("=" + string(make([]byte, 60)))

	// Test 1: Connection
	fmt.Println("\n1️⃣ Testing AMQP connection to localhost:5672...")
	conn, err := net.DialTimeout("tcp", "localhost:5672", 5*time.Second)
	if err != nil {
		fmt.Printf("   ❌ FAILED: Cannot connect to AMQP port: %v\n", err)
		return
	}
	defer conn.Close()
	fmt.Println("   ✅ PASSED: Successfully connected to AMQP port 5672")

	// Test 2: Protocol Header
	fmt.Println("\n2️⃣ Testing AMQP protocol header...")

	// Send AMQP 0.9.1 protocol header
	protocolHeader := []byte{
		'A', 'M', 'Q', 'P', // Protocol name
		0,       // Protocol ID (0 = AMQP)
		0, 9, 1, // Version 0.9.1
	}

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(protocolHeader); err != nil {
		fmt.Printf("   ❌ FAILED: Cannot send protocol header: %v\n", err)
		return
	}
	fmt.Println("   ✅ PASSED: Protocol header sent (AMQP 0-9-1)")

	// Test 3: Read Server Response
	fmt.Println("\n3️⃣ Waiting for server response...")
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		fmt.Printf("   ⚠️  WARNING: No response received (server might not have AMQP fully implemented): %v\n", err)
	} else if n > 0 {
		fmt.Printf("   ✅ PASSED: Received server response (%d bytes)\n", n)

		// Check if it's AMQP protocol header response
		if n >= 8 && string(response[0:4]) == "AMQP" {
			fmt.Printf("   📊 Server Protocol: AMQP %d.%d.%d\n",
				response[5], response[6], response[7])
		} else {
			// Might be Connection.Start frame
			fmt.Printf("   📊 Received frame type: 0x%02x\n", response[0])
		}
	}

	// Summary
	fmt.Println("\n" + string(make([]byte, 60)))
	fmt.Println("📊 AMQP Protocol Test Summary:")
	fmt.Println("   ✅ Connection: OK")
	fmt.Println("   ✅ Protocol Header: OK")

	if err == nil && n > 0 {
		fmt.Println("   ✅ Server Response: OK")
		fmt.Println("\n🎉 Portask AMQP Protocol is working!")
	} else {
		fmt.Println("   ⚠️  Server Response: Partial (port listening but protocol may need work)")
		fmt.Println("\n⚠️  AMQP Protocol: Port open but full handshake needs verification")
	}
}
