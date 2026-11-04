package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/laddawan/backend-test/model"
)

// MinSecretLength is the minimum HMAC secret length in bytes. HS256 uses a
// 256-bit hash, so the key should be at least 32 bytes.
const MinSecretLength = 32

// JWTManager issues and verifies HS256-signed JWTs.
type JWTManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

// NewJWTManager creates a JWTManager. The secret must be at least
// MinSecretLength bytes and ttl must be positive.
func NewJWTManager(secret []byte, issuer string, ttl time.Duration) (*JWTManager, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("jwt secret must be at least %d bytes", MinSecretLength)
	}
	if ttl <= 0 {
		return nil, errors.New("jwt ttl must be positive")
	}
	return &JWTManager{secret: secret, issuer: issuer, ttl: ttl, now: time.Now}, nil
}

// Issue creates a token whose subject ("sub") is userID.
func (m *JWTManager) Issue(userID string) (string, time.Time, error) {
	now := m.now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    m.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, claims.ExpiresAt.Time, nil
}

// Verify checks the signature, algorithm, issuer and time claims, and
// returns the token's subject.
func (m *JWTManager) Verify(token string) (string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		// Only accept HS256: rejects "none" and algorithm-confusion attacks.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return "", fmt.Errorf("%w: %v", model.ErrInvalidToken, err)
	}
	if claims.Subject == "" {
		return "", fmt.Errorf("%w: missing subject", model.ErrInvalidToken)
	}
	return claims.Subject, nil
}
