package breaker

import (
	"context"
	"errors"
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/chilly266futon/exchange-shared/pkg/config"
	"github.com/chilly266futon/exchange-shared/pkg/metrics"
)

func DefaultConfig() config.CircuitBreaker {
	return config.CircuitBreaker{
		MaxRequests:  3,
		Interval:     10 * time.Second,
		Timeout:      30 * time.Second,
		Attempts:     3,
		RetryDelay:   100 * time.Millisecond,
		MinRequests:  10,
		FailureRatio: 0.6,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < 10 {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= 0.6
		},
	}
}

func UnaryClientInterceptor(cfg config.CircuitBreaker, m *metrics.Metrics) grpc.UnaryClientInterceptor {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name: "grpc-client",
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if cfg.ReadyToTrip != nil {
				return cfg.ReadyToTrip(counts)
			}
			if counts.Requests < cfg.MinRequests {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= cfg.FailureRatio
		},
		Timeout:     cfg.Timeout,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			if m != nil {
				m.RecordBreakerStateChange(name, from.String(), to.String())
			}
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

func NewWrapper(name string, cfg config.CircuitBreaker) *Wrapper {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:          name,
		MaxRequests:   cfg.MaxRequests,
		Interval:      cfg.Interval,
		Timeout:       cfg.Timeout,
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
		if i == w.attempts-1 {
			return err
		}
	}
	return nil
}
