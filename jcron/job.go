// job.go
package jcron

// Job, zamanlayıcı tarafından çalıştırılabilen herhangi bir görevin
// uyması gereken arayüzdür (interface).
// Run() metoduna sahip her yapı, bir Job olarak kabul edilir.
type Job interface {
	Run() error
}

// JobFunc, basit fonksiyonları Job arayüzüne uydurmamızı sağlayan
// bir yardımcı tiptir. Bu, kullanıcıların kolayca anonim fonksiyonları
// görev olarak eklemesine olanak tanır.
type JobFunc func() error

// Run metodu, JobFunc'ın Job arayüzünü uygulamasını sağlar.
func (f JobFunc) Run() error {
	return f()
}
