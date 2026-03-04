package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func NewPool(dsn string, logger *zap.Logger) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	// Настройки пула
	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}

	// Проверяем соединение
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, err
	}

	logger.Info("postgres pool created successfully",
		zap.String("dsn", dsn),
		zap.Int32("max_conns", config.MaxConns),
	)

	return pool, nil
}
