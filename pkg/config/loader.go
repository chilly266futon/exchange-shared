package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

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
