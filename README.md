# Portask: High-Performance Queue Manager

## Project Overview

Portask is envisioned as a cutting-edge, high-performance queue manager developed in Go. Its primary goal is to offer a superior alternative to existing solutions like Kafka and RabbitMQ by providing faster message processing and lower operational costs, while simultaneously ensuring compatibility for seamless migration. Portask will achieve this through a highly optimized internal architecture leveraging CBOR for serialization and Zstandard for compression, alongside a flexible storage interface.

## Core Principles

- **Performance First:** Prioritize low latency and high throughput using Go's concurrency model and optimized libraries.
- **Protocol Agnostic Core:** A robust internal messaging system that can be exposed via multiple external protocols.
- **Ease of Migration:** Provide Kafka and RabbitMQ protocol emulation to allow existing users to switch with minimal code changes.
- **Flexibility & Extensibility:** Decouple storage via interfaces, allowing future backend integrations.
- **Operational Simplicity:** A comprehensive configuration system for easy tuning and dynamic adaptation.
- **Rapid Development:** Utilize modern Go practices and helper libraries to accelerate development cycles.

## Technology Stack

- **Language:** Go (Golang)
- **HTTP/TCP Server:** `net` package for raw TCP listeners (for Kafka/AMQP/Custom protocols). While `fasthttp` is excellent for HTTP, raw TCP is needed for queue managers. We will leverage `net` for its efficiency in this context, keeping the "fast" principle in mind.
- **Serialization:** `github.com/fxamacker/cbor/v2` (or `github.com/ugorji/go/codec` for `codec.CborHandle`) for CBOR.
- **Compression:** `github.com/klauspost/compress/zstd` for Zstandard.
- **Helper Library:** `github.com/samber/lo` for common utility functions and rapid development.
- **Configuration:** `github.com/spf13/viper` or `github.com/BurntSushi/toml` for flexible configuration file parsing (YAML/TOML).
- **Storage Backend:** Dragonfly (initially) via a custom `Storage` interface.
- **Logging:** `go.uber.org/zap` for high-performance structured logging.
- **Metrics:** `github.com/prometheus/client_golang` for Prometheus integration.

## Architectural Overview

Portask will feature a layered architecture:

1. **Protocol Emulation Layer:**
    - Separate listeners for Kafka, RabbitMQ (AMQP 0-9-1), and the custom Portask protocol.
    - Each listener will parse incoming requests according to its respective protocol.
    - Translate external protocol commands into internal Portask message operations.
2. **Core Messaging Layer:**
    - Handles internal message routing, queue management, consumer group management, and acknowledgement logic.
    - All messages within this layer will be serialized using CBOR and compressed with Zstd (either single or bulk, based on configuration).
    - Manages message persistence and retrieval via the `Storage` interface.
3. **Storage Interface Layer:**
    - Defines the `Storage` interface for all data persistence operations (e.g., `StoreMessage`, `FetchMessage`, `UpdateOffset`).
    - Initial implementation will be a `DragonflyStorage` adapter.
4. **Configuration & Management Layer:**
    - A central configuration system (e.g., `config.toml` or `config.yaml`) to define ports, compression settings, storage parameters, and dynamic ratios.
    - Admin/CLI interface for management tasks.

```javascript
+-------------------------------------------------------------------------------------------------+
|                                          Portask Server                                         |
+-------------------------------------------------------------------------------------------------+
| +---------------------+   +---------------------+   +---------------------+   +---------------+
| | Kafka Listener      |   | RabbitMQ Listener   |   | Custom Protocol     |   | Admin/Metrics |
| | (Port: 9092)        |   | (Port: 5672)        |   | Listener (Port: XXXX) |   | (HTTP/gRPC)   |
| +----------|----------+   +----------|----------+   +----------|----------+   +-------|-------+
|            |                       |                       |                             |
|            v                       v                       v                             |
| +-----------------------------------------------------------------------------------------+
| |                  Protocol Translation & Internal Message Dispatch                       |
| |  (Parses external protocol, translates to internal Portask operations)                  |
| +-----------------------------------------------------------------------------------------+
|            |
|            v
| +-----------------------------------------------------------------------------------------+
| |                          Core Messaging & Queue Management                              |
| |  (CBOR Serialization, Zstd Compression/Decompression (Single/Bulk, Dynamic),           |
| |   Message Routing, Consumer Group/Offset Management, Acknowledgements)                  |
| +-----------------------------------------------------------------------------------------+
|            |
|            v
| +-----------------------------------------------------------------------------------------+
| |                                 Storage Interface                                       |
| |  (Interface: StoreMessage, FetchMessage, UpdateOffset, etc.)                            |
| +-----------------------------------------------------------------------------------------+
|            |
|            v
| +-----------------------------------------------------------------------------------------+
| |                                 Dragonfly Adapter                                       |
| |  (Implements Storage Interface using Dragonfly/Redis commands)                          |
| +-----------------------------------------------------------------------------------------+


```

