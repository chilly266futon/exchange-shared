package grpcutil

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func NewGRPCClient(addr string, logger *zap.Logger, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// Keepalive client parameters
	keepaliveParams := keepalive.ClientParameters{
		Time:                30 * time.Second, // send pings every 30 seconds if there is no activity
		Timeout:             10 * time.Second, // wait 10 seconds for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}

	// Backoff configuration for connection retry
	backoffConfig := backoff.DefaultConfig
	backoffConfig.BaseDelay = 1 * time.Second
	backoffConfig.Multiplier = 1.6
	backoffConfig.Jitter = 0.2
	backoffConfig.MaxDelay = 30 * time.Second

	defaultOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepaliveParams),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoffConfig,
			MinConnectTimeout: 20 * time.Second,
		}),
	}
	allOpts := append(defaultOpts, opts...)

	client, err := grpc.NewClient(addr, allOpts...)
	if err != nil {
		logger.Error("failed to create gRPC client", zap.Error(err), zap.String("address", addr))
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}
	return client, nil
}
