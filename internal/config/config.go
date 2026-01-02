package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
)

const (
	MinPollingInterval      = 1
	MaxPollingInterval      = 3600
	MinDownloadWorkers      = 1
	MaxDownloadWorkers      = 100
	MinOrchestrationWorkers = 1
	MaxOrchestrationWorkers = 100
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

	info, err := os.Stat(c.DownloadDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("download_directory does not exist: %s", c.DownloadDirectory)
		}
		return fmt.Errorf("unable to stat download_directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("download_directory is not a directory: %s", c.DownloadDirectory)
	}
	tmpFile, err := os.CreateTemp(c.DownloadDirectory, ".goputioarr-perm-*")
	if err != nil {
		return fmt.Errorf("download_directory is not writable: %w", err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name())

	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if _, err := logrus.ParseLevel(c.Loglevel); err != nil {
		return fmt.Errorf("loglevel must be one of: panic, fatal, error, warn, info, debug, trace")
	}

	if c.Putio.APIKey == "" {
		return fmt.Errorf("putio.api_key is required")
	}
	if c.Sonarr == nil && c.Radarr == nil && c.Whisparr == nil {
		return fmt.Errorf("at least one of sonarr, radarr, or whisparr must be configured")
	}

	validateArr := func(name string, cfg *ArrConfig) error {
		if cfg.URL == "" {
			return fmt.Errorf("%s.url is required", name)
		}
		if _, err := url.ParseRequestURI(cfg.URL); err != nil {
			return fmt.Errorf("%s.url is invalid: %v", name, err)
		}
		if cfg.APIKey == "" {
			return fmt.Errorf("%s.api_key is required", name)
		}
		return nil
	}

	if c.Sonarr != nil {
		if err := validateArr("sonarr", c.Sonarr); err != nil {
			return err
		}
	}
	if c.Radarr != nil {
		if err := validateArr("radarr", c.Radarr); err != nil {
			return err
		}
	}
	if c.Whisparr != nil {
		if err := validateArr("whisparr", c.Whisparr); err != nil {
			return err
		}
	}

	if c.PollingInterval < MinPollingInterval || c.PollingInterval > MaxPollingInterval {
		return fmt.Errorf("polling_interval must be between %d and %d seconds", MinPollingInterval, MaxPollingInterval)
	}
	if c.DownloadWorkers < MinDownloadWorkers || c.DownloadWorkers > MaxDownloadWorkers {
		return fmt.Errorf("download_workers must be between %d and %d", MinDownloadWorkers, MaxDownloadWorkers)
	}
	if c.OrchestrationWorkers < MinOrchestrationWorkers || c.OrchestrationWorkers > MaxOrchestrationWorkers {
		return fmt.Errorf("orchestration_workers must be between %d and %d", MinOrchestrationWorkers, MaxOrchestrationWorkers)
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
