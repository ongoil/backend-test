package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/laddawan/backend-test/mock"
	"github.com/laddawan/backend-test/model"
	"github.com/laddawan/backend-test/usecase"
)

func newUserService(t *testing.T) (*usecase.UserUsecase, *mock.UserRepository) {
	t.Helper()
	repo := mock.NewUserRepository()
	return usecase.NewUserUsecase(repo, &mock.PasswordHasher{}, usecase.NewValidator()), repo
}

func strPtr(s string) *string { return &s }

func validInput() usecase.CreateUserInput {
	return usecase.CreateUserInput{Name: "Alice", Email: "alice@example.com", Password: "s3cretpass"}
}

func TestUserService_Create(t *testing.T) {
	svc, repo := newUserService(t)
	ctx := context.Background()

	user, err := svc.Create(ctx, usecase.CreateUserInput{
		Name:     "  Alice  ",
		Email:    "  Alice@Example.COM ",
		Password: "s3cretpass",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if user.ID == "" {
		t.Error("expected ID to be set")
	}
	if user.Name != "Alice" {
		t.Errorf("Name = %q, want trimmed %q", user.Name, "Alice")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Email = %q, want normalized %q", user.Email, "alice@example.com")
	}
	if user.PasswordHash != "hashed:s3cretpass" {
		t.Errorf("PasswordHash = %q, want hashed password", user.PasswordHash)
	}
	if user.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	stored, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("user was not stored: %v", err)
	}
	if stored.Email != user.Email {
		t.Errorf("stored email = %q, want %q", stored.Email, user.Email)
	}
}

func TestUserService_Create_Validation(t *testing.T) {
	tests := []struct {
		name      string
		in        usecase.CreateUserInput
		wantField string
	}{
		{"missing name", usecase.CreateUserInput{Name: "   ", Email: "a@b.co", Password: "password1"}, "name"},
		{"bad email", usecase.CreateUserInput{Name: "A", Email: "not-an-email", Password: "password1"}, "email"},
		{"missing email", usecase.CreateUserInput{Name: "A", Password: "password1"}, "email"},
		{"short password", usecase.CreateUserInput{Name: "A", Email: "a@b.co", Password: "short"}, "password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newUserService(t)
			_, err := svc.Create(context.Background(), tt.in)

			var verr *model.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("error = %v, want *model.ValidationError", err)
			}
			if _, ok := verr.Fields[tt.wantField]; !ok {
				t.Errorf("fields = %v, want an error for %q", verr.Fields, tt.wantField)
			}
		})
	}
}

func TestUserService_Create_DuplicateEmail(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	if _, err := svc.Create(ctx, validInput()); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	in := validInput()
	in.Email = "ALICE@example.com" // same email, different case
	_, err := svc.Create(ctx, in)
	if !errors.Is(err, model.ErrEmailAlreadyExists) {
		t.Fatalf("error = %v, want ErrEmailAlreadyExists", err)
	}
}

func TestUserService_Create_HashError(t *testing.T) {
	repo := mock.NewUserRepository()
	hashErr := errors.New("boom")
	svc := usecase.NewUserUsecase(repo, &mock.PasswordHasher{HashErr: hashErr}, usecase.NewValidator())

	_, err := svc.Create(context.Background(), validInput())
	if !errors.Is(err, hashErr) {
		t.Fatalf("error = %v, want wrapped hash error", err)
	}
}

func TestUserService_Get(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	created, _ := svc.Create(ctx, validInput())

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != created.ID || got.Email != created.Email {
		t.Errorf("Get() = %+v, want %+v", got, created)
	}

	for _, id := range []string{"", "does-not-exist"} {
		if _, err := svc.Get(ctx, id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("Get(%q) error = %v, want ErrNotFound", id, err)
		}
	}
}

func TestUserService_List(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()

	users, err := svc.List(ctx)
	if err != nil || len(users) != 0 {
		t.Fatalf("List() on empty repo = %v, %v; want empty, nil", users, err)
	}

	_, _ = svc.Create(ctx, validInput())
	in := validInput()
	in.Email = "bob@example.com"
	_, _ = svc.Create(ctx, in)

	users, err = svc.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(users) != 2 {
		t.Errorf("len(List()) = %d, want 2", len(users))
	}
}

func TestUserService_Update(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	created, _ := svc.Create(ctx, validInput())

	t.Run("name only", func(t *testing.T) {
		got, err := svc.Update(ctx, created.ID, usecase.UpdateUserInput{Name: strPtr(" Alicia ")})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if got.Name != "Alicia" || got.Email != created.Email {
			t.Errorf("Update() = %+v, want name changed and email kept", got)
		}
	})

	t.Run("email only", func(t *testing.T) {
		got, err := svc.Update(ctx, created.ID, usecase.UpdateUserInput{Email: strPtr("NEW@example.com")})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if got.Email != "new@example.com" || got.Name != "Alicia" {
			t.Errorf("Update() = %+v, want email changed and name kept", got)
		}
	})

	t.Run("no fields", func(t *testing.T) {
		_, err := svc.Update(ctx, created.ID, usecase.UpdateUserInput{})
		var verr *model.ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("error = %v, want ValidationError", err)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		_, err := svc.Update(ctx, created.ID, usecase.UpdateUserInput{Email: strPtr("nope")})
		var verr *model.ValidationError
		if !errors.As(err, &verr) || verr.Fields["email"] == "" {
			t.Fatalf("error = %v, want ValidationError on email", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := svc.Update(ctx, created.ID, usecase.UpdateUserInput{Name: strPtr("  ")})
		var verr *model.ValidationError
		if !errors.As(err, &verr) || verr.Fields["name"] == "" {
			t.Fatalf("error = %v, want ValidationError on name", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.Update(ctx, "missing", usecase.UpdateUserInput{Name: strPtr("X")})
		if !errors.Is(err, model.ErrNotFound) {
			t.Fatalf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("email taken", func(t *testing.T) {
		in := validInput()
		in.Email = "taken@example.com"
		_, _ = svc.Create(ctx, in)
		_, err := svc.Update(ctx, created.ID, usecase.UpdateUserInput{Email: strPtr("taken@example.com")})
		if !errors.Is(err, model.ErrEmailAlreadyExists) {
			t.Fatalf("error = %v, want ErrEmailAlreadyExists", err)
		}
	})
}

func TestUserService_Delete(t *testing.T) {
	svc, _ := newUserService(t)
	ctx := context.Background()
	created, _ := svc.Create(ctx, validInput())

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := svc.Get(ctx, created.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("Get() after delete error = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(ctx, created.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second Delete() error = %v, want ErrNotFound", err)
	}
}

func TestUserService_RepositoryError(t *testing.T) {
	svc, repo := newUserService(t)
	repoErr := errors.New("database down")
	repo.Err = repoErr

	if _, err := svc.List(context.Background()); !errors.Is(err, repoErr) {
		t.Errorf("List() error = %v, want repository error", err)
	}
	if _, err := svc.Create(context.Background(), validInput()); !errors.Is(err, repoErr) {
		t.Errorf("Create() error = %v, want repository error", err)
	}
}
