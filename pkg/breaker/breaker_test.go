package breaker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDefaultConfig_ReadyToTrip(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name   string
		counts gobreaker.Counts
		want   bool
	}{
		{
			name: "not enough requests",
			counts: gobreaker.Counts{
				Requests:      5,
				TotalFailures: 5,
			},
			want: false,
		},
		{
			name: "enough requests but low failure ratio",
			counts: gobreaker.Counts{
				Requests:      10,
				TotalFailures: 5,
			},
			want: false,
		},
		{
			name: "should trip - high failure ration",
			counts: gobreaker.Counts{
				Requests:      10,
				TotalFailures: 7,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.ReadyToTrip(tt.counts)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestWrapper_Execute_Success(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Attempts = 3
	w := NewWrapper("test", cfg)

	callCount := 0
	err := w.Execute(func() error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestWrapper_RetryOnError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Attempts = 3
	cfg.RetryDelay = 1 * time.Millisecond
	w := NewWrapper("test", cfg)

	callCount := 0
	err := w.Execute(func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestWrapper_Execute_AllAttemptsFail(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Attempts = 3
	cfg.RetryDelay = 1 * time.Millisecond
	w := NewWrapper("test", cfg)

	expectedErr := errors.New("persistent error")
	callCount := 0
	err := w.Execute(func() error {
		callCount++
		return expectedErr
	})

	assert.Error(t, err)
	assert.Equal(t, 3, callCount)
}

func TestWrapper_ExecuteWithContext_Success(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWrapper("test", cfg)

	ctx := context.Background()

	err := w.ExecuteWithContext(ctx, func() error {
		return nil
	})

	assert.NoError(t, err)
}

func TestWrapper_ExecuteWithContext(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Attempts = 5
	cfg.RetryDelay = 100 * time.Millisecond
	w := NewWrapper("test", cfg)

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := w.ExecuteWithContext(ctx, func() error {
		callCount++
		return errors.New("error")
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) || callCount < 5)
}

func TestWrapper_ZeroAttempts(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWrapper("test", cfg)

	callCount := 0
	err := w.Execute(func() error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "unavailable is retryable",
			err:  status.Error(codes.Unavailable, "unavailable"),
			want: true,
		},
		{
			name: "deadline exceeded is retryable",
			err:  status.Error(codes.DeadlineExceeded, "timeout"),
			want: true,
		},
		{
			name: "resource exhausted is retryable",
			err:  status.Error(codes.ResourceExhausted, "exhausted"),
			want: true,
		},
		{
			name: "aborted is retryable",
			err:  status.Error(codes.Aborted, "aborted"),
			want: true,
		},
		{
			name: "invalid argument is not retryable",
			err:  status.Error(codes.InvalidArgument, "invalid argument"),
			want: false,
		},
		{
			name: "not found is not retryable",
			err:  status.Error(codes.NotFound, "not found"),
			want: false,
		},
		{
			name: "non-gRPC error is not retryable",
			err:  errors.New("some error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryable(tt.err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TODO:
