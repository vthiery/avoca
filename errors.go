package avoca

import (
	"errors"
	"fmt"
)

// RequestCreationError is used to signal the request creation failed.
type RequestCreationError struct {
	Err error
}

// Error returns the error message.
func (e *RequestCreationError) Error() string {
	return fmt.Errorf("request creation failed: %w", e.Err).Error()
}

// errStatus is only used to be able to differentiate between
// an actual error from the underlying HTTP client and an
// error that must be returned for retryable HTTP codes.
var errStatus = errors.New("status error")
