package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/laddawan/backend-test/mock"
	"github.com/laddawan/backend-test/model"
	"github.com/laddawan/backend-test/usecase"
)

func newAuthService(t *testing.T, tokens *mock.TokenManager) (*usecase.AuthUsecase, *mock.UserRepository) {
	t.Helper()
	repo := mock.NewUserRepository()
	hasher := &mock.PasswordHasher{}
	v := usecase.NewValidator()
	users := usecase.NewUserUsecase(repo, hasher, v)
	auth, err := usecase.NewAuthUsecase(users, repo, hasher, tokens, v)
	if err != nil {
		t.Fatalf("NewAuthService() error = %v", err)
	}
	return auth, repo
}

func TestAuthService_RegisterAndLogin(t *testing.T) {
	auth, _ := newAuthService(t, &mock.TokenManager{})
	ctx := context.Background()

	user, err := auth.Register(ctx, validInput())
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	token, err := auth.Login(ctx, usecase.LoginInput{Email: "ALICE@example.com", Password: "s3cretpass"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if token.AccessToken != "token-"+user.ID {
		t.Errorf("AccessToken = %q, want token for user %q", token.AccessToken, user.ID)
	}
	if token.ExpiresAt.IsZero() {
		t.Error("expected ExpiresAt to be set")
	}
}

func TestAuthService_Login_Failures(t *testing.T) {
	auth, _ := newAuthService(t, &mock.TokenManager{})
	ctx := context.Background()
	if _, err := auth.Register(ctx, validInput()); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	t.Run("wrong password", func(t *testing.T) {
		_, err := auth.Login(ctx, usecase.LoginInput{Email: "alice@example.com", Password: "wrong-password"})
		if !errors.Is(err, model.ErrInvalidCredentials) {
			t.Fatalf("error = %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("unknown email", func(t *testing.T) {
		_, err := auth.Login(ctx, usecase.LoginInput{Email: "nobody@example.com", Password: "s3cretpass"})
		if !errors.Is(err, model.ErrInvalidCredentials) {
			t.Fatalf("error = %v, want ErrInvalidCredentials (same as wrong password)", err)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		_, err := auth.Login(ctx, usecase.LoginInput{Email: "bad"})
		var verr *model.ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("error = %v, want ValidationError", err)
		}
	})
}

func TestAuthService_Login_TokenError(t *testing.T) {
	issueErr := errors.New("signing failed")
	auth, _ := newAuthService(t, &mock.TokenManager{IssueErr: issueErr})
	ctx := context.Background()
	_, _ = auth.Register(ctx, validInput())

	_, err := auth.Login(ctx, usecase.LoginInput{Email: "alice@example.com", Password: "s3cretpass"})
	if !errors.Is(err, issueErr) {
		t.Fatalf("error = %v, want wrapped issue error", err)
	}
}

func TestAuthService_Login_RepositoryError(t *testing.T) {
	auth, repo := newAuthService(t, &mock.TokenManager{})
	repoErr := errors.New("database down")
	repo.Err = repoErr

	_, err := auth.Login(context.Background(), usecase.LoginInput{Email: "alice@example.com", Password: "x"})
	if !errors.Is(err, repoErr) {
		t.Fatalf("error = %v, want repository error", err)
	}
}
