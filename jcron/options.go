// options.go (Yeni Dosya)
package jcron

import (
	"fmt"
	"strings"
	"time"
)

// RetryOptions, bir görev başarısız olduğunda yeniden deneme davranışını belirler.
type RetryOptions struct {
	MaxRetries int           // Maksimum yeniden deneme sayısı. 0 ise yeniden denenmez.
	Delay      time.Duration // Her deneme arasındaki bekleme süresi.
}

// JobOption, bir göreve eklenen her türlü ayar için kullanılan fonksiyon tipidir.
// "Functional Options Pattern" olarak bilinir.
type JobOption func(*managedJob)

// WithRetries, bir göreve yeniden deneme yeteneği kazandıran bir JobOption'dır.
func WithRetries(maxRetries int, delay time.Duration) JobOption {
	return func(job *managedJob) {
		job.retryOpts = RetryOptions{
			MaxRetries: maxRetries,
			Delay:      delay,
		}
	}
}
func strPtr(s string) *string { return &s }

func FromCronSyntax(cronString string) (Schedule, error) {
	if strings.ToLower(cronString) == "@reboot" {
		return Schedule{Year: &cronString}, nil
	}
	if spec, ok := predefinedSchedules[cronString]; ok {
		cronString = spec
	}
	parts := strings.Fields(cronString)
	var s Schedule
	switch len(parts) {
	case 6:
		s = Schedule{Second: &parts[0], Minute: &parts[1], Hour: &parts[2], DayOfMonth: &parts[3], Month: &parts[4], DayOfWeek: &parts[5]}
	case 5:
		s = Schedule{Second: strPtr("0"), Minute: &parts[0], Hour: &parts[1], DayOfMonth: &parts[2], Month: &parts[3], DayOfWeek: &parts[4]}
	default:
		return Schedule{}, fmt.Errorf("invalid cron format")
	}
	return s, nil
}
