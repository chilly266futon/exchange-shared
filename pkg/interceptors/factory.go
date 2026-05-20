package interceptors

import (
	"context"
	"time"

	"buf.build/go/protovalidate"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/chilly266futon/exchange-shared/pkg/config"
	"github.com/chilly266futon/exchange-shared/pkg/metrics"
	"github.com/chilly266futon/exchange-shared/pkg/telemetry"
)

type InterceptorConfig struct {
	Logger      *zap.Logger
	Metrics     *metrics.Metrics
	RateLimit   RateLimitConfig
	AuthMethods []string
}

type RateLimitConfig struct {
	GlobalRPS   float64
	GlobalBurst int
	PerMethod   map[string]RateLimit
	PerUser     RateLimit
}

type RateLimit struct {
	RPS            float64
	Burst          int
	MaxInactiveAge time.Duration
}

// Унифицированная фабрика interceptor chain
func NewInterceptorChain(
	logger *zap.Logger,
	metrics *metrics.Metrics,
	rateLimit config.RateLimit,
	loggerCfg config.LoggerConfig,
	jwtValidator JWTValidator,
	protoValidator protovalidate.Validator,
	skipAuthMethods []string,
	operationTimeouts map[string]time.Duration,
) ([]grpc.ServerOption, *MethodRateLimiterInterceptor) {
	rateLimiter := NewMethodRateLimiterInterceptor(
		rate.Limit(rateLimit.Global.RPS),
		rateLimit.Global.Burst,
		logger,
	)

	for method, limit := range rateLimit.PerMethod {
		rateLimiter.SetMethodLimit(method, rate.Limit(limit.RPS), limit.Burst)
	}

	if rateLimit.PerUser.RPS > 0 {
		rateLimiter.SetPerUserLimit(
			rate.Limit(rateLimit.PerUser.RPS),
			rateLimit.PerUser.Burst,
			rateLimit.PerUser.MaxInactiveAge,
		)
	}

	opts := []grpc.ServerOption{
		telemetry.GRPCServerStatsHandler(),
		grpc.ChainUnaryInterceptor(
			TraceIDInterceptor(),
			TimeoutInterceptor(operationTimeouts),
			MetricsInterceptor(metrics),
			rateLimiter.Interceptor(),
			LoggerInterceptor(logger, loggerCfg.SlowThreshold, loggerCfg.LogNormalRequests),
			UnaryPanicRecoveryInterceptor(logger),
			AuthInterceptor(logger, jwtValidator, skipAuthMethods...),
			func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
				msg, ok := req.(proto.Message)
				if !ok {
					return handler(ctx, req)
				}
				if err := protoValidator.Validate(msg); err != nil {
					logger.Warn("validation failed", zap.String("method", info.FullMethod), zap.Error(err))
					return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
				}
				return handler(ctx, req)
			},
		),
	}

	return opts, rateLimiter
}
