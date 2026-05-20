package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/atomic"
)

// Metrics содержит все кастомные метрики для сервисов
type Metrics struct {
	// gRPC
	RequestTotal   metric.Int64Counter
	RequestLatency metric.Float64Histogram
	RequestErrors  metric.Int64Counter

	// Auth
	LoginsTotal              metric.Int64Counter
	RegistrationsTotal       metric.Int64Counter
	TokenRefreshTotal        metric.Int64Counter
	RefreshTokenRetention    metric.Int64ObservableGauge
	RegistrationErrors       metric.Int64Counter
	LoginErrors              metric.Int64Counter
	ActiveUsersGauge         metric.Int64ObservableGauge
	UniqueRegistrationsGauge metric.Int64ObservableGauge

	// Order
	OrdersCreatedTotal    metric.Int64Counter
	OrdersCancelledTotal  metric.Int64Counter
	OrdersByType          metric.Int64Counter
	OrdersByMarket        metric.Int64Counter
	OrdersByStatus        metric.Int64Counter
	OrderExecutionLatency metric.Float64Histogram
	OrderCancelErrors     metric.Int64Counter
	OrderExecutionErrors  metric.Int64Counter
	AverageOrderValue     metric.Float64UpDownCounter
	TotalRevenue          metric.Float64Counter

	// Spot
	MarketQueriesTotal  metric.Int64Counter
	ActiveMarketsGauge  metric.Int64UpDownCounter
	MarketEnabledGauge  metric.Int64ObservableGauge
	MarketDisabledGauge metric.Int64ObservableGauge

	// Бизнес-метрики
	RetentionRate        metric.Float64ObservableGauge
	ConversionRate       metric.Float64ObservableGauge
	ChurnRate            metric.Float64ObservableGauge
	AverageSessionTime   metric.Float64Histogram
	ErrorRateByEndpoint  metric.Int64Counter
	RevenuePerUser       metric.Float64ObservableGauge
	OrderFulfillmentTime metric.Float64Histogram
	MarketActivity       metric.Int64Counter

	// Atomic values for observable gauges
	refreshTokenRetention atomic.Int64
	activeUsers           atomic.Int64
	uniqueRegistrations   atomic.Int64
	marketEnabled         atomic.Int64
	marketDisabled        atomic.Int64
	retentionRate         atomic.Float64
	conversionRate        atomic.Float64
	churnRate             atomic.Float64
	revenuePerUser        atomic.Float64

	// Circuit breaker
	BreakerStateMetric *prometheus.CounterVec
}

