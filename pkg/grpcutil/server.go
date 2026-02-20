package grpcutil

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type ServerConfig struct {
	Host            string
	Port            int
	ShutdownTimeout time.Duration
}

type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	logger     *zap.Logger
	cfg        ServerConfig
}

func NewServer(cfg ServerConfig, logger *zap.Logger, opts ...grpc.ServerOption) (*Server, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer(opts...)

	return &Server{
		grpcServer: grpcServer,
		listener:   lis,
		logger:     logger,
		cfg:        cfg,
	}, nil
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
