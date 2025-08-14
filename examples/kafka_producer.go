package main

// Only one main function per Go package is allowed. Commenting out this main for build.
// func main() {
// 	config := network.DefaultKafkaConfig(network.KafkaClientKafkaGo)
// 	bridge, err := network.NewKafkaPortaskBridge(config, "localhost:9000", "demo-topic")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer bridge.Stop()

// 	for i := 0; i < 10; i++ {
// 		msg := fmt.Sprintf("test-message-%d", i)
// 		err := bridge.PublishToKafka("demo-topic", "", []byte(msg))
// 		if err != nil {
// 			fmt.Println("Publish error:", err)
// 		}
// 		time.Sleep(500 * time.Millisecond)
// 	}

// 	fmt.Println("Kafka producer finished.")
// }
