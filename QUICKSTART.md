# 🚀 Portask - Hızlı Başlangıç Kılavuzu

Bu kılavuz, Portask'ı 5 dakikada çalıştırmanıza yardımcı olacak!

## 📋 Gereksinimler

- Docker & Docker Compose
- Go 1.23+ (kaynak koddan build için)
- 4GB+ RAM

## ⚡ Hızlı Başlangıç (Docker ile)

### 1. Repository'yi Klonlayın

```bash
git clone https://github.com/meftunca/portask.git
cd portask
```

### 2. Full Stack'i Başlatın

```bash
docker-compose up -d
```

Bu komut şunları başlatır:

- 🐉 Dragonfly (Redis-compatible storage)
- 🚀 Portask Server (tüm protokoller)
- 📊 Prometheus (metrics)
- 📈 Grafana (dashboards)

### 3. Kontrol Edin

```bash
# Health check
curl http://localhost:8080/health

# Stats
curl http://localhost:8080/api/v1/stats
```

### 4. İlk Mesajınızı Gönderin

```bash
# HTTP API ile
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-001",
    "topic": "hello",
    "priority": "high",
    "payload": "Hello, Portask!"
  }'
```

### 5. Monitoring'i Açın

- **Grafana:** http://localhost:3000 (admin/admin)
- **Prometheus:** http://localhost:9091

🎉 **Tebrikler!** Portask çalışıyor!

---

## 🛠️ Manuel Kurulum (Kaynak Koddan)

### 1. Dependencies

```bash
make deps
```

### 2. Dragonfly Başlatın (Storage Backend)

```bash
# Docker ile
docker run -d -p 6379:6379 docker.dragonflydb.io/dragonflydb/dragonfly

# Veya Redis
docker run -d -p 6379:6379 redis:7-alpine
```

### 3. Build & Run

```bash
# Build
make build

# Run
./build/portask
```

Alternatif:

```bash
# Doğrudan çalıştır
make run
```

---

## 📝 Temel Kullanım

### HTTP API ile Mesaj Gönderme

```bash
# Mesaj gönder
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "orders",
    "priority": "high",
    "payload": "{\"order_id\": 123, \"amount\": 99.99}"
  }'
```

### RabbitMQ Client ile

```go
package main

import (
    "log"
    "github.com/streadway/amqp"
)

func main() {
    conn, err := amqp.Dial("amqp://localhost:5672/")
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    ch, err := conn.Channel()
    if err != nil {
        log.Fatal(err)
    }
    defer ch.Close()

    err = ch.Publish(
        "",      // exchange
        "hello", // routing key
        false,   // mandatory
        false,   // immediate
        amqp.Publishing{
            ContentType: "text/plain",
            Body:        []byte("Hello from RabbitMQ client!"),
        })

    log.Println("Message sent!")
}
```

### Kafka Client ile

```go
package main

import (
    "context"
    "log"
    "github.com/segmentio/kafka-go"
)

func main() {
    writer := kafka.NewWriter(kafka.WriterConfig{
        Brokers: []string{"localhost:9092"},
        Topic:   "events",
    })
    defer writer.Close()

    err := writer.WriteMessages(context.Background(),
        kafka.Message{
            Key:   []byte("key-1"),
            Value: []byte("Hello from Kafka client!"),
        },
    )

    if err != nil {
        log.Fatal(err)
    }

    log.Println("Message sent!")
}
```

### Portask Native Client

```go
package main

import (
    "context"
    "log"

    "github.com/meftunca/portask/pkg/client"
    "github.com/meftunca/portask/pkg/types"
)

func main() {
    // Connect
    c, err := client.NewPortaskClient("localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Publish
    msg := &types.PortaskMessage{
        ID:       "msg-001",
        Topic:    "events",
        Priority: types.PriorityHigh,
        Payload:  []byte("Hello from native client!"),
    }

    err = c.Publish(context.Background(), msg)
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Message published!")
}
```

---

## 🔧 Konfigürasyon

### Temel Config (configs/config.yaml)

