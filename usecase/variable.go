package usecase

import (
	"context"
	"time"
)

// CreateUserInput is the input for creating (or registering) a user.
type CreateUserInput struct {
	Name     string `validate:"required,min=1,max=100"`
	Email    string `validate:"required,email,max=254"`
	Password string `validate:"required,min=8,max=72"` // bcrypt only uses the first 72 bytes
}

// UpdateUserInput is the input for a partial update. Nil fields are left
// unchanged; at least one field must be set.
type UpdateUserInput struct {
	Name  *string `validate:"omitnil,min=1,max=100"`
	Email *string `validate:"omitnil,email,max=254"`
}

// LoginInput is the input for authenticating a user.
type LoginInput struct {
	Email    string `validate:"required,email"`
	Password string `validate:"required"`
}

// AuthToken is a signed access token returned after a successful login.
type AuthToken struct {
	AccessToken string
	ExpiresAt   time.Time
}

// PasswordHasher hashes and verifies passwords (implemented by security.BcryptHasher).
type PasswordHasher interface {
	Hash(password string) (string, error)
	// Compare returns model.ErrInvalidCredentials when the password does not match.
	Compare(hash, password string) error
}

// TokenIssuer creates signed access tokens (implemented by security.JWTManager).
type TokenIssuer interface {
	Issue(userID string) (token string, expiresAt time.Time, err error)
}

// UserCountReader reports the total number of users (implemented by the repository).
type UserCountReader interface {
	Count(ctx context.Context) (int64, error)
}
