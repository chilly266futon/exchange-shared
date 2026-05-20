package infra

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/chilly266futon/exchange-shared/pkg/config"
	"github.com/chilly266futon/exchange-shared/pkg/metrics"
	"github.com/chilly266futon/exchange-shared/pkg/postgres"
	sharedRedis "github.com/chilly266futon/exchange-shared/pkg/redis"
)

// InitMetrics инициализирует кастомные метрики
func InitMetrics(serviceName string) (*metrics.Metrics, error) {
	return metrics.New(serviceName)
}

// InitPostgres инициализирует пул соединений Postgres
func InitPostgres(cfg config.Database, l *zap.Logger) (*pgxpool.Pool, error) {
	return postgres.NewPoolWithConfig(cfg, l)
}

// InitRedis инициализирует клиент Redis
func InitRedis(cfg config.Redis, l *zap.Logger) *redis.Client {
	return sharedRedis.NewClient(cfg, l)
}
