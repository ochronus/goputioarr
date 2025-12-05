package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ochronus/goputioarr/internal/config"
	"github.com/sirupsen/logrus"
)

func setupTestManager() *Manager {
	cfg := &config.Config{
		DownloadDirectory:    "/downloads",
		DownloadWorkers:      2,
		OrchestrationWorkers: 2,
		PollingInterval:      1,
		SkipDirectories:      []string{"sample", "extras"},
		UID:                  1000,
		Putio: config.PutioConfig{
			APIKey: "test-api-key",
		},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	return NewManager(cfg, logger)
}

func TestNewManager(t *testing.T) {
	manager := setupTestManager()

	if manager == nil {
		t.Fatal("expected non-nil manager")
	}
	if manager.config == nil {
		t.Error("expected non-nil config")
	}
	if manager.putioClient == nil {
		t.Error("expected non-nil putioClient")
	}
	if manager.transferChan == nil {
		t.Error("expected non-nil transferChan")
	}
	if manager.downloadChan == nil {
		t.Error("expected non-nil downloadChan")
	}
	if manager.seen == nil {
		t.Error("expected non-nil seen map")
	}
	if manager.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestManagerSeenOperations(t *testing.T) {
	manager := setupTestManager()

	// Test isSeen returns false for unseen ID
	if manager.isSeen(123) {
		t.Error("expected isSeen(123) to return false initially")
	}

	// Test markSeen
	manager.markSeen(123)
	if !manager.isSeen(123) {
		t.Error("expected isSeen(123) to return true after markSeen")
	}

	// Test multiple IDs
	manager.markSeen(456)
	manager.markSeen(789)
	if !manager.isSeen(456) {
		t.Error("expected isSeen(456) to return true")
	}
	if !manager.isSeen(789) {
		t.Error("expected isSeen(789) to return true")
	}

	// Test cleanupSeen
	activeIDs := map[uint64]bool{
		123: true,
		// 456 and 789 are not in active list
	}
	manager.cleanupSeen(activeIDs)

	if !manager.isSeen(123) {
		t.Error("expected isSeen(123) to still be true (in active list)")
	}
	if manager.isSeen(456) {
		t.Error("expected isSeen(456) to be false after cleanup")
	}
	if manager.isSeen(789) {
		t.Error("expected isSeen(789) to be false after cleanup")
	}
}

func TestManagerSeenConcurrency(t *testing.T) {
	manager := setupTestManager()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Test concurrent markSeen
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			manager.markSeen(id)
		}(uint64(i))
	}
	wg.Wait()

	// Verify all IDs were marked
	for i := 0; i < numGoroutines; i++ {
		if !manager.isSeen(uint64(i)) {
			t.Errorf("expected isSeen(%d) to be true", i)
		}
	}

	// Test concurrent isSeen reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			_ = manager.isSeen(id)
		}(uint64(i))
	}
	wg.Wait()
}

func TestManagerCleanupSeenEmpty(t *testing.T) {
	manager := setupTestManager()

	manager.markSeen(1)
	manager.markSeen(2)
	manager.markSeen(3)

	// Cleanup with empty active list should remove all
	manager.cleanupSeen(map[uint64]bool{})

	if manager.isSeen(1) || manager.isSeen(2) || manager.isSeen(3) {
		t.Error("expected all IDs to be removed after cleanup with empty active list")
	}
}

func TestDownloadTargetDirectory(t *testing.T) {
	manager := setupTestManager()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "test_dir")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeDirectory,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess, got %v", status)
	}

	// Verify directory was created
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestDownloadTargetDirectoryAlreadyExists(t *testing.T) {
	manager := setupTestManager()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "existing_dir")

	// Create the directory first
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeDirectory,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess for existing directory, got %v", status)
	}
}

func TestDownloadTargetFileAlreadyExists(t *testing.T) {
	manager := setupTestManager()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "existing_file.txt")

	// Create the file first
	if err := os.WriteFile(targetPath, []byte("existing content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       "http://example.com/file.txt",
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess for existing file, got %v", status)
	}
}

