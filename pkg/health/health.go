package health

import (
	"context"

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