// New создаёт все метрики через OTel Meter.
// serviceName определяет namespace (например "auth", "order", "spot").
func New(serviceName string) (*Metrics, error) {
	meter := otel.Meter(serviceName)

	m := &Metrics{}
	var err error

	// gRPC метрики
	m.RequestTotal, err = meter.Int64Counter("grpc_requests_total",
		metric.WithDescription("Total number of gRPC requests"),
	)
	if err != nil {
		return nil, err
	}

	m.RequestLatency, err = meter.Float64Histogram("grpc_request_duration_seconds",
		metric.WithDescription("gRPC request latency in seconds"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5),
	)
	if err != nil {
		return nil, err
	}

	m.RequestErrors, err = meter.Int64Counter("grpc_request_errors_total",
		metric.WithDescription("Total number of failed gRPC requests"),
	)
	if err != nil {
		return nil, err
	}

	// Auth метрики
	m.LoginsTotal, err = meter.Int64Counter("auth_logins_total",
		metric.WithDescription("Total number of login attempts"),
	)
	if err != nil {
		return nil, err
	}

	m.RegistrationsTotal, err = meter.Int64Counter("auth_registrations_total",
		metric.WithDescription("Total number of user registrations"),
	)
	if err != nil {
		return nil, err
	}

	m.TokenRefreshTotal, err = meter.Int64Counter("auth_token_refreshes_total",
		metric.WithDescription("Total number of token refreshes"),
	)
	if err != nil {
		return nil, err
	}

	m.RefreshTokenRetention, err = meter.Int64ObservableGauge("auth_refresh_token_retention_seconds",
		metric.WithDescription("Retention time for refresh tokens in seconds"),
	)
	if err != nil {
		return nil, err
	}

	m.RegistrationErrors, err = meter.Int64Counter("auth_registration_errors_total",
		metric.WithDescription("Total number of registration errors"),
	)
	if err != nil {
		return nil, err
	}

	m.LoginErrors, err = meter.Int64Counter("auth_login_errors_total",
		metric.WithDescription("Total number of login errors"),
	)
	if err != nil {
		return nil, err
	}

	m.ActiveUsersGauge, err = meter.Int64ObservableGauge("auth_active_users",
		metric.WithDescription("Number of active users (logged in within period)"),
	)
	if err != nil {
		return nil, err
	}

	m.UniqueRegistrationsGauge, err = meter.Int64ObservableGauge("auth_unique_registrations",
		metric.WithDescription("Number of unique registered users"),
	)
	if err != nil {
		return nil, err
	}

	// Order метрики
	m.OrdersCreatedTotal, err = meter.Int64Counter("orders_created_total",
		metric.WithDescription("Total number of created orders"),
	)
	if err != nil {
		return nil, err
	}

	m.OrdersCancelledTotal, err = meter.Int64Counter("orders_cancelled_total",
		metric.WithDescription("Total number of cancelled orders"),
	)
	if err != nil {
		return nil, err
	}

	m.OrdersByType, err = meter.Int64Counter("orders_by_type_total",
		metric.WithDescription("Orders created by type (limit, market)"),
	)
	if err != nil {
		return nil, err
	}

	m.OrdersByMarket, err = meter.Int64Counter("orders_by_market_total",
		metric.WithDescription("Orders created by market"),
	)
	if err != nil {
		return nil, err
	}

	m.OrdersByStatus, err = meter.Int64Counter("orders_by_status_total",
		metric.WithDescription("Orders created by status"),
	)
	if err != nil {
		return nil, err
	}

	m.OrderExecutionLatency, err = meter.Float64Histogram("order_execution_latency_seconds",
		metric.WithDescription("Order execution latency in seconds"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5),
	)
	if err != nil {
		return nil, err
	}

	m.OrderCancelErrors, err = meter.Int64Counter("order_cancel_errors_total",
		metric.WithDescription("Total number of order cancel errors"),
	)
	if err != nil {
		return nil, err
	}

	m.OrderExecutionErrors, err = meter.Int64Counter("order_execution_errors_total",
		metric.WithDescription("Total number of order execution errors"),
	)
	if err != nil {
		return nil, err
	}

	m.AverageOrderValue, err = meter.Float64UpDownCounter("average_order_value",
		metric.WithDescription("Average order value"),
	)
	if err != nil {
		return nil, err
	}

	m.TotalRevenue, err = meter.Float64Counter("total_revenue",
		metric.WithDescription("Total revenue"),
	)
	if err != nil {
		return nil, err
	}

	// Spot метрики
	m.MarketQueriesTotal, err = meter.Int64Counter("market_queries_total",
		metric.WithDescription("Total number of market queries"),
	)
	if err != nil {
		return nil, err
	}

	m.ActiveMarketsGauge, err = meter.Int64UpDownCounter("active_markets",
		metric.WithDescription("Number of active/enabled markets"),
	)
	if err != nil {
		return nil, err
	}

	m.MarketEnabledGauge, err = meter.Int64ObservableGauge("spot_markets_enabled",
		metric.WithDescription("Number of enabled markets"),
	)
	if err != nil {
		return nil, err
	}

	m.MarketDisabledGauge, err = meter.Int64ObservableGauge("spot_markets_disabled",
		metric.WithDescription("Number of disabled markets"),
	)
	if err != nil {
		return nil, err
	}

	// Бизнес-метрики
	m.RetentionRate, err = meter.Float64ObservableGauge("user_retention_rate",
		metric.WithDescription("User retention rate"),
	)
	if err != nil {
		return nil, err
	}

	m.ConversionRate, err = meter.Float64ObservableGauge("user_conversion_rate",
		metric.WithDescription("User conversion rate"),
	)
	if err != nil {
		return nil, err
	}

	m.ChurnRate, err = meter.Float64ObservableGauge("user_churn_rate",
		metric.WithDescription("User churn rate"),
	)
	if err != nil {
		return nil, err
	}

	m.AverageSessionTime, err = meter.Float64Histogram("average_session_time_seconds",
		metric.WithDescription("Average session time in seconds"),
		metric.WithExplicitBucketBoundaries(10, 30, 60, 120, 300, 600, 1200, 3600),
	)
	if err != nil {
		return nil, err
	}

	m.ErrorRateByEndpoint, err = meter.Int64Counter("error_rate_by_endpoint_total",
		metric.WithDescription("Total number of errors by endpoint"),
	)
	if err != nil {
		return nil, err
	}

	m.RevenuePerUser, err = meter.Float64ObservableGauge("revenue_per_user",
		metric.WithDescription("Revenue per user"),
	)
	if err != nil {
		return nil, err
	}

	m.OrderFulfillmentTime, err = meter.Float64Histogram("order_fulfillment_time_seconds",
		metric.WithDescription("Order fulfillment time in seconds"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5),
	)
	if err != nil {
		return nil, err
	}

	m.MarketActivity, err = meter.Int64Counter("market_activity_total",
		metric.WithDescription("Total market activity"),
	)
	if err != nil {
		return nil, err
	}

	// Circuit breaker
	m.BreakerStateMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "circuit_breaker_state_changes_total",
			Help: "Total circuit breaker state changes",
		},
		[]string{"name", "from", "to"},
	)
	prometheus.MustRegister(m.BreakerStateMetric)

	// Register callbacks for observable gauges
	_, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			o.ObserveInt64(m.RefreshTokenRetention, m.refreshTokenRetention.Load())
			o.ObserveInt64(m.ActiveUsersGauge, m.activeUsers.Load())
			o.ObserveInt64(m.UniqueRegistrationsGauge, m.uniqueRegistrations.Load())
			o.ObserveInt64(m.MarketEnabledGauge, m.marketEnabled.Load())
			o.ObserveInt64(m.MarketDisabledGauge, m.marketDisabled.Load())
			o.ObserveFloat64(m.RetentionRate, m.retentionRate.Load())
			o.ObserveFloat64(m.ConversionRate, m.conversionRate.Load())
			o.ObserveFloat64(m.ChurnRate, m.churnRate.Load())
			o.ObserveFloat64(m.RevenuePerUser, m.revenuePerUser.Load())
			return nil
		},
		m.RefreshTokenRetention,
		m.ActiveUsersGauge,
		m.UniqueRegistrationsGauge,
		m.MarketEnabledGauge,
		m.MarketDisabledGauge,
		m.RetentionRate,
		m.ConversionRate,
		m.ChurnRate,
		m.RevenuePerUser,
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// Хелперы для записи метрик