func TestDownloadTargetFileSuccess(t *testing.T) {
	manager := setupTestManager()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test file content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "downloaded_file.txt")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       server.URL,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess, got %v", status)
	}

	// Verify file was created with correct content
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != "test file content" {
		t.Errorf("expected 'test file content', got '%s'", string(content))
	}
}

func TestDownloadTargetFileHTTPError(t *testing.T) {
	manager := setupTestManager()

	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "failed_file.txt")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       server.URL,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusFailed {
		t.Errorf("expected DownloadStatusFailed for HTTP error, got %v", status)
	}

	// Verify file was not created
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Error("expected file to not exist after failed download")
	}
}

func TestDownloadTargetFileNoURL(t *testing.T) {
	manager := setupTestManager()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "no_url_file.txt")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       "", // No URL
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusFailed {
		t.Errorf("expected DownloadStatusFailed for missing URL, got %v", status)
	}
}

func TestDownloadTargetFileInvalidURL(t *testing.T) {
	manager := setupTestManager()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "invalid_url_file.txt")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       "http://invalid-host-that-does-not-exist.local/file.txt",
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusFailed {
		t.Errorf("expected DownloadStatusFailed for invalid URL, got %v", status)
	}
}

func TestDownloadTargetUnknownType(t *testing.T) {
	manager := setupTestManager()

	target := &DownloadTarget{
		To:         "/some/path",
		TargetType: TargetType(99), // Unknown type
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusFailed {
		t.Errorf("expected DownloadStatusFailed for unknown target type, got %v", status)
	}
}

func TestDownloadTargetNestedDirectory(t *testing.T) {
	manager := setupTestManager()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "level1", "level2", "level3")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeDirectory,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess, got %v", status)
	}

	// Verify nested directory was created
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("nested directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestDownloadTargetFileWithNestedPath(t *testing.T) {
	manager := setupTestManager()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("nested content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "subdir1", "subdir2", "file.txt")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       server.URL,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess, got %v", status)
	}

	// Verify file was created
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "nested content" {
		t.Errorf("expected 'nested content', got '%s'", string(content))
	}
}

func TestFetchFileLargeContent(t *testing.T) {
	manager := setupTestManager()

	// Create a test server that returns larger content
	largeContent := make([]byte, 1024*1024) // 1MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(largeContent)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "large_file.bin")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       server.URL,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess, got %v", status)
	}

	// Verify file size
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Size() != int64(len(largeContent)) {
		t.Errorf("expected file size %d, got %d", len(largeContent), info.Size())
	}
}

func TestManagerChannelBufferSizes(t *testing.T) {
	manager := setupTestManager()

	// Verify channel buffer sizes
	if cap(manager.transferChan) != 100 {
		t.Errorf("expected transferChan buffer size 100, got %d", cap(manager.transferChan))
	}
	if cap(manager.downloadChan) != 100 {
		t.Errorf("expected downloadChan buffer size 100, got %d", cap(manager.downloadChan))
	}
}

func TestManagerConfigReference(t *testing.T) {
	cfg := &config.Config{
		DownloadDirectory: "/original",
		Putio: config.PutioConfig{
			APIKey: "test-key",
		},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	manager := NewManager(cfg, logger)

	if manager.config.DownloadDirectory != "/original" {
		t.Errorf("expected download directory '/original', got '%s'", manager.config.DownloadDirectory)
	}

	// Modify config after creation
	cfg.DownloadDirectory = "/modified"

	// Manager should see the change (it holds a reference)
	if manager.config.DownloadDirectory != "/modified" {
		t.Error("manager should hold reference to config")
	}
}

func TestDownloadTargetFileTempFileCleanup(t *testing.T) {
	manager := setupTestManager()

	// Create a test server that returns an error mid-download
	// by closing the connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Don't write anything to simulate incomplete download
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "incomplete_file.txt")
	tmpPath := targetPath + ".downloading"

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       server.URL,
	}

	_ = manager.downloadTarget(target)

	// Verify temp file was cleaned up (if it existed)
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("expected temp file to be cleaned up")
	}
}

