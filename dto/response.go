// Package dto defines the JSON response shapes and maps errors to HTTP status
// codes. It is shared by the api and middleware packages so every response
// has the same shape.
package dto

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/laddawan/backend-test/model"
)

// MaxBodyBytes is the request body limit (1 MiB), enforced by fiber.Config.BodyLimit.
const MaxBodyBytes = 1 << 20

// ErrorBody is the JSON shape of every error response.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes an error. Fields lists per-field validation problems.
type ErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// JSON writes v as JSON with the given status.
func JSON(c fiber.Ctx, status int, v any) error {
	return c.Status(status).JSON(v)
}

// Error writes an ErrorBody.
func Error(c fiber.Ctx, status int, code, message string, fields map[string]string) error {
	return JSON(c, status, ErrorBody{Error: ErrorDetail{Code: code, Message: message, Fields: fields}})
}

// Unauthorized writes a 401 with a WWW-Authenticate challenge.
func Unauthorized(c fiber.Ctx, message string) error {
	c.Set(fiber.HeaderWWWAuthenticate, `Bearer realm="api"`)
	return Error(c, fiber.StatusUnauthorized, "unauthorized", message, nil)
}

// InvalidJSON writes a 400 for a request body that could not be decoded.
func InvalidJSON(c fiber.Ctx, err error) error {
	return Error(c, fiber.StatusBadRequest, "invalid_json", err.Error(), nil)
}

// InternalError logs err and writes a generic 500, so internal details
// never reach the client.
func InternalError(c fiber.Ctx, logger *slog.Logger, err error) error {
	logger.ErrorContext(c.Context(), "internal error",
		"method", c.Method(), "path", c.Path(), "error", err)
	return Error(c, fiber.StatusInternalServerError, "internal_error", "internal server error", nil)
}

// FromError maps domain errors to HTTP responses.
func FromError(c fiber.Ctx, logger *slog.Logger, err error) error {
	var verr *model.ValidationError
	switch {
	case errors.As(err, &verr):
		return Error(c, fiber.StatusBadRequest, "validation_failed", "request validation failed", verr.Fields)
	case errors.Is(err, model.ErrNotFound):
		return Error(c, fiber.StatusNotFound, "not_found", err.Error(), nil)
	case errors.Is(err, model.ErrEmailAlreadyExists):
		return Error(c, fiber.StatusConflict, "email_already_exists", err.Error(), nil)
	case errors.Is(err, model.ErrInvalidCredentials):
		return Error(c, fiber.StatusUnauthorized, "invalid_credentials", err.Error(), nil)
	case errors.Is(err, model.ErrInvalidToken):
		return Unauthorized(c, model.ErrInvalidToken.Error())
	default:
		return InternalError(c, logger, err)
	}
}

// ErrorHandler handles errors returned from the Fiber stack itself (unknown
// routes, wrong methods, oversized bodies). Handlers write their own
// responses, so most requests never reach here.
func ErrorHandler(logger *slog.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		var fe *fiber.Error
		if errors.As(err, &fe) {
			switch fe.Code {
			case fiber.StatusNotFound:
				return Error(c, fe.Code, "route_not_found", "route not found", nil)
			case fiber.StatusMethodNotAllowed:
				return Error(c, fe.Code, "method_not_allowed", "method not allowed", nil)
			case fiber.StatusRequestEntityTooLarge:
				return Error(c, fe.Code, "payload_too_large",
					fmt.Sprintf("request body must not exceed %d bytes", MaxBodyBytes), nil)
			}
			if fe.Code < fiber.StatusInternalServerError {
				return Error(c, fe.Code, "http_error", fe.Message, nil)
			}
		}
		return InternalError(c, logger, err)
	}
}
