package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.BindAddress != "0.0.0.0" {
		t.Errorf("expected BindAddress to be '0.0.0.0', got '%s'", cfg.BindAddress)
	}
	if cfg.DownloadWorkers != 4 {
		t.Errorf("expected DownloadWorkers to be 4, got %d", cfg.DownloadWorkers)
	}
	if cfg.OrchestrationWorkers != 10 {
		t.Errorf("expected OrchestrationWorkers to be 10, got %d", cfg.OrchestrationWorkers)
	}
	if cfg.Loglevel != "info" {
		t.Errorf("expected Loglevel to be 'info', got '%s'", cfg.Loglevel)
	}
	if cfg.PollingInterval != 10 {
		t.Errorf("expected PollingInterval to be 10, got %d", cfg.PollingInterval)
	}
	if cfg.Port != 9091 {
		t.Errorf("expected Port to be 9091, got %d", cfg.Port)
	}
	if cfg.UID != 1000 {
		t.Errorf("expected UID to be 1000, got %d", cfg.UID)
	}
	if len(cfg.SkipDirectories) != 2 {
		t.Errorf("expected 2 SkipDirectories, got %d", len(cfg.SkipDirectories))
	}
	if cfg.SkipDirectories[0] != "sample" || cfg.SkipDirectories[1] != "extras" {
		t.Errorf("unexpected SkipDirectories: %v", cfg.SkipDirectories)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
	if filepath.Base(path) != "config.toml" {
		t.Errorf("expected path to end with 'config.toml', got '%s'", filepath.Base(path))
	}
}

