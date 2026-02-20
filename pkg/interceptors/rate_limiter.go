package interceptors

import (
	"context"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RateLimiterInterceptor ограничивает количество запросов
func RateLimiterInterceptor(limiter *rate.Limiter) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !limiter.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

// MethodRateLimiterInterceptor позволяет установить лимиты для конкретных методов
type MethodRateLimiterInterceptor struct {
	limiters       map[string]*rate.Limiter
	defaultLimiter *rate.Limiter
}

// NewMethodRateLimiterInterceptor создает новый interceptor с лимитами по методам
func NewMethodRateLimiterInterceptor(defaultLimit rate.Limit, defaultBurst int) *MethodRateLimiterInterceptor {
	return &MethodRateLimiterInterceptor{
		limiters:       make(map[string]*rate.Limiter),
		defaultLimiter: rate.NewLimiter(defaultLimit, defaultBurst),
	}
}

// SetMethodLimit устанавливает лимит для конкретного метода
func (m *MethodRateLimiterInterceptor) SetMethodLimit(method string, limit rate.Limit, burst int) {
	m.limiters[method] = rate.NewLimiter(limit, burst)
}

// Interceptor возвращает gRPC interceptor
func (m *MethodRateLimiterInterceptor) Interceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		limiter := m.defaultLimiter
		if methodLimiter, ok := m.limiters[info.FullMethod]; ok {
			limiter = methodLimiter
		}

		if !limiter.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}

		return handler(ctx, req)
	}
}
