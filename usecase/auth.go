package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/laddawan/backend-test/model"
	"github.com/laddawan/backend-test/repository"
)

type AuthUsecase struct {
	userUsecase    *UserUsecase
	userRepository repository.UserRepository
	hasher         PasswordHasher
	tokens         TokenIssuer
	validate       *Validator

	// dummyHash is compared against when the email is unknown, so a failed
	// login takes about the same time whether or not the account exists.
	dummyHash string
}

func NewAuthUsecase(
	userUsecase *UserUsecase,
	userRepository repository.UserRepository,
	hasher PasswordHasher,
	tokens TokenIssuer,
	validate *Validator,
) (*AuthUsecase, error) {
	dummy, err := hasher.Hash("timing-guard-not-a-real-password")
	if err != nil {
		return nil, fmt.Errorf("create dummy hash: %w", err)
	}
	return &AuthUsecase{
		userUsecase:    userUsecase,
		userRepository: userRepository,
		hasher:         hasher,
		tokens:         tokens,
		validate:       validate,
		dummyHash:      dummy,
	}, nil
}

// Register creates a new user account.
func (a *AuthUsecase) Register(ctx context.Context, in CreateUserInput) (*model.User, error) {
	return a.userUsecase.Create(ctx, in)
}

// Login verifies the credentials and returns a signed access token.
// Unknown emails and wrong passwords both return model.ErrInvalidCredentials.
func (a *AuthUsecase) Login(ctx context.Context, in LoginInput) (*AuthToken, error) {
	in.Email = normalizeEmail(in.Email)
	if err := a.validate.Struct(in); err != nil {
		return nil, err
	}

	user, err := a.userRepository.GetByEmail(ctx, in.Email)
	if errors.Is(err, model.ErrNotFound) {
		_ = a.hasher.Compare(a.dummyHash, in.Password)
		return nil, model.ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if err := a.hasher.Compare(user.PasswordHash, in.Password); err != nil {
		if errors.Is(err, model.ErrInvalidCredentials) {
			return nil, model.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("compare password: %w", err)
	}

	token, expiresAt, err := a.tokens.Issue(user.ID)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}
	return &AuthToken{AccessToken: token, ExpiresAt: expiresAt}, nil
}
