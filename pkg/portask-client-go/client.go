package portask

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the main Portask client
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string

	// Lazy-initialized components
	producer      *Producer
	consumer      *Consumer
	consumerGroup *ConsumerGroupClient
	transaction   *TransactionClient
}

// NewClient creates a new Portask client
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	client := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Option is a client configuration option
type Option func(*Client)

// WithAPIKey sets the API key for authentication
func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// Producer returns a producer instance
func (c *Client) Producer() *Producer {
	if c.producer == nil {
		c.producer = &Producer{client: c}
	}
	return c.producer
}

// Consumer returns a consumer instance
func (c *Client) Consumer() *Consumer {
	if c.consumer == nil {
		c.consumer = &Consumer{client: c}
	}
	return c.consumer
}

// ConsumerGroup returns a consumer group client
func (c *Client) ConsumerGroup() *ConsumerGroupClient {
	if c.consumerGroup == nil {
		c.consumerGroup = &ConsumerGroupClient{client: c}
	}
	return c.consumerGroup
}

// Transaction returns a transaction client
func (c *Client) Transaction() *TransactionClient {
	if c.transaction == nil {
		c.transaction = &TransactionClient{client: c}
	}
	return c.transaction
}

// Health checks server health
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	var status HealthStatus
	err := c.get(ctx, "/health", &status)
	return &status, err
}

// ==================== Internal HTTP methods ====================

// get performs a GET request
func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	return c.do(req, result)
}

// post performs a POST request
func (c *Client) post(ctx context.Context, path string, body, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return c.do(req, result)
}

// put performs a PUT request
func (c *Client) put(ctx context.Context, path string, body, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return c.do(req, result)
}

// delete performs a DELETE request
func (c *Client) delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	return c.do(req, nil)
}

// do executes an HTTP request
func (c *Client) do(req *http.Request, result interface{}) error {
	// Set headers
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response if result is provided
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

