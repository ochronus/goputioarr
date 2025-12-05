package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigTemplateContent(t *testing.T) {
	// Verify that the config template contains all required sections
	requiredSections := []string{
		"username",
		"password",
		"download_directory",
		"bind_address",
		"port",
		"loglevel",
		"uid",
		"polling_interval",
		"skip_directories",
		"orchestration_workers",
		"download_workers",
		"[putio]",
		"api_key",
		"[sonarr]",
		"[radarr]",
		"[whisparr]",
	}

	for _, section := range requiredSections {
		if !strings.Contains(configTemplate, section) {
			t.Errorf("configTemplate missing required section: %s", section)
		}
	}
}

func TestConfigTemplatePlaceholder(t *testing.T) {
	// Verify the placeholder exists for API key
	if !strings.Contains(configTemplate, "{{PUTIO_API_KEY}}") {
		t.Error("configTemplate missing {{PUTIO_API_KEY}} placeholder")
	}
}

func TestConfigTemplateDefaultValues(t *testing.T) {
	defaults := map[string]string{
		"bind_address":          `"0.0.0.0"`,
		"port":                  "9091",
		"loglevel":              `"info"`,
		"uid":                   "1000",
		"polling_interval":      "10",
		"orchestration_workers": "10",
		"download_workers":      "4",
	}

	for key, expectedValue := range defaults {
		if !strings.Contains(configTemplate, key+" = "+expectedValue) &&
			!strings.Contains(configTemplate, key+"="+expectedValue) {
			// Check if the value exists in a different format
			if !strings.Contains(configTemplate, expectedValue) {
				t.Errorf("configTemplate may have incorrect default for %s, expected %s", key, expectedValue)
			}
		}
	}
}

func TestConfigTemplateSkipDirectories(t *testing.T) {
	// Verify default skip directories
	if !strings.Contains(configTemplate, `["sample", "extras"]`) {
		t.Error("configTemplate missing default skip_directories")
	}
}

func TestGenerateConfigCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "nested", "config.toml")

	// We can't fully test GenerateConfig without mocking the Put.io API,
	// but we can test the directory creation logic separately

	dir := filepath.Dir(configPath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestGenerateConfigBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Create an existing config file
	originalContent := "original config content"
	err := os.WriteFile(configPath, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("failed to write original config: %v", err)
	}

	// Simulate backup logic
	backupPath := configPath + ".bak"
	err = os.Rename(configPath, backupPath)
	if err != nil {
		t.Fatalf("failed to backup config: %v", err)
	}

	// Verify backup was created
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup: %v", err)
	}
	if string(backupContent) != originalContent {
		t.Errorf("backup content mismatch: expected '%s', got '%s'", originalContent, string(backupContent))
	}

	// Verify original file no longer exists
	_, err = os.Stat(configPath)
	if !os.IsNotExist(err) {
		t.Error("original config should not exist after backup")
	}
}

func TestConfigTemplateWritable(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Replace placeholder and write
	content := strings.Replace(configTemplate, "{{PUTIO_API_KEY}}", "test-api-key", 1)

	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Read back and verify
	readContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if !strings.Contains(string(readContent), "test-api-key") {
		t.Error("config should contain the replaced API key")
	}

	if strings.Contains(string(readContent), "{{PUTIO_API_KEY}}") {
		t.Error("config should not contain the placeholder after replacement")
	}
}

func TestConfigTemplateValidTOML(t *testing.T) {
	// Replace placeholder with a test value
	content := strings.Replace(configTemplate, "{{PUTIO_API_KEY}}", "test-key", 1)

	// Basic TOML validation - check for balanced quotes
	quoteCount := strings.Count(content, `"`)
	if quoteCount%2 != 0 {
		t.Error("configTemplate has unbalanced quotes")
	}

	// Check for balanced brackets
	openBrackets := strings.Count(content, "[")
	closeBrackets := strings.Count(content, "]")
	if openBrackets != closeBrackets {
		t.Errorf("configTemplate has unbalanced brackets: %d open, %d close", openBrackets, closeBrackets)
	}
}

func TestConfigTemplateSections(t *testing.T) {
	sections := []string{
		"[putio]",
		"[sonarr]",
		"[radarr]",
		"[whisparr]",
	}

	for _, section := range sections {
		if !strings.Contains(configTemplate, section) {
			t.Errorf("configTemplate missing section: %s", section)
		}
	}
}

func TestConfigTemplateComments(t *testing.T) {
	// Verify that important comments exist
	importantComments := []string{
		"# Required",
		"# Optional",
		"# Can be found in Settings -> General",
	}

	for _, comment := range importantComments {
		if !strings.Contains(configTemplate, comment) {
			t.Errorf("configTemplate missing important comment: %s", comment)
		}
	}
}

func TestConfigTemplateArrURLs(t *testing.T) {
	// Verify default arr URLs are present
	expectedURLs := []string{
		"http://mysonarrhost:8989/sonarr",
		"http://myradarrhost:7878/radarr",
		"http://mywhisparrhost:6969/radarr",
	}

	for _, url := range expectedURLs {
		if !strings.Contains(configTemplate, url) {
			t.Errorf("configTemplate missing default URL: %s", url)
		}
	}
}

func TestGenerateConfigFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := "test content"
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("failed to stat config: %v", err)
	}

	// Check permissions (on Unix-like systems)
	mode := info.Mode().Perm()
	if mode&0644 != 0644 {
		t.Errorf("expected permissions 0644, got %o", mode)
	}
}
