package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// E2E API Test Suite
// Tests all Portask API endpoints to measure feature coverage

// TestAPIEndpointCoverage comprehensive test for all API endpoints
func TestAPIEndpointCoverage(t *testing.T) {
	// Skip if no server running
	if !isServerRunning() {
		t.Skip("Portask server is not running on localhost:8080")
	}

	baseURL := "http://localhost:8080"

	// Track test results
	results := &TestResults{
		Total:   0,
		Passed:  0,
		Failed:  0,
		Skipped: 0,
		Details: make(map[string]TestResult),
	}

	t.Run("HealthEndpoints", func(t *testing.T) {
		testHealthEndpoints(t, baseURL, results)
	})

	t.Run("CoreMessageAPI", func(t *testing.T) {
		testCoreMessageAPI(t, baseURL, results)
	})

	t.Run("ConsumerGroupsAPI", func(t *testing.T) {
		testConsumerGroupsAPI(t, baseURL, results)
	})

	t.Run("TopicsManagementAPI", func(t *testing.T) {
		testTopicsManagementAPI(t, baseURL, results)
	})

	t.Run("BatchOperationsAPI", func(t *testing.T) {
		testBatchOperationsAPI(t, baseURL, results)
	})

	t.Run("TransactionsAPI", func(t *testing.T) {
		testTransactionsAPI(t, baseURL, results)
	})

	t.Run("KafkaCompatibilityAPI", func(t *testing.T) {
		testKafkaCompatibilityAPI(t, baseURL, results)
	})

	t.Run("AMQPCompatibilityAPI", func(t *testing.T) {
		testAMQPCompatibilityAPI(t, baseURL, results)
	})

	t.Run("SystemAPI", func(t *testing.T) {
		testSystemAPI(t, baseURL, results)
	})

	t.Run("AdminAPI", func(t *testing.T) {
		testAdminAPI(t, baseURL, results)
	})

	// Print comprehensive report
	printTestReport(t, results)
}

// Test structures
type TestResults struct {
	Total   int
	Passed  int
	Failed  int
	Skipped int
	Details map[string]TestResult
}

type TestResult struct {
	Endpoint string
	Method   string
	Status   string // "passed", "failed", "skipped", "not_implemented"
	Message  string
	Duration time.Duration
}

func (r *TestResults) Add(endpoint, method, status, message string, duration time.Duration) {
	r.Total++
	result := TestResult{
		Endpoint: endpoint,
		Method:   method,
		Status:   status,
		Message:  message,
		Duration: duration,
	}

	switch status {
	case "passed":
		r.Passed++
	case "failed":
		r.Failed++
	case "skipped", "not_implemented":
		r.Skipped++
	}

	key := fmt.Sprintf("%s_%s", method, endpoint)
	r.Details[key] = result
}

// Helper functions
func isServerRunning() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:8080/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func makeRequest(method, url string, body interface{}) (*http.Response, time.Duration, error) {
	start := time.Now()
	client := &http.Client{Timeout: 10 * time.Second}

	var req *http.Request
	var err error

	if body != nil {
		jsonData, _ := json.Marshal(body)
		req, err = http.NewRequest(method, url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, 0, err
		}
	}

	resp, err := client.Do(req)
	duration := time.Since(start)
	return resp, duration, err
}

// ==================== TEST SUITES ====================

