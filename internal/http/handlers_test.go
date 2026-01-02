package http

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/services/putio"
	"github.com/ochronus/goputioarr/internal/services/transmission"
	"github.com/sirupsen/logrus"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestHandler() *Handler {
	cfg := &config.Config{
		Username:          "testuser",
		Password:          "testpass",
		DownloadDirectory: "/downloads",
		Putio: config.PutioConfig{
			APIKey: "test-api-key",
		},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Suppress log output during tests

	putioClient := putio.NewClient(cfg.Putio.APIKey)

	return NewHandler(cfg, logger, putioClient)
}

func setupTestRouter(handler *Handler) *gin.Engine {
	router := gin.New()
	router.POST("/transmission/rpc", handler.RPCPost)
	router.GET("/transmission/rpc", handler.RPCGet)
	return router
}

func basicAuthHeader(username, password string) string {
	auth := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

func TestNewHandler(t *testing.T) {
	handler := setupTestHandler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.config == nil {
		t.Error("expected non-nil config")
	}
	if handler.putioClient == nil {
		t.Error("expected non-nil putioClient")
	}
	if handler.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestValidateUser(t *testing.T) {
	handler := setupTestHandler()
	router := gin.New()

	tests := []struct {
		name     string
		auth     string
		expected bool
	}{
		{
			name:     "valid credentials",
			auth:     basicAuthHeader("testuser", "testpass"),
			expected: true,
		},
		{
			name:     "invalid username",
			auth:     basicAuthHeader("wronguser", "testpass"),
			expected: false,
		},
		{
			name:     "invalid password",
			auth:     basicAuthHeader("testuser", "wrongpass"),
			expected: false,
		},
		{
			name:     "empty auth header",
			auth:     "",
			expected: false,
		},
		{
			name:     "invalid auth format",
			auth:     "NotBasic abc123",
			expected: false,
		},
		{
			name:     "invalid base64",
			auth:     "Basic !!!invalid!!!",
			expected: false,
		},
		{
			name:     "missing colon in decoded",
			auth:     "Basic " + base64.StdEncoding.EncodeToString([]byte("nocolon")),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/", nil)
			if tt.auth != "" {
				c.Request.Header.Set("Authorization", tt.auth)
			}

			result := handler.validateUser(c)
			if result != tt.expected {
				t.Errorf("validateUser() = %v, expected %v", result, tt.expected)
			}
		})
	}

	_ = router // silence unused variable
}

func TestRPCGetValidAuth(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/transmission/rpc", nil)
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}

	sessionID := w.Header().Get("X-Transmission-Session-Id")
	if sessionID == "" {
		t.Error("expected X-Transmission-Session-Id header")
	}
}

func TestRPCGetInvalidAuth(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/transmission/rpc", nil)
	req.Header.Set("Authorization", basicAuthHeader("wrong", "creds"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestRPCGetNoAuth(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/transmission/rpc", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestRPCPostNoAuth(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "session-get"}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestRPCPostSessionGet(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "session-get"}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp transmission.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Result != "success" {
		t.Errorf("expected result 'success', got '%s'", resp.Result)
	}

	if resp.Arguments == nil {
		t.Fatal("expected non-nil arguments")
	}

	// Check that it's a session config
	argsMap, ok := resp.Arguments.(map[string]interface{})
	if !ok {
		t.Fatal("expected arguments to be a map")
	}

	if _, exists := argsMap["download-dir"]; !exists {
		t.Error("expected 'download-dir' in arguments")
	}
}

func TestRPCPostTorrentSet(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "torrent-set", "arguments": {"ids": [1]}}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp transmission.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Result != "success" {
		t.Errorf("expected result 'success', got '%s'", resp.Result)
	}
}

func TestRPCPostQueueMoveTop(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "queue-move-top", "arguments": {"ids": [1]}}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp transmission.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Result != "success" {
		t.Errorf("expected result 'success', got '%s'", resp.Result)
	}
}

func TestRPCPostUnknownMethod(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "unknown-method"}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRPCPostInvalidJSON(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": invalid json`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRPCPostEmptyBody(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	req := httptest.NewRequest("POST", "/transmission/rpc", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Empty body should fail JSON binding
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleTorrentAddNilArguments(t *testing.T) {
	handler := setupTestHandler()

	req := &transmission.Request{
		Method:    "torrent-add",
		Arguments: nil,
	}

	err := handler.handleTorrentAdd(req)
	if err != nil {
		t.Errorf("expected no error for nil arguments, got: %v", err)
	}
}

func TestHandleTorrentRemoveNilArguments(t *testing.T) {
	handler := setupTestHandler()

	req := &transmission.Request{
		Method:    "torrent-remove",
		Arguments: nil,
	}

	err := handler.handleTorrentRemove(req)
	if err != nil {
		t.Errorf("expected no error for nil arguments, got: %v", err)
	}
}

func TestSessionIDConstant(t *testing.T) {
	if sessionID == "" {
		t.Error("sessionID should not be empty")
	}
	if sessionID != "useless-session-id" {
		t.Errorf("unexpected sessionID value: %s", sessionID)
	}
}

func TestHandlerConfigAccess(t *testing.T) {
	handler := setupTestHandler()

	if handler.config.Username != "testuser" {
		t.Errorf("expected Username 'testuser', got '%s'", handler.config.Username)
	}
	if handler.config.Password != "testpass" {
		t.Errorf("expected Password 'testpass', got '%s'", handler.config.Password)
	}
	if handler.config.DownloadDirectory != "/downloads" {
		t.Errorf("expected DownloadDirectory '/downloads', got '%s'", handler.config.DownloadDirectory)
	}
}

func TestRPCPostContentType(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "session-get"}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))
	// Not setting Content-Type header

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Gin should still be able to parse JSON even without explicit Content-Type
	// The behavior depends on gin configuration, but typically it works
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("unexpected status %d", w.Code)
	}
}

