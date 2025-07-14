package processor

// BasicProcessor is a simple implementation of a processor.
type BasicProcessor struct{}

// NewBasicProcessor creates a new BasicProcessor.
func NewBasicProcessor() *BasicProcessor {
	return &BasicProcessor{}
}

// Process simulates processing data.
func (p *BasicProcessor) Process(data []byte) ([]byte, error) {
	return []byte("processed: " + string(data)), nil
}
