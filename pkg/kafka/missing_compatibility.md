# Eksik Kafka Uyumluluğu ve Tamamlanmamış Özellikler (Portask)

## 1. Genel Durum
- `pkg/kafka/` klasöründe temel Kafka wire protocol desteği ve demo amaçlı handler'lar mevcut.
- Temel API'ler (Produce, Fetch, Metadata, Create/Delete Topics, SASL, ApiVersions) için handler fonksiyonları var.
- Gelişmiş özellikler için (Consumer Groups, Transactions, Schema Registry, Advanced Auth) sadece tipler ve demo fonksiyonlar mevcut, tam uyumlu bir Kafka broker davranışı yok.

## 2. Tamamlanmamış veya Eksik Özellikler

### server.go
- Sadece demo amaçlı, gerçekçi bir production Kafka broker davranışı yok.
- `KafkaServer` sadece bağlantı kabul ediyor ve handler'a yönlendiriyor.
- Gelişmiş hata yönetimi, bağlantı havuzu, TLS, gerçekçi loglama, konfigürasyon eksik.

### protocol.go
- **Consumer Group Management**: Sadece tipler ve demo fonksiyonlar var. Gerçekçi group join/sync/offset commit/heartbeat desteği yok.
- **Transaction Support**: Sadece tipler ve demo fonksiyonlar var. Gerçek transaction isolation, atomicity, recovery yok.
- **Schema Registry**: Sadece tipler ve demo fonksiyonlar var. Gerçek REST API, subject/version yönetimi, uyumluluk kontrolleri yok.
- **SASL Auth**: Demo amaçlı, gerçekçi bir kullanıcı/rol yönetimi ve mekanizma desteği yok.
- **API Handler'lar**: Sadece temel API'ler destekleniyor. Birçok Kafka API'si (DescribeGroups, OffsetCommit, OffsetFetch, FindCoordinator, vs.) eksik.
- **Error Handling**: Birçok handler'da hata kodu dönülmüyor veya sadece sabit değerler dönülüyor.
- **Persistence**: Mesajlar ve metadata için kalıcı bir storage backend entegrasyonu yok (sadece interface var).
- **Replication/Partitioning**: Gerçekçi partition/replica yönetimi ve broker discovery yok.
- **Wire Protocol**: Sadece temel request/response akışı var, advanced framing, compression, protocol versioning eksik.

### handlers.go
- Sadece temel handler fonksiyonları var. Birçok Kafka API'si için handler eksik.
- Gelişmiş hata yönetimi, throttle, response header'ları, protocol compliance eksik.

## 3. Kodda Belirgin Demo/Stub Noktaları
- `return nil` ile biten veya sadece log atan fonksiyonlar (ör: SimpleMetricsCollector, bazı handler'lar)
- Demo amaçlı "always succeed" veya "always allow" yetkilendirme
- Gerçekçi bir test coverage ve production-grade concurrency yok

## 4. Eksik Testler
- Gelişmiş protokol uyumluluğu için integration testleri eksik
- Consumer group, transaction, schema registry, advanced auth için test yok

---

> Bu dosya, Portask'ın Kafka uyumluluğu ve eksik özelliklerini özetler. Production-grade Kafka broker uyumluluğu için eksik noktalar tamamlanmalıdır.
