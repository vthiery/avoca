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

// ErrStatusCode is used internally to be able to differentiate between
// an actual error from the underlying HTTP client and an
// error that is returned for to retry retryable HTTP codes (from policy).
var ErrStatusCode = errors.New("status code internal error")
