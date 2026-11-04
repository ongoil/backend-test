package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/laddawan/backend-test/dto"
	"github.com/laddawan/backend-test/usecase"
)

type UserHandler struct {
	userUsecase *usecase.UserUsecase
	logger      *slog.Logger
}

func NewUserHandler(userUsecase *usecase.UserUsecase, logger *slog.Logger) *UserHandler {
	return &UserHandler{
		userUsecase: userUsecase,
		logger:      logger,
	}
}

// Health handles GET /healthz.
func Health(c fiber.Ctx) error {
	return dto.JSON(c, fiber.StatusOK, fiber.Map{"status": "ok"})
}

// Create handles POST /api/v1/users.
func (h *UserHandler) Create(c fiber.Ctx) error {
	var req bodyCreateUserRequest
	if err := decodeJSON(c, &req); err != nil {
		return dto.InvalidJSON(c, err)
	}
	user, err := h.userUsecase.Create(c.Context(), usecase.CreateUserInput(req))
	if err != nil {
		return dto.FromError(c, h.logger, err)
	}
	c.Set(fiber.HeaderLocation, "/api/v1/users/"+user.ID)
	return dto.JSON(c, fiber.StatusCreated, toResponseUser(user))
}

// List handles GET /api/v1/users.
func (h *UserHandler) List(c fiber.Ctx) error {
	users, err := h.userUsecase.List(c.Context())
	if err != nil {
		return dto.FromError(c, h.logger, err)
	}
	resp := responseListUsers{Users: make([]responseUser, 0, len(users)), Total: len(users)}
	for _, u := range users {
		resp.Users = append(resp.Users, toResponseUser(u))
	}
	return dto.JSON(c, fiber.StatusOK, resp)
}

// Get handles GET /api/v1/users/:id.
func (h *UserHandler) Get(c fiber.Ctx) error {
	user, err := h.userUsecase.Get(c.Context(), pathID(c))
	if err != nil {
		return dto.FromError(c, h.logger, err)
	}
	return dto.JSON(c, fiber.StatusOK, toResponseUser(user))
}

// Update handles PATCH /api/v1/users/:id.
func (h *UserHandler) Update(c fiber.Ctx) error {
	var req bodyUpdateUserRequest
	if err := decodeJSON(c, &req); err != nil {
		return dto.InvalidJSON(c, err)
	}
	user, err := h.userUsecase.Update(c.Context(), pathID(c), usecase.UpdateUserInput(req))
	if err != nil {
		return dto.FromError(c, h.logger, err)
	}
	return dto.JSON(c, fiber.StatusOK, toResponseUser(user))
}

// Delete handles DELETE /api/v1/users/:id.
func (h *UserHandler) Delete(c fiber.Ctx) error {
	if err := h.userUsecase.Delete(c.Context(), pathID(c)); err != nil {
		return dto.FromError(c, h.logger, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// pathID returns the :id path parameter. Fiber reuses request buffers, so
// the value is copied before it leaves the handler.
func pathID(c fiber.Ctx) string {
	return strings.Clone(c.Params("id"))
}

// decodeJSON reads a single JSON object from the request body into dst,
// rejecting empty bodies, unknown fields and trailing data. The returned
// error message is safe to send to the client.
func decodeJSON(c fiber.Ctx, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.Is(err, io.EOF):
			return errors.New("request body must not be empty")
		case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("request body contains malformed JSON")
		case errors.As(err, &typeErr):
			return fmt.Errorf("field %q has the wrong type", typeErr.Field)
		default:
			return err // json: unknown field "x"
		}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
