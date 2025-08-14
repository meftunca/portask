// Package client provides a Go client for interacting with the Portask server.
// It supports task submission, status querying, and result retrieval over HTTP.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// Client is the main struct for interacting with the Portask server.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	APIToken   string // Optional: for authentication
}

// NewClient creates a new Portask client.
func NewClient(baseURL string, apiToken string) *Client {
	return &Client{
		BaseURL:    baseURL,
		APIToken:   apiToken,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// TaskRequest represents a task submission payload.
type TaskRequest struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// TaskResponse represents the response after submitting a task.
type TaskResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// TaskStatus represents the status of a submitted task.
type TaskStatus struct {
	TaskID string      `json:"task_id"`
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// SubmitTask submits a new task to the Portask server.
func (c *Client) SubmitTask(ctx context.Context, req TaskRequest) (*TaskResponse, error) {
	url := fmt.Sprintf("%s/api/tasks", c.BaseURL)
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIToken)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var taskResp TaskResponse
	if err := json.Unmarshal(respBody, &taskResp); err != nil {
		return nil, err
	}
	return &taskResp, nil
}

// GetTaskStatus queries the status/result of a submitted task.
func (c *Client) GetTaskStatus(ctx context.Context, taskID string) (*TaskStatus, error) {
	url := fmt.Sprintf("%s/api/tasks/%s", c.BaseURL, taskID)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.APIToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIToken)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var status TaskStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// CancelTask requests cancellation of a running task.
func (c *Client) CancelTask(ctx context.Context, taskID string) error {
	url := fmt.Sprintf("%s/api/tasks/%s/cancel", c.BaseURL, taskID)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	if c.APIToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIToken)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cancel failed: %s", resp.Status)
	}
	return nil
}

// ListTasks fetches a list of recent tasks (optional, if supported by server).
type ListTasksResponse struct {
	Tasks []TaskStatus `json:"tasks"`
}

func (c *Client) ListTasks(ctx context.Context) (*ListTasksResponse, error) {
	url := fmt.Sprintf("%s/api/tasks", c.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.APIToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIToken)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var listResp ListTasksResponse
	if err := json.Unmarshal(respBody, &listResp); err != nil {
		return nil, err
	}
	return &listResp, nil
}
