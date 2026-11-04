// Package router creates the Fiber app and registers every REST route.
// All route paths live in this one file.
package router

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/laddawan/backend-test/api"
	"github.com/laddawan/backend-test/dto"
	"github.com/laddawan/backend-test/middleware"
)

// NewApp creates the Fiber app with its config and global middleware.
func NewApp(logger *slog.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "user-management-api",
		ErrorHandler: dto.ErrorHandler(logger),
		BodyLimit:    dto.MaxBodyBytes,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	})

	// Logging is outermost so panics are logged with their 500 status.
	app.Use(middleware.Logging(logger))
	app.Use(middleware.Recover(logger))

	return app
}

// SetRouter registers all routes on app.
func SetRouter(app *fiber.App, authHandler *api.AuthHandler, userHandler *api.UserHandler, tokens middleware.TokenVerifier) {
	requireJWT := middleware.Authenticate(tokens)

	// Public
	app.Get("/healthz", api.Health)
	app.Post("/api/v1/auth/register", authHandler.Register)
	app.Post("/api/v1/auth/login", authHandler.Login)

	// Protected by JWT
	app.Post("/api/v1/users", requireJWT, userHandler.Create)
	app.Get("/api/v1/users", requireJWT, userHandler.List)
	app.Get("/api/v1/users/:id", requireJWT, userHandler.Get)
	app.Patch("/api/v1/users/:id", requireJWT, userHandler.Update)
	app.Delete("/api/v1/users/:id", requireJWT, userHandler.Delete)
}
