package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// LogLevel is a custom type that can be unmarshaled from string
type LogLevel struct {
	Level slog.Level
}

// UnmarshalText implements encoding.TextUnmarshaler
func (l *LogLevel) UnmarshalText(text []byte) error {
	return l.Set(string(text))
}

// UnmarshalJSON implements json.Unmarshaler
func (l *LogLevel) UnmarshalJSON(data []byte) error {
	// Remove quotes if present
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return l.Set(s)
}

// Set implements pflag.Value
func (l *LogLevel) Set(s string) error {
	switch strings.ToLower(s) {
	case "debug":
		l.Level = slog.LevelDebug
	case "info":
		l.Level = slog.LevelInfo
	case "warn", "warning":
		l.Level = slog.LevelWarn
	case "error":
		l.Level = slog.LevelError
	default:
		// Try to parse as a number
		var level slog.Level
		if _, err := fmt.Sscanf(s, "%d", &level); err != nil {
			return fmt.Errorf("invalid log level: %s", s)
		}
		l.Level = level
	}
	return nil
}

// String implements pflag.Value and fmt.Stringer
func (l LogLevel) String() string {
	switch l.Level {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return fmt.Sprintf("%d", l.Level)
	}
}

// Type implements pflag.Value
func (l LogLevel) Type() string {
	return "string"
}

// MarshalYAML implements yaml.Marshaler
func (l LogLevel) MarshalYAML() (interface{}, error) {
	return l.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler
func (l *LogLevel) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	return l.Set(s)
}

// ParseLogLevel parses a string into a slog.Level
func ParseLogLevel(level string) (slog.Level, error) {
	var l LogLevel
	if err := l.Set(level); err != nil {
		return slog.LevelInfo, err
	}
	return l.Level, nil
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
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
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
	{"log-level", "log_level", LogLevel{Level: slog.LevelInfo}, "Log level (debug, info, warn, error)"},
	{"node.host", "node.host", "localhost", "Node server host"},
	{"node.port", "node.port", 8080, "Node server port"},
	{"client.server-url", "client.server_url", "", "KVStore server URL for client commands"},
	{"controller.host", "controller.host", "localhost", "Controller host"},
	{"controller.port", "controller.port", 9090, "Controller port"},
	{"controller.admin-ui.enabled", "controller.admin_ui.enabled", true, "Enable admin UI"},
	{"controller.admin-ui.host", "controller.admin_ui.host", "localhost", "Admin UI host"},
	{"controller.admin-ui.port", "controller.admin_ui.port", 9091, "Admin UI port"},
	{"load-balancer.public-server.host", "load_balancer.public_server.host", "localhost", "Load balancer public server host"},
	{"load-balancer.public-server.port", "load_balancer.public_server.port", 8000, "Load balancer public server port"},
	{"load-balancer.private-server.host", "load_balancer.private_server.host", "localhost", "Load balancer private server host"},
	{"load-balancer.private-server.port", "load_balancer.private_server.port", 8001, "Load balancer private server port"},
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
		case LogLevel:
			cmd.Flags().String(fc.FlagName, v.String(), fc.Usage)
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