## Detailed Phases & To-Do List

### Phase 0: Project Setup & Initial Research (1 Week)

- **Objective:** Establish project foundation, understand core technologies, and define initial structures.
- **To-Do List:**
    - [ ] **Market Research:**
        - [ ] Analyze Kafka and RabbitMQ's core features, common use cases, and known limitations.
        - [ ] Identify key performance bottlenecks and architectural decisions in existing solutions.
        - [ ] Research existing high-performance Go networking patterns for queue managers.
    - [ ] **Project Repository Setup:**
        - [ ] Create GitHub repository (`portask`).
        - [ ] Define initial `go.mod` and project structure (e.g., `cmd`, `internal`, `pkg`, `configs`).
    - [ ] **Basic Go Environment Setup:**
        - [ ] Install Go.
        - [ ] Verify `fasthttp` (or `net`), `cbor`, `zstd`, `lo`, `viper` installations.
        - [ ] Implement a basic "Hello World" TCP server using `net` to confirm network setup.
    - [ ] **Documentation Setup:**
        - [ ] Create `README.md` with project vision.
        - [ ] Set up basic `CONTRIBUTING.md` and `LICENSE`.

### Phase 1: Core Custom Protocol & Internal Messaging (4 Weeks)

- **Objective:** Develop the high-performance internal messaging core and the custom Portask protocol, integrated with an abstract storage layer and dynamic configuration.
- **To-Do List:**
    - [x] **Define Core Message Structure:**
        - [x] Design the canonical `PortaskMessage` struct in Go, including fields for ID, topic/queue, payload, headers, timestamp, etc.
        - [x] Add `cbor` struct tags for efficient serialization.
    - [x] **CBOR Serialization/Deserialization:**
        - [x] Implement `EncodeCBOR(message PortaskMessage) ([]byte, error)` function.
        - [x] Implement `DecodeCBOR([]byte) (PortaskMessage, error)` function.
        - [x] Write unit tests for CBOR encoding/decoding.
    - [x] **Zstd Compression/Decompression:**
        - [x] Implement `CompressZstd(data []byte, level zstd.EncoderLevel) ([]byte, error)`.
        - [x] Implement `DecompressZstd(compressedData []byte) ([]byte, error)`.
        - [x] Implement logic for **single message compression**.
        - [x] Implement logic for **bulk message compression** (batching multiple CBOR-encoded messages before Zstd compression).
        - [x] Implement **length prefixing** for each compressed message/batch to enable stream parsing.
        - [x] Write unit tests for Zstd compression/decompression.
    - [x] **`Storage`&#32;Interface Design:**
        - [x] Define `pkg/storage/storage.go` with an interface (e.g., `MessageStore`) for methods like `Store(msg PortaskMessage) error`, `Fetch(topic string, offset int) (PortaskMessage, error)`, `UpdateOffset(topic string, consumerID string, offset int) error`, etc.
    - [x] **Dragonfly Adapter Implementation:**
        - [x] Implement `pkg/storage/dragonfly.go` that satisfies the `MessageStore` interface.
        - [x] Use `go-redis/redis` client library to interact with Dragonfly.
        - [x] Map Portask concepts (topics, messages, offsets) to Redis data structures (e.g., Redis Streams, Lists, Hashes).
        - [x] Implement basic error handling and connection management for Dragonfly.
        - [x] Write unit tests for Dragonfly adapter (mocking Redis or using a test instance).
    - [x] **Configuration System:**
        - [x] Define a comprehensive `Config` struct in `pkg/config/config.go` (e.g., using `viper` for YAML/TOML parsing).
        - [x] Include parameters for:
            - Network ports (Custom, Kafka, RabbitMQ).
            - Dragonfly connection details.
            - **Compression Strategy:** `single_message`, `bulk_batching`.
            - **Zstd Compression Level:** (e.g., `1` to `22`).
            - **Bulk Batch Size:** (e.g., `100` messages or `1MB`).
            - **Dynamic Adjustment Ratios (e.g., CPU/Memory thresholds):**
                - `cpu_threshold_high_percent`: If CPU > this, reduce compression level or switch to single.
                - `cpu_threshold_low_percent`: If CPU < this, increase compression level or switch to bulk.
                - `memory_threshold_high_percent`: Similar for memory.
                - `latency_threshold_high_ms`: If average message latency > this, prioritize speed over compression.
        - [x] Implement `LoadConfig(path string) (*Config, error)`.
        - [x] Implement a **dynamic adjustment manager** that monitors system metrics (CPU, Memory, Latency) and dynamically updates compression strategy/level based on configured ratios.
    - [x] **Custom Protocol Server:**
        - [x] Implement a `net.Listen` TCP server in `cmd/portask/main.go`.
        - [x] Handle incoming connections in goroutines.
        - [x] Implement basic custom protocol commands (e.g., `PUBLISH <topic> <payload>`, `SUBSCRIBE <topic>`, `ACK <messageID>`).
        - [x] Integrate with the Core Messaging Layer for message handling.
        - [x] Ensure internal messages are CBOR/Zstd compressed before storage and decompressed after retrieval.
    - [x] **Internal Message Bus/Queue:**
        - [x] Design and implement a simple in-memory message queue using Go channels for internal routing between protocol layers and the storage layer.
        - [x] Ensure this internal bus handles `PortaskMessage` structs, not raw bytes, allowing for consistent CBOR/Zstd handling.

