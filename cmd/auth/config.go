package main

import (
	"log/slog"
	"time"

	"github.com/osamah22/payments-microservice/internal/platform/config"
	"github.com/osamah22/payments-microservice/internal/platform/logging"
)

// Config is everything the auth service reads from its environment.
type Config struct {
	Env             string // development or production
	HTTPAddr        string
	ShutdownTimeout time.Duration
	LogLevel        slog.Level
	LogFormat       string
	DatabaseURL     string
	DBMaxConns      int
	// PrivateKeyPath is a PEM file with the Ed25519 key that signs access
	// tokens. make keys creates one for development.
	PrivateKeyPath  string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// loadConfig reads the configuration. Everything has a sensible default except
// secrets, and every missing or invalid variable is reported at once.
func loadConfig(env *config.Env) (Config, error) {
	c := Config{
		Env:             env.OneOf("APP_ENV", "development", "development", "production"),
		HTTPAddr:        env.String("AUTH_HTTP_ADDR", ":8081"),
		ShutdownTimeout: env.Duration("AUTH_SHUTDOWN_TIMEOUT", 10*time.Second),
		LogLevel:        config.Parse(env, "LOG_LEVEL", slog.LevelInfo, logging.ParseLevel),
		DatabaseURL:     env.Required("AUTH_DATABASE_URL"),
		DBMaxConns:      env.Int("AUTH_DB_MAX_CONNS", 10),
		PrivateKeyPath:  env.Required("AUTH_PRIVATE_KEY_PATH"),
		// Access tokens can't be revoked, so they're short: a stolen one is
		// only good for minutes. Refresh tokens can be, so they last.
		AccessTokenTTL:  env.Duration("AUTH_ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: env.Duration("AUTH_REFRESH_TOKEN_TTL", 30*24*time.Hour),
	}

	// Text logs for humans in development, JSON for log collectors in production.
	format := "text"
	if c.Env == "production" {
		format = "json"
	}
	c.LogFormat = env.OneOf("LOG_FORMAT", format, "text", "json")

	if c.DBMaxConns < 1 || c.DBMaxConns > 100 {
		env.Invalid("AUTH_DB_MAX_CONNS", "must be between 1 and 100")
	}
	if c.AccessTokenTTL < time.Minute || c.AccessTokenTTL > time.Hour {
		env.Invalid("AUTH_ACCESS_TOKEN_TTL", "must be between 1m and 1h")
	}
	if c.RefreshTokenTTL <= c.AccessTokenTTL {
		env.Invalid("AUTH_REFRESH_TOKEN_TTL", "must be longer than AUTH_ACCESS_TOKEN_TTL")
	}
	return c, env.Err()
}
