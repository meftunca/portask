# server/ (Production-ready Portask Sunucusu)

Bu dizin, Portask'ın tam özellikli, production-ready ana sunucu uygulamasını içerir. Tüm protokol, storage, API, monitoring, lifecycle ve production entegrasyonları burada başlatılır.

## Temel Dosyalar
- main.go: Sunucu uygulamasının giriş noktası, konfigürasyon, storage, queue, API, monitoring ve graceful shutdown yönetimi.

## Eksik Özellikler / TODO
- [ ] Gelişmiş protokol özellikleri (priority, batch, encryption, heartbeat)
- [ ] HA/replication desteği
- [ ] Production deployment otomasyonu
- [ ] Gelişmiş admin/CLI araçları
