package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerConfig defines the configuration for the server
type ServerConfig struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	MaxConnections    int           `yaml:"max_connections"`
	ConnectionTimeout time.Duration `yaml:"conn_timeout"`
	TimeWindow        time.Duration `yaml:"time_window"`
	MinDifficulty     int           `yaml:"min_difficulty"`
	MaxDifficulty     int           `yaml:"max_difficulty"`
}

// ClientConfig defines the configuration for the client
type ClientConfig struct {
	ServerAddress     string        `yaml:"server_address"`
	ConnectionTimeout time.Duration `yaml:"conn_timeout"`
	MaxNonce          int           `yaml:"max_nonce"`
}

// AppConfig is the top-level structure to hold all configurations
type AppConfig struct {
	Server ServerConfig `yaml:"server"`
	Client ClientConfig `yaml:"client"`
}

// LoadConfig reads and parses the YAML configuration file
func LoadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