```yaml
# Performans ayarları
performance:
  worker_pool_size: 32 # Worker sayısı
  batch_size: 1000 # Batch boyutu
  batch_processing: true # Batch mode

# Storage
storage:
  type: "dragonfly" # dragonfly, redis, memory
  dragonfly:
    addresses:
      - "localhost:6379"
    password: ""
    db: 0

# Network portları
network:
  custom_port: 8080 # HTTP API
  kafka_port: 9092 # Kafka protocol
  rabbitmq_port: 5672 # AMQP protocol
  admin_port: 8081 # Admin UI

# Sıkıştırma
compression:
  type: "zstd" # none, zstd, lz4, snappy
  strategy: "adaptive" # always, threshold, adaptive
  level: 3 # 1-22 arası

# Serialization
serialization:
  type: "cbor" # cbor, json, msgpack
```

### Environment Variables

```bash
# Storage override
export PORTASK_STORAGE_TYPE=dragonfly
export PORTASK_STORAGE_DRAGONFLY_ADDRESSES=localhost:6379

# Performance
export PORTASK_PERFORMANCE_WORKER_POOL_SIZE=64
export PORTASK_PERFORMANCE_BATCH_SIZE=2000

# Ports
export PORTASK_NETWORK_CUSTOM_PORT=8080
```

---

## 📊 Monitoring

### Metrics Endpoint

```bash
# Prometheus metrics
curl http://localhost:9090/metrics

# JSON stats
curl http://localhost:8080/api/v1/stats | jq
```

### Grafana Dashboard

1. http://localhost:3000 adresine gidin
2. Username: `admin`, Password: `admin`
3. "Portask Performance Dashboard" açın
4. Real-time metrics görün! 📈

---

## 🧪 Test & Benchmark

### Test Çalıştırma

```bash
# Tüm testler
make test

# Coverage report
make test-coverage

# Sadece specific package
go test -v ./pkg/queue/...
```

### Benchmark

```bash
# Tüm benchmark'lar
make benchmark

# Sadece queue benchmark
go test -bench=. -benchmem ./pkg/queue/

# CPU profiling ile
go test -bench=. -cpuprofile=cpu.prof ./pkg/queue/
go tool pprof cpu.prof
```

### Load Test

```bash
# Ultra benchmark (dikkatli kullanın!)
go run cmd/ultra-benchmark/main.go

# Veya hazır benchmark
go test -bench=BenchmarkUltra -timeout=30m ./benchmarks/
```

---

## 🐛 Sorun Giderme

### Portask Başlamıyor

```bash
# Logları kontrol et
docker-compose logs portask

# Portların boş olduğundan emin ol
lsof -i :8080
lsof -i :9092
lsof -i :5672
```

### Dragonfly Bağlantı Hatası

```bash
# Dragonfly çalışıyor mu?
docker-compose ps dragonfly

# Bağlantı test et
redis-cli -h localhost -p 6379 ping

# Restart
docker-compose restart dragonfly
```

### Yüksek Memory Kullanımı

```yaml
# config.yaml'de ayarla
performance:
  worker_pool_size: 16 # Azalt
  batch_size: 500 # Azalt
  memory_pool_enabled: true # Aktif et
```

### Düşük Throughput

```yaml
# config.yaml'de ayarla
performance:
  worker_pool_size: 64 # Artır
  batch_size: 2000 # Artır
  batch_processing: true # Aktif et

compression:
  strategy: "never" # Test için kapat
```

---

## 📚 Daha Fazla Bilgi

- 📖 [Full Documentation](docs/README.md)
- 🏗️ [Architecture Guide](docs/architecture.md)
- ⚡ [Performance Guide](docs/performance.md)
- 🔌 [API Reference](docs/api_reference.md)
- 🐰 [AMQP Emulator](docs/amqp_emulator.md)
- 🔗 [Kafka Emulator](docs/kafka_emulator.md)

---

## 💬 Yardım & Destek

- 🐛 [GitHub Issues](https://github.com/meftunca/portask/issues)
- 💬 [Discussions](https://github.com/meftunca/portask/discussions)
- 📧 Email: support@portask.dev

---

## 🎯 Sonraki Adımlar

1. ✅ Portask'ı başlattınız
2. ✅ İlk mesajınızı gönderdiniz
3. ✅ Monitoring'i kurdunuz

Şimdi şunları deneyin:

- 🔥 Load test yapın
- 📊 Grafana dashboard'ları keşfedin
- 🔧 Configuration'ı optimize edin
- 🚀 Production deployment planlayın

**Başarılar!** 🎉
