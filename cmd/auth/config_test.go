package main

import (
	"strings"
	"testing"
	"time"

	"github.com/osamah22/payments-microservice/internal/platform/config"
)

func envFrom(vars map[string]string) *config.Env {
	return config.New(func(key string) (string, bool) {
		v, ok := vars[key]
		return v, ok
	})
}

func TestLoadConfigDefaults(t *testing.T) {
	c, err := loadConfig(envFrom(map[string]string{
		"AUTH_DATABASE_URL":     "postgres://localhost/auth",
		"AUTH_PRIVATE_KEY_PATH": "secrets/auth_ed25519.pem",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8081" || c.AccessTokenTTL != 15*time.Minute || c.RefreshTokenTTL != 30*24*time.Hour {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

func TestLoadConfigReportsEveryProblem(t *testing.T) {
	_, err := loadConfig(envFrom(map[string]string{
		"AUTH_ACCESS_TOKEN_TTL":  "2h",
		"AUTH_REFRESH_TOKEN_TTL": "1h",
	}))
	if err == nil {
		t.Fatal("want an error, got nil")
	}
	for _, key := range []string{
		"AUTH_DATABASE_URL", "AUTH_PRIVATE_KEY_PATH", "AUTH_ACCESS_TOKEN_TTL", "AUTH_REFRESH_TOKEN_TTL",
	} {
		if !strings.Contains(err.Error(), key+":") {
			t.Errorf("error doesn't mention %s:\n%v", key, err)
		}
	}
}
