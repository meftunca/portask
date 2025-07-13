// runner_test.go (Yeni Dosya)
package jcron

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testRunnerSetup, her test için tamponlanmış bir logger ile yeni bir runner oluşturur.
// Bu, test sırasında logları yakalamamızı ve kontrol etmemizi sağlar.
func testRunnerSetup() (*Runner, *bytes.Buffer) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	runner := NewRunner(logger)
	return runner, &logBuf
}

// TestRunner_SuccessfulJob, bir görevin doğru zamanda çalışıp başarıyla tamamlandığını test eder.
func TestRunner_SuccessfulJob(t *testing.T) {
	runner, logBuf := testRunnerSetup()
	jobRan := make(chan bool, 1)

	_, err := runner.AddFuncCron("* * * * * *", func() error {
		jobRan <- true
		return nil
	})
	if err != nil {
		t.Fatalf("Görev eklenemedi: %v", err)
	}

	runner.Start()
	defer runner.Stop()

	select {
	case <-jobRan:
		// Başarılı, görev çalıştı.
	case <-time.After(2 * time.Second):
		t.Fatal("Zaman aşımı: Görev 2 saniye içinde çalışmadı.")
	}

	// Logları kontrol et
	logStr := logBuf.String()
	if !strings.Contains(logStr, "görev başarıyla tamamlandı") {
		t.Errorf("Başarılı görev logu bulunamadı. Alınan log: %s", logStr)
	}
}

// TestRunner_RetryLogic, bir görevin hata verdiğinde yeniden deneme mekanizmasının
// doğru çalışıp çalışmadığını ve en sonunda başarılı olduğunu test eder.
func TestRunner_RetryLogic(t *testing.T) {
	runner, logBuf := testRunnerSetup()
	var attemptCount int32
	jobSucceeded := make(chan bool, 1)

	_, err := runner.AddFuncCron(
		"* * * * * *", // Her saniye tetikle
		func() error {
			currentAttempt := atomic.AddInt32(&attemptCount, 1)
			if currentAttempt < 3 { // İlk 2 denemede hata ver
				return errors.New("geçici hata")
			}
			jobSucceeded <- true // 3. denemede başarılı ol
			return nil
		},
		WithRetries(3, 50*time.Millisecond), // 3 kez tekrar dene, 50ms arayla
	)
	if err != nil {
		t.Fatalf("Görev eklenemedi: %v", err)
	}

	runner.Start()
	defer runner.Stop()

	// Görevin mantıksal olarak başarılı olmasını bekle
	select {
	case <-jobSucceeded:
		// Başarılı. Şimdi logların yazıldığından emin olalım.
	case <-time.After(3 * time.Second):
		t.Fatal("Zaman aşımı: Görev yeniden denemelerle birlikte başarılı olamadı.")
	}

	// --- DÜZELTME BAŞLANGICI ---
	// Yarış durumunu önlemek için logların yazılmasını bekle (polling).
	// Yaklaşık 1 saniye boyunca 10ms aralıklarla logu kontrol et.
	var successLogFound bool
	for i := 0; i < 100; i++ {
		if strings.Contains(logBuf.String(), "görev başarıyla tamamlandı") {
			successLogFound = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !successLogFound {
		t.Fatalf("Nihai başarı logu bulunamadı. Alınan log: %s", logBuf.String())
	}
	// --- DÜZELTME SONU ---

	// Deneme sayısını kontrol et
	if atomic.LoadInt32(&attemptCount) != 3 {
		t.Errorf("Beklenen deneme sayısı 3, alınan %d", attemptCount)
	}

	// Logların geri kalanını şimdi güvenle kontrol edebiliriz.
	logStr := logBuf.String()
	if strings.Count(logStr, "görev hata verdi, yeniden denenecek") != 2 {
		t.Error("Beklenen yeniden deneme logu (2 adet) bulunamadı.")
	}
	if strings.Contains(logStr, "başarısız oldu") {
		t.Error("Görevin kalıcı olarak başarısız olmaması gerekiyordu.")
	}
}

// TestRunner_PanicRecovery, bir görev panic yaptığında runner'ın çökmediğini
// ve diğer görevleri çalıştırmaya devam ettiğini test eder.
func TestRunner_PanicRecovery(t *testing.T) {
	runner, logBuf := testRunnerSetup()
	goodJobRan := make(chan bool, 1)

	// Panic yapacak kötü görev
	_, err := runner.AddFuncCron("* * * * * *", func() error {
		panic("eyvah, panic!")
	})
	if err != nil {
		t.Fatalf("Kötü görev eklenemedi: %v", err)
	}

	// Düzgün çalışacak iyi görev
	_, err = runner.AddFuncCron("* * * * * *", func() error {
		goodJobRan <- true
		return nil
	})
	if err != nil {
		t.Fatalf("İyi görev eklenemedi: %v", err)
	}

	runner.Start()
	defer runner.Stop()

	// İyi görevin çalışmasını bekle, bu runner'ın hayatta kaldığını kanıtlar.
	select {
	case <-goodJobRan:
		// Başarılı, runner çökmedi.
	case <-time.After(3 * time.Second):
		t.Fatal("Zaman aşımı: Panic sonrası iyi görev çalışmadı, runner çökmüş olabilir.")
	}

	// Panic logunu kontrol et
	if !strings.Contains(logBuf.String(), "görevde panic yaşandı") {
		t.Error("Panic logu bulunamadı.")
	}
}

// TestRunner_RemoveJob, bir görevin kaldırıldıktan sonra bir daha çalışmadığını test eder.
func TestRunner_RemoveJob(t *testing.T) {
	runner, _ := testRunnerSetup()
	var runCount int32
	jobRanFirstTime := make(chan bool, 1)

	jobID, err := runner.AddFuncCron("* * * * * *", func() error {
		atomic.AddInt32(&runCount, 1)
		// Sadece ilk çalıştığında sinyal gönder
		if atomic.LoadInt32(&runCount) == 1 {
			jobRanFirstTime <- true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Görev eklenemedi: %v", err)
	}

	runner.Start()
	defer runner.Stop()

	// İlk kez çalışmasını bekle
	select {
	case <-jobRanFirstTime:
		// Başarılı.
	case <-time.After(2 * time.Second):
		t.Fatal("Zaman aşımı: Görev ilk kez çalışmadı.")
	}

	// Görevi kaldır
	runner.RemoveJob(jobID)

	// Bir süre bekle ve tekrar çalışıp çalışmadığını kontrol et
	time.Sleep(1500 * time.Millisecond)

	if atomic.LoadInt32(&runCount) > 1 {
		t.Errorf("Görev kaldırıldıktan sonra bile çalışmaya devam etti. Toplam çalışma: %d", runCount)
	}
}
