package benchmarks

// // TestRealBatchComparison - GERÇEK Dragonfly ile batch test
// func TestRealBatchComparison(t *testing.T) {
// 	ctx := context.Background()

// 	// Dragonfly bağlantısı oluştur
// 	dfConfig := &storage.DragonflyConfig{
// 		Addresses:         []string{"localhost:6379"},
// 		DB:                0,
// 		KeyPrefix:         "batch-test",
// 		EnableCompression: false, // Performans için kapalı
// 	}

// 	dragonflyStore, err := dragonfly.NewDragonflyStore(dfConfig)
// 	if err != nil {
// 		t.Fatalf("Dragonfly store oluşturulamadı: %v", err)
// 	}

// 	err = dragonflyStore.Connect(ctx)
// 	if err != nil {
// 		t.Skipf("Dragonfly bağlantısı başarısız (localhost:6379): %v\nDragonfly çalıştırın: docker run -p 6379:6379 docker.dragonflydb.io/dragonflydb/dragonfly", err)
// 		return
// 	}
// 	defer dragonflyStore.Close()

// 	// Test öncesi temizlik
// 	dragonflyStore.GetClient().FlushDB(ctx)

// 	fmt.Printf("\n")
// 	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
// 	fmt.Printf("║                                                                  ║\n")
// 	fmt.Printf("║  💾 REAL DRAGONFLY BATCH TEST                                    ║\n")
// 	fmt.Printf("║                                                                  ║\n")
// 	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
// 	fmt.Printf("\n")

// 	// Test öncesi metricler
// 	beforeStats, _ := dragonflyStore.Stats(ctx)
// 	fmt.Printf("  📊 Başlangıç Metrikleri:\n")
// 	fmt.Printf("     Total Operations: %d\n", beforeStats.TotalOperations)
// 	fmt.Printf("     Message Count: %d\n", beforeStats.MessageCount)
// 	fmt.Printf("\n")

// 	// Test 1: Non-Batch (tek tek yazma)
// 	fmt.Printf("  📊 Test 1: Non-Batch Write (GERÇEK Dragonfly, 2s)\n")
// 	nonBatchCount := runRealNonBatchTest(ctx, dragonflyStore, 2*time.Second)
// 	nonBatchThroughput := float64(nonBatchCount) / 2.0
// 	fmt.Printf("     Messages: %d\n", nonBatchCount)
// 	fmt.Printf("     Throughput: %.0f msgs/sec\n", nonBatchThroughput)
// 	fmt.Printf("\n")

// 	// Metricler
// 	afterNonBatch, _ := dragonflyStore.Stats(ctx)
// 	fmt.Printf("     Dragonfly Operations: %d\n", afterNonBatch.TotalOperations-beforeStats.TotalOperations)
// 	fmt.Printf("     Messages Stored: %d\n", afterNonBatch.MessageCount-beforeStats.MessageCount)
// 	fmt.Printf("\n")

// 	// Temizlik
// 	time.Sleep(500 * time.Millisecond)
// 	dragonflyStore.GetClient().FlushDB(ctx)

// 	// Test 2: Batch Write (10ms window)
// 	fmt.Printf("  📊 Test 2: Batch Write (GERÇEK Dragonfly, 10ms window, 2s)\n")
// 	batchBeforeStats, _ := dragonflyStore.Stats(ctx)
// 	batchCount := runRealBatchTest(ctx, dragonflyStore, 2*time.Second, 1000, 10*time.Millisecond)
// 	batchThroughput := float64(batchCount) / 2.0
// 	fmt.Printf("     Messages: %d\n", batchCount)
// 	fmt.Printf("     Throughput: %.0f msgs/sec\n", batchThroughput)
// 	fmt.Printf("\n")

// 	// Metricler
// 	afterBatch, _ := dragonflyStore.Stats(ctx)
// 	batchOps := afterBatch.TotalOperations - batchBeforeStats.TotalOperations
// 	batchMsgs := afterBatch.MessageCount - batchBeforeStats.MessageCount
// 	fmt.Printf("     Dragonfly Operations: %d\n", batchOps)
// 	fmt.Printf("     Messages Stored: %d\n", batchMsgs)

