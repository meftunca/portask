// Main client
export { PortaskClient, createClient } from "./lib/client";

// Components
export { Consumer } from "./lib/consumer";
export { ConsumerGroupClient } from "./lib/consumer-group";
export { Producer } from "./lib/producer";
export { TransactionClient } from "./lib/transaction";

// Types
export type {
  // Responses
  APIResponse,
  BatchFetchRequest,
  BatchFetchResponse,
  // Batch Operations
  BatchPublishRequest,
  BatchPublishResponse,
  // Client
  ClientOptions,
  ConsumeOptions,
  // Consumer Groups
  ConsumerGroup,
  ErrorResponse,
  FetchedMessage,
  GroupLag,
  GroupMember,
  HealthStatus,
  JoinGroupResponse,
  // Messages
  Message,
  OffsetCommit,
  OffsetInfo,
  PartitionAssignment,
  PartitionFetchRequest,
  PartitionFetchResponse,
  PartitionLag,
  ProduceResult,
  SuccessResponse,
  // Topics
  Topic,
  TopicConfig,
  TopicFetchRequest,
  TopicFetchResponse,
  TopicStats,
  // Transactions
  Transaction,
  TransactionStatus,
} from "./lib/types";
