package health

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Server struct {
	*health.Server
}

func NewServer() *Server {
	return &Server{
		Server: health.NewServer(),
	}
}

func (s *Server) SetServingStatus(service string, status grpc_health_v1.HealthCheckResponse_ServingStatus) {
	s.Server.SetServingStatus(service, status)
}

func (s *Server) SetHealthy(service string) {
	s.SetServingStatus(service, grpc_health_v1.HealthCheckResponse_SERVING)
}

func (s *Server) SetUnhealthy(service string) {
	s.SetServingStatus(service, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
}

type Checker interface {
	Check(ctx context.Context) error
}

type CheckerFunc func(ctx context.Context) error

func (f CheckerFunc) Check(ctx context.Context) error {
	return f(ctx)
}

func RunChecks(ctx context.Context, checkers ...Checker) bool {
	for _, checker := range checkers {
		if err := checker.Check(ctx); err != nil {
			return false
		}
	}
	return true
}

func CheckPostgresHealth(ctx context.Context, db *pgxpool.Pool) error {
	return db.Ping(ctx)
}

func CheckRedisHealth(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}

func RegisterHealthServer(
	ctx context.Context,
	grpcServer *grpc.Server,
	logger *zap.Logger,
	serviceName string,
	checks map[string]func(context.Context) error,
	interval time.Duration,
	healthTimeout time.Duration,
) {
	healthServer := NewServer()

	for name, check := range checks {
		if err := check(ctx); err == nil {
			healthServer.SetHealthy(name)
		} else {
			healthServer.SetUnhealthy(name)
			logger.Warn(name+" health check failed", zap.Error(err))
		}
	}

	healthServer.SetHealthy(serviceName)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer.Server)
	logger.Info("health check enabled")

	go func(ctx context.Context) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logger.Info("health check goroutine stopped")
				return
			case <-ticker.C:
				for name, check := range checks {
					checkCtx, cancel := context.WithTimeout(ctx, healthTimeout)
					if err := check(checkCtx); err == nil {
						healthServer.SetHealthy(name)
					} else {
						healthServer.SetUnhealthy(name)
						logger.Warn(name+" health check failed", zap.Error(err))
					}
					cancel()
				}
			}
		}
	}(ctx)
}
