package config

import (
	"strings"
	"testing"
	"time"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

const secret = "0123456789abcdef0123456789abcdef"

func TestLoad_Defaults(t *testing.T) {
	cfg, err := load(envMap(map[string]string{"JWT_SECRET": secret}))
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.GRPCAddr != ":50051" {
		t.Errorf("addrs = %q %q", cfg.HTTPAddr, cfg.GRPCAddr)
	}
	if cfg.UserCountInterval != 10*time.Second {
		t.Errorf("UserCountInterval = %v, want 10s", cfg.UserCountInterval)
	}
	if cfg.JWTTTL != 24*time.Hour || cfg.BcryptCost != 12 {
		t.Errorf("JWTTTL = %v, BcryptCost = %d", cfg.JWTTTL, cfg.BcryptCost)
	}
}

func TestLoad_Overrides(t *testing.T) {
	cfg, err := load(envMap(map[string]string{
		"JWT_SECRET": secret, "HTTP_PORT": "9000", "JWT_TTL": "15m", "MONGO_URI": "mongodb://db:27017",
	}))
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if cfg.HTTPAddr != ":9000" || cfg.JWTTTL != 15*time.Minute || cfg.MongoURI != "mongodb://db:27017" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoad_Errors(t *testing.T) {
	tests := map[string]struct {
		env  map[string]string
		want string
	}{
		"missing secret": {map[string]string{}, "JWT_SECRET"},
		"short secret":   {map[string]string{"JWT_SECRET": "short"}, "JWT_SECRET"},
		"bad duration":   {map[string]string{"JWT_SECRET": secret, "JWT_TTL": "forever"}, "JWT_TTL"},
		"bad cost":       {map[string]string{"JWT_SECRET": secret, "BCRYPT_COST": "99"}, "BCRYPT_COST"},
		"bad log format": {map[string]string{"JWT_SECRET": secret, "LOG_FORMAT": "xml"}, "LOG_FORMAT"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := load(envMap(tt.env))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("load() error = %v, want mention of %s", err, tt.want)
			}
		})
	}
}
