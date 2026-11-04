package model

import "testing"

func TestValidationError_Message(t *testing.T) {
	err := &ValidationError{Fields: map[string]string{"name": "is required", "email": "is invalid"}}
	if got, want := err.Error(), "validation failed: email is invalid; name is required"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
