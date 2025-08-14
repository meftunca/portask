package main

// Only one main function per Go package is allowed. Commenting out this main for build.
// func main() {
// 	config := network.DefaultKafkaConfig(network.KafkaClientKafkaGo)
// 	bridge, err := network.NewKafkaPortaskBridge(config, "localhost:9000", "demo-topic")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer bridge.Stop()

// 	err = bridge.ForwardFromKafkaToPortask([]string{"demo-topic"})
// 	if err != nil {
// 		panic(err)
// 	}

// 	fmt.Println("Kafka consumer/bridge running. Press Ctrl+C to exit.")
// 	c := make(chan os.Signal, 1)
// 	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
// 	<-c
// 	fmt.Println("Exiting.")
// }
