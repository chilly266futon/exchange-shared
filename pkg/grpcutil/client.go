package grpcutil

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientConfig конфигурация gRPC клиента
type ClientConfig struct {
	Address        string
	Timeout        time.Duration
	MaxRetries     int
	EnableTLS      bool
	ConnectTimeout time.Duration
}

// DefaultClientConfig возвращает конфигурацию по умолчанию
func DefaultClientConfig(address string) ClientConfig {
	return ClientConfig{
		Address:        address,
		Timeout:        5 * time.Second,
		MaxRetries:     3,
		EnableTLS:      false,
		ConnectTimeout: 10 * time.Second,
	}
}

// NewClient создает новое gRPC соединение с настройками
func NewClient(cfg ClientConfig, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// Базовые опции
	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(10*1024*1024), // 10MB
			grpc.MaxCallSendMsgSize(10*1024*1024), // 10MB
		),
	}

	dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	// Добавляем пользовательские опции
	dialOpts = append(dialOpts, opts...)

	// Создаем контекст с таймаутом для подключения
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	// Подключаемся
	conn, err := grpc.DialContext(ctx, cfg.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", cfg.Address, err)
	}

	return conn, nil
}

// MustNewClient создает клиент или паникует при ошибке
func MustNewClient(cfg ClientConfig, opts ...grpc.DialOption) *grpc.ClientConn {
	conn, err := NewClient(cfg, opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to create grpc client: %v", err))
	}
	return conn
}
