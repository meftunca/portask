package portask

import (
	"context"
	"fmt"
)

// TransactionClient manages transactions
type TransactionClient struct {
	client *Client
}

// Begin begins a new transaction
func (tc *TransactionClient) Begin(ctx context.Context, timeoutMs int64, topics []string) (*Transaction, error) {
	req := map[string]interface{}{}

	if timeoutMs > 0 {
		req["timeout_ms"] = timeoutMs
	}
	if len(topics) > 0 {
		req["topics"] = topics
	}

	var response struct {
		TransactionID string `json:"transaction_id"`
		State         string `json:"state"`
		ExpiresAt     string `json:"expires_at"`
	}

	err := tc.client.post(ctx, "/api/v1/transactions/begin", req, &response)
	if err != nil {
		return nil, err
	}

	// TODO: Parse expiresAt string to time.Time
	txn := &Transaction{
		ID:    response.TransactionID,
		State: response.State,
	}

	return txn, nil
}

// Commit commits a transaction
func (tc *TransactionClient) Commit(ctx context.Context, transactionID string) error {
	req := map[string]interface{}{
		"transaction_id": transactionID,
	}

	return tc.client.post(ctx, "/api/v1/transactions/commit", req, nil)
}

// Abort aborts a transaction
func (tc *TransactionClient) Abort(ctx context.Context, transactionID string, reason string) error {
	req := map[string]interface{}{
		"transaction_id": transactionID,
		"reason":         reason,
	}

	return tc.client.post(ctx, "/api/v1/transactions/abort", req, nil)
}

// GetStatus gets transaction status
func (tc *TransactionClient) GetStatus(ctx context.Context, transactionID string) (*TransactionStatus, error) {
	var status TransactionStatus

	path := fmt.Sprintf("/api/v1/transactions/%s", transactionID)
	err := tc.client.get(ctx, path, &status)
	return &status, err
}

// List lists all active transactions
func (tc *TransactionClient) List(ctx context.Context) ([]Transaction, error) {
	var response struct {
		Success      bool          `json:"success"`
		Transactions []Transaction `json:"transactions"`
	}

	err := tc.client.get(ctx, "/api/v1/transactions", &response)
	return response.Transactions, err
}

// Delete deletes/cancels a transaction
func (tc *TransactionClient) Delete(ctx context.Context, transactionID string) error {
	path := fmt.Sprintf("/api/v1/transactions/%s", transactionID)
	return tc.client.delete(ctx, path)
}

