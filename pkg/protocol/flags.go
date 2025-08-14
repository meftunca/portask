package protocol

// Flags for message features: priority, batch, encryption.
const (
	FlagPriority = 1 << 0
	FlagBatch    = 1 << 1
	FlagEncrypt  = 1 << 2
)
