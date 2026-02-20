package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Level       string `yaml:"level"`
	Development bool   `yaml:"development"`
	Encoding    string `yaml:"encoding"`
}

func New(cfg Config) (*zap.Logger, error) {
	var config zap.Config

	if cfg.Development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}

	// Установка уровня логирования
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(level)

	// Установка encoding
	if cfg.Encoding != "" {
		config.Encoding = cfg.Encoding
	}

	return config.Build()
}

// NewDefault создает logger с настройками по умолчанию (production)
func NewDefault() (*zap.Logger, error) {
	return New(Config{
		Level:       "info",
		Development: false,
		Encoding:    "json",
	})
}

// NewDevelopment создает logger для разработки
func NewDevelopment() (*zap.Logger, error) {
	return New(Config{
		Level:       "debug",
		Development: true,
		Encoding:    "console",
	})
}
