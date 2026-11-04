package security

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/laddawan/backend-test/model"
)

var testSecret = []byte("test-secret-that-is-at-least-32-bytes-long")

func TestBcryptHasher(t *testing.T) {
	h := NewBcryptHasher(bcrypt.MinCost) // keep tests fast

	hash, err := h.Hash("s3cretpass")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if hash == "s3cretpass" || !strings.HasPrefix(hash, "$2") {
		t.Fatalf("Hash() = %q, want a bcrypt hash", hash)
	}
	if err := h.Compare(hash, "s3cretpass"); err != nil {
		t.Errorf("Compare(correct) error = %v", err)
	}
	if err := h.Compare(hash, "wrong"); !errors.Is(err, model.ErrInvalidCredentials) {
		t.Errorf("Compare(wrong) error = %v, want ErrInvalidCredentials", err)
	}
}

func TestNewJWTManager_Validation(t *testing.T) {
	if _, err := NewJWTManager([]byte("short"), "iss", time.Hour); err == nil {
		t.Error("expected error for short secret")
	}
	if _, err := NewJWTManager(testSecret, "iss", 0); err == nil {
		t.Error("expected error for zero ttl")
	}
}

func TestJWTManager_IssueAndVerify(t *testing.T) {
	m, err := NewJWTManager(testSecret, "backend-test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	token, expiresAt, err := m.Issue("user-123")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if until := time.Until(expiresAt); until < 59*time.Minute || until > time.Hour+time.Second {
		t.Errorf("expiresAt in %v, want ~1h", until)
	}

	userID, err := m.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if userID != "user-123" {
		t.Errorf("Verify() = %q, want %q", userID, "user-123")
	}

	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Method.Alg() != "HS256" {
		t.Errorf("alg = %q, want HS256", parsed.Method.Alg())
	}
}

func TestJWTManager_Verify_Rejects(t *testing.T) {
	m, _ := NewJWTManager(testSecret, "backend-test", time.Hour)
	valid, _, _ := m.Issue("user-123")

	expiredMgr, _ := NewJWTManager(testSecret, "backend-test", time.Hour)
	expiredMgr.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	expired, _, _ := expiredMgr.Issue("user-123")

	otherSecret, _ := NewJWTManager([]byte("another-secret-that-is-32-bytes-long!!"), "backend-test", time.Hour)
	wrongSig, _, _ := otherSecret.Issue("user-123")

	otherIssuer, _ := NewJWTManager(testSecret, "someone-else", time.Hour)
	wrongIss, _, _ := otherIssuer.Issue("user-123")

	claims := jwt.RegisteredClaims{
		Subject:   "user-123",
		Issuer:    "backend-test",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(testSecret)
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)

	noExp, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: "user-123", Issuer: "backend-test",
	}).SignedString(testSecret)

	noSub, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "backend-test", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString(testSecret)

	tests := map[string]string{
		"empty":           "",
		"garbage":         "not.a.jwt",
		"expired":         expired,
		"wrong signature": wrongSig,
		"wrong issuer":    wrongIss,
		"HS512 algorithm": hs512,
		"none algorithm":  none,
		"missing exp":     noExp,
		"missing sub":     noSub,
		"tampered":        valid[:len(valid)-2] + "xx",
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := m.Verify(token); !errors.Is(err, model.ErrInvalidToken) {
				t.Errorf("Verify() error = %v, want ErrInvalidToken", err)
			}
		})
	}
}
