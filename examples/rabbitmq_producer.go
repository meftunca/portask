package main

// Only one main function per Go package is allowed. Commenting out this main for build.
// func main() {
// 	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer conn.Close()

// 	ch, err := conn.Channel()
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer ch.Close()

// 	err = ch.ExchangeDeclare("portask_test_exchange", "topic", true, false, false, false, nil)
// 	if err != nil {
// 		panic(err)
// 	}

// 	for i := 0; i < 10; i++ {
// 		msg := fmt.Sprintf("rabbitmq-message-%d", i)
// 		err = ch.Publish("portask_test_exchange", "portask.test", false, false, amqp.Publishing{
// 			ContentType: "text/plain",
// 			Body:        []byte(msg),
// 		})
// 		if err != nil {
// 			fmt.Println("Publish error:", err)
// 		}
// 		time.Sleep(500 * time.Millisecond)
// 	}

// 	fmt.Println("RabbitMQ producer finished.")
// }
