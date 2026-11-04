// Package config loads application settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// MinJWTSecretLength mirrors security.MinSecretLength so misconfiguration
// is reported at startup with a clear message.
const MinJWTSecretLength = 32

// Config holds all runtime settings.
type Config struct {
	HTTPAddr          string
	GRPCAddr          string
	MongoURI          string
	MongoDatabase     string
	JWTSecret         []byte
	JWTIssuer         string
	JWTTTL            time.Duration
	BcryptCost        int
	UserCountInterval time.Duration
	ShutdownTimeout   time.Duration
	LogFormat         string // "text" or "json"
}

// Load reads the configuration from the environment, applying defaults.
// JWT_SECRET is required.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		HTTPAddr:      ":" + get("HTTP_PORT", "8080"),
		GRPCAddr:      ":" + get("GRPC_PORT", "50051"),
		MongoURI:      get("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase: get("MONGO_DB", "user_management"),
		JWTSecret:     []byte(getenv("JWT_SECRET")),
		JWTIssuer:     get("JWT_ISSUER", "backend-test"),
		LogFormat:     get("LOG_FORMAT", "text"),
	}

	var errs []error
	if len(cfg.JWTSecret) < MinJWTSecretLength {
		errs = append(errs, fmt.Errorf("JWT_SECRET must be set and at least %d bytes", MinJWTSecretLength))
	}
	if cfg.LogFormat != "text" && cfg.LogFormat != "json" {
		errs = append(errs, fmt.Errorf("LOG_FORMAT must be \"text\" or \"json\", got %q", cfg.LogFormat))
	}

	durations := []struct {
		key, def string
		dst      *time.Duration
	}{
		{"JWT_TTL", "24h", &cfg.JWTTTL},
		{"USER_COUNT_INTERVAL", "10s", &cfg.UserCountInterval},
		{"SHUTDOWN_TIMEOUT", "15s", &cfg.ShutdownTimeout},
	}
	for _, d := range durations {
		v, err := time.ParseDuration(get(d.key, d.def))
		if err != nil || v <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive duration (e.g. 10s, 24h)", d.key))
			continue
		}
		*d.dst = v
	}

	cost, err := strconv.Atoi(get("BCRYPT_COST", "12"))
	if err != nil || cost < 4 || cost > 31 {
		errs = append(errs, errors.New("BCRYPT_COST must be an integer between 4 and 31"))
	}
	cfg.BcryptCost = cost

	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
