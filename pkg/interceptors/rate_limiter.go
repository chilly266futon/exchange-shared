package interceptors

import (
	"context"
	"sync"
	"time"

	"github.com/chilly266futon/exchange-shared/pkg/common"
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
	// per-method (меняется редко)
	methodLimiters map[string]*rate.Limiter // per-method
	defaultLimiter *rate.Limiter            // global
	methodMu       sync.RWMutex             // защита map limiters

	// per-user (hot)
	perUserLimiters map[string]*userLimiterStruct // per-user
	perUserMu       sync.RWMutex                  // защита map perUserLimiters

	perUserRate     rate.Limit
	perUserBurst    int
	cleanupInterval time.Duration
	maxAge          time.Duration
}

type userLimiterStruct struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

// NewMethodRateLimiterInterceptor создает новый interceptor с лимитами по методам
func NewMethodRateLimiterInterceptor(defaultLimit rate.Limit, defaultBurst int) *MethodRateLimiterInterceptor {
	m := &MethodRateLimiterInterceptor{
		methodLimiters:  make(map[string]*rate.Limiter),
		defaultLimiter:  rate.NewLimiter(defaultLimit, defaultBurst),
		perUserLimiters: make(map[string]*userLimiterStruct),
		cleanupInterval: 5 * time.Minute,
		maxAge:          15 * time.Minute,
	}

	go m.cleanupOldUsers()

	return m
}

// SetMethodLimit устанавливает лимит для конкретного метода
func (m *MethodRateLimiterInterceptor) SetMethodLimit(method string, limit rate.Limit, burst int) {
	m.methodMu.Lock()
	defer m.methodMu.Unlock()
	m.methodLimiters[method] = rate.NewLimiter(limit, burst)
}

func (m *MethodRateLimiterInterceptor) SetPerUserLimit(limit rate.Limit, burst int) {
	m.perUserRate = limit
	m.perUserBurst = burst
}

// Interceptor возвращает gRPC interceptor
func (m *MethodRateLimiterInterceptor) Interceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// Global limit
		if !m.defaultLimiter.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "global rate limit exceeded")
		}

		// Per-method limit
		m.methodMu.RLock()
		limiter := m.defaultLimiter
		if methodLimiter, ok := m.methodLimiters[info.FullMethod]; ok {
			limiter = methodLimiter
		}
		m.methodMu.RUnlock()

		if !limiter.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "method rate limit exceeded")
		}

		// Per-user limit
		if m.perUserRate > 0 {
			userID := common.GetUserID(ctx)
			if userID == "" {
				return nil, status.Error(codes.InvalidArgument, "user_id is required")
			}

			m.perUserMu.RLock()
			user, ok := m.perUserLimiters[userID]
			m.perUserMu.RUnlock()

			if !ok {
				m.perUserMu.Lock()
				if user, ok = m.perUserLimiters[userID]; !ok {
					user = &userLimiterStruct{
						limiter:  rate.NewLimiter(m.perUserRate, m.perUserBurst),
						lastUsed: time.Now(),
					}
					m.perUserMu.Unlock()
				}
			}

			user.lastUsed = time.Now()

			if !user.limiter.Allow() {
				return nil, status.Error(codes.ResourceExhausted, "per-user rate limit exceeded")
			}
		}

		return handler(ctx, req)
	}
}

func (m *MethodRateLimiterInterceptor) cleanupOldUsers() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		var toDelete []string

		m.perUserMu.RLock()
		for userID, user := range m.perUserLimiters {
			if now.Sub(user.lastUsed) > m.maxAge {
				toDelete = append(toDelete, userID)
			}
		}
		m.perUserMu.RUnlock()

		if len(toDelete) > 0 {
			continue
		}

		m.perUserMu.Lock()
		for _, userID := range toDelete {
			delete(m.perUserLimiters, userID)
		}
		m.perUserMu.Unlock()
	}
}