// 1. Health & Monitoring Endpoints
func testHealthEndpoints(t *testing.T, baseURL string, results *TestResults) {
	tests := []struct {
		name     string
		endpoint string
		method   string
	}{
		{"Root Health", "/health", "GET"},
		{"API Health", "/api/v1/health", "GET"},
		{"Root Metrics", "/metrics", "GET"},
		{"API Metrics", "/api/v1/metrics", "GET"},
		{"Root Status", "/status", "GET"},
		{"API Status", "/api/v1/status", "GET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := baseURL + tt.endpoint
			resp, duration, err := makeRequest(tt.method, url, nil)

			if err != nil {
				results.Add(tt.endpoint, tt.method, "failed", err.Error(), duration)
				t.Errorf("Failed to connect: %v", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == 200 {
				results.Add(tt.endpoint, tt.method, "passed", "OK", duration)
			} else {
				results.Add(tt.endpoint, tt.method, "failed", fmt.Sprintf("Status: %d", resp.StatusCode), duration)
				t.Errorf("Expected 200, got %d", resp.StatusCode)
			}
		})
	}
}

// 2. Core Message API
func testCoreMessageAPI(t *testing.T, baseURL string, results *TestResults) {
	tests := []struct {
		name     string
		endpoint string
		method   string
		body     interface{}
		expected int
	}{
		{
			"List Messages",
			"/api/v1/messages",
			"GET",
			nil,
			200,
		},
		{
			"Publish Message",
			"/api/v1/messages/publish",
			"POST",
			fiber.Map{
				"topic": "test-topic",
				"value": "Hello, Portask!",
				"key":   "test-key",
			},
			200,
		},
		{
			"Fetch Messages",
			"/api/v1/messages/fetch",
			"POST",
			fiber.Map{
				"topic": "test-topic",
				"limit": 10,
			},
			200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := baseURL + tt.endpoint
			resp, duration, err := makeRequest(tt.method, url, tt.body)

			if err != nil {
				results.Add(tt.endpoint, tt.method, "failed", err.Error(), duration)
				t.Errorf("Failed: %v", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == tt.expected || resp.StatusCode == 404 {
				// 404 is acceptable for not implemented features
				status := "passed"
				message := fmt.Sprintf("Status: %d", resp.StatusCode)
				if resp.StatusCode == 404 {
					status = "not_implemented"
					message = "Endpoint not implemented"
				}
				results.Add(tt.endpoint, tt.method, status, message, duration)
			} else {
				results.Add(tt.endpoint, tt.method, "failed", fmt.Sprintf("Status: %d", resp.StatusCode), duration)
				t.Errorf("Expected %d, got %d", tt.expected, resp.StatusCode)
			}
		})
	}
}

// 3. Consumer Groups API
func testConsumerGroupsAPI(t *testing.T, baseURL string, results *TestResults) {
	groupID := fmt.Sprintf("test-group-%d", time.Now().Unix())

	tests := []struct {
		name     string
		endpoint string
		method   string
		body     interface{}
	}{
		{
			"Create Consumer Group",
			"/api/v1/consumer-groups",
			"POST",
			fiber.Map{
				"name":   groupID,
				"topics": []string{"test-topic"},
			},
		},
		{
			"List Consumer Groups",
			"/api/v1/consumer-groups",
			"GET",
			nil,
		},
		{
			"Get Consumer Group",
			fmt.Sprintf("/api/v1/consumer-groups/%s", groupID),
			"GET",
			nil,
		},
		{
			"Join Consumer Group",
			fmt.Sprintf("/api/v1/consumer-groups/%s/join", groupID),
			"POST",
			fiber.Map{
				"member_id":          "test-member-1",
				"client_id":          "test-client-1",
				"session_timeout_ms": 10000,
			},
		},
		{
			"Heartbeat",
			fmt.Sprintf("/api/v1/consumer-groups/%s/heartbeat", groupID),
			"POST",
			fiber.Map{
				"member_id": "test-member-1",
			},
		},
		{
			"Fetch Offsets",
			fmt.Sprintf("/api/v1/consumer-groups/%s/offsets", groupID),
			"GET",
			nil,
		},
		{
			"Commit Offsets",
			fmt.Sprintf("/api/v1/consumer-groups/%s/offsets/commit", groupID),
			"POST",
			fiber.Map{
				"offsets": []fiber.Map{
					{
						"topic":     "test-topic",
						"partition": 0,
						"offset":    100,
						"metadata":  "test-metadata",
					},
				},
			},
		},
		{
			"Get Group Lag",
			fmt.Sprintf("/api/v1/consumer-groups/%s/lag", groupID),
			"GET",
			nil,
		},
		{
			"Leave Consumer Group",
			fmt.Sprintf("/api/v1/consumer-groups/%s/leave", groupID),
			"POST",
			fiber.Map{
				"member_id": "test-member-1",
			},
		},
		{
			"Delete Consumer Group",
			fmt.Sprintf("/api/v1/consumer-groups/%s", groupID),
			"DELETE",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := baseURL + tt.endpoint
			resp, duration, err := makeRequest(tt.method, url, tt.body)

			if err != nil {
				results.Add(tt.endpoint, tt.method, "failed", err.Error(), duration)
				return
			}
			defer resp.Body.Close()

			status := "passed"
			message := fmt.Sprintf("Status: %d", resp.StatusCode)

			if resp.StatusCode == 404 {
				status = "not_implemented"
				message = "Endpoint not implemented"
			} else if resp.StatusCode >= 400 && resp.StatusCode != 404 {
				status = "failed"
			}

			results.Add(tt.endpoint, tt.method, status, message, duration)
		})
	}
}

// 4. Topics Management API
func testTopicsManagementAPI(t *testing.T, baseURL string, results *TestResults) {
	topicName := fmt.Sprintf("test-topic-%d", time.Now().Unix())

	tests := []struct {
		name     string
		endpoint string
		method   string
		body     interface{}
	}{
		{
			"Create Topic",
			"/api/v1/topics",
			"POST",
			fiber.Map{
				"name":             topicName,
				"partitions":       3,
				"replication":      1,
				"retention_ms":     86400000,
				"compression_type": "gzip",
			},
		},
		{
			"List Topics",
			"/api/v1/topics",
			"GET",
			nil,
		},
		{
			"Get Topic",
			fmt.Sprintf("/api/v1/topics/%s", topicName),
			"GET",
			nil,
		},
		{
			"Get Topic Stats",
			fmt.Sprintf("/api/v1/topics/%s/stats", topicName),
			"GET",
			nil,
		},
		{
			"Get Topic Partitions",
			fmt.Sprintf("/api/v1/topics/%s/partitions", topicName),
			"GET",
			nil,
		},
		{
			"Update Topic",
			fmt.Sprintf("/api/v1/topics/%s", topicName),
			"PUT",
			fiber.Map{
				"retention_ms": 172800000,
			},
		},
		{
			"Purge Topic",
			fmt.Sprintf("/api/v1/topics/%s/purge", topicName),
			"POST",
			nil,
		},
		{
			"Delete Topic",
			fmt.Sprintf("/api/v1/topics/%s", topicName),
			"DELETE",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := baseURL + tt.endpoint
			resp, duration, err := makeRequest(tt.method, url, tt.body)

			if err != nil {
				results.Add(tt.endpoint, tt.method, "failed", err.Error(), duration)
				return
			}
			defer resp.Body.Close()

			status := "passed"
			message := fmt.Sprintf("Status: %d", resp.StatusCode)

			if resp.StatusCode == 404 {
				status = "not_implemented"
				message = "Endpoint not implemented"
			} else if resp.StatusCode >= 400 && resp.StatusCode != 404 {
				status = "failed"
			}

			results.Add(tt.endpoint, tt.method, status, message, duration)
		})
	}
}