### Phase 2: Kafka Protocol Emulation (6 Weeks)

- **Objective:** Implement a functional Kafka protocol listener that can communicate with standard Kafka clients.
- **To-Do List:**
    - [ ] **Kafka Protocol Deep Dive:**
        - [ ] Thoroughly study Kafka's wire protocol specification (API versions, request/response formats).
        - [ ] Focus on key APIs: `Produce`, `Fetch`, `Metadata`, `OffsetCommit`, `OffsetFetch`.
    - [ ] **Kafka Listener:**
        - [ ] Implement a dedicated TCP listener for Kafka on the configured port (e.g., 9092) in `pkg/protocols/kafka/server.go`.
        - [ ] Implement a generic Kafka request/response parser/builder.
    - [ ] **API Implementations:**
        - [ ] **`Produce`&#32;API:**
            - [ ] Parse incoming `ProduceRequest`.
            - [ ] Convert Kafka records to `PortaskMessage` (CBOR/Zstd applied internally).
            - [ ] Store messages via `Storage` interface.
            - [ ] Send `ProduceResponse`.
        - [ ] **`Fetch`&#32;API:**
            - [ ] Parse incoming `FetchRequest`.
            - [ ] Retrieve messages from `Storage` interface based on topic/partition/offset.
            - [ ] Convert `PortaskMessage` to Kafka records (CBOR/Zstd handled internally).
            - [ ] Send `FetchResponse`.
        - [ ] **`Metadata`&#32;API:**
            - [ ] Respond with cluster/topic/partition metadata.
            - [ ] Map Portask's internal topic/partition structure to Kafka's metadata.
        - [ ] **`OffsetCommit`&#32;/&#32;`OffsetFetch`&#32;APIs:**
            - [ ] Implement consumer group management and offset storage using the `Storage` interface.
    - [ ] **Kafka Concept Mapping:**
        - [ ] Map Kafka "topics" and "partitions" to Portask's internal queue/topic management.
        - [ ] Map Kafka "consumer groups" to Portask's consumer tracking.
    - [ ] **Internal Compatibility:**
        - [ ] Ensure all messages produced via Kafka protocol are stored and retrieved using Portask's internal CBOR/Zstd format.
        - [ ] Verify that messages produced via Kafka can be consumed via the Custom Protocol, and vice-versa.
    - [ ] **Testing:**
        - [ ] Use standard Kafka client libraries (e.g., `confluent-kafka-go` or `sarama`) to connect and test basic produce/consume operations against Portask.