// 	// Batch efficiency
// 	if nonBatchCount > 0 {
// 		expectedOps := nonBatchCount
// 		actualOps := int64(batchOps)
// 		reduction := float64(expectedOps-actualOps) / float64(expectedOps) * 100
// 		fmt.Printf("     Operation Reduction: %.1f%%\n", reduction)
// 	}
// 	fmt.Printf("\n")

// 	// Sonuçlar
// 	improvement := batchThroughput / nonBatchThroughput

// 	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
// 	fmt.Printf("║  📈 REAL DRAGONFLY BATCH RESULTS                                 ║\n")
// 	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
// 	fmt.Printf("║  Non-Batch:              %.0f msgs/sec                          ║\n", nonBatchThroughput)
// 	fmt.Printf("║  Batch (10ms):           %.0f msgs/sec                          ║\n", batchThroughput)
// 	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
// 	fmt.Printf("║  Improvement Factor:     %.1fx faster 🚀                        ║\n", improvement)
// 	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
// 	fmt.Printf("║  Dragonfly Metrics:                                              ║\n")
// 	fmt.Printf("║  • Non-Batch Ops:        %d operations                          ║\n", afterNonBatch.TotalOperations-beforeStats.TotalOperations)
// 	fmt.Printf("║  • Batch Ops:            %d operations                          ║\n", batchOps)
// 	if nonBatchCount > 0 {
// 		reduction := float64(nonBatchCount-int64(batchOps)) / float64(nonBatchCount) * 100
// 		fmt.Printf("║  • Reduction:            %.1f%% fewer operations 🔥             ║\n", reduction)
// 	}
// 	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
// 	fmt.Printf("\n")

// 	if improvement < 5.0 {
// 		t.Logf("Warning: Batch improvement (%.1fx) is lower than expected", improvement)
// 	}
// }

// // runRealNonBatchTest tests non-batched writes with real Dragonfly
// func runRealNonBatchTest(ctx context.Context, store *dragonfly.DragonflyStore, duration time.Duration) int64 {
// 	var count atomic.Int64
// 	start := time.Now()

// 	for time.Since(start) < duration {
// 		msg := &types.PortaskMessage{
// 			ID:        types.MessageID(fmt.Sprintf("nb-%d", time.Now().UnixNano())),
// 			Topic:     "test-topic",
// 			Partition: 0,
// 			Payload:   []byte("test-message-payload"),
// 			Timestamp: time.Now().UnixNano(),
// 			TTL:       int64(time.Hour),
// 		}

// 		err := store.Store(ctx, msg)
// 		if err == nil {
// 			count.Add(1)
// 		}
// 	}

// 	return count.Load()
// }

// // runRealBatchTest tests batched writes with real Dragonfly
// func runRealBatchTest(ctx context.Context, store *dragonfly.DragonflyStore, duration time.Duration, batchSize int, flushInterval time.Duration) int64 {
// 	batchWriter := kafka.NewBatchWriter(&kafka.BatchWriterConfig{
// 		Store:         store,
// 		Ctx:           ctx,
// 		BatchSize:     batchSize,
// 		FlushInterval: flushInterval,
// 	})
// 	defer batchWriter.Close()

// 	var count atomic.Int64
// 	start := time.Now()

// 	for time.Since(start) < duration {
// 		msg := &types.PortaskMessage{
// 			ID:        types.MessageID(fmt.Sprintf("b-%d", time.Now().UnixNano())),
// 			Topic:     "test-topic",
// 			Partition: 0,
// 			Payload:   []byte("test-message-payload"),
// 			Timestamp: time.Now().UnixNano(),
// 			TTL:       int64(time.Hour),
// 		}

// 		err := batchWriter.Write(msg)
// 		if err == nil {
// 			count.Add(1)
// 		}
// 	}

// 	// Final flush
// 	batchWriter.Flush()

// 	return count.Load()
// }
