package avoca

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestCreationError(t *testing.T) {
	err := RequestCreationError{
		errors.New("an error message"),
	}
	assert.Equal(t, "request creation failed: an error message", err.Error())
}
