package breaker

import (
	"context"
	"errors"
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Config struct {
	MaxRequests uint32        // Максимальное количество запросов в полуоткрытом состоянии
	Interval    time.Duration // Интервал для сброса счетчиков
	Timeout     time.Duration // Таймаут для перехода в полуоткрытое состояние
	Attempts    uint32        // Количество попыток перед возвратом ошибки
	RetryDelay  time.Duration // Задержка между попытками
	ReadyToTrip func(counts gobreaker.Counts) bool
}

func DefaultConfig() Config {
	return Config{
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		Attempts:    3,
		RetryDelay:  100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.6
		},
	}
}

func UnaryClientInterceptor(cfg Config) grpc.UnaryClientInterceptor {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "grpc-client",
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: cfg.ReadyToTrip,
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			// TODO: интеграция с внешним мониторингом
		},
	})

	attempts := cfg.Attempts
	if attempts == 0 {
		attempts = 1
	}

	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		var lastErr error

		for i := uint32(0); i < attempts; i++ {
			if i > 0 && cfg.RetryDelay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(cfg.RetryDelay):
				}
			}

			_, err := cb.Execute(func() (any, error) {
				err := invoker(ctx, method, req, reply, cc, opts...)

				if err != nil {
					st, ok := status.FromError(err)
					if ok {
						switch st.Code() {
						case codes.Unavailable, codes.Internal, codes.DeadlineExceeded:
							return nil, err
						}
					}
				}

				return nil, err
			})

			if err == nil {
				return nil
			}

			lastErr = err

			if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
				return err
			}

			if !isRetryable(err) {
				return err
			}
		}

		return lastErr
	}
}

func isRetryable(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}

	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Aborted:
		return true
	default:
		return false
	}
}

type Wrapper struct {
	cb       *gobreaker.CircuitBreaker
	attempts uint32
	delay    time.Duration
}

func NewWrapper(name string, cfg Config) *Wrapper {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:          name,
		MaxRequests:   cfg.MaxRequests,
		Interval:      cfg.Interval,
		Timeout:       cfg.Timeout,
		ReadyToTrip:   cfg.ReadyToTrip,
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {},
	})

	attempts := cfg.Attempts
	if attempts == 0 {
		attempts = 1
	}

	return &Wrapper{
		cb:       cb,
		attempts: attempts,
		delay:    cfg.RetryDelay,
	}
}

func (w *Wrapper) Execute(fn func() error) error {
	var lastErr error

	for i := uint32(0); i < w.attempts; i++ {
		if i > 0 && w.delay > 0 {
			time.Sleep(w.delay)
		}

		_, err := w.cb.Execute(func() (any, error) {
			return nil, fn()
		})

		if err == nil {
			return nil
		}

		lastErr = err

		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return err
		}
	}

	return lastErr
}

func (w *Wrapper) ExecuteWithContext(ctx context.Context, fn func() error) error {
	var lastErr error

	for i := uint32(0); i < w.attempts; i++ {
		if i > 0 && w.delay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(w.delay):
			}
		}

		_, err := w.cb.Execute(func() (any, error) {
			return nil, fn()
		})

		if err == nil {
			return nil
		}

		lastErr = err

		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return err
		}
	}

	return lastErr
}

func (w *Wrapper) State() gobreaker.State {
	return w.cb.State()
}
