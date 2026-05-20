package interceptors

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TimeoutConfig конфигурация таймаутов по методам.
type TimeoutConfig map[string]time.Duration

// TimeoutInterceptor возвращает unary interceptor, который устанавливает deadline
// на обработку запроса на основе конфигурации.
func TimeoutInterceptor(timeouts TimeoutConfig) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if timeout, ok := timeouts[info.FullMethod]; ok && timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		// Если для метода таймаут не задан, передаём контекст как есть
		return handler(ctx, req)
	}
}

// DeadlineExceededInterceptor — альтернативный interceptor, который проверяет
// дедлайн в контексте и возвращает ошибку, если дедлайн истёк.
// (не используется, если TimeoutInterceptor уже установил дедлайн)
func DeadlineExceededInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= 0 {
			return nil, status.Errorf(codes.DeadlineExceeded, "deadline exceeded before processing")
		}
		return handler(ctx, req)
	}
}
