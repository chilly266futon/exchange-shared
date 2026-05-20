package interceptors

import (
	"context"
	"fmt"
	"runtime/debug"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryPanicRecoveryInterceptor перехватывает панику и возвращает Internal error
func UnaryPanicRecoveryInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				traceID := GetTraceID(ctx)

				fields := []zap.Field{
					zap.String("method", info.FullMethod),
					zap.String("panic", fmt.Sprintf("%v", r)),
					zap.String("panic_type", fmt.Sprintf("%T", r)),
				}
				if traceID != "" {
					fields = append(fields, zap.String("trace_id", traceID))
				}
				if logger.Core().Enabled(zap.DebugLevel) {
					fields = append(fields, zap.String("stack", string(debug.Stack())))
				}
				logger.Error("panic recovered", fields...)
				code := mapPanicToGRPCCode(r)
				err = status.Errorf(code, "%v", r)
			}
		}()

		return handler(ctx, req)
	}
}

// mapPanicToGRPCCode определяет gRPC-код для panic, если это клиентская ошибка
func mapPanicToGRPCCode(r any) codes.Code {
	err, ok := r.(error)
	if !ok {
		return codes.Internal
	}
	msg := err.Error()
	// Примеры: можно расширить по необходимости
	switch {
	case contains(msg, "invalid argument"):
		return codes.InvalidArgument
	case contains(msg, "unauthenticated"):
		return codes.Unauthenticated
	case contains(msg, "permission denied"):
		return codes.PermissionDenied
	case contains(msg, "not found"):
		return codes.NotFound
	case contains(msg, "already exists"):
		return codes.AlreadyExists
	case contains(msg, "resource exhausted"):
		return codes.ResourceExhausted
	case contains(msg, "failed precondition"):
		return codes.FailedPrecondition
	case contains(msg, "out of range"):
		return codes.OutOfRange
	case contains(msg, "aborted"):
		return codes.Aborted
	case contains(msg, "canceled"):
		return codes.Canceled
	default:
		return codes.Internal
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr))))
}
