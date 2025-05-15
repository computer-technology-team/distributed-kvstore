package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// FlagConfig defines a mapping between a flag and its viper configuration
type FlagConfig struct {
	FlagName string
	ViperKey string
	Default  interface{}
	Usage    string
}

// Config represents the application configuration
type Config struct {
	LogLevel slog.Level `mapstructure:"log_level"`
	Server   struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"server"`

	Client struct {
		ServerURL string `mapstructure:"server_url"`
	} `mapstructure:"client"`
}

// ConfigFlags defines all the configuration flags for the application
var ConfigFlags = []FlagConfig{
	{"log-level", "log_level", slog.LevelInfo, "Log level (debug, info, warn, error)"},
	{"server.host", "server.host", "localhost", "Server host"},
	{"server.port", "server.port", 8080, "Server port"},
	{"client.server-url", "client.server_url", "", "KVStore server URL for client commands"},
}

// InitViper initializes a new Viper instance with default settings
func InitViper(configFile string) *viper.Viper {
	v := viper.New()

	// Set up config file settings
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.dist-kv")
	v.AddConfigPath("/etc/dist-kv")


	// Override with specific config file if provided
	if configFile != "" {
		v.SetConfigFile(configFile)
	}

	// Configure environment variables
	v.SetEnvPrefix("DIST_KV")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()

	// Set default values
	for _, fc := range ConfigFlags {
		v.SetDefault(fc.ViperKey, fc.Default)
	}

	return v
}

// AddFlags adds configuration flags to the given cobra command
func AddFlags(cmd *cobra.Command) {
	// Add config file flag
	cmd.Flags().String("config", "", "Config file path")

	// Add flags for all configuration options using the global ConfigFlags
	for _, fc := range ConfigFlags {
		switch v := fc.Default.(type) {
		case string:
			cmd.Flags().String(fc.FlagName, v, fc.Usage)
		case int:
			cmd.Flags().Int(fc.FlagName, v, fc.Usage)
		case bool:
			cmd.Flags().Bool(fc.FlagName, v, fc.Usage)
		case float64:
			cmd.Flags().Float64(fc.FlagName, v, fc.Usage)
		default:
			slog.Error("invalid value type", "value", v)
		}
	}
}

// BindFlags binds cobra flags to viper configuration
func BindFlags(cmd *cobra.Command, v *viper.Viper) {
	// Bind flags to viper keys
	for _, fc := range ConfigFlags {
		flag := cmd.Flags().Lookup(fc.FlagName)
		if flag != nil {
			v.BindPFlag(fc.ViperKey, flag)
		}
	}
}

// LoadConfig loads the configuration from all sources and returns a Config struct
func LoadConfig(cmd *cobra.Command) (*Config, error) {
	// Initialize viper

	var configFile string
	// Check if config file is specified via flag
	if cmd != nil && cmd.Flag("config") != nil && cmd.Flag("config").Changed {
		tmpConfigFile := cmd.Flag("config").Value.String()
		if tmpConfigFile != "" {
			configFile = tmpConfigFile
		}
	}

	v := InitViper(configFile)

	// Add flags to the command if provided
	if cmd != nil {
		AddFlags(cmd)
		BindFlags(cmd, v)
	}

	// Try to read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		slog.Info("No config file found, using defaults, environment variables, and flags")
	} else {
		slog.Info("Using config file", "path", v.ConfigFileUsed())
	}

	// Create a new config with default values
	var config Config

	// Apply the configuration to our struct
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
