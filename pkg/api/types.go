package api

import "time"

// Health endpoint response
type HealthResponse struct {
	Status      string                 `json:"status"`
	Version     string                 `json:"version"`
	Uptime      time.Duration          `json:"uptime"`
	Connections int                    `json:"connections"`
	Memory      map[string]interface{} `json:"memory"`
	Storage     map[string]interface{} `json:"storage"`
}

// Metrics endpoint response
type MetricsResponse struct {
	Core      map[string]interface{} `json:"core"`
	Network   map[string]interface{} `json:"network"`
	Storage   map[string]interface{} `json:"storage"`
	API       map[string]interface{} `json:"api"`
	WebSocket map[string]interface{} `json:"websocket"`
	System    map[string]interface{} `json:"system"`
}

// Message publish request
type MessageRequest struct {
	Topic     string                 `json:"topic"`
	Partition int                    `json:"partition"`
	Key       string                 `json:"key"`
	Value     interface{}            `json:"value"`
	Headers   map[string]interface{} `json:"headers"`
	TTL       *time.Duration         `json:"ttl"`
}

// Message fetch request
type FetchRequest struct {
	Topic     string `json:"topic"`
	Partition int    `json:"partition"`
	Offset    int64  `json:"offset"`
	Limit     int    `json:"limit"`
}
