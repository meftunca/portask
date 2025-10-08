import type { PortaskClient } from './client';
import type { ConsumeOptions, FetchedMessage, BatchFetchResponse } from './types';

export class Consumer {
  constructor(private client: PortaskClient) {}

  /**
   * Fetch messages from a topic
   */
  async fetch(options: ConsumeOptions): Promise<FetchedMessage[]> {
    const request = {
      topics: [
        {
          topic: options.topic,
          partitions: [
            {
              partition: options.partition ?? 0,
              fetch_offset: options.startOffset ?? 0,
            },
          ],
        },
      ],
      max_messages: options.maxMessages ?? 100,
      max_wait_ms: options.maxWaitMs ?? 1000,
    };

    const response = await this.client.post<BatchFetchResponse>(
      '/api/v1/messages/batch/fetch',
      request
    );

    // Flatten messages from all partitions
    const messages: FetchedMessage[] = [];
    for (const topic of response.topics) {
      for (const partition of topic.partitions) {
        messages.push(...partition.messages);
      }
    }

    return messages;
  }

  /**
   * Fetch messages with long-polling (waits until messages available or timeout)
   */
  async fetchPoll(options: ConsumeOptions): Promise<FetchedMessage[]> {
    const request = {
      topics: [
        {
          topic: options.topic,
          partitions: [
            {
              partition: options.partition ?? 0,
              fetch_offset: options.startOffset ?? 0,
            },
          ],
        },
      ],
      max_messages: options.maxMessages ?? 100,
      max_wait_ms: options.maxWaitMs ?? 5000,
    };

    const response = await this.client.post<BatchFetchResponse>(
      '/api/v1/messages/batch/fetch/poll',
      request
    );

    // Flatten messages from all partitions
    const messages: FetchedMessage[] = [];
    for (const topic of response.topics) {
      for (const partition of topic.partitions) {
        messages.push(...partition.messages);
      }
    }

    return messages;
  }

  /**
   * Acknowledge a message
   */
  async acknowledge(messageId: string, groupId?: string): Promise<void> {
    await this.client.post('/api/v1/messages/batch/ack', {
      message_ids: [messageId],
      group_id: groupId,
    });
  }

  /**
   * Acknowledge multiple messages
   */
  async acknowledgeBatch(messageIds: string[], groupId?: string): Promise<void> {
    await this.client.post('/api/v1/messages/batch/ack', {
      message_ids: messageIds,
      group_id: groupId,
    });
  }

  /**
   * Negative acknowledge a message (requeue or send to DLQ)
   */
  async negativeAcknowledge(
    messageId: string,
    reason?: string,
    requeue: boolean = false,
    groupId?: string
  ): Promise<void> {
    await this.client.post('/api/v1/messages/batch/nack', {
      message_ids: [messageId],
      reason,
      requeue,
      group_id: groupId,
    });
  }

  /**
   * Negative acknowledge multiple messages
   */
  async negativeAcknowledgeBatch(
    messageIds: string[],
    reason?: string,
    requeue: boolean = false,
    groupId?: string
  ): Promise<void> {
    await this.client.post('/api/v1/messages/batch/nack', {
      message_ids: messageIds,
      reason,
      requeue,
      group_id: groupId,
    });
  }
}

