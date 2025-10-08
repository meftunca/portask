package api

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

// ============================================
// KAFKA API HANDLERS
// ============================================

// handleKafkaConsumerGroups lists all consumer groups
func (s *FiberServer) handleKafkaConsumerGroups(c *fiber.Ctx) error {
	// TODO: Get actual consumer groups from Kafka coordinator
	// For now, return sample data that matches frontend expectations

	sampleGroups := []map[string]interface{}{
		{
			"id":           "consumer-group-1",
			"name":         "consumer-group-1",
			"state":        "Stable",
			"protocol":     "range",
			"protocolType": "consumer",
			"members": []map[string]interface{}{
				{
					"id":         "consumer-1",
					"clientId":   "consumer-1",
					"clientHost": "/127.0.0.1",
					"metadata":   "consumer-1-metadata",
					"assignment": []map[string]interface{}{
						{
							"topic":      "orders",
							"partitions": []int{0, 1},
						},
						{
							"topic":      "payments",
							"partitions": []int{0},
						},
					},
				},
			},
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"groups":  sampleGroups,
		"count":   len(sampleGroups),
	})
}

// handleKafkaConsumerGroupDetail gets details for a specific consumer group
func (s *FiberServer) handleKafkaConsumerGroupDetail(c *fiber.Ctx) error {
	groupID := c.Params("id")
	log.Printf("[API] Get consumer group detail: %s", groupID)

	// TODO: Get from actual Kafka coordinator
	sampleGroup := map[string]interface{}{
		"id":           groupID,
		"name":         groupID,
		"state":        "Stable",
		"protocol":     "range",
		"protocolType": "consumer",
		"members": []map[string]interface{}{
			{
				"id":         "consumer-1",
				"clientId":   "consumer-1",
				"clientHost": "/127.0.0.1",
				"metadata":   "consumer-1-metadata",
				"assignment": []map[string]interface{}{
					{
						"topic":      "orders",
						"partitions": []int{0, 1},
					},
				},
			},
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"group":   sampleGroup,
	})
}

// handleKafkaConsumerGroupLag gets lag information for a consumer group
func (s *FiberServer) handleKafkaConsumerGroupLag(c *fiber.Ctx) error {
	groupID := c.Params("id")
	log.Printf("[API] Get consumer group lag: %s", groupID)

	// TODO: Get actual lag from Kafka coordinator
	sampleLag := []map[string]interface{}{
		{
			"group":         groupID,
			"topic":         "orders",
			"partition":     0,
			"currentOffset": 1500,
			"logEndOffset":  1502,
			"lag":           2,
		},
		{
			"group":         groupID,
			"topic":         "orders",
			"partition":     1,
			"currentOffset": 3200,
			"logEndOffset":  3200,
			"lag":           0,
		},
		{
			"group":         groupID,
			"topic":         "payments",
			"partition":     0,
			"currentOffset": 890,
			"logEndOffset":  895,
			"lag":           5,
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"lag":     sampleLag,
		"count":   len(sampleLag),
	})
}
