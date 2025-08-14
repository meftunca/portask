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

// 	q, err := ch.QueueDeclare("portask_test", false, false, false, false, nil)
// 	if err != nil {
// 		panic(err)
// 	}

// 	err = ch.QueueBind(q.Name, "portask.test", "portask_test_exchange", false, nil)
// 	if err != nil {
// 		panic(err)
// 	}

// 	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
// 	if err != nil {
// 		panic(err)
// 	}

// 	fmt.Println("RabbitMQ consumer running. Press Ctrl+C to exit.")
// 	c := make(chan os.Signal, 1)
// 	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
// 	for {
// 		select {
// 		case msg := <-msgs:
// 			fmt.Println("Received:", string(msg.Body))
// 		case <-c:
// 			fmt.Println("Exiting.")
// 			return
// 		}
// 	}
// }
