// Package usecase implements the business logic. It depends on the
// repository interfaces, never on a concrete database.
package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/laddawan/backend-test/model"
	"github.com/laddawan/backend-test/repository"
)

type UserUsecase struct {
	userRepository repository.UserRepository
	hasher         PasswordHasher
	validate       *Validator
	now            func() time.Time
}

func NewUserUsecase(userRepository repository.UserRepository, hasher PasswordHasher, validate *Validator) *UserUsecase {
	return &UserUsecase{
		userRepository: userRepository,
		hasher:         hasher,
		validate:       validate,
		now:            time.Now,
	}
}

// Create validates the input, hashes the password and stores a new user.
func (u *UserUsecase) Create(ctx context.Context, in CreateUserInput) (*model.User, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = normalizeEmail(in.Email)
	if err := u.validate.Struct(in); err != nil {
		return nil, err
	}

	hash, err := u.hasher.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		Name:         in.Name,
		Email:        in.Email,
		PasswordHash: hash,
		// MongoDB stores millisecond precision; truncate so the returned
		// value matches what is persisted.
		CreatedAt: u.now().UTC().Truncate(time.Millisecond),
	}
	if err := u.userRepository.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Get returns the user with the given ID.
func (u *UserUsecase) Get(ctx context.Context, id string) (*model.User, error) {
	if strings.TrimSpace(id) == "" {
		return nil, model.ErrNotFound
	}
	return u.userRepository.GetByID(ctx, id)
}

// List returns all users.
func (u *UserUsecase) List(ctx context.Context) ([]*model.User, error) {
	return u.userRepository.List(ctx)
}

// Update changes a user's name and/or email.
func (u *UserUsecase) Update(ctx context.Context, id string, in UpdateUserInput) (*model.User, error) {
	if in.Name == nil && in.Email == nil {
		return nil, &model.ValidationError{Fields: map[string]string{
			"body": "must contain at least one of name or email",
		}}
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		in.Name = &name
	}
	if in.Email != nil {
		email := normalizeEmail(*in.Email)
		in.Email = &email
	}
	if err := u.validate.Struct(in); err != nil {
		return nil, err
	}

	user, err := u.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		user.Name = *in.Name
	}
	if in.Email != nil {
		user.Email = *in.Email
	}
	if err := u.userRepository.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Delete removes the user with the given ID.
func (u *UserUsecase) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return model.ErrNotFound
	}
	return u.userRepository.Delete(ctx, id)
}

// normalizeEmail makes email comparison case-insensitive so the unique
// index cannot be bypassed with different casing.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
