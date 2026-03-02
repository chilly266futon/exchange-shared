package interceptors

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userLimiter struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

type MethodRateLimiterInterceptor struct {
	// per-method (меняется редко)
	methodLimiters map[string]*rate.Limiter
	defaultLimiter *rate.Limiter
	methodMu       sync.RWMutex

	// per-user (горячая часть)
	perUserLimiters map[string]*userLimiter
	perUserMu       sync.RWMutex

	perUserRate     rate.Limit
	perUserBurst    int
	cleanupInterval time.Duration
	maxAge          time.Duration

	logger *zap.Logger
}

func NewMethodRateLimiterInterceptor(
	defaultLimit rate.Limit,
	defaultBurst int,
	logger *zap.Logger,
) *MethodRateLimiterInterceptor {
	m := &MethodRateLimiterInterceptor{
		methodLimiters:  make(map[string]*rate.Limiter),
		defaultLimiter:  rate.NewLimiter(defaultLimit, defaultBurst),
		perUserLimiters: make(map[string]*userLimiter),
		cleanupInterval: 5 * time.Minute,
		maxAge:          30 * time.Minute,
		logger:          logger,
	}

	// Запускаем очистку старых пользователей
	go m.cleanupOldUsers()

	return m
}

func (m *MethodRateLimiterInterceptor) SetMethodLimit(method string, limit rate.Limit, burst int) {
	m.methodMu.Lock()
	defer m.methodMu.Unlock()
	m.methodLimiters[method] = rate.NewLimiter(limit, burst)
}

func (m *MethodRateLimiterInterceptor) SetPerUserLimit(limit rate.Limit, burst int) {
	m.perUserRate = limit
	m.perUserBurst = burst
}

func (m *MethodRateLimiterInterceptor) Interceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 1. Global limit
		if !m.defaultLimiter.Allow() {
			m.logger.Warn("global rate limit exceeded", zap.String("method", info.FullMethod))
			return nil, status.Error(codes.ResourceExhausted, "global rate limit exceeded")
		}

		// 2. Per-method limit
		m.methodMu.RLock()
		limiter := m.defaultLimiter
		if methodLimiter, ok := m.methodLimiters[info.FullMethod]; ok {
			limiter = methodLimiter
		}
		m.methodMu.RUnlock()

		if !limiter.Allow() {
			m.logger.Warn("method rate limit exceeded", zap.String("method", info.FullMethod))
			return nil, status.Error(codes.ResourceExhausted, "method rate limit exceeded")
		}

		// 3. Per-user limit (из контекста после авторизации)
		if m.perUserRate > 0 {
			userID, ok := ctx.Value("user_id").(string)
			if !ok || userID == "" {
				m.logger.Warn("user_id missing in context", zap.String("method", info.FullMethod))
				return nil, status.Error(codes.InvalidArgument, "user_id required")
			}

			m.perUserMu.RLock()
			user, exists := m.perUserLimiters[userID]
			m.perUserMu.RUnlock()

			if !exists {
				m.perUserMu.Lock()
				if user, exists = m.perUserLimiters[userID]; !exists {
					user = &userLimiter{
						limiter:  rate.NewLimiter(m.perUserRate, m.perUserBurst),
						lastUsed: time.Now(),
					}
					m.perUserLimiters[userID] = user
				}
				m.perUserMu.Unlock()
			}

			// Обновляем lastUsed
			user.lastUsed = time.Now()

			if !user.limiter.Allow() {
				m.logger.Warn("per-user rate limit exceeded", zap.String("user_id", userID), zap.String("method", info.FullMethod))
				return nil, status.Error(codes.ResourceExhausted, "per-user rate limit exceeded")
			}
		}

		return handler(ctx, req)
	}
}

// cleanupOldUsers — периодическая очистка неактивных пользователей
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

		if len(toDelete) == 0 {
			continue
		}

		m.perUserMu.Lock()
		for _, userID := range toDelete {
			delete(m.perUserLimiters, userID)
			m.logger.Debug("removed inactive user limiter", zap.String("user_id", userID))
		}
		m.perUserMu.Unlock()
	}
}
