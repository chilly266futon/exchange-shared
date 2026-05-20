package grpcutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type ServerConfig struct {
	Host            string
	Port            int
	ShutdownTimeout time.Duration
}

type Server struct {
	grpcServer    *grpc.Server
	listener      net.Listener
	logger        *zap.Logger
	cfg           ServerConfig
	metricsServer *http.Server
}

func NewServer(cfg ServerConfig, logger *zap.Logger, opts ...grpc.ServerOption) (*Server, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Keepalive enforcement policy
	enforcementPolicy := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second, // Minimum time between pings
		PermitWithoutStream: true,            // Allow pings without active streams
	}
	// Keepalive server parameters
	serverParams := keepalive.ServerParameters{
		MaxConnectionIdle:     5 * time.Minute,  // Close idle connections after 5 minutes
		MaxConnectionAge:      30 * time.Minute, // Close connections after 30 minutes
		MaxConnectionAgeGrace: 10 * time.Second, // Extra time to finish ongoing requests
		Time:                  2 * time.Hour,    // Ping interval (if needed)
		Timeout:               20 * time.Second, // Ping timeout
	}

	// Combine keepalive options with user-provided options
	allOpts := []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(enforcementPolicy),
		grpc.KeepaliveParams(serverParams),
	}
	allOpts = append(allOpts, opts...)

	grpcServer := grpc.NewServer(allOpts...)

	return &Server{
		grpcServer: grpcServer,
		listener:   lis,
		logger:     logger,
		cfg:        cfg,
	}, nil
}

// StartMetricsServer запускает HTTP-сервер для /metrics в отдельной горутине
func (s *Server) StartMetricsServer(ctx context.Context, port int, handler http.Handler) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", handler)

	s.metricsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		s.logger.Info("starting metrics HTTP server", zap.Int("port", port))
		errCh := make(chan error, 1)
		go func() {
			errCh <- s.metricsServer.ListenAndServe()
		}()
		select {
		case <-ctx.Done():
			s.logger.Info("shutting down metrics HTTP server by context")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.metricsServer.Shutdown(shutdownCtx)
		case err := <-errCh:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.logger.Error("metrics server error", zap.Error(err))
			}
		}
	}()
}

func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// Start запускает сервер с graceful shutdown
func (s *Server) Start() error {
	// Канал для сигналов остановки
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Канал для ошибок сервера
	errCh := make(chan error, 1)

	// Запускаем сервер в горутине
	go func() {
		s.logger.Info("starting gRPC server",
			zap.String("addr", s.listener.Addr().String()),
		)
		if err := s.grpcServer.Serve(s.listener); err != nil {
			errCh <- fmt.Errorf("grpc server error: %w", err)
		}
	}()

	// Ждем сигнала остановки или ошибки
	select {
	case <-stop:
		s.logger.Info("received shutdown signal")
		return s.gracefulShutdown()
	case err := <-errCh:
		return err
	}
}

// gracefulShutdown выполняет graceful shutdown
func (s *Server) gracefulShutdown() error {
	s.logger.Info("initiating graceful shutdown",
		zap.Duration("timeout", s.cfg.ShutdownTimeout),
	)

	// Останавливаем metrics HTTP сервер
	if s.metricsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.metricsServer.Shutdown(ctx); err != nil {
			s.logger.Warn("metrics server shutdown error", zap.Error(err))
		} else {
			s.logger.Info("metrics server stopped")
		}
	}

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	// Канал для завершения graceful stop
	done := make(chan struct{})

	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	// Ждем завершения или таймаута
	select {
	case <-done:
		s.logger.Info("graceful shutdown completed")
		return nil
	case <-ctx.Done():
		s.logger.Warn("graceful shutdown timed out, forcing stop")
		s.grpcServer.Stop()
		return nil
	}
}

func (s *Server) Stop() {
	s.grpcServer.Stop()
}