func TestLoad(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
username = "testuser"
password = "testpass"
download_directory = "/downloads"
bind_address = "127.0.0.1"
port = 8080
loglevel = "debug"
uid = 500
polling_interval = 5
skip_directories = ["sample"]
orchestration_workers = 5
download_workers = 2

[putio]
api_key = "test-api-key"

[sonarr]
url = "http://localhost:8989"
api_key = "sonarr-key"

[radarr]
url = "http://localhost:7878"
api_key = "radarr-key"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Check loaded values override defaults
	if cfg.Username != "testuser" {
		t.Errorf("expected Username 'testuser', got '%s'", cfg.Username)
	}
	if cfg.Password != "testpass" {
		t.Errorf("expected Password 'testpass', got '%s'", cfg.Password)
	}
	if cfg.DownloadDirectory != "/downloads" {
		t.Errorf("expected DownloadDirectory '/downloads', got '%s'", cfg.DownloadDirectory)
	}
	if cfg.BindAddress != "127.0.0.1" {
		t.Errorf("expected BindAddress '127.0.0.1', got '%s'", cfg.BindAddress)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected Port 8080, got %d", cfg.Port)
	}
	if cfg.Loglevel != "debug" {
		t.Errorf("expected Loglevel 'debug', got '%s'", cfg.Loglevel)
	}
	if cfg.UID != 500 {
		t.Errorf("expected UID 500, got %d", cfg.UID)
	}
	if cfg.PollingInterval != 5 {
		t.Errorf("expected PollingInterval 5, got %d", cfg.PollingInterval)
	}
	if cfg.OrchestrationWorkers != 5 {
		t.Errorf("expected OrchestrationWorkers 5, got %d", cfg.OrchestrationWorkers)
	}
	if cfg.DownloadWorkers != 2 {
		t.Errorf("expected DownloadWorkers 2, got %d", cfg.DownloadWorkers)
	}
	if cfg.Putio.APIKey != "test-api-key" {
		t.Errorf("expected Putio.APIKey 'test-api-key', got '%s'", cfg.Putio.APIKey)
	}
	if cfg.Sonarr == nil {
		t.Fatal("expected Sonarr config to be set")
	}
	if cfg.Sonarr.URL != "http://localhost:8989" {
		t.Errorf("expected Sonarr.URL 'http://localhost:8989', got '%s'", cfg.Sonarr.URL)
	}
	if cfg.Sonarr.APIKey != "sonarr-key" {
		t.Errorf("expected Sonarr.APIKey 'sonarr-key', got '%s'", cfg.Sonarr.APIKey)
	}
	if cfg.Radarr == nil {
		t.Fatal("expected Radarr config to be set")
	}
	if cfg.Radarr.URL != "http://localhost:7878" {
		t.Errorf("expected Radarr.URL 'http://localhost:7878', got '%s'", cfg.Radarr.URL)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.toml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	invalidContent := `
username = "test
password = incomplete
`
	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with sonarr",
			config: &Config{
				Username:          "user",
				Password:          "pass",
				DownloadDirectory: "/downloads",
				Putio:             PutioConfig{APIKey: "key"},
				Sonarr:            &ArrConfig{URL: "http://localhost", APIKey: "key"},
			},
			wantErr: false,
		},
		{
			name: "valid config with radarr",
			config: &Config{
				Username:          "user",
				Password:          "pass",
				DownloadDirectory: "/downloads",
				Putio:             PutioConfig{APIKey: "key"},
				Radarr:            &ArrConfig{URL: "http://localhost", APIKey: "key"},
			},
			wantErr: false,
		},
		{
			name: "valid config with whisparr",
			config: &Config{
				Username:          "user",
				Password:          "pass",
				DownloadDirectory: "/downloads",
				Putio:             PutioConfig{APIKey: "key"},
				Whisparr:          &ArrConfig{URL: "http://localhost", APIKey: "key"},
			},
			wantErr: false,
		},
		{
			name: "missing username",
			config: &Config{
				Password:          "pass",
				DownloadDirectory: "/downloads",
				Putio:             PutioConfig{APIKey: "key"},
				Sonarr:            &ArrConfig{URL: "http://localhost", APIKey: "key"},
			},
			wantErr: true,
			errMsg:  "username is required",
		},
		{
			name: "missing password",
			config: &Config{
				Username:          "user",
				DownloadDirectory: "/downloads",
				Putio:             PutioConfig{APIKey: "key"},
				Sonarr:            &ArrConfig{URL: "http://localhost", APIKey: "key"},
			},
			wantErr: true,
			errMsg:  "password is required",
		},
		{
			name: "missing download_directory",
			config: &Config{
				Username: "user",
				Password: "pass",
				Putio:    PutioConfig{APIKey: "key"},
				Sonarr:   &ArrConfig{URL: "http://localhost", APIKey: "key"},
			},
			wantErr: true,
			errMsg:  "download_directory is required",
		},
		{
			name: "missing putio api_key",
			config: &Config{
				Username:          "user",
				Password:          "pass",
				DownloadDirectory: "/downloads",
				Sonarr:            &ArrConfig{URL: "http://localhost", APIKey: "key"},
			},
			wantErr: true,
			errMsg:  "putio.api_key is required",
		},
		{
			name: "no arr configured",
			config: &Config{
				Username:          "user",
				Password:          "pass",
				DownloadDirectory: "/downloads",
				Putio:             PutioConfig{APIKey: "key"},
			},
			wantErr: true,
			errMsg:  "at least one of sonarr, radarr, or whisparr must be configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errMsg)
				} else if err.Error() != tt.errMsg {
					t.Errorf("expected error '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetArrConfigs(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected int
	}{
		{
			name:     "no arr configs",
			config:   &Config{},
			expected: 0,
		},
		{
			name: "only sonarr",
			config: &Config{
				Sonarr: &ArrConfig{URL: "http://sonarr", APIKey: "key1"},
			},
			expected: 1,
		},
		{
			name: "sonarr and radarr",
			config: &Config{
				Sonarr: &ArrConfig{URL: "http://sonarr", APIKey: "key1"},
				Radarr: &ArrConfig{URL: "http://radarr", APIKey: "key2"},
			},
			expected: 2,
		},
		{
			name: "all three",
			config: &Config{
				Sonarr:   &ArrConfig{URL: "http://sonarr", APIKey: "key1"},
				Radarr:   &ArrConfig{URL: "http://radarr", APIKey: "key2"},
				Whisparr: &ArrConfig{URL: "http://whisparr", APIKey: "key3"},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs := tt.config.GetArrConfigs()
			if len(configs) != tt.expected {
				t.Errorf("expected %d configs, got %d", tt.expected, len(configs))
			}
		})
	}
}

func TestGetArrConfigsContent(t *testing.T) {
	cfg := &Config{
		Sonarr:   &ArrConfig{URL: "http://sonarr:8989", APIKey: "sonarr-key"},
		Radarr:   &ArrConfig{URL: "http://radarr:7878", APIKey: "radarr-key"},
		Whisparr: &ArrConfig{URL: "http://whisparr:6969", APIKey: "whisparr-key"},
	}

	configs := cfg.GetArrConfigs()

	// Check Sonarr
	found := false
	for _, c := range configs {
		if c.Name == "Sonarr" {
			found = true
			if c.URL != "http://sonarr:8989" {
				t.Errorf("expected Sonarr URL 'http://sonarr:8989', got '%s'", c.URL)
			}
			if c.APIKey != "sonarr-key" {
				t.Errorf("expected Sonarr APIKey 'sonarr-key', got '%s'", c.APIKey)
			}
		}
	}
	if !found {
		t.Error("Sonarr config not found")
	}

	// Check Radarr
	found = false
	for _, c := range configs {
		if c.Name == "Radarr" {
			found = true
			if c.URL != "http://radarr:7878" {
				t.Errorf("expected Radarr URL 'http://radarr:7878', got '%s'", c.URL)
			}
			if c.APIKey != "radarr-key" {
				t.Errorf("expected Radarr APIKey 'radarr-key', got '%s'", c.APIKey)
			}
		}
	}
	if !found {
		t.Error("Radarr config not found")
	}

	// Check Whisparr
	found = false
	for _, c := range configs {
		if c.Name == "Whisparr" {
			found = true
			if c.URL != "http://whisparr:6969" {
				t.Errorf("expected Whisparr URL 'http://whisparr:6969', got '%s'", c.URL)
			}
			if c.APIKey != "whisparr-key" {
				t.Errorf("expected Whisparr APIKey 'whisparr-key', got '%s'", c.APIKey)
			}
		}
	}
	if !found {
		t.Error("Whisparr config not found")
	}
}
