package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/laddawan/backend-test/api"
	"github.com/laddawan/backend-test/mock"
	"github.com/laddawan/backend-test/router"
	"github.com/laddawan/backend-test/security"
	"github.com/laddawan/backend-test/usecase"
)

// syncBuffer is a bytes.Buffer safe for concurrent use: the server writes
// logs from its goroutines while the test reads them.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type testServer struct {
	t      *testing.T
	app    *fiber.App
	repo   *mock.UserRepository
	tokens *security.JWTManager
	logs   *syncBuffer
}

// newTestServer wires the real router, handlers, services and JWT manager
// on top of the in-memory repository mock.
func newTestServer(t *testing.T) *testServer {
	t.Helper()
	logs := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))

	repo := mock.NewUserRepository()
	hasher := &mock.PasswordHasher{}
	v := usecase.NewValidator()
	tokens, err := security.NewJWTManager([]byte("test-secret-that-is-at-least-32-bytes-long"), "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	users := usecase.NewUserUsecase(repo, hasher, v)
	auth, err := usecase.NewAuthUsecase(users, repo, hasher, tokens, v)
	if err != nil {
		t.Fatal(err)
	}

	app := router.NewApp(logger)
	router.SetRouter(app, api.NewAuthHandler(auth, logger), api.NewUserHandler(users, logger), tokens)
	return &testServer{t: t, app: app, repo: repo, tokens: tokens, logs: logs}
}

func (s *testServer) do(method, path, token, body string) (*http.Response, map[string]any) {
	s.t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		s.t.Fatal(err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			s.t.Fatalf("response is not a JSON object: %s", raw)
		}
	}
	return resp, out
}

// registerAndLogin creates a user and returns a valid token for it.
func (s *testServer) registerAndLogin() string {
	s.t.Helper()
	resp, _ := s.do("POST", "/api/v1/auth/register", "",
		`{"name":"Admin","email":"admin@example.com","password":"adminpass1"}`)
	if resp.StatusCode != http.StatusCreated {
		s.t.Fatalf("register status = %d", resp.StatusCode)
	}
	resp, body := s.do("POST", "/api/v1/auth/login", "",
		`{"email":"admin@example.com","password":"adminpass1"}`)
	if resp.StatusCode != http.StatusOK {
		s.t.Fatalf("login status = %d, body = %v", resp.StatusCode, body)
	}
	return body["access_token"].(string)
}

func errorCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func TestRegister(t *testing.T) {
	s := newTestServer(t)

	resp, body := s.do("POST", "/api/v1/auth/register", "",
		`{"name":"Alice","email":"alice@example.com","password":"s3cretpass"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %v", resp.StatusCode, body)
	}
	if body["id"] == "" || body["email"] != "alice@example.com" || body["name"] != "Alice" {
		t.Errorf("unexpected body: %v", body)
	}
	if _, ok := body["password"]; ok {
		t.Error("response must not contain password")
	}
	if _, ok := body["password_hash"]; ok {
		t.Error("response must not contain password_hash")
	}

	resp, body = s.do("POST", "/api/v1/auth/register", "",
		`{"name":"Alice","email":"alice@example.com","password":"s3cretpass"}`)
	if resp.StatusCode != http.StatusConflict || errorCode(body) != "email_already_exists" {
		t.Errorf("duplicate register = %d %v, want 409 email_already_exists", resp.StatusCode, body)
	}
}

func TestRegister_BadRequests(t *testing.T) {
	s := newTestServer(t)
	tests := []struct {
		name, body, wantCode string
	}{
		{"validation", `{"name":"","email":"nope","password":"x"}`, "validation_failed"},
		{"malformed json", `{"name":`, "invalid_json"},
		{"empty body", ``, "invalid_json"},
		{"unknown field", `{"name":"A","email":"a@b.co","password":"password1","role":"admin"}`, "invalid_json"},
		{"wrong type", `{"name":123,"email":"a@b.co","password":"password1"}`, "invalid_json"},
		{"trailing data", `{"name":"A","email":"a@b.co","password":"password1"} {}`, "invalid_json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := s.do("POST", "/api/v1/auth/register", "", tt.body)
			if resp.StatusCode != http.StatusBadRequest || errorCode(body) != tt.wantCode {
				t.Errorf("got %d %v, want 400 %s", resp.StatusCode, body, tt.wantCode)
			}
		})
	}

	_, body := s.do("POST", "/api/v1/auth/register", "", `{"name":"","email":"nope","password":"x"}`)
	fields := body["error"].(map[string]any)["fields"].(map[string]any)
	for _, f := range []string{"name", "email", "password"} {
		if _, ok := fields[f]; !ok {
			t.Errorf("validation fields %v missing %q", fields, f)
		}
	}
}

func TestLogin(t *testing.T) {
	s := newTestServer(t)
	token := s.registerAndLogin()

	userID, err := s.tokens.Verify(token)
	if err != nil || userID == "" {
		t.Fatalf("login returned an invalid token: %v", err)
	}

	resp, body := s.do("POST", "/api/v1/auth/login", "", `{"email":"admin@example.com","password":"wrongpass"}`)
	if resp.StatusCode != http.StatusUnauthorized || errorCode(body) != "invalid_credentials" {
		t.Errorf("wrong password = %d %v, want 401 invalid_credentials", resp.StatusCode, body)
	}
	resp, body = s.do("POST", "/api/v1/auth/login", "", `{"email":"ghost@example.com","password":"whatever1"}`)
	if resp.StatusCode != http.StatusUnauthorized || errorCode(body) != "invalid_credentials" {
		t.Errorf("unknown email = %d %v, want 401 invalid_credentials", resp.StatusCode, body)
	}
}

func TestProtectedRoutes_RequireToken(t *testing.T) {
	s := newTestServer(t)
	routes := []struct{ method, path string }{
		{"GET", "/api/v1/users"},
		{"POST", "/api/v1/users"},
		{"GET", "/api/v1/users/abc"},
		{"PATCH", "/api/v1/users/abc"},
		{"DELETE", "/api/v1/users/abc"},
	}
	for _, rt := range routes {
		for _, token := range []string{"", "invalid.token.value"} {
			resp, body := s.do(rt.method, rt.path, token, "")
			if resp.StatusCode != http.StatusUnauthorized || errorCode(body) != "unauthorized" {
				t.Errorf("%s %s token=%q = %d %v, want 401", rt.method, rt.path, token, resp.StatusCode, body)
			}
			if resp.Header.Get("WWW-Authenticate") == "" {
				t.Errorf("%s %s: missing WWW-Authenticate header", rt.method, rt.path)
			}
		}
	}
}

func TestUserCRUD(t *testing.T) {
	s := newTestServer(t)
	token := s.registerAndLogin()

	// Create
	resp, created := s.do("POST", "/api/v1/users", token,
		`{"name":"Bob","email":"bob@example.com","password":"bobpass123"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d %v", resp.StatusCode, created)
	}
	id := created["id"].(string)
	if loc := resp.Header.Get("Location"); loc != "/api/v1/users/"+id {
		t.Errorf("Location = %q", loc)
	}

	// Get
	resp, got := s.do("GET", "/api/v1/users/"+id, token, "")
	if resp.StatusCode != http.StatusOK || got["email"] != "bob@example.com" {
		t.Fatalf("get = %d %v", resp.StatusCode, got)
	}

	// List
	resp, list := s.do("GET", "/api/v1/users", token, "")
	if resp.StatusCode != http.StatusOK || list["total"] != float64(2) {
		t.Fatalf("list = %d %v, want 2 users", resp.StatusCode, list)
	}

	// Update (partial)
	resp, updated := s.do("PATCH", "/api/v1/users/"+id, token, `{"name":"Robert"}`)
	if resp.StatusCode != http.StatusOK || updated["name"] != "Robert" || updated["email"] != "bob@example.com" {
		t.Fatalf("update = %d %v", resp.StatusCode, updated)
	}

	// Update cannot change the password
	resp, body := s.do("PATCH", "/api/v1/users/"+id, token, `{"password":"newpass123"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("update password = %d %v, want 400", resp.StatusCode, body)
	}

	// Update to an email that is taken
	resp, body = s.do("PATCH", "/api/v1/users/"+id, token, `{"email":"admin@example.com"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("update taken email = %d %v, want 409", resp.StatusCode, body)
	}

	// Delete
	resp, _ = s.do("DELETE", "/api/v1/users/"+id, token, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", resp.StatusCode)
	}
	resp, body = s.do("GET", "/api/v1/users/"+id, token, "")
	if resp.StatusCode != http.StatusNotFound || errorCode(body) != "not_found" {
		t.Errorf("get deleted = %d %v, want 404", resp.StatusCode, body)
	}
	resp, _ = s.do("DELETE", "/api/v1/users/"+id, token, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("delete again = %d, want 404", resp.StatusCode)
	}
}

func TestInternalErrorIsHidden(t *testing.T) {
	s := newTestServer(t)
	token := s.registerAndLogin()
	s.repo.Err = errors.New("secret database detail")

	resp, body := s.do("GET", "/api/v1/users", token, "")
	if resp.StatusCode != http.StatusInternalServerError || errorCode(body) != "internal_error" {
		t.Fatalf("got %d %v, want 500 internal_error", resp.StatusCode, body)
	}
	if strings.Contains(body["error"].(map[string]any)["message"].(string), "secret") {
		t.Error("internal error details leaked to client")
	}
	if !strings.Contains(s.logs.String(), "secret database detail") {
		t.Error("internal error was not logged")
	}
}

func TestHealthAndUnknownRoute(t *testing.T) {
	s := newTestServer(t)
	resp, body := s.do("GET", "/healthz", "", "")
	if resp.StatusCode != http.StatusOK || body["status"] != "ok" {
		t.Errorf("healthz = %d %v", resp.StatusCode, body)
	}
	resp, body = s.do("GET", "/nope", "", "")
	if resp.StatusCode != http.StatusNotFound || errorCode(body) != "route_not_found" {
		t.Errorf("unknown route = %d %v", resp.StatusCode, body)
	}
	resp, body = s.do("PUT", "/api/v1/auth/login", "", "")
	if resp.StatusCode != http.StatusMethodNotAllowed || errorCode(body) != "method_not_allowed" {
		t.Errorf("wrong method = %d %v", resp.StatusCode, body)
	}
}
