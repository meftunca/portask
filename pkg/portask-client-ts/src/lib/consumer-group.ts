import type { PortaskClient } from './client';
import type {
  ConsumerGroup,
  JoinGroupResponse,
  GroupLag,
  GroupMember,
  OffsetCommit,
  OffsetInfo,
} from './types';

export class ConsumerGroupClient {
  constructor(private client: PortaskClient) {}

  /**
   * Create a new consumer group
   */
  async create(name: string, topics: string[]): Promise<ConsumerGroup> {
    const response = await this.client.post<{ success: boolean; group: ConsumerGroup }>(
      '/api/v1/consumer-groups',
      { name, topics }
    );
    return response.group;
  }

  /**
   * List all consumer groups
   */
  async list(): Promise<ConsumerGroup[]> {
    const response = await this.client.get<{ success: boolean; groups: ConsumerGroup[] }>(
      '/api/v1/consumer-groups'
    );
    return response.groups;
  }

  /**
   * Get details of a consumer group
   */
  async get(groupId: string): Promise<ConsumerGroup> {
    const response = await this.client.get<{ success: boolean; group: ConsumerGroup }>(
      `/api/v1/consumer-groups/${groupId}`
    );
    return response.group;
  }

  /**
   * Delete a consumer group
   */
  async delete(groupId: string): Promise<void> {
    await this.client.delete(`/api/v1/consumer-groups/${groupId}`);
  }

  /**
   * Update consumer group topics
   */
  async update(groupId: string, topics: string[]): Promise<void> {
    await this.client.put(`/api/v1/consumer-groups/${groupId}`, { topics });
  }

  /**
   * Join a consumer group
   */
  async join(groupId: string, clientId: string): Promise<JoinGroupResponse> {
    const response = await this.client.post<{ success: boolean; response: JoinGroupResponse }>(
      `/api/v1/consumer-groups/${groupId}/join`,
      { client_id: clientId }
    );
    return response.response;
  }

  /**
   * Leave a consumer group
   */
  async leave(groupId: string, memberId: string): Promise<void> {
    await this.client.post(`/api/v1/consumer-groups/${groupId}/leave`, {
      member_id: memberId,
    });
  }

  /**
   * Send heartbeat to consumer group
   */
  async heartbeat(groupId: string, memberId: string, generation: number): Promise<void> {
    await this.client.post(`/api/v1/consumer-groups/${groupId}/heartbeat`, {
      member_id: memberId,
      generation,
    });
  }

  /**
   * Commit offsets for a consumer group
   */
  async commitOffsets(groupId: string, offsets: OffsetCommit[]): Promise<void> {
    await this.client.post(`/api/v1/consumer-groups/${groupId}/offsets/commit`, {
      offsets,
    });
  }

  /**
   * Fetch committed offsets for a consumer group
   */
  async fetchOffsets(groupId: string): Promise<Record<string, Record<number, OffsetInfo>>> {
    const response = await this.client.get<{
      success: boolean;
      offsets: Record<string, Record<number, OffsetInfo>>;
    }>(`/api/v1/consumer-groups/${groupId}/offsets`);
    return response.offsets;
  }

  /**
   * Reset offsets for a consumer group
   */
  async resetOffsets(
    groupId: string,
    topics: string[],
    position: 'earliest' | 'latest'
  ): Promise<void> {
    await this.client.post(`/api/v1/consumer-groups/${groupId}/offsets/reset`, {
      topics,
      position,
    });
  }

  /**
   * Get consumer lag for a consumer group
   */
  async getLag(groupId: string): Promise<GroupLag> {
    const response = await this.client.get<{ success: boolean; lag: GroupLag }>(
      `/api/v1/consumer-groups/${groupId}/lag`
    );
    return response.lag;
  }

  /**
   * List active members of a consumer group
   */
  async listMembers(groupId: string): Promise<GroupMember[]> {
    const response = await this.client.get<{ success: boolean; members: GroupMember[] }>(
      `/api/v1/consumer-groups/${groupId}/members`
    );
    return response.members;
  }

  /**
   * Get the state of a consumer group
   */
  async getState(groupId: string): Promise<Record<string, any>> {
    const response = await this.client.get<{ success: boolean; state: Record<string, any> }>(
      `/api/v1/consumer-groups/${groupId}/state`
    );
    return response.state;
  }
}

