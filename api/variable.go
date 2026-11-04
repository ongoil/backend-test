package api

import (
	"time"

	"github.com/laddawan/backend-test/model"
)

type bodyCreateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type bodyUpdateUserRequest struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

type bodyLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type responseUser struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type responseListUsers struct {
	Users []responseUser `json:"users"`
	Total int            `json:"total"`
}

type responseToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// toResponseUser never exposes the password hash.
func toResponseUser(u *model.User) responseUser {
	return responseUser{ID: u.ID, Name: u.Name, Email: u.Email, CreatedAt: u.CreatedAt}
}
