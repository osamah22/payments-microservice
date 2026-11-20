package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, dns string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dns)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse dns: %w", err)
	}

	cfg.MinConns = min(2, maxConns)
	cfg.MaxConns = maxConns

	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnLifetimeJitter = time.Minute * 5
	cfg.MaxConnIdleTime = time.Minute * 15
	cfg.HealthCheckPeriod = time.Second * 30
	cfg.ConnConfig.ConnectTimeout = time.Second * 5
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}
