// jcron_test.go// core_test.go (Düzeltilmiş Hali)
package jcron

import (
	"fmt"
	"math/bits"
	"strings"
	"testing"
	"time"
)

func mustParseTime(layout, value string) time.Time {
	t, err := time.Parse(layout, value)
	if err != nil {
		panic(err)
	}
	return t
}

// DÜZELTİLDİ: Artık []int yerine uint64 bitmask'larını test ediyor.
func TestExpandPart(t *testing.T) {
	testCases := []struct {
		name     string
		expr     string
		min, max int
		expected uint64
		hasError bool
	}{
		{"Yıldız", "*", 0, 5, (1 << 0) | (1 << 1) | (1 << 2) | (1 << 3) | (1 << 4) | (1 << 5), false},
		{"Tek Değer", "3", 0, 5, (1 << 3), false},
		{"Haftanın Günü (Metin)", "MON-FRI", 0, 6, (1 << 1) | (1 << 2) | (1 << 3) | (1 << 4) | (1 << 5), false},
		{"Geçersiz İfade", "a", 0, 5, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := expandPart(tc.expr, tc.min, tc.max)
			if (err != nil) != tc.hasError {
				t.Fatalf("expandPart() error = %v, wantErr %v", err, tc.hasError)
			}
			if result != tc.expected {
				t.Errorf("expandPart() = %d (%b), want %d (%b)", result, result, tc.expected, tc.expected)
			}
		})
	}
}

