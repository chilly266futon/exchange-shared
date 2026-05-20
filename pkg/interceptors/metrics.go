package interceptors

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/chilly266futon/exchange-shared/pkg/metrics"
)

// isClientError определяет, является ли ошибка клиентской
func isClientError(code codes.Code) bool {
	switch code {
	case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.PermissionDenied,
		codes.Unauthenticated, codes.FailedPrecondition, codes.OutOfRange, codes.Aborted,
		codes.Canceled, codes.ResourceExhausted, codes.Unimplemented:
		return true
	default:
		return false
	}
}

// MetricsInterceptor записывает метрики для каждого gRPC-вызова с разделением ошибок клиента и сервера
func MetricsInterceptor(m *metrics.Metrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		st, _ := status.FromError(err)
		errorType := "none"
		if st.Code() != codes.OK {
			if isClientError(st.Code()) {
				errorType = "client"
			} else {
				errorType = "server"
			}
		}
		m.RecordRequest(ctx, info.FullMethod, st.Code().String(), errorType, duration)
		return resp, err
	}
}
