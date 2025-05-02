package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	LogLevel slog.Level `mapstructure:"log_level"`
	Server   struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"server"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		LogLevel: slog.LevelInfo,
		Server: struct {
			Host string `mapstructure:"host"`
			Port int    `mapstructure:"port"`
		}{
			Host: "localhost",
			Port: 8080,
		},
	}
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configFile string) (*Config, error) {
	config := DefaultConfig()

	v := viper.New()

	v.SetDefault("log_level", config.LogLevel)
	v.SetDefault("server.host", config.Server.Host)
	v.SetDefault("server.port", config.Server.Port)

	v.SetEnvPrefix("DIST_KV")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()

	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		slog.Info("Using config file", "path", v.ConfigFileUsed())
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")

		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.dist-kv")
		v.AddConfigPath("/etc/dist-kv")

		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
			slog.Info("No config file found, using defaults and environment variables")
		} else {
			slog.Info("Using config file", "path", v.ConfigFileUsed())
		}
	}

	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}
