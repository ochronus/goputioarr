package arr

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8989", "test-api-key")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.baseURL != "http://localhost:8989" {
		t.Errorf("expected baseURL 'http://localhost:8989', got '%s'", client.baseURL)
	}
	if client.apiKey != "test-api-key" {
		t.Errorf("expected apiKey 'test-api-key', got '%s'", client.apiKey)
	}
	if client.httpClient == nil {
		t.Error("expected non-nil httpClient")
	}
}

func TestHistoryResponseParsing(t *testing.T) {
	jsonData := `{
		"totalRecords": 100,
		"records": [
			{
				"eventType": "downloadFolderImported",
				"data": {
					"droppedPath": "/downloads/movie.mkv",
					"importedPath": "/movies/Movie (2024)/movie.mkv"
				}
			},
			{
				"eventType": "grabbed",
				"data": {
					"indexer": "TestIndexer"
				}
			}
		]
	}`

	var response HistoryResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if response.TotalRecords != 100 {
		t.Errorf("expected TotalRecords 100, got %d", response.TotalRecords)
	}

	if len(response.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(response.Records))
	}

	// Check first record
	r1 := response.Records[0]
	if r1.EventType != "downloadFolderImported" {
		t.Errorf("expected EventType 'downloadFolderImported', got '%s'", r1.EventType)
	}
	if r1.Data["droppedPath"] != "/downloads/movie.mkv" {
		t.Errorf("expected droppedPath '/downloads/movie.mkv', got '%s'", r1.Data["droppedPath"])
	}
	if r1.Data["importedPath"] != "/movies/Movie (2024)/movie.mkv" {
		t.Errorf("expected importedPath '/movies/Movie (2024)/movie.mkv', got '%s'", r1.Data["importedPath"])
	}

	// Check second record
	r2 := response.Records[1]
	if r2.EventType != "grabbed" {
		t.Errorf("expected EventType 'grabbed', got '%s'", r2.EventType)
	}
	if r2.Data["indexer"] != "TestIndexer" {
		t.Errorf("expected indexer 'TestIndexer', got '%s'", r2.Data["indexer"])
	}
}

func TestCheckImported(t *testing.T) {
	tests := []struct {
		name           string
		targetPath     string
		serverResponse string
		statusCode     int
		expected       bool
		expectError    bool
	}{
		{
			name:       "found imported",
			targetPath: "/downloads/movie.mkv",
			serverResponse: `{
				"totalRecords": 1,
				"records": [
					{
						"eventType": "downloadFolderImported",
						"data": {
							"droppedPath": "/downloads/movie.mkv"
						}
					}
				]
			}`,
			statusCode:  http.StatusOK,
			expected:    true,
			expectError: false,
		},
		{
			name:       "not found - different path",
			targetPath: "/downloads/movie.mkv",
			serverResponse: `{
				"totalRecords": 1,
				"records": [
					{
						"eventType": "downloadFolderImported",
						"data": {
							"droppedPath": "/downloads/other.mkv"
						}
					}
				]
			}`,
			statusCode:  http.StatusOK,
			expected:    false,
			expectError: false,
		},
		{
			name:       "not found - different event type",
			targetPath: "/downloads/movie.mkv",
			serverResponse: `{
				"totalRecords": 1,
				"records": [
					{
						"eventType": "grabbed",
						"data": {
							"droppedPath": "/downloads/movie.mkv"
						}
					}
				]
			}`,
			statusCode:  http.StatusOK,
			expected:    false,
			expectError: false,
		},
		{
			name:       "empty records",
			targetPath: "/downloads/movie.mkv",
			serverResponse: `{
				"totalRecords": 0,
				"records": []
			}`,
			statusCode:  http.StatusOK,
			expected:    false,
			expectError: false,
		},
		{
			name:           "server error",
			targetPath:     "/downloads/movie.mkv",
			serverResponse: "",
			statusCode:     http.StatusInternalServerError,
			expected:       false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify API key header
				if r.Header.Get("X-Api-Key") != "test-key" {
					t.Errorf("expected X-Api-Key header 'test-key', got '%s'", r.Header.Get("X-Api-Key"))
				}

				// Verify path
				if r.URL.Path != "/api/v3/history" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-key")
			result, err := client.CheckImported(tt.targetPath)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestCheckImportedPagination(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return different responses based on page
		var response string
		if page == 0 {
			response = `{
				"totalRecords": 2000,
				"records": [
					{
						"eventType": "grabbed",
						"data": {}
					}
				]
			}`
		} else {
			response = `{
				"totalRecords": 2000,
				"records": [
					{
						"eventType": "downloadFolderImported",
						"data": {
							"droppedPath": "/downloads/movie.mkv"
						}
					}
				]
			}`
		}
		page++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	result, err := client.CheckImported("/downloads/movie.mkv")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected to find imported file on second page")
	}
	if page < 2 {
		t.Errorf("expected at least 2 page requests, got %d", page)
	}
}

func TestCheckImportedMultiService(t *testing.T) {
	tests := []struct {
		name            string
		services        []struct{ found bool }
		expectedFound   bool
		expectedService string
	}{
		{
			name: "found in first service",
			services: []struct{ found bool }{
				{found: true},
				{found: false},
			},
			expectedFound:   true,
			expectedService: "Service0",
		},
		{
			name: "found in second service",
			services: []struct{ found bool }{
				{found: false},
				{found: true},
			},
			expectedFound:   true,
			expectedService: "Service1",
		},
		{
			name: "not found in any",
			services: []struct{ found bool }{
				{found: false},
				{found: false},
			},
			expectedFound:   false,
			expectedService: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var servers []*httptest.Server
			var serviceConfigs []struct {
				Name   string
				URL    string
				APIKey string
			}

			for i, svc := range tt.services {
				found := svc.found
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var response string
					if found {
						response = `{
							"totalRecords": 1,
							"records": [
								{
									"eventType": "downloadFolderImported",
									"data": {
										"droppedPath": "/downloads/movie.mkv"
									}
								}
							]
						}`
					} else {
						response = `{
							"totalRecords": 0,
							"records": []
						}`
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(response))
				}))
				servers = append(servers, server)
				serviceConfigs = append(serviceConfigs, struct {
					Name   string
					URL    string
					APIKey string
				}{
					Name:   "Service" + string(rune('0'+i)),
					URL:    server.URL,
					APIKey: "key",
				})
			}

			defer func() {
				for _, s := range servers {
					s.Close()
				}
			}()

			found, serviceName, err := CheckImportedMultiService("/downloads/movie.mkv", serviceConfigs)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if found != tt.expectedFound {
				t.Errorf("expected found=%v, got %v", tt.expectedFound, found)
			}
			if found && serviceName != tt.expectedService {
				t.Errorf("expected serviceName '%s', got '%s'", tt.expectedService, serviceName)
			}
		})
	}
}

func TestCheckImportedInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	_, err := client.CheckImported("/downloads/movie.mkv")

	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestHistoryRecordDataAccess(t *testing.T) {
	record := HistoryRecord{
		EventType: "downloadFolderImported",
		Data: map[string]string{
			"droppedPath":  "/downloads/test.mkv",
			"importedPath": "/movies/test.mkv",
		},
	}

	if record.Data["droppedPath"] != "/downloads/test.mkv" {
		t.Errorf("unexpected droppedPath: %s", record.Data["droppedPath"])
	}

	// Test missing key
	if _, exists := record.Data["nonexistent"]; exists {
		t.Error("expected nonexistent key to not exist")
	}
}

func TestClientTimeout(t *testing.T) {
	client := NewClient("http://localhost:8989", "test-key")
	if client.httpClient.Timeout != timeout {
		t.Errorf("expected timeout %v, got %v", timeout, client.httpClient.Timeout)
	}
}

func TestHistoryResponseEmptyRecords(t *testing.T) {
	jsonData := `{
		"totalRecords": 0,
		"records": []
	}`

	var response HistoryResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if response.TotalRecords != 0 {
		t.Errorf("expected TotalRecords 0, got %d", response.TotalRecords)
	}
	if len(response.Records) != 0 {
		t.Errorf("expected 0 records, got %d", len(response.Records))
	}
}

func TestCheckImportedMissingDroppedPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"totalRecords": 1,
			"records": [
				{
					"eventType": "downloadFolderImported",
					"data": {
						"importedPath": "/movies/movie.mkv"
					}
				}
			]
		}`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	result, err := client.CheckImported("/downloads/movie.mkv")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false when droppedPath is missing")
	}
}

func TestHistoryRecordEventTypes(t *testing.T) {
	eventTypes := []string{
		"grabbed",
		"downloadFolderImported",
		"downloadFailed",
		"episodeFileDeleted",
		"movieFileDeleted",
	}

	for _, eventType := range eventTypes {
		t.Run(eventType, func(t *testing.T) {
			record := HistoryRecord{
				EventType: eventType,
				Data:      map[string]string{},
			}
			if record.EventType != eventType {
				t.Errorf("expected eventType %s, got %s", eventType, record.EventType)
			}
		})
	}
}

func TestCheckImportedMultiplePages(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var response string
		if callCount == 1 {
			// First page - not found
			response = `{
				"totalRecords": 3000,
				"records": [
					{"eventType": "grabbed", "data": {}},
					{"eventType": "grabbed", "data": {}}
				]
			}`
		} else if callCount == 2 {
			// Second page - found
			response = `{
				"totalRecords": 3000,
				"records": [
					{
						"eventType": "downloadFolderImported",
						"data": {"droppedPath": "/downloads/movie.mkv"}
					}
				]
			}`
		} else {
			response = `{"totalRecords": 0, "records": []}`
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	result, err := client.CheckImported("/downloads/movie.mkv")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected to find imported file across multiple pages")
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 API calls for pagination, got %d", callCount)
	}
}

func TestCheckImportedMultiServiceWithErrors(t *testing.T) {
	// First server returns error
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server1.Close()

	// Second server returns success
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"totalRecords": 1,
			"records": [
				{
					"eventType": "downloadFolderImported",
					"data": {"droppedPath": "/downloads/movie.mkv"}
				}
			]
		}`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server2.Close()

	services := []struct {
		Name   string
		URL    string
		APIKey string
	}{
		{Name: "Sonarr", URL: server1.URL, APIKey: "key1"},
		{Name: "Radarr", URL: server2.URL, APIKey: "key2"},
	}

	found, serviceName, err := CheckImportedMultiService("/downloads/movie.mkv", services)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected to find imported file from second service")
	}
	if serviceName != "Radarr" {
		t.Errorf("expected service name 'Radarr', got '%s'", serviceName)
	}
}

func TestCheckImportedMultiServiceAllErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	services := []struct {
		Name   string
		URL    string
		APIKey string
	}{
		{Name: "Sonarr", URL: server.URL, APIKey: "key1"},
		{Name: "Radarr", URL: server.URL, APIKey: "key2"},
	}

	found, serviceName, err := CheckImportedMultiService("/downloads/movie.mkv", services)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected not to find imported file when all services error")
	}
	if serviceName != "" {
		t.Errorf("expected empty service name, got '%s'", serviceName)
	}
}

func TestCheckImportedMultiServiceEmptyList(t *testing.T) {
	services := []struct {
		Name   string
		URL    string
		APIKey string
	}{}

	found, serviceName, err := CheckImportedMultiService("/downloads/movie.mkv", services)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected not to find imported file with empty service list")
	}
	if serviceName != "" {
		t.Errorf("expected empty service name, got '%s'", serviceName)
	}
}

func TestClientBaseURLAndAPIKey(t *testing.T) {
	client := NewClient("http://test-host:8989/sonarr", "my-api-key")

	if client.baseURL != "http://test-host:8989/sonarr" {
		t.Errorf("expected baseURL 'http://test-host:8989/sonarr', got '%s'", client.baseURL)
	}
	if client.apiKey != "my-api-key" {
		t.Errorf("expected apiKey 'my-api-key', got '%s'", client.apiKey)
	}
}

func TestHistoryRecordWithEmptyData(t *testing.T) {
	record := HistoryRecord{
		EventType: "grabbed",
		Data:      map[string]string{},
	}

	if len(record.Data) != 0 {
		t.Errorf("expected empty data map, got %d entries", len(record.Data))
	}
}

func TestHistoryRecordWithNilData(t *testing.T) {
	jsonData := `{
		"eventType": "grabbed"
	}`

	var record HistoryRecord
	err := json.Unmarshal([]byte(jsonData), &record)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if record.Data != nil && len(record.Data) != 0 {
		t.Errorf("expected nil or empty data, got %v", record.Data)
	}
}

func TestCheckImportedRetriesThenSucceedsOn5xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"totalRecords": 1,
			"records": [
				{
					"eventType": "downloadFolderImported",
					"data": {"droppedPath": "/downloads/movie.mkv"}
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	result, err := client.CheckImported("/downloads/movie.mkv")

	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if !result {
		t.Fatal("expected imported to be true after retry")
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts (1 retry), got %d", attempts)
	}
}

func TestCheckImportedRetriesAndFailsAfterMaxAttempts(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	result, err := client.CheckImported("/downloads/movie.mkv")

	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if _, ok := err.(*HTTPError); !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if result {
		t.Fatal("expected result to be false after retries fail")
	}
	if attempts != maxRetries {
		t.Fatalf("expected %d attempts, got %d", maxRetries, attempts)
	}
}

func TestCheckImportedRetriesOn429(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"totalRecords": 1,
			"records": [
				{
					"eventType": "downloadFolderImported",
					"data": {"droppedPath": "/downloads/movie.mkv"}
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	result, err := client.CheckImported("/downloads/movie.mkv")
	if err != nil {
		t.Fatalf("expected success after retry on 429, got error: %v", err)
	}
	if !result {
		t.Fatal("expected result to be true after retry on 429")
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts (one retry on 429), got %d", attempts)
	}
}

func TestCheckImportedRespectsRetryAfter(t *testing.T) {
	attempts := 0
	var sleeps []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"totalRecords": 0,
			"records": []
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	client.sleeper = func(d time.Duration) {
		sleeps = append(sleeps, d)
	}

	_, err := client.CheckImported("/downloads/movie.mkv")
	if err != nil {
		t.Fatalf("expected success after respecting Retry-After, got error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts (one retry on 429), got %d", attempts)
	}
	if len(sleeps) != 1 {
		t.Fatalf("expected one recorded sleep, got %d", len(sleeps))
	}
	if sleeps[0] != time.Second {
		t.Fatalf("expected sleep of 1s from Retry-After, got %v", sleeps[0])
	}
}

func TestCheckImportedDoesNotRetryOn400(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	_, err := client.CheckImported("/downloads/movie.mkv")
	if err == nil {
		t.Fatal("expected error on 400, got nil")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if attempts != 1 {
		t.Fatalf("expected no retries on 400, got %d attempts", attempts)
	}
}
