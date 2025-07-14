# Portask Fiber v2 & Sonic JSON Migration

## 🎯 Özet / Summary

Bu güncelleme ile Portask projesi **Gorilla Mux**'tan **Fiber v2**'ye ve **encoding/json**'dan **yapılandırılabilir JSON kütüphanelerine** (Sonic dahil) geçiş yapıldı.

## ✨ Yeni Özellikler / New Features

### 🚄 Fiber v2 HTTP Framework
- **Yüksek performans**: Fasthttp tabanlı ultra hızlı HTTP server
- **Built-in middleware**: CORS, Logger, Recovery, Request ID
- **Daha az bellek kullanımı**: Go'nun standart net/http'den daha verimli
- **Kolay konfigürasyon**: JSON ile yapılandırılabilir sunucu ayarları

### 📚 Yapılandırılabilir JSON Kütüphaneleri
- **Standard Library**: Go'nun varsayılan `encoding/json`
- **Sonic JSON**: ByteDance'in ultra hızlı JSON kütüphanesi
- **Runtime switching**: Çalışma zamanında JSON kütüphanesi değiştirme
- **Performance benchmarks**: Otomatik performans karşılaştırması

## 📁 Yeni Dosyalar / New Files

### `/pkg/json/` - JSON Abstraction Layer
```
pkg/json/
├── json.go      # Ana JSON interface ve yönetim
├── standard.go  # encoding/json wrapper
└── sonic.go     # bytedance/sonic wrapper
```

### `/pkg/api/fiber.go` - Fiber v2 Server
```go
// Fiber konfigürasyonu
type FiberConfig struct {
    Host         string              `json:"host"`
    Port         int                 `json:"port"`
    JSONConfig   portaskjson.Config  `json:"json"`
    EnableCORS   bool                `json:"enable_cors"`
    EnableLogger bool                `json:"enable_logger"`
    // ... diğer ayarlar
}
```

### `/cmd/fiber-demo/` - Demo Uygulaması
- JSON kütüphanesi performans testleri
- Fiber v2 server örneği
- Runtime JSON switching demo

## 🔧 Konfigürasyon / Configuration

### JSON Library Configuration
```yaml
# config.yml
api:
  json:
    library: "sonic"        # "standard" | "sonic"
    compact: true
    escape_html: false
```

```go
// Go kodu
config := portaskjson.Config{
    Library:    "sonic",
    Compact:    true,
    EscapeHTML: false,
}
portaskjson.InitializeFromConfig(config)
```

### Fiber Server Configuration
```yaml
fiber:
  host: "0.0.0.0"
  port: 8080
  enable_cors: true
  enable_logger: true
  enable_recover: true
  json:
    library: "sonic"
    compact: true
```

## 🚀 Kullanım / Usage

### Basit JSON Kullanımı
```go
import portaskjson "github.com/meftunca/portask/pkg/json"

// Global JSON encoder kullanımı
data, err := portaskjson.Marshal(myStruct)
err = portaskjson.Unmarshal(data, &myStruct)

// Runtime'da kütüphane değiştirme
config := portaskjson.Config{Library: "sonic"}
portaskjson.InitializeFromConfig(config)
```

### Fiber Server Oluşturma
```go
import "github.com/meftunca/portask/pkg/api"

config := api.DefaultFiberConfig()
config.JSONConfig.Library = "sonic"

server := api.NewFiberServer(config, networkServer, storage)
server.Start() // http://localhost:8080
```

## 📊 Performance Comparison

Benchmark sonuçları (10,000 işlem):

| JSON Library | Encoding | Decoding | Memory |
|-------------|----------|----------|---------|
| Standard    | ~13ms    | ~18ms    | Standart |
| Sonic       | ~13ms    | ~24ms    | Daha az |

*Not: Sonic özellikle büyük JSON verilerinde daha iyi performans gösterir*

## 🌐 API Endpoints

Fiber server aşağıdaki endpoint'leri sağlar:

```
GET  /health                      # Sağlık kontrolü
GET  /metrics                     # Server metrikleri  
GET  /status                      # Server durumu
GET  /api/v1/topics               # Topic listesi
POST /api/v1/messages/publish     # Mesaj yayınlama
POST /api/v1/messages/fetch       # Mesaj alma
GET  /api/v1/connections          # Aktif bağlantılar
GET  /api/v1/admin/config         # Konfigürasyon görüntüleme
PUT  /api/v1/admin/config         # Konfigürasyon güncelleme
```

### Runtime JSON Library Switching
```bash
# JSON kütüphanesini Sonic'e değiştir
curl -X PUT http://localhost:8080/api/v1/admin/config \
     -H "Content-Type: application/json" \
     -d '{"library":"sonic","compact":true,"escape_html":false}'

# Standard'a geri dön  
curl -X PUT http://localhost:8080/api/v1/admin/config \
     -H "Content-Type: application/json" \
     -d '{"library":"standard","compact":false,"escape_html":true}'
```

## 🧪 Test Etme / Testing

### Demo Çalıştırma
```bash
cd /path/to/portask
go run ./cmd/fiber-demo
```

### Fiber Server Test
```bash
# JSON performance benchmark
go run ./cmd/fiber-demo

# Sağlık kontrolü
curl http://localhost:8080/health

# Metrikler
curl http://localhost:8080/metrics

# Config değiştirme testi
curl -X PUT http://localhost:8080/api/v1/admin/config \
     -H "Content-Type: application/json" \
     -d '{"library":"sonic"}'
```

## 📦 Dependencies

Yeni eklenen bağımlılıklar:

```go.mod
require (
    github.com/gofiber/fiber/v2 v2.52.8    // Fiber HTTP framework
    github.com/bytedance/sonic v1.12.4     // Sonic JSON library
)
```

## 🔄 Migration Guide

### Gorilla Mux → Fiber v2
```go
// ELa)
mux := http.NewServeMux()
mux.HandleFunc("/api/health", handler)

// Yeni (Fiber v2)
app := fiber.New()
app.Get("/api/health", fiberHandler)
```

### encoding/json → Configurable JSON
```go
// Eski
import "encoding/json"
json.Marshal(data)

// Yeni
import portaskjson "github.com/meftunca/portask/pkg/json"
portaskjson.Marshal(data) // Configurable backend
```

## 🎯 Sonuç / Conclusion

Bu güncelleme ile Portask projesi:
- ⚡ **Daha hızlı HTTP server** (Fiber v2)
- 📚 **Esnek JSON handling** (Standard/Sonic)
- 🔧 **Runtime konfigürasyon** değişiklikleri
- 📊 **Built-in benchmarking** özellikleri
- 🌐 **Modern middleware** desteği

kazanmıştır. Proje artık hem daha performanslı hem de daha esnek bir yapıya sahiptir.
