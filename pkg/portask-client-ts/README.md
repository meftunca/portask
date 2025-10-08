# @portask/client

Official TypeScript/JavaScript client library for **Portask Native API**.

## ✨ Features

- **🎯 Unified API**: Single client for all messaging operations
- **📦 Producer & Consumer**: Simple message publish/consume
- **👥 Consumer Groups**: Full consumer group lifecycle management
- **💳 Transactions**: Distributed transaction support
- **⚡ Batch Operations**: High-throughput batch publish/fetch/ack
- **🔒 Type-Safe**: Full TypeScript support with IntelliSense
- **🌐 Universal**: Works in Node.js and modern browsers
- **📦 Zero Dependencies**: Only `ws` for WebSocket support (optional)

## 📦 Installation

```bash
# npm
npm install @portask/client

# yarn
yarn add @portask/client

# pnpm
pnpm add @portask/client

# bun
bun add @portask/client
```

## 🚀 Quick Start

### Create Client

```typescript
import { createClient } from '@portask/client';

const client = createClient({
  baseURL: 'http://localhost:8080',
});

// Check health
const health = await client.health();
console.log(`Connected to Portask ${health.version}`);
```

### Producer (Publish Messages)

```typescript
// Single message
const result = await client.producer().publish({
  topic: 'orders',
  key: 'order-123',
  value: {
    order_id: 123,
    customer: 'John Doe',
    total: 99.99,
  },
  headers: {
    source: 'web-app',
  },
});

console.log(`Published: ${result.message_id} (offset: ${result.offset})`);
```

### Batch Producer

```typescript
// Batch publish
const messages = [
  { topic: 'orders', value: { order_id: 1 } },
  { topic: 'orders', value: { order_id: 2 } },
  { topic: 'orders', value: { order_id: 3 } },
];

const results = await client.producer().publishBatch(messages);
console.log(`Published ${results.length} messages`);
```

### Consumer (Fetch Messages)

```typescript
// Fetch messages
const messages = await client.consumer().fetch({
  topic: 'orders',
  maxMessages: 100,
  maxWaitMs: 5000,
});

for (const msg of messages) {
  console.log(`Received: ${msg.message_id}`, msg.value);
  
  // Acknowledge message
  await client.consumer().acknowledge(msg.message_id);
}
```

### Consumer Groups

```typescript
// Create consumer group
const group = await client.consumerGroup().create('order-processors', ['orders']);
console.log(`Created group: ${group.id}`);

// Join group
const joinResp = await client.consumerGroup().join('order-processors', 'client-1');
console.log(`Joined as member: ${joinResp.member_id}`);

// Commit offsets
await client.consumerGroup().commitOffsets('order-processors', [
  { topic: 'orders', partition: 0, offset: 100 },
  { topic: 'orders', partition: 1, offset: 150 },
]);

// Get lag
const lag = await client.consumerGroup().getLag('order-processors');
console.log(`Total lag: ${lag.total_lag}`);
```

### Transactions

```typescript
// Begin transaction
const txn = await client.transaction().begin(60000, ['orders', 'inventory']);
console.log(`Transaction started: ${txn.id}`);

// Publish messages in transaction
await client.producer().publishBatch([
  { topic: 'orders', value: { order_id: 1 } },
  { topic: 'inventory', value: { product_id: 100, qty: -1 } },
], txn.id);

// Commit transaction
await client.transaction().commit(txn.id);
console.log(`Transaction committed: ${txn.id}`);
```

## 🛠️ Advanced Usage

### With API Key

```typescript
const client = createClient({
  baseURL: 'http://localhost:8080',
  apiKey: 'your-api-key',
});
```

### With Custom Timeout

```typescript
const client = createClient({
  baseURL: 'http://localhost:8080',
  timeout: 60000, // 60 seconds
});
```

### With Custom Headers

```typescript
const client = createClient({
  baseURL: 'http://localhost:8080',
  headers: {
    'X-Custom-Header': 'value',
  },
});
```

### Async Publishing (Fire-and-Forget)

```typescript
await client.producer().publishAsync([
  { topic: 'logs', value: { level: 'info', msg: 'Hello' } },
]);
```

### Long-Polling Consumer

```typescript
// Blocks until messages available or timeout
const messages = await client.consumer().fetchPoll({
  topic: 'orders',
  maxMessages: 10,
  maxWaitMs: 30000, // Wait up to 30 seconds
});
```

