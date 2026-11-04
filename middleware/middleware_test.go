package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/laddawan/backend-test/dto"
	"github.com/laddawan/backend-test/mock"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLogging_RecordsMethodPathStatusAndDuration(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	app := fiber.New()
	app.Use(Logging(logger))
	app.Post("/some/path", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusTeapot).SendString("hi")
	})

	if _, err := app.Test(httptest.NewRequest("POST", "/some/path?q=1", nil)); err != nil {
		t.Fatal(err)
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log is not JSON: %q", buf.String())
	}
	if entry["method"] != "POST" || entry["path"] != "/some/path" {
		t.Errorf("method/path = %v %v", entry["method"], entry["path"])
	}
	if entry["status"] != float64(http.StatusTeapot) || entry["bytes"] != float64(2) {
		t.Errorf("status/bytes = %v %v", entry["status"], entry["bytes"])
	}
	if d, _ := entry["duration"].(string); d == "" {
		t.Error("duration not logged")
	}
}

func TestLogging_LogsStatusFromErrorHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	app := fiber.New(fiber.Config{ErrorHandler: dto.ErrorHandler(discardLogger())})
	app.Use(Logging(logger))

	resp, err := app.Test(httptest.NewRequest("GET", "/missing", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}

	var entry map[string]any
	_ = json.Unmarshal(buf.Bytes(), &entry)
	if entry["status"] != float64(http.StatusNotFound) {
		t.Errorf("logged status = %v, want 404", entry["status"])
	}
}

func TestRecover(t *testing.T) {
	app := fiber.New()
	app.Use(Recover(discardLogger()))
	app.Get("/", func(fiber.Ctx) error { panic("boom") })

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestAuthenticate(t *testing.T) {
	app := fiber.New()
	app.Use(Authenticate(&mock.TokenManager{}))
	app.Get("/", func(c fiber.Ctx) error {
		id, _ := UserIDFromContext(c)
		return c.SendString(id)
	})

	tests := []struct {
		name, header string
		wantStatus   int
		wantUserID   string
	}{
		{"valid", "Bearer token-u1", http.StatusOK, "u1"},
		{"lowercase scheme", "bearer token-u2", http.StatusOK, "u2"},
		{"missing header", "", http.StatusUnauthorized, ""},
		{"wrong scheme", "Basic dXNlcjpwYXNz", http.StatusUnauthorized, ""},
		{"no token", "Bearer ", http.StatusUnauthorized, ""},
		{"invalid token", "Bearer garbage", http.StatusUnauthorized, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK && string(body) != tt.wantUserID {
				t.Errorf("user ID in locals = %q, want %q", body, tt.wantUserID)
			}
		})
	}
}
