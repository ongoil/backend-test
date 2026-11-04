// Package security provides password hashing (bcrypt) and JWT tokens.
package security

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/laddawan/backend-test/model"
)

// BcryptHasher hashes passwords with bcrypt.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher returns a hasher using the given cost.
// A cost of 0 uses bcrypt.DefaultCost.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

// Hash returns the bcrypt hash of password.
func (h *BcryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(b), nil
}

// Compare returns model.ErrInvalidCredentials if password does not match hash.
func (h *BcryptHasher) Compare(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return model.ErrInvalidCredentials
	}
	return err
}