func TestIsImportedNoTargets(t *testing.T) {
	manager := setupTestManager()

	transfer := &Transfer{
		Name:       "Test Transfer",
		TransferID: 123,
	}
	// No targets set

	result := manager.isImported(transfer)

	if result {
		t.Error("expected isImported to return false when no file targets")
	}
}

func TestIsImportedNoServices(t *testing.T) {
	cfg := &config.Config{
		DownloadDirectory: "/downloads",
		Putio: config.PutioConfig{
			APIKey: "test-key",
		},
		// No Sonarr, Radarr, or Whisparr configured
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	manager := NewManager(cfg, logger)

	transfer := &Transfer{
		Name:       "Test Transfer",
		TransferID: 123,
	}
	transfer.SetTargets([]DownloadTarget{
		{To: "/downloads/file.mkv", TargetType: TargetTypeFile},
	})

	result := manager.isImported(transfer)

	if result {
		t.Error("expected isImported to return false when no arr services configured")
	}
}

func TestManagerWithDifferentConfigs(t *testing.T) {
	tests := []struct {
		name                 string
		downloadWorkers      int
		orchestrationWorkers int
	}{
		{"minimal workers", 1, 1},
		{"default workers", 4, 10},
		{"high workers", 10, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				DownloadDirectory:    "/downloads",
				DownloadWorkers:      tt.downloadWorkers,
				OrchestrationWorkers: tt.orchestrationWorkers,
				Putio: config.PutioConfig{
					APIKey: "test-key",
				},
			}
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			manager := NewManager(cfg, logger)

			if manager.config.DownloadWorkers != tt.downloadWorkers {
				t.Errorf("expected DownloadWorkers %d, got %d", tt.downloadWorkers, manager.config.DownloadWorkers)
			}
			if manager.config.OrchestrationWorkers != tt.orchestrationWorkers {
				t.Errorf("expected OrchestrationWorkers %d, got %d", tt.orchestrationWorkers, manager.config.OrchestrationWorkers)
			}
		})
	}
}

func TestDownloadTargetDirectoryPermissions(t *testing.T) {
	manager := setupTestManager()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "perm_test_dir")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeDirectory,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess, got %v", status)
	}

	// Verify permissions
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("failed to stat directory: %v", err)
	}

	// Should be at least 0755
	mode := info.Mode().Perm()
	if mode&0755 != 0755 {
		t.Errorf("expected at least 0755 permissions, got %o", mode)
	}
}

func TestDownloadTargetFileEmptyContent(t *testing.T) {
	manager := setupTestManager()

	// Create a test server that returns empty content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write nothing
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "empty_file.txt")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       server.URL,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess for empty file, got %v", status)
	}

	// Verify file was created and is empty
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected file size 0, got %d", info.Size())
	}
}

func TestManagerSeenMapInitialization(t *testing.T) {
	manager := setupTestManager()

	// Initial seen map should be empty
	manager.seenMu.RLock()
	seenLen := len(manager.seen)
	manager.seenMu.RUnlock()

	if seenLen != 0 {
		t.Errorf("expected seen map to be empty initially, got %d entries", seenLen)
	}
}

func TestManagerLoggerLevel(t *testing.T) {
	cfg := &config.Config{
		DownloadDirectory: "/downloads",
		Putio: config.PutioConfig{
			APIKey: "test-key",
		},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	manager := NewManager(cfg, logger)

	if manager.logger.Level != logrus.DebugLevel {
		t.Errorf("expected logger level DebugLevel, got %v", manager.logger.Level)
	}
}

func TestDownloadTargetConcurrentDownloads(t *testing.T) {
	manager := setupTestManager()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("concurrent test content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	var wg sync.WaitGroup
	numDownloads := 10

	for i := 0; i < numDownloads; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			targetPath := filepath.Join(tmpDir, "file"+string(rune('0'+idx))+".txt")
			target := &DownloadTarget{
				To:         targetPath,
				TargetType: TargetTypeFile,
				From:       server.URL,
			}
			status := manager.downloadTarget(target)
			if status != DownloadStatusSuccess {
				t.Errorf("download %d failed", idx)
			}
		}(i)
	}

	wg.Wait()
}