### Phase 3: RabbitMQ (AMQP 0-9-1) Protocol Emulation (6 Weeks)

- **Objective:** Implement a functional AMQP 0-9-1 protocol listener that can communicate with standard RabbitMQ clients.
- **To-Do List:**
    - [ ] **AMQP 0-9-1 Protocol Deep Dive:**
        - [ ] Thoroughly study AMQP 0-9-1 specification (frames, methods, classes).
        - [ ] Focus on core commands: `Connection`, `Channel`, `Exchange`, `Queue`, `Basic` methods.
    - [ ] **AMQP Listener:**
        - [ ] Implement a dedicated TCP listener for AMQP on the configured port (e.g., 5672) in `pkg/protocols/amqp/server.go`.
        - [ ] Implement an AMQP frame parser/builder.
    - [ ] **API Implementations:**
        - [ ] **`Connection`&#32;&&#32;`Channel`&#32;Management:** Handle connection setup, channel open/close.
        - [ ] **`Exchange.Declare`:** Map AMQP exchanges to internal routing logic.
        - [ ] **`Queue.Declare`:** Map AMQP queues to internal Portask queues.
        - [ ] **`Queue.Bind`:** Implement binding logic between exchanges and queues.
        - [ ] **`Basic.Publish`:**
            - [ ] Parse incoming `Basic.Publish` messages.
            - [ ] Convert AMQP message to `PortaskMessage` (CBOR/Zstd applied internally).
            - [ ] Route message based on exchange/routing key to internal queues.
            - [ ] Store messages via `Storage` interface.
        - [ ] **`Basic.Consume`:**
            - [ ] Handle consumer registration.
            - [ ] Fetch messages from internal queues via `Storage` interface.
            - [ ] Convert `PortaskMessage` to AMQP message format (CBOR/Zstd handled internally).
            - [ ] Deliver messages to consumers.
        - [ ] **`Basic.Ack`&#32;/&#32;`Basic.Nack`:** Implement message acknowledgement using the `Storage` interface.
    - [ ] **AMQP Concept Mapping:**
        - [ ] Map AMQP "exchanges," "queues," and "bindings" to Portask's internal routing and queue management.
        - [ ] Map AMQP "channels" and "consumer tags" to Portask's session management.
    - [ ] **Internal Compatibility:**
        - [ ] Ensure all messages produced via AMQP protocol are stored and retrieved using Portask's internal CBOR/Zstd format.
        - [ ] Verify that messages produced via AMQP can be consumed via the Custom Protocol, and vice-versa.
    - [ ] **Testing:**
        - [ ] Use standard AMQP client libraries (e.g., `streadway/amqp` for Go, `pika` for Python) to connect and test basic publish/consume operations against Portask.

### Phase 4: Advanced Features & Optimization (5 Weeks)

