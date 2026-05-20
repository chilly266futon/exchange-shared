package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/chilly266futon/exchange-shared/pkg/config"
)

func NewClient(cfg config.Redis, l *zap.Logger) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:            cfg.Addr,
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
	})

	retryTimeout := cfg.RetryTimeout
	if retryTimeout == 0 {
		retryTimeout = 30 * time.Second
	}
	interval := cfg.RetryInterval
	if interval == 0 {
		interval = 2 * time.Second
	}
	start := time.Now()
	var lastErr error
	connected := false

	for time.Since(start) < retryTimeout {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			lastErr = err
			l.Warn("failed to connect to redis, retrying...", zap.Error(err))
			time.Sleep(interval)
			continue
		}
		connected = true
		break
	}

	if !connected {
		l.Warn("redis unavailable after retries, degraded mode enabled", zap.Error(lastErr))
	} else {
		l.Info("redis connected",
			zap.String("addr", cfg.Addr),
			zap.Int("db", cfg.DB),
		)
	}

	return client
}