// RecordRequest записывает метрику gRPC-запроса с типом ошибки
func (m *Metrics) RecordRequest(ctx context.Context, method string, code string, errorType string, duration time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("status", code),
		attribute.String("error_type", errorType),
	)
	m.RequestTotal.Add(ctx, 1, attrs)
	m.RequestLatency.Record(ctx, duration.Seconds(), attrs)

	if code != "OK" {
		m.RequestErrors.Add(ctx, 1, attrs)
	}
}

// IncLogin увеличивает счётчик логинов
func (m *Metrics) IncLogin(ctx context.Context, success bool) {
	m.LoginsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool("success", success),
	))
}

// IncRegistration увеличивает счётчик регистраций
func (m *Metrics) IncRegistration(ctx context.Context, success bool) {
	m.RegistrationsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool("success", success),
	))
}

// IncTokenRefresh увеличивает счётчик рефреша токенов
func (m *Metrics) IncTokenRefresh(ctx context.Context, success bool) {
	m.TokenRefreshTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool("success", success),
	))
}

// SetActiveUsersGauge устанавливает количество активных пользователей
func (m *Metrics) SetActiveUsersGauge(ctx context.Context, count float64) {
	m.activeUsers.Store(int64(count))
}

// SetUniqueRegistrationsGauge устанавливает количество уникальных регистраций
func (m *Metrics) SetUniqueRegistrationsGauge(ctx context.Context, count float64) {
	m.uniqueRegistrations.Store(int64(count))
}

// IncOrderCreated увеличивает счётчик созданных ордеров
func (m *Metrics) IncOrderCreated(ctx context.Context, marketID, orderType string) {
	m.OrdersCreatedTotal.Add(ctx, 1)
	m.OrdersByType.Add(ctx, 1, metric.WithAttributes(
		attribute.String("type", orderType),
	))
	m.OrdersByMarket.Add(ctx, 1, metric.WithAttributes(
		attribute.String("market", marketID),
	))
}