// 5. Batch Operations API
func testBatchOperationsAPI(t *testing.T, baseURL string, results *TestResults) {
	tests := []struct {
		name     string
		endpoint string
		method   string
		body     interface{}
	}{
		{
			"Batch Publish",
			"/api/v1/messages/batch/publish",
			"POST",
			fiber.Map{
				"messages": []fiber.Map{
					{"topic": "test-topic", "value": "msg1"},
					{"topic": "test-topic", "value": "msg2"},
					{"topic": "test-topic", "value": "msg3"},
				},
			},
		},
		{
			"Batch Publish Async",
			"/api/v1/messages/batch/publish/async",
			"POST",
			fiber.Map{
				"messages": []fiber.Map{
					{"topic": "test-topic", "value": "msg1"},
				},
			},
		},
		{
			"Batch Fetch",
			"/api/v1/messages/batch/fetch",
			"POST",
			fiber.Map{
				"topics": []fiber.Map{
					{
						"topic": "test-topic",
						"partitions": []fiber.Map{
							{
								"partition":    0,
								"fetch_offset": 0,
							},
						},
					},
				},
				"max_messages": 10,
			},
		},
		{
			"Batch Ack",
			"/api/v1/messages/batch/ack",
			"POST",
			fiber.Map{
				"message_ids": []string{"msg-1", "msg-2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := baseURL + tt.endpoint
			resp, duration, err := makeRequest(tt.method, url, tt.body)

			if err != nil {
				results.Add(tt.endpoint, tt.method, "failed", err.Error(), duration)
				return
			}
			defer resp.Body.Close()

			status := "passed"
			message := fmt.Sprintf("Status: %d", resp.StatusCode)

			if resp.StatusCode == 404 {
				status = "not_implemented"
				message = "Endpoint not implemented"
			} else if resp.StatusCode >= 400 && resp.StatusCode != 404 {
				status = "failed"
			}

			results.Add(tt.endpoint, tt.method, status, message, duration)
		})
	}
}

// 6. Transactions API
func testTransactionsAPI(t *testing.T, baseURL string, results *TestResults) {
	tests := []struct {
		name     string
		endpoint string
		method   string
		body     interface{}
	}{
		{
			"Begin Transaction",
			"/api/v1/transactions/begin",
			"POST",
			fiber.Map{
				"producer_id": "test-producer",
			},
		},
		{
			"List Transactions",
			"/api/v1/transactions",
			"GET",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := baseURL + tt.endpoint
			resp, duration, err := makeRequest(tt.method, url, tt.body)

			if err != nil {
				results.Add(tt.endpoint, tt.method, "failed", err.Error(), duration)
				return
			}
			defer resp.Body.Close()

			status := "passed"
			message := fmt.Sprintf("Status: %d", resp.StatusCode)

			if resp.StatusCode == 404 {
				status = "not_implemented"
				message = "Endpoint not implemented"
			} else if resp.StatusCode >= 400 && resp.StatusCode != 404 {
				status = "failed"
			}

			results.Add(tt.endpoint, tt.method, status, message, duration)
		})
	}
}

