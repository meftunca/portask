package protocol

// TTLConfig holds message TTL and dead-letter settings.
type TTLConfig struct {
	Enabled    bool
	TTLSeconds int
	DLX        string // Dead-letter exchange/topic
}
