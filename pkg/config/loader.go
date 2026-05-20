package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/sony/gobreaker"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type LoggerConfig struct {
	SlowThreshold     time.Duration `mapstructure:"slow_threshold"`
	LogNormalRequests bool          `mapstructure:"log_normal_requests"`
}

type BaseConfig struct {
	Server            Server                   `mapstructure:"server"`
	Database          Database                 `mapstructure:"database"`
	Redis             Redis                    `mapstructure:"redis"`
	JWT               JWT                      `mapstructure:"jwt"`
	RateLimit         RateLimit                `mapstructure:"rate_limit"`
	CircuitBreaker    CircuitBreaker           `mapstructure:"circuit_breaker"`
	GrpcBackoff       GrpcBackoff              `mapstructure:"grpc_backoff"`
	Logger            LoggerConfig             `mapstructure:"logger"`
	OperationTimeouts map[string]time.Duration `mapstructure:"operation_timeouts"`
}

type Server struct {
	Host               string        `mapstructure:"host"`
	Port               int           `mapstructure:"port"`
	MetricsPort        int           `mapstructure:"metrics_port"`
	ShutdownTimeout    time.Duration `mapstructure:"shutdown_timeout"`
	HealthEnabled      bool          `mapstructure:"health_enabled"`
	HealthCheckTimeout time.Duration `mapstructure:"health_check_timeout"`
}

type Database struct {
	Driver            string        `mapstructure:"driver"`
	DSN               string        `mapstructure:"dsn"`
	MaxConns          int32         `mapstructure:"max_conns"`
	MinConns          int32         `mapstructure:"min_conns"`
	MaxConnLifetime   time.Duration `mapstructure:"max_conn_lifetime"`
	MaxConnIdleTime   time.Duration `mapstructure:"max_conn_idle_time"`
	HealthCheckPeriod time.Duration `mapstructure:"health_check_period"`
	RetryTimeout      time.Duration `mapstructure:"retry_timeout"`
	RetryInterval     time.Duration `mapstructure:"retry_interval"`
	MaxRetries        int           `mapstructure:"max_retries"`
}

