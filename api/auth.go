// Package api contains the Fiber handlers for the REST API. Handlers decode
// requests, call the usecases and write responses.
package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/laddawan/backend-test/dto"
	"github.com/laddawan/backend-test/usecase"
)

type AuthHandler struct {
	authUsecase *usecase.AuthUsecase
	logger      *slog.Logger
}

func NewAuthHandler(authUsecase *usecase.AuthUsecase, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authUsecase: authUsecase,
		logger:      logger,
	}
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req bodyCreateUserRequest
	if err := decodeJSON(c, &req); err != nil {
		return dto.InvalidJSON(c, err)
	}
	user, err := h.authUsecase.Register(c.Context(), usecase.CreateUserInput(req))
	if err != nil {
		return dto.FromError(c, h.logger, err)
	}
	return dto.JSON(c, fiber.StatusCreated, toResponseUser(user))
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req bodyLoginRequest
	if err := decodeJSON(c, &req); err != nil {
		return dto.InvalidJSON(c, err)
	}
	token, err := h.authUsecase.Login(c.Context(), usecase.LoginInput(req))
	if err != nil {
		return dto.FromError(c, h.logger, err)
	}
	return dto.JSON(c, fiber.StatusOK, responseToken{
		AccessToken: token.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   token.ExpiresAt,
	})
}