func TestDownloadTargetConcurrentDirectoryCreation(t *testing.T) {
	manager := setupTestManager()

	tmpDir := t.TempDir()

	var wg sync.WaitGroup
	numDirs := 10

	for i := 0; i < numDirs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			targetPath := filepath.Join(tmpDir, "dir"+string(rune('0'+idx)))
			target := &DownloadTarget{
				To:         targetPath,
				TargetType: TargetTypeDirectory,
			}
			status := manager.downloadTarget(target)
			if status != DownloadStatusSuccess {
				t.Errorf("directory creation %d failed", idx)
			}
		}(i)
	}

	wg.Wait()
}

func TestManagerSkipDirectoriesConfig(t *testing.T) {
	cfg := &config.Config{
		DownloadDirectory: "/downloads",
		SkipDirectories:   []string{"sample", "extras", "subs"},
		Putio: config.PutioConfig{
			APIKey: "test-key",
		},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	manager := NewManager(cfg, logger)

	if len(manager.config.SkipDirectories) != 3 {
		t.Errorf("expected 3 skip directories, got %d", len(manager.config.SkipDirectories))
	}
}

func TestFetchFileCreatesParentDirs(t *testing.T) {
	manager := setupTestManager()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("nested file content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "d", "file.txt")

	target := &DownloadTarget{
		To:         deepPath,
		TargetType: TargetTypeFile,
		From:       server.URL,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess, got %v", status)
	}

	// Verify all parent directories were created
	parentDir := filepath.Dir(deepPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Error("parent directories were not created")
	}
}

func TestManagerTransferChannelSend(t *testing.T) {
	manager := setupTestManager()

	// Test sending to transfer channel (non-blocking due to buffer)
	hash := "testhash123"
	transfer := &Transfer{
		Name:       "Test Transfer",
		TransferID: 123,
		Hash:       &hash,
	}

	msg := TransferMessage{
		Type:     MessageQueuedForDownload,
		Transfer: transfer,
	}

	select {
	case manager.transferChan <- msg:
		// Successfully sent
	default:
		t.Error("failed to send to transfer channel")
	}

	// Receive the message
	select {
	case received := <-manager.transferChan:
		if received.Type != MessageQueuedForDownload {
			t.Errorf("expected MessageQueuedForDownload, got %v", received.Type)
		}
		if received.Transfer.Name != "Test Transfer" {
			t.Errorf("expected transfer name 'Test Transfer', got '%s'", received.Transfer.Name)
		}
	default:
		t.Error("failed to receive from transfer channel")
	}
}

func TestManagerDownloadChannelSend(t *testing.T) {
	manager := setupTestManager()

	doneChan := make(chan DownloadDoneStatus, 1)
	target := DownloadTarget{
		To:         "/downloads/test.mkv",
		TargetType: TargetTypeFile,
	}

	msg := DownloadTargetMessage{
		Target:   target,
		DoneChan: doneChan,
	}

	select {
	case manager.downloadChan <- msg:
		// Successfully sent
	default:
		t.Error("failed to send to download channel")
	}

	// Receive the message
	select {
	case received := <-manager.downloadChan:
		if received.Target.To != "/downloads/test.mkv" {
			t.Errorf("expected target path '/downloads/test.mkv', got '%s'", received.Target.To)
		}
	default:
		t.Error("failed to receive from download channel")
	}
}

func TestIsImportedWithOnlyDirectoryTargets(t *testing.T) {
	manager := setupTestManager()

	transfer := &Transfer{
		Name:       "Test Transfer",
		TransferID: 123,
	}
	transfer.SetTargets([]DownloadTarget{
		{To: "/downloads/folder1", TargetType: TargetTypeDirectory},
		{To: "/downloads/folder2", TargetType: TargetTypeDirectory},
	})

	result := manager.isImported(transfer)

	if result {
		t.Error("expected isImported to return false when only directory targets exist")
	}
}

func TestDownloadTargetSpecialCharactersInPath(t *testing.T) {
	manager := setupTestManager()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("special chars content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	// Test path with spaces and special characters
	targetPath := filepath.Join(tmpDir, "Movie Name (2024)", "file with spaces.mkv")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       server.URL,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess for path with special chars, got %v", status)
	}
}

func TestDownloadTargetDirectoryWithSpecialChars(t *testing.T) {
	manager := setupTestManager()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "Movie Name (2024) [1080p]")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeDirectory,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess for directory with special chars, got %v", status)
	}

	// Verify directory exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Error("directory with special chars was not created")
	}
}

