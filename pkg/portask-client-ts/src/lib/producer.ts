import type { PortaskClient } from "./client";
import type { BatchPublishResponse, Message, ProduceResult } from "./types";

export class Producer {
  constructor(private client: PortaskClient) {}

  /**
   * Publish a single message
   */
  async publish(message: Message): Promise<ProduceResult> {
    const response = await this.client.post<{
      success: boolean;
      result: ProduceResult;
    }>("/api/v1/messages/publish", message);
    return response.result || (response as any);
  }

  /**
   * Publish multiple messages in a batch
   */
  async publishBatch(
    messages: Message[],
    transactionId?: string
  ): Promise<ProduceResult[]> {
    const body = {
      messages,
      transaction_id: transactionId,
    };

    const response = await this.client.post<BatchPublishResponse>(
      "/api/v1/messages/batch/publish",
      body
    );

    return response.results;
  }

  /**
   * Publish messages asynchronously (fire-and-forget)
   */
  async publishAsync(messages: Message[]): Promise<void> {
    await this.client.post<{ success: boolean; accepted: number }>(
      "/api/v1/messages/batch/publish/async",
      { messages }
    );
  }
}
