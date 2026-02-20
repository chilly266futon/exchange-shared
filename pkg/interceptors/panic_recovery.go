package interceptors

import (
	"context"

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
					zap.Any("panic", r),
				}
				if traceID != "" {
					fields = append(fields, zap.String("trace_id", traceID))
				}

				logger.Error("panic recovered", fields...)
				err = status.Errorf(codes.Internal, "internal server error: %v", r)
			}
		}()

		return handler(ctx, req)
	}
}
