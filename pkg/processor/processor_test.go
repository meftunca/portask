package processor_test

import (
	"testing"

	"github.com/meftunca/portask/pkg/processor"
)

func TestBasicProcessor(t *testing.T) {
	p := processor.NewBasicProcessor()
	data := []byte("hello world")
	processedData, err := p.Process(data)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if string(processedData) != "processed: hello world" {
		t.Errorf("Process() = %v, want %v", string(processedData), "processed: hello world")
	}
}
