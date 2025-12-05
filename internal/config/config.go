package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the main application configuration
type Config struct {
	BindAddress          string      `toml:"bind_address"`
	DownloadDirectory    string      `toml:"download_directory"`
	DownloadWorkers      int         `toml:"download_workers"`
	Loglevel             string      `toml:"loglevel"`
	OrchestrationWorkers int         `toml:"orchestration_workers"`
	Password             string      `toml:"password"`
	PollingInterval      int         `toml:"polling_interval"`
	Port                 int         `toml:"port"`
	SkipDirectories      []string    `toml:"skip_directories"`
	UID                  int         `toml:"uid"`
	Username             string      `toml:"username"`
	Putio                PutioConfig `toml:"putio"`
	Sonarr               *ArrConfig  `toml:"sonarr"`
	Radarr               *ArrConfig  `toml:"radarr"`
	Whisparr             *ArrConfig  `toml:"whisparr"`
}

// PutioConfig holds put.io API configuration
type PutioConfig struct {
	APIKey string `toml:"api_key"`
}

// ArrConfig holds sonarr/radarr/whisparr configuration
type ArrConfig struct {
	URL    string `toml:"url"`
	APIKey string `toml:"api_key"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		BindAddress:          "0.0.0.0",
		DownloadWorkers:      4,
		OrchestrationWorkers: 10,
		Loglevel:             "info",
		PollingInterval:      10,
		Port:                 9091,
		UID:                  1000,
		SkipDirectories:      []string{"sample", "extras"},
	}
}

// DefaultConfigPath returns the default configuration file path
func DefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Use XDG config directory on Linux, Application Support on macOS
	configDir := filepath.Join(homeDir, ".config", "putioarr")

	return filepath.Join(configDir, "config.toml"), nil
}

// Load loads configuration from a TOML file
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Username == "" {
		return fmt.Errorf("username is required")
	}
	if c.Password == "" {
		return fmt.Errorf("password is required")
	}
	if c.DownloadDirectory == "" {
		return fmt.Errorf("download_directory is required")
	}
	if c.Putio.APIKey == "" {
		return fmt.Errorf("putio.api_key is required")
	}
	if c.Sonarr == nil && c.Radarr == nil && c.Whisparr == nil {
		return fmt.Errorf("at least one of sonarr, radarr, or whisparr must be configured")
	}

	return nil
}

// GetArrConfigs returns a list of configured arr services
func (c *Config) GetArrConfigs() []struct {
	Name   string
	URL    string
	APIKey string
} {
	var configs []struct {
		Name   string
		URL    string
		APIKey string
	}

	if c.Sonarr != nil {
		configs = append(configs, struct {
			Name   string
			URL    string
			APIKey string
		}{"Sonarr", c.Sonarr.URL, c.Sonarr.APIKey})
	}
	if c.Radarr != nil {
		configs = append(configs, struct {
			Name   string
			URL    string
			APIKey string
		}{"Radarr", c.Radarr.URL, c.Radarr.APIKey})
	}
	if c.Whisparr != nil {
		configs = append(configs, struct {
			Name   string
			URL    string
			APIKey string
		}{"Whisparr", c.Whisparr.URL, c.Whisparr.APIKey})
	}

	return configs
}
