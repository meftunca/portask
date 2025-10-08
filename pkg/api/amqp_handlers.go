package api

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

// ============================================
// AMQP API HANDLERS
// ============================================

// handleAMQPQueues lists all AMQP queues
func (s *FiberServer) handleAMQPQueues(c *fiber.Ctx) error {
	log.Printf("[API] Get AMQP queues")

	// TODO: Get actual queues from AMQP server
	// For now, return sample data

	sampleQueues := []map[string]interface{}{
		{
			"name":      "orders",
			"messages":  150,
			"consumers": 2,
			"state":     "running",
			"durable":   true,
			"autoDelete": false,
			"exclusive": false,
		},
		{
			"name":      "notifications",
			"messages":  45,
			"consumers": 1,
			"state":     "running",
			"durable":   true,
			"autoDelete": false,
			"exclusive": false,
		},
		{
			"name":      "logs",
			"messages":  0,
			"consumers": 0,
			"state":     "idle",
			"durable":   false,
			"autoDelete": true,
			"exclusive": false,
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"queues":  sampleQueues,
		"count":   len(sampleQueues),
	})
}

// handleAMQPExchanges lists all AMQP exchanges
func (s *FiberServer) handleAMQPExchanges(c *fiber.Ctx) error {
	log.Printf("[API] Get AMQP exchanges")

	// TODO: Get actual exchanges from AMQP server
	sampleExchanges := []map[string]interface{}{
		{
			"name":       "amq.direct",
			"type":       "direct",
			"durable":    true,
			"autoDelete": false,
			"internal":   false,
		},
		{
			"name":       "amq.fanout",
			"type":       "fanout",
			"durable":    true,
			"autoDelete": false,
			"internal":   false,
		},
		{
			"name":       "amq.topic",
			"type":       "topic",
			"durable":    true,
			"autoDelete": false,
			"internal":   false,
		},
		{
			"name":       "amq.headers",
			"type":       "headers",
			"durable":    true,
			"autoDelete": false,
			"internal":   false,
		},
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"exchanges": sampleExchanges,
		"count":     len(sampleExchanges),
	})
}

// handleAMQPBindings lists all AMQP bindings
func (s *FiberServer) handleAMQPBindings(c *fiber.Ctx) error {
	log.Printf("[API] Get AMQP bindings")

	// TODO: Get actual bindings from AMQP server
	sampleBindings := []map[string]interface{}{
		{
			"source":      "amq.direct",
			"destination": "orders",
			"routingKey":  "order.created",
			"arguments":   map[string]interface{}{},
		},
		{
			"source":      "amq.topic",
			"destination": "notifications",
			"routingKey":  "notify.*",
			"arguments":   map[string]interface{}{},
		},
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"bindings": sampleBindings,
		"count":    len(sampleBindings),
	})
}

