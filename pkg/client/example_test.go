package client

import (
	"context"
	"fmt"
)

// Example usage of the Portask Go client
func Example() {
	client := NewClient("http://localhost:8080", "")
	ctx := context.Background()

	// Submit a task
	taskReq := TaskRequest{
		Type: "echo",
		Data: map[string]interface{}{"message": "hello world"},
	}
	resp, err := client.SubmitTask(ctx, taskReq)
	if err != nil {
		panic(err)
	}
	fmt.Println("Task submitted:", resp.TaskID)

	// Query status
	status, err := client.GetTaskStatus(ctx, resp.TaskID)
	if err != nil {
		panic(err)
	}
	fmt.Println("Task status:", status.Status)

	// Cancel task (optional)
	// err = client.CancelTask(ctx, resp.TaskID)
	// ...

	// List tasks
	list, err := client.ListTasks(ctx)
	if err == nil {
		fmt.Println("Recent tasks:", len(list.Tasks))
	}
}
