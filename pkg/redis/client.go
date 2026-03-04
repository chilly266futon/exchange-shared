package redis

import (
	"context"
	"time"

	"github.com/chilly266futon/exchange-shared/pkg/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewClient(cfg config.Redis, l *zap.Logger) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		l.Fatal("failed to connect to redis", zap.Error(err))
	}

	l.Info("redis connected",
		zap.String("addr", cfg.Addr),
		zap.Int("db", cfg.DB),
	)

	return client
}
