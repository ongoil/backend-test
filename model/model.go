// Package model is the domain layer: entities and errors shared by every
// layer. It has no dependency on MongoDB, Fiber, gRPC or any other package
// of this project.
package model

import (
	"errors"
	"sort"
	"strings"
)

// Errors shared across layers. The api and grpcapi packages translate them
// into HTTP status codes and gRPC codes.
var (
	ErrNotFound           = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// ValidationError describes invalid input, keyed by field name.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+" "+e.Fields[k])
	}
	return "validation failed: " + strings.Join(parts, "; ")
}