// 7. Kafka Compatibility API
func testKafkaCompatibilityAPI(t *testing.T, baseURL string, results *TestResults) {
	tests := []struct {
		name     string
		endpoint string
		method   string
	}{
		{"Kafka Consumer Groups", "/api/v1/kafka/consumer-groups", "GET"},
		{"Kafka Consumer Group Detail", "/api/v1/kafka/consumer-groups/test-group", "GET"},
		{"Kafka Consumer Group Lag", "/api/v1/kafka/consumer-groups/test-group/lag", "GET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := baseURL + tt.endpoint
			resp, duration, err := makeRequest(tt.method, url, nil)

			if err != nil {
				results.Add(tt.endpoint, tt.method, "failed", err.Error(), duration)
				return
			}
			defer resp.Body.Close()

			status := "passed"
			message := fmt.Sprintf("Status: %d", resp.StatusCode)

			if resp.StatusCode == 404 {
				status = "not_implemented"
				message = "Endpoint not implemented"
			} else if resp.StatusCode >= 400 && resp.StatusCode != 404 {
				status = "failed"
			}

			results.Add(tt.endpoint, tt.method, status, message, duration)
		})
	}
}

// 8. AMQP Compatibility API
func testAMQPCompatibilityAPI(t *testing.T, baseURL string, results *TestResults) {
	tests := []struct {
		name     string
		endpoint string
		method   string
	}{
		{"AMQP Queues", "/api/v1/amqp/queues", "GET"},
		{"AMQP Exchanges", "/api/v1/amqp/exchanges", "GET"},
		{"AMQP Bindings", "/api/v1/amqp/bindings", "GET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := baseURL + tt.endpoint
			resp, duration, err := makeRequest(tt.method, url, nil)

			if err != nil {
				results.Add(tt.endpoint, tt.method, "failed", err.Error(), duration)
				return
			}
			defer resp.Body.Close()

			status := "passed"
			message := fmt.Sprintf("Status: %d", resp.StatusCode)

			if resp.StatusCode == 404 {
				status = "not_implemented"
				message = "Endpoint not implemented"
			} else if resp.StatusCode >= 400 && resp.StatusCode != 404 {
				status = "failed"
			}

			results.Add(tt.endpoint, tt.method, status, message, duration)
		})
	}
}

// 9. System API
func testSystemAPI(t *testing.T, baseURL string, results *TestResults) {
	tests := []struct {
		name     string
		endpoint string
		method   string
	}{
		{"System Workers", "/api/v1/system/workers", "GET"},
		{"System Storage", "/api/v1/system/storage", "GET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := baseURL + tt.endpoint
			resp, duration, err := makeRequest(tt.method, url, nil)

			if err != nil {
				results.Add(tt.endpoint, tt.method, "failed", err.Error(), duration)
				return
			}
			defer resp.Body.Close()

			status := "passed"
			message := fmt.Sprintf("Status: %d", resp.StatusCode)

			if resp.StatusCode == 404 {
				status = "not_implemented"
				message = "Endpoint not implemented"
			} else if resp.StatusCode >= 400 && resp.StatusCode != 404 {
				status = "failed"
			}

			results.Add(tt.endpoint, tt.method, status, message, duration)
		})
	}
}