### Batch Acknowledgment

```typescript
const messageIds = messages.map(m => m.message_id);
await client.consumer().acknowledgeBatch(messageIds);
```

### Negative Acknowledgment (Requeue)

```typescript
await client.consumer().negativeAcknowledge(
  messageId,
  'Processing failed',
  true, // requeue
  'order-processors' // group ID
);
```

## 📖 API Reference

### Client Methods

- `producer(): Producer` - Get producer instance
- `consumer(): Consumer` - Get consumer instance
- `consumerGroup(): ConsumerGroupClient` - Get consumer group client
- `transaction(): TransactionClient` - Get transaction client
- `health(): Promise<HealthStatus>` - Check server health

### Producer Methods

- `publish(msg): Promise<ProduceResult>` - Publish single message
- `publishBatch(messages, txnId?): Promise<ProduceResult[]>` - Batch publish
- `publishAsync(messages): Promise<void>` - Async publish (fire-and-forget)

### Consumer Methods

- `fetch(opts): Promise<FetchedMessage[]>` - Fetch messages
- `fetchPoll(opts): Promise<FetchedMessage[]>` - Long-polling fetch
- `acknowledge(msgId, groupId?): Promise<void>` - Acknowledge message
- `acknowledgeBatch(msgIds, groupId?): Promise<void>` - Batch acknowledge
- `negativeAcknowledge(msgId, reason?, requeue?, groupId?): Promise<void>` - Negative ack
- `negativeAcknowledgeBatch(msgIds, reason?, requeue?, groupId?): Promise<void>` - Batch negative ack

### Consumer Group Methods

- `create(name, topics): Promise<ConsumerGroup>` - Create group
- `list(): Promise<ConsumerGroup[]>` - List groups
- `get(groupId): Promise<ConsumerGroup>` - Get group
- `delete(groupId): Promise<void>` - Delete group
- `update(groupId, topics): Promise<void>` - Update topics
- `join(groupId, clientId): Promise<JoinGroupResponse>` - Join group
- `leave(groupId, memberId): Promise<void>` - Leave group
- `heartbeat(groupId, memberId, generation): Promise<void>` - Send heartbeat
- `commitOffsets(groupId, offsets): Promise<void>` - Commit offsets
- `fetchOffsets(groupId): Promise<Record<string, Record<number, OffsetInfo>>>` - Fetch offsets
- `resetOffsets(groupId, topics, position): Promise<void>` - Reset offsets
- `getLag(groupId): Promise<GroupLag>` - Get consumer lag
- `listMembers(groupId): Promise<GroupMember[]>` - List members
- `getState(groupId): Promise<Record<string, any>>` - Get state

### Transaction Methods

- `begin(timeoutMs?, topics?): Promise<Transaction>` - Begin transaction
- `commit(txnId): Promise<void>` - Commit transaction
- `abort(txnId, reason?): Promise<void>` - Abort transaction
- `getStatus(txnId): Promise<TransactionStatus>` - Get status
- `list(): Promise<Transaction[]>` - List transactions
- `delete(txnId): Promise<void>` - Delete transaction

## 🌟 Why Portask Native API?

Unlike Kafka or RabbitMQ client libraries, Portask provides:

✅ **Unified Concepts**: Topic (not Queue), Consumer Group (not Consumer Tag)  
✅ **Protocol-Agnostic**: Same API works for Kafka, AMQP, or native clients  
✅ **Modern API**: RESTful HTTP/JSON + WebSocket  
✅ **Simple & Fast**: No complex protocols, no heavyweight dependencies  
✅ **Built-in Features**: Transactions, batching, consumer groups, lag monitoring  

## 📚 Examples

See `examples/` directory for more:

- `simple-producer.ts` - Basic producer
- `simple-consumer.ts` - Basic consumer
- `consumer-group.ts` - Consumer group usage
- `transactions.ts` - Transaction support

## 🔧 Build from Source

```bash
# Install dependencies
npm install

# Build
npm run build

# Test
npm test

# Watch mode
npm run dev
```

## 📄 License

MIT

## 🤝 Contributing

Contributions are welcome! Please open an issue or submit a PR.

## 📞 Support

- GitHub Issues: https://github.com/meftunca/portask/issues
- Documentation: https://github.com/meftunca/portask/tree/main/docs

