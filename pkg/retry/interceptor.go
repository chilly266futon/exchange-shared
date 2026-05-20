package retry

import (
	"context"
	"time"

	"github.com/sethvargo/go-retry"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Config holds retry with backoff configuration.
type Config struct {
	Enabled        bool          `mapstructure:"enabled"`
	MaxAttempts    uint64        `mapstructure:"max_attempts"`
	BaseDelay      time.Duration `mapstructure:"base_delay"`
	MaxDelay       time.Duration `mapstructure:"max_delay"`
	Multiplier     float64       `mapstructure:"multiplier"`
	RetryableCodes []codes.Code  `mapstructure:"retryable_codes"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:     true,
		MaxAttempts: 5,
		BaseDelay:   200 * time.Millisecond,
		MaxDelay:    15 * time.Second,
		Multiplier:  2.0,
		RetryableCodes: []codes.Code{
			codes.Unavailable,
			codes.DeadlineExceeded,
			codes.ResourceExhausted,
			codes.Aborted,
		},
	}
}

// UnaryClientInterceptor returns a gRPC unary client interceptor that performs retry with exponential backoff.
func UnaryClientInterceptor(cfg Config, logger *zap.Logger) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if !cfg.Enabled {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		// Создаём политику retry с экспоненциальным backoff.
		backoff := retry.NewExponential(cfg.BaseDelay)
		if cfg.Multiplier != 0 {
			// Игнорируем, так как NewExponential уже имеет множитель по умолчанию.
		}
		backoff = retry.WithMaxRetries(cfg.MaxAttempts-1, backoff)
		if cfg.MaxDelay > 0 {
			backoff = retry.WithCappedDuration(cfg.MaxDelay, backoff)
		}

		var lastErr error
		err := retry.Do(ctx, backoff, func(ctx context.Context) error {
			err := invoker(ctx, method, req, reply, cc, opts...)
			if err == nil {
				return nil
			}
			lastErr = err

			// Проверяем, является ли ошибка retryable.
			if isRetryable(err, cfg.RetryableCodes) {
				logger.Debug("retrying gRPC call",
					zap.String("method", method),
					zap.Error(err),
				)
				// Обёртываем в RetryableError, чтобы библиотека выполнила повтор.
				return retry.RetryableError(err)
			}
			// Неретрируемая ошибка — возвращаем как есть, retry остановится.
			return err
		})

		if err != nil {
			// Если все попытки исчерпаны, возвращаем последнюю ошибку.
			// err будет равен последней RetryableError, но нам нужна исходная ошибка.
			return lastErr
		}
		return nil
	}
}

// isRetryable checks whether a gRPC error is retryable according to the list of codes.
func isRetryable(err error, retryableCodes []codes.Code) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	for _, code := range retryableCodes {
		if st.Code() == code {
			return true
		}
	}
	return false
}