type Redis struct {
	Addr            string        `mapstructure:"addr"`
	Password        string        `mapstructure:"password"`
	DB              int           `mapstructure:"db"`
	PoolSize        int           `mapstructure:"pool_size"`
	RetryTimeout    time.Duration `mapstructure:"retry_timeout"`
	RetryInterval   time.Duration `mapstructure:"retry_interval"`
	MaxRetries      int           `mapstructure:"max_retries"`
	MinRetryBackoff time.Duration `mapstructure:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `mapstructure:"max_retry_backoff"`
}

type JWT struct {
	Secret string `mapstructure:"secret"`
}

type RateLimit struct {
	Global struct {
		RPS   float64 `mapstructure:"rps"`
		Burst int     `mapstructure:"burst"`
	} `mapstructure:"global"`
	PerMethod map[string]struct {
		RPS   float64 `mapstructure:"rps"`
		Burst int     `mapstructure:"burst"`
	} `mapstructure:"per_method"`
	PerUser struct {
		RPS            float64       `mapstructure:"rps"`
		Burst          int           `mapstructure:"burst"`
		MaxInactiveAge time.Duration `mapstructure:"max_inactive_age"`
	} `mapstructure:"per_user"`
}

type CircuitBreaker struct {
	Enabled      bool          `mapstructure:"enabled"`
	MaxFailures  uint32        `mapstructure:"max_failures"`
	Timeout      time.Duration `mapstructure:"timeout"`
	Attempts     uint32        `mapstructure:"attempts"`
	Interval     time.Duration `mapstructure:"interval"`
	MaxRequests  uint32        `mapstructure:"max_requests"`
	RetryDelay   time.Duration `mapstructure:"retry_delay"`
	MinRequests  uint32        `mapstructure:"min_requests"`
	FailureRatio float64       `mapstructure:"failure_ratio"`
	ReadyToTrip  func(gobreaker.Counts) bool
}

type GrpcBackoff struct {
	Enabled     bool          `mapstructure:"enabled"`
	BaseDelay   time.Duration `mapstructure:"base_delay"`
	Multiplier  float64       `mapstructure:"multiplier"`
	MaxDelay    time.Duration `mapstructure:"max_delay"`
	MaxAttempts uint64        `mapstructure:"max_attempts"`
}

func LoadBase(path string, envPrefix string, logger *zap.Logger) *BaseConfig {
	viper.SetConfigFile(path)
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Явные биндинги для ключевых полей, чтобы env-переменные
	// вида PREFIX_DATABASE_DSN гарантированно перезаписывали конфиг
	_ = viper.BindEnv("database.dsn")
	_ = viper.BindEnv("redis.addr")
	_ = viper.BindEnv("redis.password")
	_ = viper.BindEnv("redis.db")
	_ = viper.BindEnv("jwt.secret")

	if err := viper.ReadInConfig(); err != nil {
		logger.Fatal("failed to read config", zap.Error(err))
	}

	var cfg BaseConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		logger.Fatal("failed to unmarshal config", zap.Error(err))
	}

	// viper.Unmarshal не подхватывает env для вложенных структур,
	// поэтому перезаписываем вручную
	if v := viper.GetString("database.dsn"); v != "" {
		cfg.Database.DSN = v
	}
	if v := viper.GetString("redis.addr"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := viper.GetString("redis.password"); v != "" {
		cfg.Redis.Password = v
	}
	if v := viper.GetInt("redis.db"); v != 0 {
		cfg.Redis.DB = v
	}
	if v := viper.GetString("jwt.secret"); v != "" {
		cfg.JWT.Secret = v
	}

	// Дефолты
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.ShutdownTimeout == 0 {
		cfg.Server.ShutdownTimeout = 30 * time.Second
	}
	if cfg.Server.MetricsPort == 0 {
		cfg.Server.MetricsPort = 9090
	}
	if cfg.CircuitBreaker.MaxFailures == 0 {
		cfg.CircuitBreaker.MaxFailures = 5
	}
	if cfg.CircuitBreaker.Timeout == 0 {
		cfg.CircuitBreaker.Timeout = 5 * time.Second
	}
	if cfg.CircuitBreaker.Attempts == 0 {
		cfg.CircuitBreaker.Attempts = 3
	}
	if cfg.CircuitBreaker.Interval == 0 {
		cfg.CircuitBreaker.Interval = 30 * time.Second
	}
	if cfg.CircuitBreaker.MaxRequests == 0 {
		cfg.CircuitBreaker.MaxRequests = 3
	}
	if cfg.CircuitBreaker.RetryDelay == 0 {
		cfg.CircuitBreaker.RetryDelay = 100 * time.Millisecond
	}
	if cfg.CircuitBreaker.MinRequests == 0 {
		cfg.CircuitBreaker.MinRequests = 10
	}
	if cfg.CircuitBreaker.FailureRatio == 0 {
		cfg.CircuitBreaker.FailureRatio = 0.6
	}

	// Дефолты для GrpcBackoff
	if cfg.GrpcBackoff.BaseDelay == 0 {
		cfg.GrpcBackoff.BaseDelay = 200 * time.Millisecond
	}
	if cfg.GrpcBackoff.MaxDelay == 0 {
		cfg.GrpcBackoff.MaxDelay = 15 * time.Second
	}
	if cfg.GrpcBackoff.MaxAttempts == 0 {
		cfg.GrpcBackoff.MaxAttempts = 5
	}
	if cfg.GrpcBackoff.Multiplier == 0 {
		cfg.GrpcBackoff.Multiplier = 2.0
	}
	if !cfg.GrpcBackoff.Enabled {
		cfg.GrpcBackoff.Enabled = true
	}

	// Дефолты для OperationTimeouts
	if cfg.OperationTimeouts == nil {
		cfg.OperationTimeouts = make(map[string]time.Duration)
	}
	// Можно установить какие-то общие таймауты, но оставим пустыми

	return &cfg
}

// Load загружает конфигурацию из файла и environment variables
// configPath - путь к конфигурационному файлу (например, "configs/config.yaml")
// cfg - структура для unmarshalling конфигурации
func Load(configPath string, cfg any) error {
	v := viper.New()

	// Установка пути к конфигу
	v.SetConfigFile(configPath)

	// Чтение конфига из файла
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Environment variables
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Unmarshal в структуру
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// LoadWithDefaults загружает конфиг с возможностью использования дефолтных значений
func LoadWithDefaults(configPath string, cfg any, defaults map[string]any) error {
	v := viper.New()

	// Установка дефолтных значений
	for key, value := range defaults {
		v.SetDefault(key, value)
	}

	v.SetConfigFile(configPath)

	// Попытка прочитать файл (не критично если файла нет)
	_ = v.ReadInConfig()

	// Environment variables
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}