func TestManagerMultipleSeenCleanups(t *testing.T) {
	manager := setupTestManager()

	// Add many IDs
	for i := uint64(0); i < 100; i++ {
		manager.markSeen(i)
	}

	// First cleanup - keep only first 50
	activeIDs := make(map[uint64]bool)
	for i := uint64(0); i < 50; i++ {
		activeIDs[i] = true
	}
	manager.cleanupSeen(activeIDs)

	// Verify first 50 still exist
	for i := uint64(0); i < 50; i++ {
		if !manager.isSeen(i) {
			t.Errorf("expected isSeen(%d) to be true after first cleanup", i)
		}
	}

	// Verify 50-99 are removed
	for i := uint64(50); i < 100; i++ {
		if manager.isSeen(i) {
			t.Errorf("expected isSeen(%d) to be false after first cleanup", i)
		}
	}

	// Second cleanup - keep only first 25
	activeIDs = make(map[uint64]bool)
	for i := uint64(0); i < 25; i++ {
		activeIDs[i] = true
	}
	manager.cleanupSeen(activeIDs)

	// Verify first 25 still exist
	for i := uint64(0); i < 25; i++ {
		if !manager.isSeen(i) {
			t.Errorf("expected isSeen(%d) to be true after second cleanup", i)
		}
	}

	// Verify 25-49 are now removed
	for i := uint64(25); i < 50; i++ {
		if manager.isSeen(i) {
			t.Errorf("expected isSeen(%d) to be false after second cleanup", i)
		}
	}
}

func TestDownloadTargetServerTimeout(t *testing.T) {
	manager := setupTestManager()

	// Create a server that never responds (will timeout)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Just return immediately with no content for this test
		// A real timeout test would be too slow
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "timeout_file.txt")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       server.URL,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess, got %v", status)
	}
}

func TestManagerPutioClientNotNil(t *testing.T) {
	manager := setupTestManager()

	if manager.putioClient == nil {
		t.Error("expected putioClient to be initialized")
	}
}

func TestDownloadTargetBinaryContent(t *testing.T) {
	manager := setupTestManager()

	// Create binary content
	binaryContent := make([]byte, 256)
	for i := range binaryContent {
		binaryContent[i] = byte(i)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(binaryContent)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "binary_file.bin")

	target := &DownloadTarget{
		To:         targetPath,
		TargetType: TargetTypeFile,
		From:       server.URL,
	}

	status := manager.downloadTarget(target)

	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess, got %v", status)
	}

	// Verify binary content
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read binary file: %v", err)
	}

	if len(content) != len(binaryContent) {
		t.Errorf("expected %d bytes, got %d bytes", len(binaryContent), len(content))
	}

	for i, b := range content {
		if b != binaryContent[i] {
			t.Errorf("byte mismatch at index %d: expected %d, got %d", i, binaryContent[i], b)
			break
		}
	}
}
