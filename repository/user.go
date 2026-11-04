// Package repository defines the storage interfaces. Implementations live
// in sub-packages (repository/mongodb), and tests use the mock package.
package repository

import (
	"context"

	"github.com/laddawan/backend-test/model"
)

// UserRepository stores users.
//
// Implementations must return model.ErrNotFound when a user does not exist
// and model.ErrEmailAlreadyExists when the email is already used.
type UserRepository interface {
	// Create stores a new user and sets its ID.
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	List(ctx context.Context) ([]*model.User, error)
	// Update saves the user's name and email.
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}
