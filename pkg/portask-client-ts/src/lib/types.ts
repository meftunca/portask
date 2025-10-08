// ==================== Health & Status ====================

export interface HealthStatus {
  status: string;
  version: string;
  uptime: number;
  connections: number;
  memory: Record<string, any>;
  storage: Record<string, any>;
}

// ==================== Message ====================

export interface Message {
  topic: string;
  key?: string;
  value: any;
  headers?: Record<string, any>;
  partition?: number;
  ttl_ms?: number;
}

export interface ProduceResult {
  message_id: string;
  topic: string;
  partition: number;
  offset: number;
  success: boolean;
  error?: string;
}

export interface FetchedMessage {
  message_id: string;
  topic: string;
  partition: number;
  offset: number;
  key?: string;
  value: any;
  headers?: Record<string, any>;
  timestamp: string;
  size_bytes: number;
}

// ==================== Consumer Group ====================

export interface ConsumerGroup {
  id: string;
  name: string;
  state: string;
  protocol: string;
  protocol_type: string;
  leader: string;
  generation: number;
  members: GroupMember[];
  subscriptions: string[];
  created_at: string;
  updated_at: string;
}

export interface GroupMember {
  id: string;
  client_id: string;
  client_host: string;
  session_timeout_ms: number;
  assignment: PartitionAssignment[];
  joined_at: string;
  last_heartbeat: string;
}

export interface PartitionAssignment {
  topic: string;
  partitions: number[];
}

export interface JoinGroupResponse {
  member_id: string;
  generation: number;
  is_leader: boolean;
  members?: GroupMember[];
  assignment: PartitionAssignment[];
}

export interface GroupLag {
  group_id: string;
  total_lag: number;
  partitions: PartitionLag[];
}

export interface PartitionLag {
  topic: string;
  partition: number;
  current_offset: number;
  log_end_offset: number;
  lag: number;
}

export interface OffsetCommit {
  topic: string;
  partition: number;
  offset: number;
  metadata?: string;
}

export interface OffsetInfo {
  offset: number;
  metadata?: string;
}

// ==================== Transaction ====================

export interface Transaction {
  id: string;
  state: string;
  topics: string[];
  messages_count: number;
  created_at: string;
  updated_at: string;
  expires_at: string;
  timeout_ms: number;
}

export interface TransactionStatus {
  transaction: Transaction;
  healthy: boolean;
  can_commit: boolean;
}

// ==================== Topic ====================

export interface Topic {
  name: string;
  partitions: number;
  replication_factor: number;
  config: TopicConfig;
  created_at: string;
  updated_at: string;
  message_count: number;
  total_bytes: number;
}

export interface TopicConfig {
  retention_ms: number;
  compression_type: string;
  max_message_bytes: number;
  min_insync_replicas: number;
}

export interface TopicStats {
  name: string;
  partitions: number;
  message_count: number;
  total_bytes: number;
  first_offset: number;
  last_offset: number;
  replicas: number;
  in_sync_replicas: number;
}

// ==================== Options ====================

export interface ClientOptions {
  baseURL: string;
  apiKey?: string;
  timeout?: number;
  headers?: Record<string, string>;
}

export interface ConsumeOptions {
  topic: string;
  partition?: number;
  startOffset?: number;
  maxMessages?: number;
  maxWaitMs?: number;
  groupId?: string;
  autoCommit?: boolean;
}

export interface BatchPublishRequest {
  messages: Message[];
  transaction_id?: string;
}

export interface BatchPublishResponse {
  success: boolean;
  published: number;
  failed: number;
  results: ProduceResult[];
  duration: string;
}

export interface BatchFetchRequest {
  topics: TopicFetchRequest[];
  max_messages?: number;
  max_wait_ms?: number;
  min_bytes?: number;
  isolation_level?: string;
}

export interface TopicFetchRequest {
  topic: string;
  partitions: PartitionFetchRequest[];
}

export interface PartitionFetchRequest {
  partition: number;
  fetch_offset: number;
  max_bytes?: number;
}

export interface BatchFetchResponse {
  success: boolean;
  topics: TopicFetchResponse[];
  total_messages: number;
  duration: string;
}

export interface TopicFetchResponse {
  topic: string;
  partitions: PartitionFetchResponse[];
}

export interface PartitionFetchResponse {
  partition: number;
  high_water_mark: number;
  messages: FetchedMessage[];
  error?: string;
}

// ==================== Error ====================

export interface ErrorResponse {
  success: false;
  error: string;
}

export interface SuccessResponse<T = any> {
  success: true;
  [key: string]: any;
}

export type APIResponse<T = any> = SuccessResponse<T> | ErrorResponse;