// 10. Admin API
func testAdminAPI(t *testing.T, baseURL string, results *TestResults) {
	tests := []struct {
		name     string
		endpoint string
		method   string
	}{
		{"Get Config", "/api/v1/admin/config", "GET"},
		{"Connections", "/api/v1/connections", "GET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := baseURL + tt.endpoint
			resp, duration, err := makeRequest(tt.method, url, nil)

			if err != nil {
				results.Add(tt.endpoint, tt.method, "failed", err.Error(), duration)
				return
			}
			defer resp.Body.Close()

			status := "passed"
			message := fmt.Sprintf("Status: %d", resp.StatusCode)

			if resp.StatusCode == 404 {
				status = "not_implemented"
				message = "Endpoint not implemented"
			} else if resp.StatusCode >= 400 && resp.StatusCode != 404 {
				status = "failed"
			}

			results.Add(tt.endpoint, tt.method, status, message, duration)
		})
	}
}

// Print comprehensive test report
func printTestReport(t *testing.T, results *TestResults) {
	separator := "================================================================================"
	t.Logf("\n")
	t.Logf("%s", separator)
	t.Logf("🎯 PORTASK API COVERAGE REPORT")
	t.Logf("%s", separator)
	t.Logf("")

	// Summary
	coverage := float64(results.Passed) / float64(results.Total) * 100
	implementedRate := float64(results.Passed+results.Failed) / float64(results.Total) * 100

	t.Logf("📊 SUMMARY:")
	t.Logf("   Total Endpoints Tested: %d", results.Total)
	t.Logf("   ✅ Passed:              %d (%.1f%%)", results.Passed, float64(results.Passed)/float64(results.Total)*100)
	t.Logf("   ❌ Failed:              %d (%.1f%%)", results.Failed, float64(results.Failed)/float64(results.Total)*100)
	t.Logf("   ⚠️  Not Implemented:    %d (%.1f%%)", results.Skipped, float64(results.Skipped)/float64(results.Total)*100)
	t.Logf("")
	t.Logf("🎯 COVERAGE:")
	t.Logf("   Working Endpoints:      %.1f%%", coverage)
	t.Logf("   Implemented Endpoints:  %.1f%%", implementedRate)
	t.Logf("")

	// Categorize results
	passed := []TestResult{}
	failed := []TestResult{}
	notImplemented := []TestResult{}

	for _, result := range results.Details {
		switch result.Status {
		case "passed":
			passed = append(passed, result)
		case "failed":
			failed = append(failed, result)
		case "not_implemented", "skipped":
			notImplemented = append(notImplemented, result)
		}
	}

	// Print passed endpoints
	if len(passed) > 0 {
		t.Logf("✅ WORKING ENDPOINTS (%d):", len(passed))
		for _, r := range passed {
			t.Logf("   ✓ %-6s %-50s (%v)", r.Method, r.Endpoint, r.Duration)
		}
		t.Logf("")
	}

	// Print failed endpoints
	if len(failed) > 0 {
		t.Logf("❌ FAILED ENDPOINTS (%d):", len(failed))
		for _, r := range failed {
			t.Logf("   ✗ %-6s %-50s - %s", r.Method, r.Endpoint, r.Message)
		}
		t.Logf("")
	}

	// Print not implemented endpoints
	if len(notImplemented) > 0 {
		t.Logf("⚠️  NOT IMPLEMENTED ENDPOINTS (%d):", len(notImplemented))
		for _, r := range notImplemented {
			t.Logf("   ○ %-6s %-50s", r.Method, r.Endpoint)
		}
		t.Logf("")
	}

	// Final verdict
	t.Logf("%s", separator)
	t.Logf("📈 FINAL SCORE: %.1f%% of promised features are working!", coverage)
	t.Logf("%s", separator)
	t.Logf("")

	// Assert minimum coverage
	require.GreaterOrEqual(t, coverage, 60.0, "API coverage should be at least 60%%")
}

// Benchmark: API Response Times
func BenchmarkAPIResponseTimes(b *testing.B) {
	if !isServerRunning() {
		b.Skip("Portask server is not running")
	}

	baseURL := "http://localhost:8080"

	b.Run("Health", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			makeRequest("GET", baseURL+"/health", nil)
		}
	})

	b.Run("Metrics", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			makeRequest("GET", baseURL+"/metrics", nil)
		}
	})

	b.Run("PublishMessage", func(b *testing.B) {
		body := fiber.Map{
			"topic": "benchmark-topic",
			"value": "benchmark message",
		}
		for i := 0; i < b.N; i++ {
			makeRequest("POST", baseURL+"/api/v1/messages/publish", body)
		}
	})
}

// Test server startup detection
func TestServerStartupDetection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Skip("Portask server did not start within 30 seconds")
			return
		case <-ticker.C:
			if isServerRunning() {
				t.Log("✅ Portask server is running and ready for E2E tests")
				return
			}
		}
	}
}
