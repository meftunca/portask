// Main client
export { PortaskClient, createClient } from './lib/client';

// Components
export { Producer } from './lib/producer';
export { Consumer } from './lib/consumer';
export { ConsumerGroupClient } from './lib/consumer-group';
export { TransactionClient } from './lib/transaction';

// Types
export type {
  // Client
  ClientOptions,
  HealthStatus,
  
  // Messages
  Message,
  ProduceResult,
  FetchedMessage,
  ConsumeOptions,
  
  // Batch Operations
  BatchPublishRequest,
  BatchPublishResponse,
  BatchFetchRequest,
  BatchFetchResponse,
  TopicFetchRequest,
  PartitionFetchRequest,
  TopicFetchResponse,
  PartitionFetchResponse,
  
  // Consumer Groups
  ConsumerGroup,
  GroupMember,
  PartitionAssignment,
  JoinGroupResponse,
  GroupLag,
  PartitionLag,
  OffsetCommit,
  OffsetInfo,
  
  // Transactions
  Transaction,
  TransactionStatus,
  
  // Topics
  Topic,
  TopicConfig,
  TopicStats,
  
  // Responses
  APIResponse,
  SuccessResponse,
  ErrorResponse,
} from './lib/types';