// IncOrderStatus увеличивает счётчик ордеров по статусу
func (m *Metrics) IncOrderStatus(ctx context.Context, status string) {
	m.OrdersByStatus.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
	))
}

// IncOrderCancelled увеличивает счётчик отменённых ордеров
func (m *Metrics) IncOrderCancelled(ctx context.Context) {
	m.OrdersCancelledTotal.Add(ctx, 1)
}

// IncMarketQuery увеличивает счётчик запросов рынков
func (m *Metrics) IncMarketQuery(ctx context.Context) {
	m.MarketQueriesTotal.Add(ctx, 1)
}

// IncMarketActivity увеличивает счётчик активности рынков
func (m *Metrics) IncMarketActivity(ctx context.Context) {
	m.MarketActivity.Add(ctx, 1)
}

// IncErrorRateByEndpoint увеличивает счётчик ошибок по эндпоинту
func (m *Metrics) IncErrorRateByEndpoint(ctx context.Context, endpoint string) {
	m.ErrorRateByEndpoint.Add(ctx, 1, metric.WithAttributes(attribute.String("endpoint", endpoint)))
}

// RecordAverageSessionTime записывает длительность сессии
func (m *Metrics) RecordAverageSessionTime(ctx context.Context, durationSeconds float64) {
	m.AverageSessionTime.Record(ctx, durationSeconds)
}

// SetRetentionRate устанавливает уровень удержания пользователей
func (m *Metrics) SetRetentionRate(value float64) {
	m.retentionRate.Store(value)
}

// SetConversionRate устанавливает уровень конверсии
func (m *Metrics) SetConversionRate(value float64) {
	m.conversionRate.Store(value)
}

// SetChurnRate устанавливает уровень оттока
func (m *Metrics) SetChurnRate(value float64) {
	m.churnRate.Store(value)
}

// SetRevenuePerUser устанавливает доход на пользователя
func (m *Metrics) SetRevenuePerUser(value float64) {
	m.revenuePerUser.Store(value)
}

// RecordOrderFulfillmentTime записывает время выполнения ордера
func (m *Metrics) RecordOrderFulfillmentTime(ctx context.Context, durationSeconds float64) {
	m.OrderFulfillmentTime.Record(ctx, durationSeconds)
}

// SetActiveMarkets изменяет счётчик активных рынков (delta: +N или -N)
func (m *Metrics) SetActiveMarkets(ctx context.Context, delta int64) {
	m.ActiveMarketsGauge.Add(ctx, delta)
}

// SetMarketEnabledGauge устанавливает количество включенных рынков
func (m *Metrics) SetMarketEnabledGauge(ctx context.Context, value int64) {
	m.marketEnabled.Store(value)
}

// SetMarketDisabledGauge устанавливает количество отключенных рынков
func (m *Metrics) SetMarketDisabledGauge(ctx context.Context, value int64) {
	m.marketDisabled.Store(value)
}

// RecordBreakerStateChange теперь метод Metrics
func (m *Metrics) RecordBreakerStateChange(name, from, to string) {
	m.BreakerStateMetric.WithLabelValues(name, from, to).Inc()
}

// RecordRegistrationError увеличивает счётчик ошибок регистрации по типу
func (m *Metrics) RecordRegistrationError(ctx context.Context, errorType string) {
	m.RegistrationErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("type", errorType)))
}

// RecordLoginError увеличивает счётчик ошибок логина по типу
func (m *Metrics) RecordLoginError(ctx context.Context, errorType string) {
	m.LoginErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("type", errorType)))
}

// Order
func (m *Metrics) RecordOrderExecutionLatency(ctx context.Context, latencySeconds float64) {
	m.OrderExecutionLatency.Record(ctx, latencySeconds)
}

func (m *Metrics) IncOrderCancelError(ctx context.Context) {
	m.OrderCancelErrors.Add(ctx, 1)
}

func (m *Metrics) IncOrderExecutionError(ctx context.Context) {
	m.OrderExecutionErrors.Add(ctx, 1)
}

// RecordAverageOrderValue устанавливает средний чек
func (m *Metrics) RecordAverageOrderValue(ctx context.Context, value float64) {
	m.AverageOrderValue.Add(ctx, value)
}

// IncTotalRevenue увеличивает общий доход
func (m *Metrics) IncTotalRevenue(ctx context.Context, amount float64) {
	m.TotalRevenue.Add(ctx, amount)
}
