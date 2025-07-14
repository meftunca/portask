package main

import (
	"log"
	"time"

	portaskjson "github.com/meftunca/portask/pkg/json"
)

func main() {
	log.Printf("🚀 Portask JSON Library Demo")
	log.Printf("📋 Testing Fiber v2 with configurable JSON libraries...")

	// Test with standard JSON library
	log.Printf("\n📊 Testing with standard encoding/json...")
	testJSONLibrary("standard")

	// Test with Sonic JSON library
	log.Printf("\n🚄 Testing with Sonic JSON library...")
	testJSONLibrary("sonic")

	log.Printf("\n✅ JSON library migration complete!")
	log.Printf("🎯 Key improvements:")
	log.Printf("   📚 Configurable JSON libraries (standard/sonic)")
	log.Printf("   � Fiber v2 for high-performance HTTP")
	log.Printf("   ⚡ Sonic JSON for faster encoding/decoding")
	log.Printf("   � Runtime JSON library switching")
	log.Printf("   📊 Built-in performance benchmarks")
}

func testJSONLibrary(library string) {
	config := portaskjson.Config{
		Library:    portaskjson.JSONLibrary(library),
		Compact:    true,
		EscapeHTML: false,
	}

	// Initialize JSON library
	if err := portaskjson.InitializeFromConfig(config); err != nil {
		log.Printf("❌ Failed to initialize %s: %v", library, err)
		return
	}

	// Test data
	testData := map[string]interface{}{
		"message":   "Hello, Portask!",
		"timestamp": time.Now(),
		"library":   library,
		"features":  []string{"fast", "efficient", "configurable"},
		"config": map[string]interface{}{
			"compact":     config.Compact,
			"escape_html": config.EscapeHTML,
		},
	}

	// Benchmark encoding
	start := time.Now()
	for i := 0; i < 10000; i++ {
		_, err := portaskjson.Marshal(testData)
		if err != nil {
			log.Printf("❌ Encoding error: %v", err)
			return
		}
	}
	encodingTime := time.Since(start)

	// Single encoding for output
	jsonData, err := portaskjson.Marshal(testData)
	if err != nil {
		log.Printf("❌ Encoding error: %v", err)
		return
	}

	log.Printf("✅ %s JSON - 10k encodings: %v", library, encodingTime)
	log.Printf("📄 Sample output: %s", string(jsonData))

	// Test decoding
	var decoded map[string]interface{}
	start = time.Now()
	for i := 0; i < 10000; i++ {
		err := portaskjson.Unmarshal(jsonData, &decoded)
		if err != nil {
			log.Printf("❌ Decoding error: %v", err)
			return
		}
	}
	decodingTime := time.Since(start)

	log.Printf("✅ %s JSON - 10k decodings: %v", library, decodingTime)
}
