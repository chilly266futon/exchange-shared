package interceptors

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggerInterceptor возвращает grpc.UnaryServerInterceptor с поддержкой гибкой настройки логирования.
// Ошибки всегда логируются на Error.
// Медленные запросы (дольше slowThreshold) — на Warn.
// Обычные успешные — на Debug, если logNormalRequests=true, иначе не логируются.
func LoggerInterceptor(logger *zap.Logger, slowThreshold time.Duration, logNormalRequests bool) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()

		traceID := GetTraceID(ctx)

		resp, err := handler(ctx, req)

		duration := time.Since(start)

		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
		}

		if traceID != "" {
			fields = append(fields, zap.String("trace_id", traceID))
		}

		if err != nil {
			st, _ := status.FromError(err)
			fields = append(fields,
				zap.String("grpc_code", st.Code().String()),
				zap.String("error", err.Error()),
			)
			logger.Error("grpc request failed", fields...)
		} else if slowThreshold > 0 && duration > slowThreshold {
			logger.Warn("grpc request completed (slow)", fields...)
		} else if logNormalRequests {
			logger.Debug("grpc request completed", fields...)
		}

		return resp, err
	}
}