func TestResponseFormat(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "session-get"}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type 'application/json; charset=utf-8', got '%s'", contentType)
	}
}

func TestMultipleRequests(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	// Make multiple requests to ensure handler is reusable
	methods := []string{"session-get", "torrent-set", "queue-move-top"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			body := `{"method": "` + method + `"}`
			req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("method %s: expected status %d, got %d", method, http.StatusOK, w.Code)
			}
		})
	}
}

func TestTorrentAddWithMetainfo(t *testing.T) {
	handler := setupTestHandler()

	// Create a mock torrent file content (base64 encoded)
	torrentContent := base64.StdEncoding.EncodeToString([]byte("mock torrent data"))

	req := &transmission.Request{
		Method: "torrent-add",
		Arguments: map[string]interface{}{
			"metainfo": torrentContent,
		},
	}

	// This will fail because we can't actually upload to put.io in tests
	// but we can verify the code path doesn't panic
	_ = handler.handleTorrentAdd(req)
}

func TestTorrentAddWithMagnetLink(t *testing.T) {
	handler := setupTestHandler()

	magnetLink := "magnet:?xt=urn:btih:abc123&dn=Test+File"

	req := &transmission.Request{
		Method: "torrent-add",
		Arguments: map[string]interface{}{
			"filename": magnetLink,
		},
	}

	// This will fail because we can't actually add to put.io in tests
	// but we can verify the code path doesn't panic
	_ = handler.handleTorrentAdd(req)
}

func TestTorrentAddWithInvalidMetainfo(t *testing.T) {
	handler := setupTestHandler()

	req := &transmission.Request{
		Method: "torrent-add",
		Arguments: map[string]interface{}{
			"metainfo": "!!!invalid-base64!!!",
		},
	}

	err := handler.handleTorrentAdd(req)
	if err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
}

func TestRPCPostTorrentRemoveNilArguments(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "torrent-remove"}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRPCPostTorrentAddNilArguments(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "torrent-add"}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestValidateUserPasswordWithColon(t *testing.T) {
	handler := setupTestHandler()
	handler.config.Password = "pass:word:with:colons"

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", basicAuthHeader("testuser", "pass:word:with:colons"))

	result := handler.validateUser(c)
	if !result {
		t.Error("expected validateUser to return true for password with colons")
	}
}

