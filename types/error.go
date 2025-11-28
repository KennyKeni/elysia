package types

import (
	"errors"
	"fmt"
)

var ErrUnsupportedResponseMode = errors.New("adapter does not support this response format mode")

type SchemaValidationError struct {
	RawResponse string
	Err         error
}

func (e *SchemaValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Err)
}

func (e *SchemaValidationError) Unwrap() error {
	return e.Err
}

type ToolNotCalledError struct {
	ExpectedTool string
	Response     *Message
}

func (e *ToolNotCalledError) Error() string {
	return fmt.Sprintf("expected tool %q was not called", e.ExpectedTool)
}
