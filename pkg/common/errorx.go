package common

import "fmt"

// ErrorWithStack is a custom error with stack trace.
type ErrorWithStack struct {
	Msg   string
	Stack string
}

func (e *ErrorWithStack) Error() string {
	return fmt.Sprintf("%s\nStack: %s", e.Msg, e.Stack)
}

func NewErrorWithStack(msg string) error {
	return &ErrorWithStack{Msg: msg, Stack: "<stack trace stub>"}
}