func TestValidateUserEmptyPassword(t *testing.T) {
	handler := setupTestHandler()
	handler.config.Password = ""

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", basicAuthHeader("testuser", ""))

	result := handler.validateUser(c)
	if !result {
		t.Error("expected validateUser to return true for empty password when configured")
	}
}

func TestSessionGetResponseFields(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "session-get"}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp transmission.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	argsMap, ok := resp.Arguments.(map[string]interface{})
	if !ok {
		t.Fatal("expected arguments to be a map")
	}

	expectedFields := []string{"download-dir", "rpc-version", "version"}
	for _, field := range expectedFields {
		if _, exists := argsMap[field]; !exists {
			t.Errorf("expected '%s' in session-get response", field)
		}
	}
}

func TestRPCGetSessionIdHeader(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/transmission/rpc", nil)
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	sessionIdHeader := w.Header().Get("X-Transmission-Session-Id")
	if sessionIdHeader != "useless-session-id" {
		t.Errorf("expected session ID 'useless-session-id', got '%s'", sessionIdHeader)
	}
}

func TestRPCPostWithSessionIdHeader(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "session-get"}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))
	req.Header.Set("X-Transmission-Session-Id", "useless-session-id")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHandlerPutioClientInitialized(t *testing.T) {
	handler := setupTestHandler()

	if handler.putioClient == nil {
		t.Error("expected putioClient to be initialized")
	}
}

func TestTorrentAddMagnetWithEncodedName(t *testing.T) {
	handler := setupTestHandler()

	magnetLink := "magnet:?xt=urn:btih:abc123&dn=Test%20Movie%20%282024%29"

	req := &transmission.Request{
		Method: "torrent-add",
		Arguments: map[string]interface{}{
			"filename": magnetLink,
		},
	}

	// This will fail to add to put.io but shouldn't panic
	_ = handler.handleTorrentAdd(req)
}

func TestTorrentAddMagnetWithoutName(t *testing.T) {
	handler := setupTestHandler()

	magnetLink := "magnet:?xt=urn:btih:abc123"

	req := &transmission.Request{
		Method: "torrent-add",
		Arguments: map[string]interface{}{
			"filename": magnetLink,
		},
	}

	// This will fail to add to put.io but shouldn't panic
	_ = handler.handleTorrentAdd(req)
}

func TestRPCPostWithEmptyMethod(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": ""}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for empty method, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRPCPostWithWhitespaceMethod(t *testing.T) {
	handler := setupTestHandler()
	router := setupTestRouter(handler)

	body := `{"method": "   "}`
	req := httptest.NewRequest("POST", "/transmission/rpc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuthHeader("testuser", "testpass"))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for whitespace method, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestTorrentRemoveEmptyIDs(t *testing.T) {
	handler := setupTestHandler()

	// Test that nil arguments doesn't cause an error
	req := &transmission.Request{
		Method:    "torrent-remove",
		Arguments: nil,
	}

	// Should not error with nil arguments
	err := handler.handleTorrentRemove(req)
	if err != nil {
		t.Errorf("unexpected error for nil arguments: %v", err)
	}
}

func TestBasicAuthHeaderGeneration(t *testing.T) {
	header := basicAuthHeader("user", "pass")
	expected := "Basic dXNlcjpwYXNz"
	if header != expected {
		t.Errorf("expected '%s', got '%s'", expected, header)
	}
}

func TestHandlerConfigDownloadDirectory(t *testing.T) {
	cfg := &config.Config{
		Username:          "testuser",
		Password:          "testpass",
		DownloadDirectory: "/custom/downloads",
		Putio: config.PutioConfig{
			APIKey: "test-api-key",
		},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewHandler(cfg, logger, putio.NewClient(cfg.Putio.APIKey))

	if handler.config.DownloadDirectory != "/custom/downloads" {
		t.Errorf("expected DownloadDirectory '/custom/downloads', got '%s'", handler.config.DownloadDirectory)
	}
}
