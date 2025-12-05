package download

import (
	"testing"

	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/services/putio"
)

func TestDownloadTargetString(t *testing.T) {
	tests := []struct {
		name     string
		target   DownloadTarget
		expected string
	}{
		{
			name: "full hash",
			target: DownloadTarget{
				TransferHash: "abcdef1234567890",
				To:           "/downloads/movie.mkv",
			},
			expected: "[abcd: /downloads/movie.mkv]",
		},
		{
			name: "short hash",
			target: DownloadTarget{
				TransferHash: "abc",
				To:           "/downloads/file.mkv",
			},
			expected: "[abc: /downloads/file.mkv]",
		},
		{
			name: "exact 4 char hash",
			target: DownloadTarget{
				TransferHash: "1234",
				To:           "/path/to/file",
			},
			expected: "[1234: /path/to/file]",
		},
		{
			name: "empty hash",
			target: DownloadTarget{
				TransferHash: "",
				To:           "/downloads/test.mkv",
			},
			expected: "[: /downloads/test.mkv]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.target.String()
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestNewTransfer(t *testing.T) {
	cfg := &config.Config{
		DownloadDirectory: "/downloads",
	}

	name := "Test Transfer"
	hash := "abc123def456"
	fileID := int64(999)

	pt := &putio.Transfer{
		ID:     123,
		Name:   &name,
		Hash:   &hash,
		FileID: &fileID,
	}

	transfer := NewTransfer(cfg, pt)

	if transfer.TransferID != 123 {
		t.Errorf("expected TransferID 123, got %d", transfer.TransferID)
	}
	if transfer.Name != "Test Transfer" {
		t.Errorf("expected Name 'Test Transfer', got '%s'", transfer.Name)
	}
	if transfer.Hash == nil || *transfer.Hash != "abc123def456" {
		t.Errorf("unexpected Hash: %v", transfer.Hash)
	}
	if transfer.FileID == nil || *transfer.FileID != 999 {
		t.Errorf("unexpected FileID: %v", transfer.FileID)
	}
	if transfer.Config != cfg {
		t.Error("Config not set correctly")
	}
	if transfer.Targets != nil {
		t.Error("expected Targets to be nil initially")
	}
}

func TestNewTransferWithNilName(t *testing.T) {
	cfg := &config.Config{}

	pt := &putio.Transfer{
		ID:   456,
		Name: nil,
	}

	transfer := NewTransfer(cfg, pt)

	if transfer.Name != "Unknown" {
		t.Errorf("expected Name 'Unknown' when nil, got '%s'", transfer.Name)
	}
}

func TestTransferString(t *testing.T) {
	tests := []struct {
		name     string
		hash     *string
		tName    string
		expected string
	}{
		{
			name:     "with full hash",
			hash:     ptrString("abcdef123456"),
			tName:    "My Transfer",
			expected: "[abcd: My Transfer]",
		},
		{
			name:     "with nil hash",
			hash:     nil,
			tName:    "No Hash Transfer",
			expected: "[0000: No Hash Transfer]",
		},
		{
			name:     "with short hash",
			hash:     ptrString("ab"),
			tName:    "Short Hash",
			expected: "[0000: Short Hash]",
		},
		{
			name:     "with exact 4 char hash",
			hash:     ptrString("wxyz"),
			tName:    "Exact Hash",
			expected: "[wxyz: Exact Hash]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transfer := &Transfer{
				Hash: tt.hash,
				Name: tt.tName,
			}
			result := transfer.String()
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestTransferGetHash(t *testing.T) {
	tests := []struct {
		name     string
		hash     *string
		expected string
	}{
		{
			name:     "with hash",
			hash:     ptrString("myhash123"),
			expected: "myhash123",
		},
		{
			name:     "nil hash",
			hash:     nil,
			expected: "0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transfer := &Transfer{Hash: tt.hash}
			result := transfer.GetHash()
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestTransferSetAndGetTargets(t *testing.T) {
	transfer := &Transfer{}

	targets := []DownloadTarget{
		{To: "/downloads/file1.mkv", TargetType: TargetTypeFile},
		{To: "/downloads/folder", TargetType: TargetTypeDirectory},
	}

	transfer.SetTargets(targets)
	result := transfer.GetTargets()

	if len(result) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(result))
	}
	if result[0].To != "/downloads/file1.mkv" {
		t.Errorf("unexpected first target: %v", result[0])
	}
	if result[1].To != "/downloads/folder" {
		t.Errorf("unexpected second target: %v", result[1])
	}
}

func TestTransferGetTopLevel(t *testing.T) {
	tests := []struct {
		name     string
		targets  []DownloadTarget
		expected *DownloadTarget
	}{
		{
			name: "has top level",
			targets: []DownloadTarget{
				{To: "/downloads/sub/file.mkv", TopLevel: false},
				{To: "/downloads/main", TopLevel: true},
				{To: "/downloads/sub/other.mkv", TopLevel: false},
			},
			expected: &DownloadTarget{To: "/downloads/main", TopLevel: true},
		},
		{
			name: "no top level",
			targets: []DownloadTarget{
				{To: "/downloads/file1.mkv", TopLevel: false},
				{To: "/downloads/file2.mkv", TopLevel: false},
			},
			expected: nil,
		},
		{
			name:     "empty targets",
			targets:  []DownloadTarget{},
			expected: nil,
		},
		{
			name:     "nil targets",
			targets:  nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transfer := &Transfer{}
			transfer.SetTargets(tt.targets)
			result := transfer.GetTopLevel()

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if result.To != tt.expected.To {
					t.Errorf("expected To '%s', got '%s'", tt.expected.To, result.To)
				}
			}
		})
	}
}

func TestTransferGetFileTargets(t *testing.T) {
	transfer := &Transfer{}
	transfer.SetTargets([]DownloadTarget{
		{To: "/downloads/folder", TargetType: TargetTypeDirectory},
		{To: "/downloads/file1.mkv", TargetType: TargetTypeFile},
		{To: "/downloads/subfolder", TargetType: TargetTypeDirectory},
		{To: "/downloads/file2.mkv", TargetType: TargetTypeFile},
		{To: "/downloads/file3.mkv", TargetType: TargetTypeFile},
	})

	result := transfer.GetFileTargets()

	if len(result) != 3 {
		t.Fatalf("expected 3 file targets, got %d", len(result))
	}

	for _, target := range result {
		if target.TargetType != TargetTypeFile {
			t.Errorf("expected TargetTypeFile, got %v", target.TargetType)
		}
	}
}

func TestTransferGetFileTargetsEmpty(t *testing.T) {
	transfer := &Transfer{}
	transfer.SetTargets([]DownloadTarget{
		{To: "/downloads/folder1", TargetType: TargetTypeDirectory},
		{To: "/downloads/folder2", TargetType: TargetTypeDirectory},
	})

	result := transfer.GetFileTargets()

	if len(result) != 0 {
		t.Errorf("expected 0 file targets, got %d", len(result))
	}
}

func TestTransferMessageTypeString(t *testing.T) {
	tests := []struct {
		msgType  TransferMessageType
		expected string
	}{
		{MessageQueuedForDownload, "QueuedForDownload"},
		{MessageDownloaded, "Downloaded"},
		{MessageImported, "Imported"},
		{TransferMessageType(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.msgType.String()
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestTransferMessageTypeValues(t *testing.T) {
	if MessageQueuedForDownload != 0 {
		t.Errorf("expected MessageQueuedForDownload = 0, got %d", MessageQueuedForDownload)
	}
	if MessageDownloaded != 1 {
		t.Errorf("expected MessageDownloaded = 1, got %d", MessageDownloaded)
	}
	if MessageImported != 2 {
		t.Errorf("expected MessageImported = 2, got %d", MessageImported)
	}
}

func TestTargetTypeValues(t *testing.T) {
	if TargetTypeDirectory != 0 {
		t.Errorf("expected TargetTypeDirectory = 0, got %d", TargetTypeDirectory)
	}
	if TargetTypeFile != 1 {
		t.Errorf("expected TargetTypeFile = 1, got %d", TargetTypeFile)
	}
}

func TestDownloadDoneStatusValues(t *testing.T) {
	if DownloadStatusSuccess != 0 {
		t.Errorf("expected DownloadStatusSuccess = 0, got %d", DownloadStatusSuccess)
	}
	if DownloadStatusFailed != 1 {
		t.Errorf("expected DownloadStatusFailed = 1, got %d", DownloadStatusFailed)
	}
}

func TestShouldSkipDirectory(t *testing.T) {
	skipDirs := []string{"sample", "extras", "Sample", "EXTRAS"}

	tests := []struct {
		name     string
		dirName  string
		expected bool
	}{
		{"exact match lowercase", "sample", true},
		{"exact match uppercase", "SAMPLE", true},
		{"exact match mixed case", "Sample", true},
		{"exact match extras", "extras", true},
		{"exact match EXTRAS", "EXTRAS", true},
		{"not in list", "videos", false},
		{"partial match should not skip", "samples", false},
		{"empty string", "", false},
		{"substring of skip dir", "samp", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldSkipDirectory(tt.dirName, skipDirs)
			if result != tt.expected {
				t.Errorf("ShouldSkipDirectory(%s) = %v, expected %v", tt.dirName, result, tt.expected)
			}
		})
	}
}

func TestShouldSkipDirectoryEmptyList(t *testing.T) {
	result := ShouldSkipDirectory("sample", []string{})
	if result {
		t.Error("expected false for empty skip list")
	}
}

func TestShouldSkipDirectoryNilList(t *testing.T) {
	result := ShouldSkipDirectory("sample", nil)
	if result {
		t.Error("expected false for nil skip list")
	}
}

func TestDownloadTargetMessage(t *testing.T) {
	doneChan := make(chan DownloadDoneStatus, 1)
	target := DownloadTarget{
		To:         "/downloads/file.mkv",
		TargetType: TargetTypeFile,
	}

	msg := DownloadTargetMessage{
		Target:   target,
		DoneChan: doneChan,
	}

	if msg.Target.To != "/downloads/file.mkv" {
		t.Errorf("unexpected target To: %s", msg.Target.To)
	}
	if msg.DoneChan == nil {
		t.Error("expected non-nil DoneChan")
	}

	// Test channel works
	go func() {
		msg.DoneChan <- DownloadStatusSuccess
	}()

	status := <-msg.DoneChan
	if status != DownloadStatusSuccess {
		t.Errorf("expected DownloadStatusSuccess, got %v", status)
	}
}

func TestTransferMessage(t *testing.T) {
	transfer := &Transfer{
		Name:       "Test",
		TransferID: 123,
	}

	msg := TransferMessage{
		Type:     MessageQueuedForDownload,
		Transfer: transfer,
	}

	if msg.Type != MessageQueuedForDownload {
		t.Errorf("expected MessageQueuedForDownload, got %v", msg.Type)
	}
	if msg.Transfer != transfer {
		t.Error("Transfer not set correctly")
	}
}

func TestDownloadTargetFields(t *testing.T) {
	target := DownloadTarget{
		From:         "https://example.com/file.mkv",
		To:           "/downloads/file.mkv",
		TargetType:   TargetTypeFile,
		TopLevel:     true,
		TransferHash: "abc123",
	}

	if target.From != "https://example.com/file.mkv" {
		t.Errorf("unexpected From: %s", target.From)
	}
	if target.To != "/downloads/file.mkv" {
		t.Errorf("unexpected To: %s", target.To)
	}
	if target.TargetType != TargetTypeFile {
		t.Errorf("unexpected TargetType: %v", target.TargetType)
	}
	if !target.TopLevel {
		t.Error("expected TopLevel to be true")
	}
	if target.TransferHash != "abc123" {
		t.Errorf("unexpected TransferHash: %s", target.TransferHash)
	}
}

// Helper function to create pointer to string
func ptrString(v string) *string {
	return &v
}
