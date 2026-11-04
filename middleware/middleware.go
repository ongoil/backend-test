// Package middleware provides the Fiber middleware: request logging, panic
// recovery and JWT authentication.
package middleware

import (
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/laddawan/backend-test/dto"
)

// TokenVerifier validates an access token and returns its user ID
// (implemented by security.JWTManager).
type TokenVerifier interface {
	Verify(token string) (userID string, err error)
}

type userIDKey struct{}

// UserIDFromContext returns the authenticated user's ID set by Authenticate.
func UserIDFromContext(c fiber.Ctx) (string, bool) {
	id, ok := c.Locals(userIDKey{}).(string)
	return id, ok
}

// Logging logs the method, path, status and execution time of each request.
//
// Errors returned further down the chain are passed to the app's error
// handler here, so the logged status matches what the client receives.
func Logging(logger *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		if err := c.Next(); err != nil {
			if herr := c.App().Config().ErrorHandler(c, err); herr != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		logger.InfoContext(c.Context(), "http request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"bytes", len(c.Response().Body()),
			"duration", time.Since(start).String(),
		)
		return nil
	}
}

// Recover turns panics into a 500 response and logs the stack trace.
func Recover(logger *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) (err error) {
		defer func() {
			if v := recover(); v != nil {
				logger.ErrorContext(c.Context(), "panic recovered",
					"panic", v, "stack", string(debug.Stack()))
				err = dto.Error(c, fiber.StatusInternalServerError, "internal_error", "internal server error", nil)
			}
		}()
		return c.Next()
	}
}

// Authenticate requires a valid "Authorization: Bearer <token>" header and
// stores the token's user ID in the request locals.
func Authenticate(tokens TokenVerifier) fiber.Handler {
	return func(c fiber.Ctx) error {
		token, ok := bearerToken(c.Get(fiber.HeaderAuthorization))
		if !ok {
			return dto.Unauthorized(c, "missing or malformed Authorization header")
		}
		userID, err := tokens.Verify(token)
		if err != nil {
			return dto.Unauthorized(c, "invalid or expired token")
		}
		c.Locals(userIDKey{}, userID)
		return c.Next()
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>"
// header value. The scheme is case-insensitive (RFC 7235). The token is
// copied because Fiber reuses header buffers between requests.
func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.Clone(strings.TrimSpace(token))
	return token, token != ""
}
