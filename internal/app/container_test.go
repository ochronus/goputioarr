package app

import (
	"testing"

	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/services/putio"
)

type mockPutioClient struct {
	accountInfoCalled bool
}

func (m *mockPutioClient) GetAccountInfo() (*putio.AccountInfoResponse, error) {
	m.accountInfoCalled = true
	return &putio.AccountInfoResponse{}, nil
}
func (m *mockPutioClient) ListTransfers() (*putio.ListTransferResponse, error) {
	return &putio.ListTransferResponse{Transfers: []putio.Transfer{}}, nil
}
func (m *mockPutioClient) GetTransfer(id uint64) (*putio.GetTransferResponse, error) {
	return &putio.GetTransferResponse{Transfer: putio.Transfer{ID: id, Status: "SEEDING"}}, nil
}
func (m *mockPutioClient) RemoveTransfer(uint64) error { return nil }
func (m *mockPutioClient) DeleteFile(int64) error      { return nil }
func (m *mockPutioClient) AddTransfer(string) error    { return nil }
func (m *mockPutioClient) UploadFile([]byte) error     { return nil }
func (m *mockPutioClient) ListFiles(fileID int64) (*putio.ListFileResponse, error) {
	return &putio.ListFileResponse{
		Parent: putio.FileResponse{ID: fileID, Name: "parent", FileType: "FOLDER"},
		Files:  []putio.FileResponse{},
	}, nil
}
func (m *mockPutioClient) GetFileURL(int64) (string, error) { return "http://example.com", nil }

type mockArrClient struct {
	calls int
}

func (m *mockArrClient) CheckImported(string) (bool, error) {
	m.calls++
	return false, nil
}

func baseConfig() *config.Config {
	return &config.Config{
		DownloadDirectory: "/downloads",
		DownloadWorkers:   1,
		Loglevel:          "info",
		Putio:             config.PutioConfig{APIKey: "abc"},
		Sonarr: &config.ArrConfig{
			URL:    "http://sonarr",
			APIKey: "sonarr-key",
		},
	}
}

func TestNewContainerDefaults(t *testing.T) {
	cfg := baseConfig()
	mockPutio := &mockPutioClient{}

	container, err := NewContainer(cfg, WithPutioClient(mockPutio))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if container.Logger == nil {
		t.Fatal("expected logger to be initialized")
	}
	if container.PutioClient != mockPutio {
		t.Errorf("expected PutioClient to be overridden with mock")
	}
	if len(container.ArrClients) != 1 {
		t.Fatalf("expected 1 Arr client, got %d", len(container.ArrClients))
	}
	if container.ArrClients[0].Name != "Sonarr" {
		t.Errorf("expected Arr client name 'Sonarr', got %q", container.ArrClients[0].Name)
	}
}

func TestContainerOverrides(t *testing.T) {
	cfg := baseConfig()
	mockPutio := &mockPutioClient{}
	customLogger := buildDefaultLogger("debug")
	customArr := []ArrServiceClient{{Name: "custom", Client: &mockArrClient{}}}

	container, err := NewContainer(
		cfg,
		WithLogger(customLogger),
		WithPutioClient(mockPutio),
		WithArrClients(customArr),
		WithPutioValidation(false),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if container.Logger != customLogger {
		t.Error("expected custom logger to be used")
	}
	if container.PutioClient != mockPutio {
		t.Error("expected custom put.io client to be used")
	}
	if len(container.ArrClients) != 1 || container.ArrClients[0].Name != "custom" {
		t.Errorf("expected custom Arr clients to be used, got %+v", container.ArrClients)
	}
	if container.ValidatePutio {
		t.Error("expected put.io validation to be disabled via option")
	}
}

func TestNewContainerNilConfigError(t *testing.T) {
	if _, err := NewContainer(nil); err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestWithLoggerNilError(t *testing.T) {
	cfg := baseConfig()
	_, err := NewContainer(cfg, WithLogger(nil))
	if err == nil {
		t.Fatal("expected error when logger is nil")
	}
}

func TestWithPutioClientNilError(t *testing.T) {
	cfg := baseConfig()
	_, err := NewContainer(cfg, WithPutioClient(nil))
	if err == nil {
		t.Fatal("expected error when put.io client is nil")
	}
}

func TestPutioValidationCallsAccountInfo(t *testing.T) {
	cfg := baseConfig()
	mockPutio := &mockPutioClient{}

	container, err := NewContainer(cfg, WithPutioClient(mockPutio))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !mockPutio.accountInfoCalled {
		t.Error("expected GetAccountInfo to be called during container construction")
	}
	if container.PutioClient != mockPutio {
		t.Error("expected mock put.io client to be retained")
	}
}
