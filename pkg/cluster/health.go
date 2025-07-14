package cluster

import (
	"net/http"
	"time"
)

// HealthChecker provides health checking for cluster nodes.
type HealthChecker struct {
	client http.Client
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		client: http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// CheckNode checks the health of a given node address.
func (hc *HealthChecker) CheckNode(addr string) (bool, error) {
	resp, err := hc.client.Get("http://" + addr + "/health")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
