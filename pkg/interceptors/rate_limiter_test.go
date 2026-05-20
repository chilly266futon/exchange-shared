package interceptors

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func TestPerUserRateLimit(t *testing.T) {
	logger := zap.NewNop()
	limiter := NewMethodRateLimiterInterceptor(1000, 1000, logger)
	limiter.SetPerUserLimit(2, 2, 10*time.Minute) // 2 rps, burst 2

	ctx := context.WithValue(context.Background(), "user_id", "user-1")

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	// 2 запроса подряд должны пройти (burst)
	for i := 0; i < 2; i++ {
		_, err := limiter.Interceptor()(ctx, nil, info, handler)
		if err != nil {
			t.Fatalf("unexpected error on burst: %v", err)
		}
	}

	// 3-й подряд должен быть ограничен
	_, err := limiter.Interceptor()(ctx, nil, info, handler)
	if err == nil {
		t.Fatalf("expected rate limit error, got nil")
	}

	// Подождать 0.6s (чтобы лимит восстановился на 1)
	time.Sleep(600 * time.Millisecond)
	_, err = limiter.Interceptor()(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error after wait: %v", err)
	}
}
