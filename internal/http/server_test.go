package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ochronus/goputioarr/internal/config"
	"github.com/sirupsen/logrus"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestConfig() *config.Config {
	return &config.Config{
		BindAddress:       "127.0.0.1",
		Port:              9091,
		Username:          "testuser",
		Password:          "testpass",
		DownloadDirectory: "/downloads",
		Loglevel:          "info",
		Putio: config.PutioConfig{
			APIKey: "test-api-key",
		},
	}
}

func setupTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Suppress log output during tests
	return logger
}

func TestNewServer(t *testing.T) {
	cfg := setupTestConfig()
	logger := setupTestLogger()

	server := NewServer(cfg, logger)

	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.config != cfg {
		t.Error("config not set correctly")
	}
	if server.logger != logger {
		t.Error("logger not set correctly")
	}
	if server.handler == nil {
		t.Error("expected non-nil handler")
	}
	if server.router == nil {
		t.Error("expected non-nil router")
	}
}

func TestNewServerDebugMode(t *testing.T) {
	cfg := setupTestConfig()
	cfg.Loglevel = "debug"
	logger := setupTestLogger()

	server := NewServer(cfg, logger)

	if server == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestGetRouter(t *testing.T) {
	cfg := setupTestConfig()
	logger := setupTestLogger()

	server := NewServer(cfg, logger)
	router := server.GetRouter()

	if router == nil {
		t.Fatal("expected non-nil router from GetRouter()")
	}
	if router != server.router {
		t.Error("GetRouter() should return the same router instance")
	}
}

func TestServerRouteRegistration(t *testing.T) {
	cfg := setupTestConfig()
	logger := setupTestLogger()

	server := NewServer(cfg, logger)
	router := server.GetRouter()

	// Test that routes are registered
	routes := router.Routes()

	foundPostRPC := false
	foundGetRPC := false

	for _, route := range routes {
		if route.Path == "/transmission/rpc" {
			if route.Method == "POST" {
				foundPostRPC = true
			}
			if route.Method == "GET" {
				foundGetRPC = true
			}
		}
	}

	if !foundPostRPC {
		t.Error("POST /transmission/rpc route not registered")
	}
	if !foundGetRPC {
		t.Error("GET /transmission/rpc route not registered")
	}
}

func TestServerRoutesRespond(t *testing.T) {
	cfg := setupTestConfig()
	logger := setupTestLogger()

	server := NewServer(cfg, logger)
	router := server.GetRouter()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "GET /transmission/rpc without auth",
			method:         "GET",
			path:           "/transmission/rpc",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "POST /transmission/rpc without auth",
			method:         "POST",
			path:           "/transmission/rpc",
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "GET unknown path",
			method:         "GET",
			path:           "/unknown",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestServerRecoveryMiddleware(t *testing.T) {
	cfg := setupTestConfig()
	logger := setupTestLogger()

	server := NewServer(cfg, logger)
	router := server.GetRouter()

	// Add a route that panics
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	// This should not panic due to recovery middleware
	defer func() {
		if r := recover(); r != nil {
			t.Error("server should have recovered from panic")
		}
	}()

	router.ServeHTTP(w, req)

	// Recovery middleware returns 500 on panic
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d after panic, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestServerHandlerIntegration(t *testing.T) {
	cfg := setupTestConfig()
	logger := setupTestLogger()

	server := NewServer(cfg, logger)

	// Verify handler was created with correct config
	if server.handler.config.Username != cfg.Username {
		t.Error("handler config username mismatch")
	}
	if server.handler.config.Password != cfg.Password {
		t.Error("handler config password mismatch")
	}
	if server.handler.config.DownloadDirectory != cfg.DownloadDirectory {
		t.Error("handler config download directory mismatch")
	}
}

func TestServerMultipleInstances(t *testing.T) {
	cfg1 := setupTestConfig()
	cfg1.Port = 9091

	cfg2 := setupTestConfig()
	cfg2.Port = 9092

	logger := setupTestLogger()

	server1 := NewServer(cfg1, logger)
	server2 := NewServer(cfg2, logger)

	if server1.config.Port == server2.config.Port {
		t.Error("servers should have different ports")
	}
	if server1.router == server2.router {
		t.Error("servers should have different router instances")
	}
}

func TestServerConfigReference(t *testing.T) {
	cfg := setupTestConfig()
	logger := setupTestLogger()

	server := NewServer(cfg, logger)

	// Modify config after server creation
	originalDir := cfg.DownloadDirectory
	cfg.DownloadDirectory = "/new/path"

	// Server should see the change (it holds a reference)
	if server.config.DownloadDirectory != "/new/path" {
		t.Error("server should hold reference to config")
	}

	// Restore
	cfg.DownloadDirectory = originalDir
}

func TestServerLoggerReference(t *testing.T) {
	cfg := setupTestConfig()
	logger := setupTestLogger()

	server := NewServer(cfg, logger)

	// Server should hold reference to logger
	if server.logger != logger {
		t.Error("server should hold reference to logger")
	}
}

func TestServerReleaseModeForNonDebug(t *testing.T) {
	testCases := []string{"info", "warn", "error", "fatal"}

	for _, level := range testCases {
		t.Run(level, func(t *testing.T) {
			cfg := setupTestConfig()
			cfg.Loglevel = level
			logger := setupTestLogger()

			// This should set gin to release mode
			server := NewServer(cfg, logger)

			if server == nil {
				t.Fatal("expected non-nil server")
			}
		})
	}
}
