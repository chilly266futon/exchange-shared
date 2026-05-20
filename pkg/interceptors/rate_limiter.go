package interceptors

import (
	"context"
	"strings"
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
	batchSize       int           // максимальное количество удаляемых пользователей за один проход
	shutdownCh      chan struct{} // канал для остановки фонового воркера
	logger          *zap.Logger
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
		batchSize:       100, // по умолчанию 100
		shutdownCh:      make(chan struct{}),
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

func (m *MethodRateLimiterInterceptor) SetPerUserLimit(limit rate.Limit, burst int, maxInactiveAge time.Duration) {
	m.perUserRate = limit
	m.perUserBurst = burst
	if maxInactiveAge > 0 {
		m.maxAge = maxInactiveAge
	}
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
		shortMethod := info.FullMethod
		if idx := strings.LastIndex(info.FullMethod, "/"); idx >= 0 {
			shortMethod = info.FullMethod[idx+1:]
		}

		m.methodMu.RLock()
		methodLimiter, hasMethodLimit := m.methodLimiters[shortMethod]
		m.methodMu.RUnlock()

		if hasMethodLimit && !methodLimiter.Allow() {
			m.logger.Warn("method rate limit exceeded", zap.String("method", info.FullMethod))
			return nil, status.Error(codes.ResourceExhausted, "method rate limit exceeded")
		}

		// 3. Per-user limit (до handler!)
		if m.perUserRate > 0 {
			userID, ok := ctx.Value("user_id").(string)
			if ok && userID != "" {
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

				// Проверяем лимит ДО вызова handler
				if !user.limiter.Allow() {
					m.logger.Warn("per-user rate limit exceeded", zap.String("user_id", userID), zap.String("method", info.FullMethod))
					return nil, status.Error(codes.ResourceExhausted, "per-user rate limit exceeded")
				}
				// Обновляем lastUsed только если лимит не превышен
				user.lastUsed = time.Now()
			}
		}

		resp, err := handler(ctx, req)
		if err != nil {
			return nil, err
		}

		return resp, nil
	}
}

// Остановка фонового воркера
func (m *MethodRateLimiterInterceptor) Stop() {
	close(m.shutdownCh)
}

// cleanupOldUsers — периодическая очистка неактивных пользователей
func (m *MethodRateLimiterInterceptor) cleanupOldUsers() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			var toDelete []string

			m.perUserMu.RLock()
			for userID, user := range m.perUserLimiters {
				if now.Sub(user.lastUsed) > m.maxAge {
					toDelete = append(toDelete, userID)
					if len(toDelete) >= m.batchSize {
						break
					}
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
		case <-m.shutdownCh:
			m.logger.Info("rate limiter cleanup worker stopped")
			return
		}
	}
}