func TestEngine_Next(t *testing.T) {
	engine := New()
	testCases := []struct {
		name         string
		schedule     Schedule
		fromTime     time.Time
		expectedTime time.Time
	}{
		// --- Temel Testler ---
		{"1. Basit Sonraki Dakika", strPtrSchedule("0", "*", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-10-26T10:00:30Z"), mustParseTime(time.RFC3339, "2025-10-26T10:01:00Z")},
		{"2. Sonraki Saatin Başı", strPtrSchedule("0", "0", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-10-26T10:59:00Z"), mustParseTime(time.RFC3339, "2025-10-26T11:00:00Z")},
		{"3. Sonraki Günün Başı", strPtrSchedule("0", "0", "0", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-10-26T23:59:00Z"), mustParseTime(time.RFC3339, "2025-10-27T00:00:00Z")},
		{"4. Sonraki Ayın Başı", strPtrSchedule("0", "0", "0", "1", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-02-15T12:00:00Z"), mustParseTime(time.RFC3339, "2025-03-01T00:00:00Z")},
		{"5. Sonraki Yılın Başı", strPtrSchedule("0", "0", "0", "1", "1", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-06-15T12:00:00Z"), mustParseTime(time.RFC3339, "2026-01-01T00:00:00Z")},

		// --- Aralık, Adım ve Liste Testleri ---
		{"6. İş Saatleri İçinde Sonraki Saat", strPtrSchedule("0", "0", "9-17", "*", "*", "MON-FRI", "*", "UTC"), mustParseTime(time.RFC3339, "2025-03-03T10:30:00Z"), mustParseTime(time.RFC3339, "2025-03-03T11:00:00Z")},       // Pzt 10:30 -> Pzt 11:00
		{"7. İş Saati Sonundan Sonraki Güne Atlama", strPtrSchedule("0", "0", "9-17", "*", "*", "MON-FRI", "*", "UTC"), mustParseTime(time.RFC3339, "2025-03-03T17:30:00Z"), mustParseTime(time.RFC3339, "2025-03-04T09:00:00Z")}, // Pzt 17:30 -> Salı 09:00
		{"8. Hafta Sonuna Atlama (Cuma -> Pazartesi)", strPtrSchedule("0", "0", "9-17", "*", "*", "1-5", "*", "UTC"), mustParseTime(time.RFC3339, "2025-03-07T18:00:00Z"), mustParseTime(time.RFC3339, "2025-03-10T09:00:00Z")},   // Cuma 18:00 -> Pzt 09:00
		{"9. Her 15 Dakikada Bir", strPtrSchedule("0", "*/15", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-05-10T14:16:00Z"), mustParseTime(time.RFC3339, "2025-05-10T14:30:00Z")},
		{"10. Belirli Aylarda Çalışma", strPtrSchedule("0", "0", "0", "1", "3,6,9,12", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-03-15T10:00:00Z"), mustParseTime(time.RFC3339, "2025-06-01T00:00:00Z")}, // Mart -> Haziran

		// --- Özel Karakterler (L, #) Testleri ---
		{"11. Ayın Son Günü (L)", strPtrSchedule("0", "0", "12", "L", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2024-02-10T00:00:00Z"), mustParseTime(time.RFC3339, "2024-02-29T12:00:00Z")}, // Artık yıl (Leap year)
		{"12. Ayın Son Günü (L) - Sonraki Ay", strPtrSchedule("0", "0", "12", "L", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-04-30T13:00:00Z"), mustParseTime(time.RFC3339, "2025-05-31T12:00:00Z")},
		{"13. Ayın Son Cuması (5L)", strPtrSchedule("0", "0", "22", "*", "*", "5L", "*", "UTC"), mustParseTime(time.RFC3339, "2025-08-01T00:00:00Z"), mustParseTime(time.RFC3339, "2025-08-29T22:00:00Z")},
		{"14. Ayın İkinci Salısı (2#2)", strPtrSchedule("0", "0", "8", "*", "*", "2#2", "*", "UTC"), mustParseTime(time.RFC3339, "2025-11-01T00:00:00Z"), mustParseTime(time.RFC3339, "2025-11-11T08:00:00Z")},
		{"15. Vixie-Cron (OR Mantığı)", strPtrSchedule("0", "0", "0", "15", "*", "MON", "*", "UTC"), mustParseTime(time.RFC3339, "2025-09-09T00:00:00Z"), mustParseTime(time.RFC3339, "2025-09-15T00:00:00Z")}, // Ayın 15'i (Pzt) daha erken

		// --- Kısaltmalar ve Zaman Dilimi Testleri ---
		{"16. Kısaltma (@weekly)", strPtrSchedule("0", "0", "0", "*", "*", "0", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T12:00:00Z"), mustParseTime(time.RFC3339, "2025-01-05T00:00:00Z")},                             // Çrş -> Pzr
		{"17. Kısaltma (@hourly) - DÜZELTİLDİ", strPtrSchedule("0", "0", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T15:00:00Z"), mustParseTime(time.RFC3339, "2025-01-01T16:00:00Z")},                // Her saatin başında
		{"18. Zaman Dilimi (Istanbul)", strPtrSchedule("0", "30", "9", "*", "*", "*", "*", "Europe/Istanbul"), mustParseTime(time.RFC3339, "2025-10-26T03:00:00+03:00"), mustParseTime(time.RFC3339, "2025-10-26T09:30:00+03:00")}, // Yaz saati bitişine yakın
		{"19. Zaman Dilimi (New York)", strPtrSchedule("0", "0", "20", "4", "7", "*", "*", "America/New_York"), mustParseTime(time.RFC3339, "2025-07-01T00:00:00Z"), mustParseTime(time.RFC3339, "2025-07-04T20:00:00-04:00")},
		{"20. Yıl Belirtme", strPtrSchedule("0", "0", "0", "1", "1", "*", "2027", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T00:00:00Z"), mustParseTime(time.RFC3339, "2027-01-01T00:00:00Z")},
		{"21. Her Saniyenin Sonu", strPtrSchedule("59", "59", "23", "31", "12", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-12-31T23:59:58Z"), mustParseTime(time.RFC3339, "2025-12-31T23:59:59Z")},

		// --- YENİ: Ek Kapsamlı Testler ---
		{"22. Her 5 Saniyede", strPtrSchedule("*/5", "*", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T12:00:03Z"), mustParseTime(time.RFC3339, "2025-01-01T12:00:05Z")},
		{"23. Belirli Saniyeler", strPtrSchedule("15,30,45", "*", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T12:00:20Z"), mustParseTime(time.RFC3339, "2025-01-01T12:00:30Z")},
		{"24. Hafta İçi Öğle", strPtrSchedule("0", "0", "12", "*", "*", "1-5", "*", "UTC"), mustParseTime(time.RFC3339, "2025-08-08T14:00:00Z"), mustParseTime(time.RFC3339, "2025-08-11T12:00:00Z")}, // Cuma -> Pazartesi
		{"25. Ay Sonu ve Başı", strPtrSchedule("0", "0", "0", "1,L", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-15T12:00:00Z"), mustParseTime(time.RFC3339, "2025-01-31T00:00:00Z")}, // Ocak 15 -> Ocak 31 (L)
		{"26. Çeyrek Saatler", strPtrSchedule("0", "0,15,30,45", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T10:20:00Z"), mustParseTime(time.RFC3339, "2025-01-01T10:30:00Z")},
		{"27. Artık Yıl Şubat 29", strPtrSchedule("0", "0", "12", "29", "2", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2023-12-01T00:00:00Z"), mustParseTime(time.RFC3339, "2024-02-29T12:00:00Z")},                       // 2024 artık yıl
		{"28. Ayın 1. ve 3. Pazartesi", strPtrSchedule("0", "0", "9", "*", "*", "1#1,1#3", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-07T10:00:00Z"), mustParseTime(time.RFC3339, "2025-01-20T09:00:00Z")},              // 1. Pzt sonrası 3. Pzt
		{"29. Zaman Dilimi Farkı", strPtrSchedule("0", "0", "12", "*", "*", "*", "*", "America/New_York"), mustParseTime(time.RFC3339, "2025-01-01T10:00:00-05:00"), mustParseTime(time.RFC3339, "2025-01-01T12:00:00-05:00")}, // Basit zaman dilimi testi
		{"30. Yılın Son Günü", strPtrSchedule("59", "59", "23", "31", "12", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-12-30T12:00:00Z"), mustParseTime(time.RFC3339, "2025-12-31T23:59:59Z")},

		// --- İLERİ DÜZEY TEST SENARYOLARI ---
		{"31. Karma Özel Karakterler", strPtrSchedule("0", "0", "12", "L", "*", "5L", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T00:00:00Z"), mustParseTime(time.RFC3339, "2025-01-31T12:00:00Z")},                      // Son gün VE son Cuma
		{"32. Çoklu # Patterns", strPtrSchedule("0", "0", "14", "*", "*", "1#2,3#3,5#4", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T00:00:00Z"), mustParseTime(time.RFC3339, "2025-01-13T14:00:00Z")},                   // 2. Pzt (13), 3. Çrş (15), 4. Cuma (24) - en yakın 13 Ocak
		{"33. Saniye Seviyesi Adım", strPtrSchedule("*/10", "*/5", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T12:05:25Z"), mustParseTime(time.RFC3339, "2025-01-01T12:05:30Z")},                     // Her 10 saniye, her 5 dakika
		{"34. Gece Yarısı Geçişi", strPtrSchedule("30", "59", "23", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-12-31T23:59:25Z"), mustParseTime(time.RFC3339, "2025-12-31T23:59:30Z")},                         // Yıl sonu
		{"35. Şubat 29 (Normal Yıl)", strPtrSchedule("0", "0", "12", "29", "2", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T00:00:00Z"), mustParseTime(time.RFC3339, "2028-02-29T12:00:00Z")},                       // Sonraki artık yıl
		{"36. Çoklu Zaman Dilimleri", strPtrSchedule("0", "0", "12", "*", "*", "*", "*", "Pacific/Auckland"), mustParseTime(time.RFC3339, "2025-01-01T23:00:00+13:00"), mustParseTime(time.RFC3339, "2025-01-02T12:00:00+13:00")}, // Gelecek gün
		{"37. Hafta İçi + Belirli Gün Kombinasyonu", strPtrSchedule("0", "0", "9", "15", "*", "1-5", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-10T00:00:00Z"), mustParseTime(time.RFC3339, "2025-01-10T09:00:00Z")},       // 15. gün VEYA hafta içi - 10 Ocak Cuma geçerli
		{"38. Maksimum Değerler", strPtrSchedule("59", "59", "23", "31", "12", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-12-31T23:59:58Z"), mustParseTime(time.RFC3339, "2025-12-31T23:59:59Z")},                        // Son saniye
		{"39. Minimum Değerler", strPtrSchedule("0", "0", "0", "1", "1", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2024-12-31T23:59:59Z"), mustParseTime(time.RFC3339, "2025-01-01T00:00:00Z")},                              // Yıl başı
		{"40. Karma Liste ve Aralık", strPtrSchedule("0", "0,30", "8-12,14-18", "1,15", "1,6,12", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T08:15:00Z"), mustParseTime(time.RFC3339, "2025-01-01T08:30:00Z")},     // Karma pattern
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nextTime, err := engine.Next(tc.schedule, tc.fromTime)
			if err != nil {
				t.Fatalf("Beklenmeyen hata: %v", err)
			}
			if !nextTime.Equal(tc.expectedTime) {
				t.Errorf("Hatalı sonuç!\nBeklenen: %s\nAlınan  : %s",
					tc.expectedTime.Format(time.RFC3339Nano),
					nextTime.Format(time.RFC3339Nano))
			}
		})
	}
}

// --- YENİ: PREV Fonksiyonu Testleri ---
func TestEngine_Prev(t *testing.T) {
	engine := New()
	testCases := []struct {
		name         string
		schedule     Schedule
		fromTime     time.Time
		expectedTime time.Time
	}{
		// --- Temel ve "Taşma" (Rollover) Testleri ---
		{"1. Basit Önceki Dakika", strPtrSchedule("0", "*", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-10-26T10:00:30Z"), mustParseTime(time.RFC3339, "2025-10-26T10:00:00Z")},
		{"2. Önceki Saatin Başı", strPtrSchedule("0", "0", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-10-26T11:00:00Z"), mustParseTime(time.RFC3339, "2025-10-26T10:00:00Z")},
		{"3. Önceki Günün Başı", strPtrSchedule("0", "0", "0", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-10-27T00:00:00Z"), mustParseTime(time.RFC3339, "2025-10-26T00:00:00Z")},
		{"4. Önceki Ayın Başı", strPtrSchedule("0", "0", "0", "1", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-03-15T12:00:00Z"), mustParseTime(time.RFC3339, "2025-03-01T00:00:00Z")},
		{"5. Önceki Yılın Başı", strPtrSchedule("0", "0", "0", "1", "1", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2026-06-15T12:00:00Z"), mustParseTime(time.RFC3339, "2026-01-01T00:00:00Z")},

		// --- Aralık, Adım ve Liste Testleri ---
		{"6. İş Saatleri İçinde Önceki Saat", strPtrSchedule("0", "0", "9-17", "*", "*", "MON-FRI", "*", "UTC"), mustParseTime(time.RFC3339, "2025-03-03T10:30:00Z"), mustParseTime(time.RFC3339, "2025-03-03T10:00:00Z")},       // Pzt 10:30 -> Pzt 10:00
		{"7. İş Saati Başından Önceki Güne Atlama", strPtrSchedule("0", "0", "9-17", "*", "*", "MON-FRI", "*", "UTC"), mustParseTime(time.RFC3339, "2025-03-04T09:00:00Z"), mustParseTime(time.RFC3339, "2025-03-03T17:00:00Z")}, // Salı 09:00 -> Pzt 17:00
		{"8. Hafta Başına Atlama (Pazartesi -> Cuma)", strPtrSchedule("0", "0", "9-17", "*", "*", "1-5", "*", "UTC"), mustParseTime(time.RFC3339, "2025-03-10T08:00:00Z"), mustParseTime(time.RFC3339, "2025-03-07T17:00:00Z")},  // Pzt 08:00 -> Cuma 17:00
		{"9. Her 15 Dakikada Bir", strPtrSchedule("0", "*/15", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-05-10T14:31:00Z"), mustParseTime(time.RFC3339, "2025-05-10T14:30:00Z")},
		{"10. Belirli Aylarda Çalışma", strPtrSchedule("0", "0", "0", "1", "3,6,9,12", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-05-15T10:00:00Z"), mustParseTime(time.RFC3339, "2025-03-01T00:00:00Z")}, // Mayıs -> Mart

		// --- Özel Karakterler (L, #) Testleri ---
		{"11. Ayın Son Günü (L)", strPtrSchedule("0", "0", "12", "L", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2024-03-10T00:00:00Z"), mustParseTime(time.RFC3339, "2024-02-29T12:00:00Z")}, // Artık yıl (Leap year)
		{"12. Ayın Son Günü (L) - Ay İçinde", strPtrSchedule("0", "0", "12", "L", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-05-31T11:00:00Z"), mustParseTime(time.RFC3339, "2025-04-30T12:00:00Z")},
		{"13. Ayın Son Cuması (5L)", strPtrSchedule("0", "0", "22", "*", "*", "5L", "*", "UTC"), mustParseTime(time.RFC3339, "2025-09-01T00:00:00Z"), mustParseTime(time.RFC3339, "2025-08-29T22:00:00Z")},
		{"14. Ayın İkinci Salısı (2#2)", strPtrSchedule("0", "0", "8", "*", "*", "2#2", "*", "UTC"), mustParseTime(time.RFC3339, "2025-11-20T00:00:00Z"), mustParseTime(time.RFC3339, "2025-11-11T08:00:00Z")},
		{"15. Vixie-Cron (OR Mantığı)", strPtrSchedule("0", "0", "0", "15", "*", "MON", "*", "UTC"), mustParseTime(time.RFC3339, "2025-09-17T00:00:00Z"), mustParseTime(time.RFC3339, "2025-09-15T00:00:00Z")}, // Ayın 15'i (Pzt) daha yakın

		// --- Kısaltmalar ve Zaman Dilimi Testleri ---
		{"16. Kısaltma (@weekly)", strPtrSchedule("0", "0", "0", "*", "*", "0", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-08T12:00:00Z"), mustParseTime(time.RFC3339, "2025-01-05T00:00:00Z")}, // Çrş -> Pzr
		{"17. Kısaltma (@hourly)", strPtrSchedule("0", "0", "*", "*", "*", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T15:00:00Z"), mustParseTime(time.RFC3339, "2025-01-01T14:00:00Z")},
		{"18. Zaman Dilimi (Istanbul)", strPtrSchedule("0", "30", "9", "*", "*", "*", "*", "Europe/Istanbul"), mustParseTime(time.RFC3339, "2025-10-26T09:30:00+03:00"), mustParseTime(time.RFC3339, "2025-10-25T09:30:00+03:00")},
		{"19. Zaman Dilimi (New York)", strPtrSchedule("0", "0", "20", "4", "7", "*", "*", "America/New_York"), mustParseTime(time.RFC3339, "2025-07-10T00:00:00Z"), mustParseTime(time.RFC3339, "2025-07-04T20:00:00-04:00")},
		{"20. Yıl Belirtme", strPtrSchedule("0", "0", "0", "1", "1", "*", "2025", "UTC"), mustParseTime(time.RFC3339, "2027-01-01T00:00:00Z"), mustParseTime(time.RFC3339, "2025-01-01T00:00:00Z")},
		{"21. Her Saniyenin Başlangıcı", strPtrSchedule("0", "0", "0", "1", "1", "*", "*", "UTC"), mustParseTime(time.RFC3339, "2025-01-01T00:00:01Z"), mustParseTime(time.RFC3339, "2025-01-01T00:00:00Z")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prevTime, err := engine.Prev(tc.schedule, tc.fromTime)
			if err != nil {
				t.Fatalf("Beklenmeyen hata: %v", err)
			}
			if !prevTime.Equal(tc.expectedTime) {
				t.Errorf("Hatalı sonuç!\nBeklenen: %s\nAlınan  : %s",
					tc.expectedTime.Format(time.RFC3339Nano),
					prevTime.Format(time.RFC3339Nano))
			}
		})
	}
}

// strPtrSchedule, test senaryolarını yazmayı kolaylaştıran bir yardımcı fonksiyondur.
func strPtrSchedule(s, m, h, D, M, dow, Y, tz string) Schedule {
	return Schedule{
		Second: &s, Minute: &m, Hour: &h,
		DayOfMonth: &D, Month: &M, DayOfWeek: &dow,
		Year: &Y, Timezone: &tz,
	}
}

// Benchmark testlerinde kullanılacak sabitler
var (
	benchEngine = New()
	benchTime   = mustParseTime(time.RFC3339, "2025-01-01T12:00:00Z")
	schedulesGo = map[string]Schedule{
		"simple":       strPtrSchedule("0", "0", "*", "*", "*", "*", "*", "UTC"),                     // Every hour
		"complex":      strPtrSchedule("0", "5,15,25,35,45,55", "8-17", "*", "*", "1-5", "*", "UTC"), // Business hours
		"specialChars": strPtrSchedule("0", "0", "12", "L", "*", "5#3", "*", "UTC"),                  // Special patterns
		"timezone":     strPtrSchedule("0", "30", "9", "*", "*", "1-5", "*", "America/New_York"),     // Timezone
		"frequent":     strPtrSchedule("*/5", "*", "*", "*", "*", "*", "*", "UTC"),                   // Every 5 seconds
		"rare":         strPtrSchedule("0", "0", "0", "1", "1", "*", "*", "UTC"),                     // Once a year
	}
)

// Hata kontrolünü basitleştiren yardımcı
func must(s Schedule, err error) Schedule {
	if err != nil {
		panic(err)
	}
	return s
}

// === Performance Benchmarks ===

func BenchmarkEngineNext_Simple(b *testing.B) {
	schedule := schedulesGo["simple"]
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

func BenchmarkEngineNext_Complex(b *testing.B) {
	schedule := schedulesGo["complex"]
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

func BenchmarkEngineNext_SpecialChars(b *testing.B) {
	schedule := schedulesGo["specialChars"]
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

func BenchmarkEngineNext_Timezone(b *testing.B) {
	schedule := schedulesGo["timezone"]
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

func BenchmarkEngineNext_Frequent(b *testing.B) {
	schedule := schedulesGo["frequent"]
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

func BenchmarkEngineNext_Rare(b *testing.B) {
	schedule := schedulesGo["rare"]
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

func BenchmarkEnginePrev_Simple(b *testing.B) {
	schedule := schedulesGo["simple"]
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Prev(schedule, benchTime)
	}
}

func BenchmarkEnginePrev_Complex(b *testing.B) {
	schedule := schedulesGo["complex"]
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Prev(schedule, benchTime)
	}
}

// Cache performance benchmark
func BenchmarkCacheHit(b *testing.B) {
	schedule := schedulesGo["simple"]
	// Warm up cache
	benchEngine.Next(schedule, benchTime)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

func BenchmarkCacheMiss(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Create unique schedule each time to force cache miss
		uniqueSchedule := strPtrSchedule("0", "0", "*", "*", "*", "*", "*", fmt.Sprintf("UTC-%d", i))
		_, _ = benchEngine.Next(uniqueSchedule, benchTime)
	}
}

// Parse performance benchmark
func BenchmarkExpandPart_Simple(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = expandPart("*", 0, 59)
	}
}

func BenchmarkExpandPart_Range(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = expandPart("10-50", 0, 59)
	}
}

func BenchmarkExpandPart_List(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = expandPart("5,15,25,35,45,55", 0, 59)
	}
}

func BenchmarkExpandPart_Step(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = expandPart("*/5", 0, 59)
	}
}

// Bit operation benchmarks
func BenchmarkFindNextSetBit(b *testing.B) {
	mask := uint64(0b101010101010101010101010101010101010101010101010101010101010)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = findNextSetBit(mask, 10, 59)
	}
}

func BenchmarkFindPrevSetBit(b *testing.B) {
	mask := uint64(0b101010101010101010101010101010101010101010101010101010101010)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = findPrevSetBit(mask, 50, 59)
	}
}

// === YÜKSEK PERFORMANSLI SPECIAL CHARS BENCHMARK ===

func BenchmarkEngineNext_OptimizedSpecial(b *testing.B) {
	// Optimize edilmiş special char test: basit L pattern
	schedule := strPtrSchedule("0", "0", "12", "L", "*", "*", "*", "UTC")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

func BenchmarkEngineNext_SimpleSpecial(b *testing.B) {
	// Tek L pattern (karmaşık 5#3 yerine)
	schedule := strPtrSchedule("0", "0", "12", "*", "*", "5L", "*", "UTC")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

func BenchmarkEngineNext_HashPattern(b *testing.B) {
	// Tek # pattern
	schedule := strPtrSchedule("0", "0", "12", "*", "*", "1#2", "*", "UTC")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

// === GELIŞMIŞ PERFORMANS TESTLERI ===

func BenchmarkCacheMissOptimized(b *testing.B) {
	// Daha gerçekçi cache miss senaryosu
	schedules := []Schedule{
		strPtrSchedule("0", "0", "1", "*", "*", "*", "*", "UTC"),
		strPtrSchedule("0", "0", "2", "*", "*", "*", "*", "UTC"),
		strPtrSchedule("0", "0", "3", "*", "*", "*", "*", "UTC"),
		strPtrSchedule("0", "0", "4", "*", "*", "*", "*", "UTC"),
		strPtrSchedule("0", "0", "5", "*", "*", "*", "*", "UTC"),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		schedule := schedules[i%len(schedules)]
		_, _ = benchEngine.Next(schedule, benchTime)
	}
}

func BenchmarkStringOperations(b *testing.B) {
	b.Run("Split", func(b *testing.B) {
		s := "1#2,3#3,5#4"
		for i := 0; i < b.N; i++ {
			_ = strings.Split(s, ",")
		}
	})

	b.Run("Contains", func(b *testing.B) {
		s := "5L"
		for i := 0; i < b.N; i++ {
			_ = strings.Contains(s, "#")
		}
	})

	b.Run("HasSuffix", func(b *testing.B) {
		s := "5L"
		for i := 0; i < b.N; i++ {
			_ = strings.HasSuffix(s, "L")
		}
	})
}

func BenchmarkBitOperationsAdvanced(b *testing.B) {
	b.Run("PopCount", func(b *testing.B) {
		mask := uint64(0xAAAAAAAAAAAAAAAA)
		for i := 0; i < b.N; i++ {
			_ = bits.OnesCount64(mask)
		}
	})

	b.Run("TrailingZeros", func(b *testing.B) {
		mask := uint64(0xAAAAAAAAAAAAAAAA)
		for i := 0; i < b.N; i++ {
			_ = bits.TrailingZeros64(mask)
		}
	})
}

// Micro-optimization benchmarks for recent changes
func BenchmarkSpecialCharsOptimized(b *testing.B) {
	engine := New()
	// Test multiple special patterns in one expression
	schedule := &Schedule{
		Minute:     strPtr("0"),
		Hour:       strPtr("9"),
		DayOfMonth: strPtr("*"),
		Month:      strPtr("*"),
		DayOfWeek:  strPtr("1L,2#1,3-5"), // Mixed special patterns
		Year:       strPtr("*"),
	}

	// Pre-compile the schedule
	exp, _ := engine.getExpandedSchedule(*schedule)
	testTime := time.Date(2024, 3, 25, 9, 0, 0, 0, time.UTC) // Monday

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.checkSpecialDayPatterns(testTime, exp)
	}
}

func BenchmarkCacheKeyOptimized(b *testing.B) {
	engine := New()
	schedule := Schedule{
		Second:     strPtr("0"),
		Minute:     strPtr("30"),
		Hour:       strPtr("9-17"),
		DayOfMonth: strPtr("1-15"),
		Month:      strPtr("JAN-JUN"),
		DayOfWeek:  strPtr("MON-FRI"),
		Year:       strPtr("2024-2025"),
		Timezone:   strPtr("America/New_York"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.buildCacheKey(schedule)
	}
}

// === NEW: Ultra-Fast Special Chars Direct Algorithm Benchmark ===
func BenchmarkDirectAlgorithm_L_Pattern(b *testing.B) {
	engine := New()
	schedule := strPtrSchedule("0", "0", "12", "L", "*", "*", "*", "UTC")
	testTime := mustParseTime(time.RFC3339, "2025-01-15T10:00:00Z")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Next(schedule, testTime)
	}
}

func BenchmarkDirectAlgorithm_Hash_Pattern(b *testing.B) {
	engine := New()
	schedule := strPtrSchedule("0", "0", "12", "*", "*", "1#2", "*", "UTC")
	testTime := mustParseTime(time.RFC3339, "2025-01-01T10:00:00Z")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Next(schedule, testTime)
	}
}

func BenchmarkDirectAlgorithm_LastWeekday_Pattern(b *testing.B) {
	engine := New()
	schedule := strPtrSchedule("0", "0", "12", "*", "*", "5L", "*", "UTC")
	testTime := mustParseTime(time.RFC3339, "2025-01-01T10:00:00Z")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Next(schedule, testTime)
	}
}