- **Objective:** Enhance Portask with high availability, security, and advanced monitoring capabilities.
- **To-Do List:**
    - [ ] **High Availability & Replication:**
        - [ ] Design and implement a leader election mechanism (e.g., using Raft or a simpler consensus algorithm if Dragonfly supports it for coordination).
        - [ ] Implement data replication strategies for the `Storage` layer (if Dragonfly's native replication is not sufficient or if moving to a different storage backend).
        - [ ] Implement graceful failover for client connections.
    - [ ] **Authentication & Authorization:**
        - [ ] Implement pluggable authentication mechanisms (e.g., username/password, API keys).
        - [ ] Implement basic authorization (ACLs) for topics/queues.
        - [ ] Integrate security into all protocol layers.
    - [ ] **Monitoring & Metrics:**
        - [ ] Instrument Portask with Prometheus metrics (message rates, latency, queue sizes, CPU/memory usage).
        - [ ] Expose a `/metrics` endpoint.
        - [ ] Create sample Grafana dashboards.
    - [ ] **Admin/CLI Interface:**
        - [ ] Develop a simple command-line interface for common administrative tasks (e.g., `portask create-topic`, `portask list-consumers`, `portask config-reload`).
        - [ ] Potentially a basic HTTP management API.
    - [ ] **Advanced Compression Strategies:**
        - [ ] Implement Zstd dictionary training for highly repetitive message payloads to achieve even better compression ratios.
        - [ ] Refine the dynamic compression adjustment logic based on real-world load tests.

### Phase 5: Testing, Benchmarking & Documentation (4 Weeks)

- **Objective:** Ensure Portask is robust, performs as expected, and is well-documented for users.
- **To-Do List:**
    - [ ] **Comprehensive Testing:**
        - [ ] Expand unit tests for all components.
        - [ ] Develop extensive integration tests covering cross-protocol message flow.
        - [ ] Create end-to-end tests simulating real-world scenarios.
        - [ ] Implement chaos engineering principles (e.g., simulating network partitions, Dragonfly failures).
    - [ ] **Benchmarking:**
        - [ ] Develop a dedicated benchmarking suite.
        - [ ] Conduct rigorous performance tests comparing Portask against Kafka and RabbitMQ under various loads (message size, throughput, number of producers/consumers).
        - [ ] Measure CPU, memory, network usage, and end-to-end latency.
        - [ ] Publish benchmark results.
    - [ ] **User Documentation:**
        - [ ] Installation guide.
        - [ ] Configuration guide (with examples for dynamic ratios).
        - [ ] Custom Protocol API reference.
        - [ ] Kafka/RabbitMQ emulation compatibility details and known limitations.
        - [ ] Client usage examples for various languages.
        - [ ] Troubleshooting guide.
        - [ ] Developer guide.

### Phase 6: Deployment & Operationalization (2 Weeks)

- **Objective:** Make Portask easily deployable and manageable in production environments.
- **To-Do List:**
    - [ ] **Containerization:**
        - [ ] Create optimized Dockerfiles for Portask.
        - [ ] Publish Docker images.
    - [ ] **Orchestration:**
        - [ ] Provide example Kubernetes manifests for deploying Portask clusters.
        - [ ] Document Helm charts (if applicable).
    - [ ] **Monitoring & Alerting:**
        - [ ] Provide Prometheus `scrape_configs` examples.
        - [ ] Suggest common alerts and thresholds.
    - [ ] **Release Strategy:**
        - [ ] Define versioning strategy (e.g., SemVer).
        - [ ] Automate release builds.

## Market Viability: Why Portask Might (or Might Not) Succeed

Even if Portask proves to be faster and cheaper than Kafka and RabbitMQ, there are significant reasons why it might struggle to gain widespread adoption:

1. **Maturity & Battle-Hardening:**
    - **Kafka and RabbitMQ** are incredibly mature systems, battle-tested over years in thousands of production environments. They have handled countless edge cases, failure scenarios, and performance challenges. Portask, being new, will lack this proven track record.
    - **Trust:** Enterprises are often risk-averse. Migrating core infrastructure to an unproven system, regardless of its performance claims, is a huge leap of faith.
2. **Ecosystem & Community:**
    - **Vast Ecosystems:** Kafka and RabbitMQ boast enormous ecosystems:
        - **Client Libraries:** Highly optimized, officially supported client libraries in virtually every programming language.
        - **Integrations:** Hundreds of connectors, plugins, and integrations with databases, data warehouses, stream processing frameworks (Spark, Flink), monitoring tools, etc.
        - **Tooling:** Rich tooling for monitoring, management, security, and schema evolution.
        - **Community Support:** Large, active communities, extensive documentation, forums, and commercial support options.
    - **Portask** would start with a minimal ecosystem. Building this out to match Kafka/RabbitMQ would be a monumental effort, requiring significant investment in client libraries, connectors, and partnerships.
3. **Features & Guarantees:**
    - **Feature Richness:** Kafka and RabbitMQ offer a wealth of advanced features (e.g., Kafka Streams, KSQL, exactly-once semantics, complex routing, dead-letter queues, message priorities, transactional messaging, advanced security features like Kerberos/TLS, federation). Portask would need to implement a substantial subset of these to be truly competitive beyond raw speed.
    - **Guarantees:** Providing strong guarantees (e.g., exactly-once delivery, message ordering, durability) in a distributed system is incredibly complex. Kafka and RabbitMQ have refined these over years. Portask would need to prove its reliability under extreme conditions.
4. **Operational Overhead (Perceived vs. Actual):**
    - While Portask aims for lower operational cost, the *perceived* operational overhead of a new system can be high due to lack of familiarity, existing playbooks, and readily available expertise.
    - Hiring talent for a niche technology can be harder and more expensive than for widely adopted ones.
5. **Vendor Lock-in & Open Source Dynamics:**
    - Both Kafka (Apache Kafka) and RabbitMQ (Mozilla Public License) are open-source projects with strong communities. While commercial offerings exist, the core technology is open. Portask would need to decide its licensing and community strategy.
    - Companies might prefer to stick with established open-source projects to avoid potential vendor lock-in, even if a proprietary solution offers performance gains.
6. **"Good Enough" Syndrome:**
    - For many use cases, Kafka and RabbitMQ are "good enough" or even overkill. The marginal performance gains of Portask might not justify the cost and risk of migration for many organizations.

**Conclusion on Market Viability:**

Portask's success would hinge on:

- **Exceptional Performance Differentiator:** The performance gains must be *so significant* that they outweigh the risks and costs of adopting a new system.
- **Targeted Niche:** Focusing on specific use cases where existing solutions genuinely struggle (e.g., extremely high-throughput, low-latency scenarios with very specific message patterns) could be a viable strategy.
- **Strong Ecosystem Development:** Rapidly building out client libraries, connectors, and tooling is crucial.
- **Robustness & Reliability:** Proving its reliability and data integrity under stress.
- **Community Building:** Fostering an active community around the project.

It's a challenging but potentially rewarding endeavor if executed meticulously and marketed strategically to the right audience.

## Cron Rule for Odd/Even Weeks

Yes, you can specify a cron rule that covers odd or even weeks of the year, but it's not directly supported by a single cron field. Standard cron syntax typically allows for minute, hour, day of month, month, and day of week.

To achieve odd/even week logic, you usually combine:

1. **Day of Week:** To run on specific days within the week (e.g., `MON` for Monday).
2. **Day of Month:** To narrow down the run days.
3. **Script Logic:** The most robust way is to add logic within the script executed by cron to check if the current week is odd or even.

**Example for a task to run every Monday on odd weeks:**

```javascript
0 0 * * MON /path/to/your/script.sh


```

And inside `/path/to/your/script.sh`:

```javascript
#!/bin/bash

# Get the current week number (e.g., from 1 to 53)
# Using %V for ISO 8601 week number (week starts on Monday)
# Or %U for week number of year (Sunday as first day of week 00-53)
# Or %W for week number of year (Monday as first day of week 00-53)
CURRENT_WEEK=$(date +%V) # Using ISO 8601 week number

# Check if the week number is odd
if (( CURRENT_WEEK % 2 != 0 )); then
    echo "Running task for odd week $CURRENT_WEEK"
    # Your actual command goes here
    /path/to/your/actual_task_command
else
    echo "Skipping task for even week $CURRENT_WEEK"
fi


```

**Explanation:**

- `0 0 * * MON`: This part schedules the script to run every Monday at midnight.
- `date +%V`: This command gets the current week number (ISO 8601 standard, where week 01 is the first week with at least 4 days in the new year, and Monday is the first day of the week).
- `(( CURRENT_WEEK % 2 != 0 ))`: This bash arithmetic expression checks if the `CURRENT_WEEK` number is odd. If it is, the task runs. If it's even, the script exits without running the main task.

You can adjust `date +%V` to `date +%U` or `date +%W` depending on how you define the start of your week and the first week of the year.

This approach ensures the cron job itself runs regularly, but the actual workload is conditionally executed based on the week number, providing the flexibility you need.