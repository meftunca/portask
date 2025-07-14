# API Reference

## Overview

The Portask API provides comprehensive interfaces for message queue operations, supporting both AMQP and Kafka protocols. This reference documents all available endpoints, methods, and configuration options.

## Table of Contents

- [Core API](#core-api)
- [AMQP API](#amqp-api)
- [Kafka API](#kafka-api)
- [Management API](#management-api)
- [WebSocket API](#websocket-api)
- [Configuration API](#configuration-api)
- [Monitoring API](#monitoring-api)

## Core API

### Base URL
```
http://localhost:8080/api/v1
```

### Authentication
```http
POST /auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 3600,
  "refresh_token": "refresh_token_here"
}
```

### Headers
All authenticated requests must include:
```http
Authorization: Bearer <token>
Content-Type: application/json
```

## AMQP API

### Exchange Operations

#### Create Exchange
```http
POST /amqp/exchanges
```

**Request Body:**
```json
{
  "name": "my_exchange",
  "type": "direct",
  "durable": true,
  "auto_delete": false,
  "arguments": {
    "x-message-ttl": 60000,
    "x-max-length": 1000
  }
}
```

**Response:**
```json
{
  "success": true,
  "exchange": {
    "name": "my_exchange",
    "type": "direct",
    "durable": true,
    "auto_delete": false,
    "created_at": "2024-01-01T12:00:00Z"
  }
}
```

#### List Exchanges
```http
GET /amqp/exchanges
```

**Query Parameters:**
- `type` (string): Filter by exchange type
- `durable` (boolean): Filter by durability
- `limit` (integer): Maximum number of results
- `offset` (integer): Pagination offset

**Response:**
```json
{
  "exchanges": [
    {
      "name": "amq.direct",
      "type": "direct",
      "durable": true,
      "auto_delete": false,
      "message_count": 0,
      "created_at": "2024-01-01T12:00:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

#### Get Exchange Details
```http
GET /amqp/exchanges/{name}
```

**Response:**
```json
{
  "name": "my_exchange",
  "type": "direct",
  "durable": true,
  "auto_delete": false,
  "arguments": {
    "x-message-ttl": 60000
  },
  "bindings": [
    {
      "destination": "my_queue",
      "destination_type": "queue",
      "routing_key": "routing.key",
      "arguments": {}
    }
  ],
  "message_stats": {
    "publish_in": 100,
    "publish_out": 95
  }
}
```

#### Delete Exchange
```http
DELETE /amqp/exchanges/{name}
```

**Query Parameters:**
- `if_unused` (boolean): Only delete if no bindings exist

### Queue Operations

#### Create Queue
```http
POST /amqp/queues
```

**Request Body:**
```json
{
  "name": "my_queue",
  "durable": true,
  "exclusive": false,
  "auto_delete": false,
  "arguments": {
    "x-message-ttl": 60000,
    "x-max-length": 1000,
    "x-dead-letter-exchange": "dlx"
  }
}
```

#### List Queues
```http
GET /amqp/queues
```

**Response:**
```json
{
  "queues": [
    {
      "name": "my_queue",
      "durable": true,
      "exclusive": false,
      "auto_delete": false,
      "message_count": 42,
      "consumer_count": 2,
      "memory": 1024,
      "state": "running"
    }
  ]
}
```

#### Get Queue Details
```http
GET /amqp/queues/{name}
```

#### Purge Queue
```http
DELETE /amqp/queues/{name}/contents
```

#### Delete Queue
```http
DELETE /amqp/queues/{name}
```

### Binding Operations

#### Create Binding
```http
POST /amqp/bindings
```

**Request Body:**
```json
{
  "source": "my_exchange",
  "destination": "my_queue",
  "destination_type": "queue",
  "routing_key": "routing.key",
  "arguments": {}
}
```

#### List Bindings
```http
GET /amqp/bindings
```

#### Delete Binding
```http
DELETE /amqp/bindings/{id}
```

### Message Operations

#### Publish Message
```http
POST /amqp/exchanges/{name}/publish
```

**Request Body:**
```json
{
  "routing_key": "routing.key",
  "payload": "Hello, World!",
  "properties": {
    "content_type": "text/plain",
    "delivery_mode": 2,
    "priority": 0,
    "correlation_id": "123",
    "reply_to": "reply_queue",
    "expiration": "60000",
    "message_id": "msg_001",
    "timestamp": "2024-01-01T12:00:00Z",
    "user_id": "guest",
    "app_id": "my_app"
  },
  "headers": {
    "custom_header": "value"
  }
}
```

#### Get Messages
```http
GET /amqp/queues/{name}/get
```

**Query Parameters:**
- `count` (integer): Number of messages to retrieve
- `requeue` (boolean): Whether to requeue messages
- `encoding` (string): Message encoding (auto, base64)

**Response:**
```json
{
  "messages": [
    {
      "payload_bytes": 1024,
      "redelivered": false,
      "exchange": "my_exchange",
      "routing_key": "routing.key",
      "message_count": 41,
      "properties": {
        "content_type": "text/plain",
        "delivery_mode": 2
      },
      "payload": "Hello, World!",
      "payload_encoding": "string"
    }
  ]
}
```

## Kafka API

### Topic Operations

#### Create Topic
```http
POST /kafka/topics
```

**Request Body:**
```json
{
  "name": "my_topic",
  "partitions": 3,
  "replication_factor": 1,
  "config": {
    "cleanup.policy": "delete",
    "retention.ms": "86400000",
    "compression.type": "snappy"
  }
}
```

#### List Topics
```http
GET /kafka/topics
```

**Response:**
```json
{
  "topics": [
    {
      "name": "my_topic",
      "partitions": 3,
      "replication_factor": 1,
      "config": {
        "cleanup.policy": "delete",
        "retention.ms": "86400000"
      }
    }
  ]
}
```

#### Get Topic Details
```http
GET /kafka/topics/{name}
```

#### Delete Topic
```http
DELETE /kafka/topics/{name}
```

### Producer Operations

#### Produce Messages
```http
POST /kafka/topics/{name}/produce
```

**Request Body:**
```json
{
  "records": [
    {
      "key": "key1",
      "value": "message1",
      "partition": 0,
      "headers": {
        "header1": "value1"
      },
      "timestamp": "2024-01-01T12:00:00Z"
    }
  ]
}
```

**Response:**
```json
{
  "offsets": [
    {
      "partition": 0,
      "offset": 123,
      "timestamp": "2024-01-01T12:00:00.123Z"
    }
  ]
}
```

### Consumer Operations

#### Create Consumer
```http
POST /kafka/consumers/{group_id}
```

**Request Body:**
```json
{
  "name": "consumer_instance",
  "format": "json",
  "auto.offset.reset": "earliest",
  "auto.commit.enable": "false"
}
```

#### Subscribe to Topics
```http
POST /kafka/consumers/{group_id}/instances/{instance}/subscription
```

**Request Body:**
```json
{
  "topics": ["topic1", "topic2"]
}
```

#### Consume Messages
```http
GET /kafka/consumers/{group_id}/instances/{instance}/records
```

**Query Parameters:**
- `timeout` (integer): Long polling timeout in milliseconds
- `max_bytes` (integer): Maximum bytes to return

**Response:**
```json
{
  "records": [
    {
      "topic": "my_topic",
      "key": "key1",
      "value": "message1",
      "partition": 0,
      "offset": 123,
      "timestamp": "2024-01-01T12:00:00.123Z",
      "headers": {
        "header1": "value1"
      }
    }
  ]
}
```

#### Commit Offsets
```http
POST /kafka/consumers/{group_id}/instances/{instance}/offsets
```

**Request Body:**
```json
{
  "offsets": [
    {
      "topic": "my_topic",
      "partition": 0,
      "offset": 124
    }
  ]
}
```

#### Delete Consumer
```http
DELETE /kafka/consumers/{group_id}/instances/{instance}
```

### Consumer Group Operations

#### List Consumer Groups
```http
GET /kafka/consumer-groups
```

#### Get Consumer Group Details
```http
GET /kafka/consumer-groups/{group_id}
```

**Response:**
```json
{
  "group_id": "my_group",
  "state": "Stable",
  "protocol": "range",
  "protocol_type": "consumer",
  "members": [
    {
      "member_id": "consumer-1",
      "client_id": "my_client",
      "client_host": "/127.0.0.1",
      "assignment": {
        "topic_partitions": [
          {
            "topic": "my_topic",
            "partitions": [0, 1]
          }
        ]
      }
    }
  ]
}
```

## Management API

### Health Check
```http
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "1h30m45s",
  "services": {
    "amqp": "healthy",
    "kafka": "healthy",
    "database": "healthy"
  }
}
```

### System Information
```http
GET /system/info
```

**Response:**
```json
{
  "version": "1.0.0",
  "go_version": "go1.21.0",
  "os": "linux",
  "arch": "amd64",
  "cpu_count": 8,
  "memory": {
    "total": "16GB",
    "available": "8GB"
  },
  "uptime": "1h30m45s"
}
```

### Configuration
```http
GET /system/config
```

**Response:**
```json
{
  "amqp": {
    "port": 5672,
    "frame_max": 131072,
    "heartbeat": 60
  },
  "kafka": {
    "port": 9092,
    "num_partitions": 3,
    "replication_factor": 1
  },
  "management": {
    "port": 8080,
    "enable_cors": true
  }
}
```

## WebSocket API

### Real-time Connections
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

// Subscribe to queue events
ws.send(JSON.stringify({
  type: 'subscribe',
  resource: 'queue',
  name: 'my_queue'
}));

// Subscribe to exchange events
ws.send(JSON.stringify({
  type: 'subscribe',
  resource: 'exchange',
  name: 'my_exchange'
}));

// Subscribe to topic events
ws.send(JSON.stringify({
  type: 'subscribe',
  resource: 'topic',
  name: 'my_topic'
}));
```

### Event Types
```json
{
  "type": "message_published",
  "resource": "exchange",
  "name": "my_exchange",
  "data": {
    "routing_key": "routing.key",
    "message_count": 1,
    "timestamp": "2024-01-01T12:00:00Z"
  }
}
```

```json
{
  "type": "message_consumed",
  "resource": "queue",
  "name": "my_queue",
  "data": {
    "consumer_tag": "consumer_1",
    "message_count": 41,
    "timestamp": "2024-01-01T12:00:00Z"
  }
}
```

## Monitoring API

### Metrics
```http
GET /metrics
```

**Response:** Prometheus format metrics

### Statistics
```http
GET /stats
```

**Response:**
```json
{
  "amqp": {
    "connections": 5,
    "channels": 12,
    "exchanges": 3,
    "queues": 8,
    "messages": {
      "published": 1000,
      "delivered": 950,
      "acknowledged": 940,
      "redelivered": 10
    }
  },
  "kafka": {
    "topics": 5,
    "partitions": 15,
    "producers": 3,
    "consumers": 2,
    "messages": {
      "produced": 2000,
      "consumed": 1950
    }
  },
  "system": {
    "memory_usage": "256MB",
    "cpu_usage": "15%",
    "goroutines": 45,
    "gc_runs": 23
  }
}
```

### Detailed Queue Stats
```http
GET /amqp/queues/{name}/stats
```

**Response:**
```json
{
  "message_stats": {
    "publish": 100,
    "publish_details": {
      "rate": 1.5
    },
    "deliver": 95,
    "deliver_details": {
      "rate": 1.4
    },
    "get": 5,
    "get_details": {
      "rate": 0.1
    },
    "ack": 90,
    "ack_details": {
      "rate": 1.3
    }
  },
  "messages": 10,
  "messages_ready": 8,
  "messages_unacknowledged": 2,
  "consumers": 2,
  "memory": 1024,
  "backing_queue_status": {
    "mode": "default",
    "q1": 0,
    "q2": 0,
    "delta": ["finite", 0, 0],
    "q3": 8,
    "q4": 0,
    "len": 8,
    "target_ram_count": "infinity",
    "next_seq_id": 101,
    "avg_ingress_rate": 1.5,
    "avg_egress_rate": 1.4,
    "avg_ack_ingress_rate": 1.3,
    "avg_ack_egress_rate": 1.3
  }
}
```

### Performance Metrics
```http
GET /metrics/performance
```

**Response:**
```json
{
  "throughput": {
    "messages_per_second": 1500,
    "bytes_per_second": 1048576
  },
  "latency": {
    "publish_latency_p50": "1ms",
    "publish_latency_p95": "5ms",
    "publish_latency_p99": "10ms",
    "consume_latency_p50": "2ms",
    "consume_latency_p95": "8ms",
    "consume_latency_p99": "15ms"
  },
  "errors": {
    "publish_errors": 5,
    "consume_errors": 2,
    "connection_errors": 1
  }
}
```

## Error Responses

### Standard Error Format
```json
{
  "error": {
    "code": "QUEUE_NOT_FOUND",
    "message": "Queue 'nonexistent_queue' does not exist",
    "details": {
      "queue_name": "nonexistent_queue",
      "available_queues": ["queue1", "queue2"]
    },
    "timestamp": "2024-01-01T12:00:00Z"
  }
}
```

### HTTP Status Codes

| Status Code | Description |
|-------------|-------------|
| 200 | Success |
| 201 | Created |
| 204 | No Content |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 409 | Conflict |
| 422 | Unprocessable Entity |
| 500 | Internal Server Error |
| 503 | Service Unavailable |

### Error Codes

#### AMQP Errors
- `EXCHANGE_NOT_FOUND` - Exchange does not exist
- `QUEUE_NOT_FOUND` - Queue does not exist
- `BINDING_EXISTS` - Binding already exists
- `EXCHANGE_TYPE_INVALID` - Invalid exchange type
- `QUEUE_IN_USE` - Queue is being used by consumers

#### Kafka Errors
- `TOPIC_NOT_FOUND` - Topic does not exist
- `PARTITION_NOT_FOUND` - Partition does not exist
- `CONSUMER_GROUP_NOT_FOUND` - Consumer group does not exist
- `OFFSET_OUT_OF_RANGE` - Requested offset is out of range
- `TOPIC_ALREADY_EXISTS` - Topic already exists

#### General Errors
- `INVALID_REQUEST` - Request format is invalid
- `AUTHENTICATION_FAILED` - Authentication credentials are invalid
- `AUTHORIZATION_FAILED` - User lacks required permissions
- `RATE_LIMIT_EXCEEDED` - Too many requests
- `SERVICE_UNAVAILABLE` - Service is temporarily unavailable

## Rate Limiting

Portask implements rate limiting to prevent abuse:

```http
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1640995200
Retry-After: 60

{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit of 1000 requests per hour exceeded"
  }
}
```

## Pagination

For endpoints that return lists, pagination is supported:

```http
GET /amqp/queues?limit=50&offset=100
```

**Response includes pagination metadata:**
```json
{
  "queues": [...],
  "pagination": {
    "limit": 50,
    "offset": 100,
    "total": 500,
    "has_next": true,
    "has_prev": true,
    "next_url": "/amqp/queues?limit=50&offset=150",
    "prev_url": "/amqp/queues?limit=50&offset=50"
  }
}
```

## SDK Examples

### Go SDK
```go
import "github.com/meftunca/portask-go-sdk"

client := portask.NewClient("http://localhost:8080", "your_token")

// Create exchange
exchange := &portask.Exchange{
    Name:    "my_exchange",
    Type:    "direct",
    Durable: true,
}
err := client.AMQP.CreateExchange(exchange)

// Publish message
message := &portask.Message{
    RoutingKey: "routing.key",
    Payload:    "Hello, World!",
    Properties: portask.MessageProperties{
        ContentType:  "text/plain",
        DeliveryMode: 2,
    },
}
err = client.AMQP.PublishMessage("my_exchange", message)
```

### JavaScript SDK
```javascript
import { PortaskClient } from 'portask-js-sdk';

const client = new PortaskClient('http://localhost:8080', 'your_token');

// Create topic
await client.kafka.createTopic({
  name: 'my_topic',
  partitions: 3,
  replication_factor: 1
});

// Produce message
await client.kafka.produce('my_topic', {
  records: [{
    key: 'key1',
    value: 'Hello, World!'
  }]
});
```

### Python SDK
```python
from portask import PortaskClient

client = PortaskClient('http://localhost:8080', 'your_token')

# Create queue
client.amqp.create_queue({
    'name': 'my_queue',
    'durable': True,
    'arguments': {
        'x-message-ttl': 60000
    }
})

# Get messages
messages = client.amqp.get_messages('my_queue', count=10)
```

## OpenAPI Specification

The complete OpenAPI 3.0 specification is available at:
```
GET /api/openapi.json
```

Interactive documentation (Swagger UI) is available at:
```
http://localhost:8080/docs
```

---

For more information, see:
- [Main Documentation](./README.md)
- [AMQP Emulator](./amqp_emulator.md)
- [Kafka Emulator](./kafka_emulator.md)
- [Performance Guide](./performance.md)
