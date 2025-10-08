import type { PortaskClient } from './client';
import type { Transaction, TransactionStatus } from './types';

export class TransactionClient {
  constructor(private client: PortaskClient) {}

  /**
   * Begin a new transaction
   */
  async begin(timeoutMs?: number, topics?: string[]): Promise<Transaction> {
    const request: any = {};

    if (timeoutMs) {
      request.timeout_ms = timeoutMs;
    }
    if (topics && topics.length > 0) {
      request.topics = topics;
    }

    const response = await this.client.post<{
      transaction_id: string;
      state: string;
      expires_at: string;
    }>('/api/v1/transactions/begin', request);

    return {
      id: response.transaction_id,
      state: response.state,
      topics: topics || [],
      messages_count: 0,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      expires_at: response.expires_at,
      timeout_ms: timeoutMs || 60000,
    };
  }

  /**
   * Commit a transaction
   */
  async commit(transactionId: string): Promise<void> {
    await this.client.post('/api/v1/transactions/commit', {
      transaction_id: transactionId,
    });
  }

  /**
   * Abort a transaction
   */
  async abort(transactionId: string, reason?: string): Promise<void> {
    await this.client.post('/api/v1/transactions/abort', {
      transaction_id: transactionId,
      reason,
    });
  }

  /**
   * Get transaction status
   */
  async getStatus(transactionId: string): Promise<TransactionStatus> {
    return this.client.get<TransactionStatus>(`/api/v1/transactions/${transactionId}`);
  }

  /**
   * List all active transactions
   */
  async list(): Promise<Transaction[]> {
    const response = await this.client.get<{ success: boolean; transactions: Transaction[] }>(
      '/api/v1/transactions'
    );
    return response.transactions;
  }

  /**
   * Delete/cancel a transaction
   */
  async delete(transactionId: string): Promise<void> {
    await this.client.delete(`/api/v1/transactions/${transactionId}`);
  }
}

