package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/chilly266futon/exchange-shared/pkg/config"
)

func NewPoolWithConfig(cfg config.Database, logger *zap.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, err
	}

	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolConfig.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod
	}

	retryTimeout := cfg.RetryTimeout
	if retryTimeout == 0 {
		retryTimeout = 30 * time.Second
	}
	interval := cfg.RetryInterval
	if interval == 0 {
		interval = 2 * time.Second
	}
	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = 15
	}

	start := time.Now()
	var lastErr error
	for attempt := 1; time.Since(start) < retryTimeout && attempt <= maxRetries; attempt++ {
		pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
		if err != nil {
			lastErr = err
			logger.Warn("failed to create postgres pool, retrying...", zap.Error(err), zap.Int("attempt", attempt))
			time.Sleep(interval)
			continue
		}
		if err := pool.Ping(context.Background()); err != nil {
			pool.Close()
			lastErr = err
			logger.Warn("failed to ping postgres, retrying...", zap.Error(err), zap.Int("attempt", attempt))
			time.Sleep(interval)
			continue
		}
		logger.Info("postgres pool created successfully",
			zap.String("host", poolConfig.ConnConfig.Host),
			zap.String("database", poolConfig.ConnConfig.Database),
			zap.Int32("max_conns", poolConfig.MaxConns),
		)
		return pool, nil
	}
	logger.Warn("postgres unavailable after retries, degraded mode enabled", zap.Error(lastErr))
	return nil, lastErr
}
