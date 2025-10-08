package portask

import (
	"context"
	"fmt"
)

// ConsumerGroupClient manages consumer groups
type ConsumerGroupClient struct {
	client *Client
}

// Create creates a new consumer group
func (cg *ConsumerGroupClient) Create(ctx context.Context, name string, topics []string) (*ConsumerGroup, error) {
	req := map[string]interface{}{
		"name":   name,
		"topics": topics,
	}

	var response struct {
		Success bool          `json:"success"`
		Group   ConsumerGroup `json:"group"`
	}

	err := cg.client.post(ctx, "/api/v1/consumer-groups", req, &response)
	return &response.Group, err
}

// List lists all consumer groups
func (cg *ConsumerGroupClient) List(ctx context.Context) ([]ConsumerGroup, error) {
	var response struct {
		Success bool            `json:"success"`
		Groups  []ConsumerGroup `json:"groups"`
	}

	err := cg.client.get(ctx, "/api/v1/consumer-groups", &response)
	return response.Groups, err
}

// Get gets details of a consumer group
func (cg *ConsumerGroupClient) Get(ctx context.Context, groupID string) (*ConsumerGroup, error) {
	var response struct {
		Success bool          `json:"success"`
		Group   ConsumerGroup `json:"group"`
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s", groupID)
	err := cg.client.get(ctx, path, &response)
	return &response.Group, err
}

// Delete deletes a consumer group
func (cg *ConsumerGroupClient) Delete(ctx context.Context, groupID string) error {
	path := fmt.Sprintf("/api/v1/consumer-groups/%s", groupID)
	return cg.client.delete(ctx, path)
}

// Update updates consumer group topics
func (cg *ConsumerGroupClient) Update(ctx context.Context, groupID string, topics []string) error {
	req := map[string]interface{}{
		"topics": topics,
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s", groupID)
	return cg.client.put(ctx, path, req, nil)
}

// Join joins a consumer group
func (cg *ConsumerGroupClient) Join(ctx context.Context, groupID, clientID string) (*JoinGroupResponse, error) {
	req := map[string]interface{}{
		"client_id": clientID,
	}

	var response struct {
		Success  bool              `json:"success"`
		Response JoinGroupResponse `json:"response"`
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s/join", groupID)
	err := cg.client.post(ctx, path, req, &response)
	return &response.Response, err
}

// Leave leaves a consumer group
func (cg *ConsumerGroupClient) Leave(ctx context.Context, groupID, memberID string) error {
	req := map[string]interface{}{
		"member_id": memberID,
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s/leave", groupID)
	return cg.client.post(ctx, path, req, nil)
}

// Heartbeat sends a heartbeat to the consumer group
func (cg *ConsumerGroupClient) Heartbeat(ctx context.Context, groupID, memberID string, generation int32) error {
	req := map[string]interface{}{
		"member_id":  memberID,
		"generation": generation,
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s/heartbeat", groupID)
	return cg.client.post(ctx, path, req, nil)
}

// CommitOffsets commits offsets for a consumer group
func (cg *ConsumerGroupClient) CommitOffsets(ctx context.Context, groupID string, offsets []OffsetCommit) error {
	req := map[string]interface{}{
		"offsets": offsets,
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s/offsets/commit", groupID)
	return cg.client.post(ctx, path, req, nil)
}

// OffsetCommit represents an offset to commit
type OffsetCommit struct {
	Topic     string `json:"topic"`
	Partition int32  `json:"partition"`
	Offset    int64  `json:"offset"`
	Metadata  string `json:"metadata,omitempty"`
}

// FetchOffsets fetches committed offsets for a consumer group
func (cg *ConsumerGroupClient) FetchOffsets(ctx context.Context, groupID string) (map[string]map[int32]OffsetInfo, error) {
	var response struct {
		Success bool                              `json:"success"`
		Offsets map[string]map[int32]OffsetInfo `json:"offsets"`
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s/offsets", groupID)
	err := cg.client.get(ctx, path, &response)
	return response.Offsets, err
}

// OffsetInfo contains offset and metadata
type OffsetInfo struct {
	Offset   int64  `json:"offset"`
	Metadata string `json:"metadata"`
}

// ResetOffsets resets offsets for a consumer group
func (cg *ConsumerGroupClient) ResetOffsets(ctx context.Context, groupID string, topics []string, position string) error {
	req := map[string]interface{}{
		"topics":   topics,
		"position": position, // "earliest" or "latest"
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s/offsets/reset", groupID)
	return cg.client.post(ctx, path, req, nil)
}

// GetLag gets consumer lag for a consumer group
func (cg *ConsumerGroupClient) GetLag(ctx context.Context, groupID string) (*GroupLag, error) {
	var response struct {
		Success bool     `json:"success"`
		Lag     GroupLag `json:"lag"`
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s/lag", groupID)
	err := cg.client.get(ctx, path, &response)
	return &response.Lag, err
}

// ListMembers lists active members of a consumer group
func (cg *ConsumerGroupClient) ListMembers(ctx context.Context, groupID string) ([]GroupMember, error) {
	var response struct {
		Success bool          `json:"success"`
		Members []GroupMember `json:"members"`
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s/members", groupID)
	err := cg.client.get(ctx, path, &response)
	return response.Members, err
}

// GetState gets the state of a consumer group
func (cg *ConsumerGroupClient) GetState(ctx context.Context, groupID string) (map[string]interface{}, error) {
	var response struct {
		Success bool                   `json:"success"`
		State   map[string]interface{} `json:"state"`
	}

	path := fmt.Sprintf("/api/v1/consumer-groups/%s/state", groupID)
	err := cg.client.get(ctx, path, &response)
	return response.State, err
}

