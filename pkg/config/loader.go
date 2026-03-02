package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type BaseConfig struct {
	Server    Server    `mapstructure:"server"`
	Database  Database  `mapstructure:"database"`
	Redis     Redis     `mapstructure:"redis"`
	JWT       JWT       `mapstructure:"jwt"`
	RateLimit RateLimit `mapstructure:"rate_limit"`
}

type Server struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	HealthEnabled   bool          `mapstructure:"health_enabled"`
}

type Database struct {
	Driver string `mapstructure:"driver"`
	DSN    string `mapstructure:"dsn"`
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
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

func LoadBase(path string, envPrefix string, logger *zap.Logger) *BaseConfig {
	viper.SetConfigFile(path)
	viper.AutomaticEnv()
	viper.SetEnvPrefix(envPrefix)

	if err := viper.ReadInConfig(); err != nil {
		logger.Fatal("failed to read config", zap.Error(err))
	}

	var cfg BaseConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		logger.Fatal("failed to unmarshal config", zap.Error(err))
	}

	// Дефолты
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.ShutdownTimeout == 0 {
		cfg.Server.ShutdownTimeout = 30 * time.Second
	}

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
