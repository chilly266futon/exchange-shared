package retry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockInvoker struct {
	calls      int
	failUntil  int
	errorCode  codes.Code
	successRes string
}

func (m *mockInvoker) invoker(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
	m.calls++
	if m.calls <= m.failUntil {
		return status.Error(m.errorCode, "transient error")
	}
	if reply != nil {
		if strPtr, ok := reply.(*string); ok {
			*strPtr = m.successRes
		}
	}
	return nil
}

func TestUnaryClientInterceptor_Disabled(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{Enabled: false}
	interceptor := UnaryClientInterceptor(cfg, logger)

	mock := &mockInvoker{failUntil: 0}
	conn := &grpc.ClientConn{}
	var reply string
	err := interceptor(context.Background(), "/test.Method", nil, &reply, conn, mock.invoker)
	require.NoError(t, err)
	assert.Equal(t, 1, mock.calls)
}

func TestUnaryClientInterceptor_RetryOnUnavailable(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.BaseDelay = 10 * time.Millisecond
	cfg.MaxDelay = 100 * time.Millisecond
	interceptor := UnaryClientInterceptor(cfg, logger)

	mock := &mockInvoker{
		failUntil:  2,
		errorCode:  codes.Unavailable,
		successRes: "success",
	}
	conn := &grpc.ClientConn{}
	var reply string
	err := interceptor(context.Background(), "/test.Method", nil, &reply, conn, mock.invoker)
	require.NoError(t, err)
	assert.Equal(t, 3, mock.calls) // 2 failures + 1 success
	assert.Equal(t, "success", reply)
}

func TestUnaryClientInterceptor_NonRetryableError(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.BaseDelay = 10 * time.Millisecond
	interceptor := UnaryClientInterceptor(cfg, logger)

	mock := &mockInvoker{
		failUntil:  5, // больше чем max attempts, но ошибка не ретрируемая
		errorCode:  codes.InvalidArgument,
		successRes: "success",
	}
	conn := &grpc.ClientConn{}
	var reply string
	err := interceptor(context.Background(), "/test.Method", nil, &reply, conn, mock.invoker)
	require.Error(t, err)
	assert.Equal(t, 1, mock.calls) // должен остановиться после первой ошибки
}

func TestUnaryClientInterceptor_ExhaustedAttempts(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.MaxAttempts = 2
	cfg.BaseDelay = 10 * time.Millisecond
	interceptor := UnaryClientInterceptor(cfg, logger)

	mock := &mockInvoker{
		failUntil:  5, // всегда фейлится
		errorCode:  codes.Unavailable,
		successRes: "success",
	}
	conn := &grpc.ClientConn{}
	var reply string
	err := interceptor(context.Background(), "/test.Method", nil, &reply, conn, mock.invoker)
	require.Error(t, err)
	assert.Equal(t, 2, mock.calls) // 2 попытки
}
