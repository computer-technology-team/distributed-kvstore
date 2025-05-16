package config

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func logLevelDecodeHookFunc(input, target reflect.Type, data interface{}) (interface{}, error) {
	if input.Kind() != reflect.String {
		return data, nil
	}

	if target.Kind() != reflect.TypeOf(LogLevel{}).Kind() {
		return data, nil
	}

	var level slog.Level

	err := level.UnmarshalText([]byte(data.(string)))
	if err != nil {
		return nil, fmt.Errorf("error in unmarshalling log level: %w", err)
	}

	return LogLevel{level}, nil
}

// FlagConfig defines a mapping between a flag and its viper configuration
type FlagConfig struct {
	FlagName string
	ViperKey string
	Default  interface{}
	Usage    string
}

// NodeConfig represents the configuration for a node server
type NodeConfig struct {
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	ControllerURL string `mapstructure:"controller_url"`
}

// ClientConfig represents the configuration for a client
type ClientConfig struct {
	ServerURL string `mapstructure:"server_url"`
}

// LoadBalancerConfig represents the configuration for the load balancer
type LoadBalancerConfig struct {
	PublicServer struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"public_server"`
	PrivateServer struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"private_server"`
	ControllerURL string `mapstructure:"controller_url"`
}

// ControllerConfig represents the configuration for the controller
type ControllerConfig struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	AdminUI struct {
		Enabled bool   `mapstructure:"enabled"`
		Host    string `mapstructure:"host"`
		Port    int    `mapstructure:"port"`
	} `mapstructure:"admin_ui"`
	LoadBalancerURL     string        `mapstructure:"load_balancer_url"`
	HealthCheckDuration time.Duration `mapstructure:"health_check_duration"`
	HealthCheckTimeout  time.Duration `mapstructure:"health_check_timeout"`
}

type LogLevel struct {
	Level slog.Level
}

// Config represents the application configuration
type Config struct {
	LogLevel     LogLevel           `mapstructure:"log_level"`
	Node         NodeConfig         `mapstructure:"node"`
	Client       ClientConfig       `mapstructure:"client"`
	Controller   ControllerConfig   `mapstructure:"controller"`
	LoadBalancer LoadBalancerConfig `mapstructure:"load_balancer"`
}

// ConfigFlags defines all the configuration flags for the application
var ConfigFlags = []FlagConfig{
	{"log-level", "log_level", slog.LevelInfo, "Log level (debug, info, warn, error)"},
	{"node.host", "node.host", "localhost", "Node server host"},
	{"node.port", "node.port", 8080, "Node server port"},
	{"client.server-url", "client.server_url", "", "KVStore server URL for client commands"},
	{"controller.host", "controller.host", "localhost", "Controller host"},
	{"controller.port", "controller.port", 9090, "Controller port"},
	{"controller.admin-ui.enabled", "controller.admin_ui.enabled", true, "Enable admin UI"},
	{"controller.admin-ui.host", "controller.admin_ui.host", "localhost", "Admin UI host"},
	{"controller.admin-ui.port", "controller.admin_ui.port", 9091, "Admin UI port"},
	{"controller.load_balancer_url", "controller.load_balancer_url", "http://localhost:8001", "Load Balancer URL"},
	{"controller.health_check_duration", "controller.health_check_duration", time.Second * 5, "Health Check Duration"},
	{"controller.health_check_timeout", "controller.health_check_timeout", time.Second * 2, "Health Check Timeout"},
	{"load-balancer.public-server.host", "load_balancer.public_server.host", "localhost", "Load balancer public server host"},
	{"load-balancer.public-server.port", "load_balancer.public_server.port", 8000, "Load balancer public server port"},
	{"load-balancer.private-server.host", "load_balancer.private_server.host", "localhost", "Load balancer private server host"},
	{"load-balancer.private-server.port", "load_balancer.private_server.port", 8001, "Load balancer private server port"},
}

// initViper initializes a new Viper instance with default settings
func initViper(configFile string) *viper.Viper {
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
	cmd.PersistentFlags().String("config", "", "Config file path")

	// Add flags for all configuration options using the global ConfigFlags
	for _, fc := range ConfigFlags {
		switch v := fc.Default.(type) {
		case string:
			cmd.PersistentFlags().String(fc.FlagName, v, fc.Usage)
		case int:
			cmd.PersistentFlags().Int(fc.FlagName, v, fc.Usage)
		case bool:
			cmd.PersistentFlags().Bool(fc.FlagName, v, fc.Usage)
		case float64:
			cmd.PersistentFlags().Float64(fc.FlagName, v, fc.Usage)
		case slog.Level:
			cmd.PersistentFlags().String(fc.FlagName, v.String(), fc.Usage)
		default:
			slog.Error("invalid value type", "value", v)
		}
	}
}

func LoadConfig(flags *pflag.FlagSet) (*Config, error) {
	configFile, err := flags.GetString("config")
	if err != nil {
		return nil, fmt.Errorf("could not get config flag value: %w", err)
	}

	v := initViper(configFile)

	err = v.BindPFlags(flags)
	if err != nil {
		slog.Error("could not bind flags", "error", err)
		return nil, fmt.Errorf("could not bind flags to viper: %w", err)
	}

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
	if err := v.Unmarshal(&config, viper.DecodeHook(logLevelDecodeHookFunc)); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
