package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeTask struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type fakeStatus struct {
	TaskID string      `json:"task_id"`
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
}

func TestClient_SubmitAndStatus(t *testing.T) {
	// Fake server
	h := http.NewServeMux()
	h.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var req fakeTask
			json.NewDecoder(r.Body).Decode(&req)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"task_id": "t123",
				"status":  "queued",
			})
		} else if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tasks": []interface{}{},
			})
		}
	})
	h.HandleFunc("/api/tasks/t123", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(fakeStatus{
			TaskID: "t123",
			Status: "done",
			Result: map[string]interface{}{"msg": "ok"},
		})
	})

	ts := httptest.NewServer(h)
	defer ts.Close()

	c := NewClient(ts.URL, "")
	ctx := context.Background()

	resp, err := c.SubmitTask(ctx, TaskRequest{Type: "echo", Data: map[string]interface{}{"msg": "hi"}})
	if err != nil || resp.TaskID != "t123" {
		t.Fatalf("submit failed: %v, resp: %+v", err, resp)
	}

	status, err := c.GetTaskStatus(ctx, "t123")
	if err != nil || status.Status != "done" {
		t.Fatalf("status failed: %v, status: %+v", err, status)
	}
}
