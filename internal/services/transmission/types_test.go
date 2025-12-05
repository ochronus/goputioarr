package transmission

import (
	"encoding/json"
	"testing"

	"github.com/ochronus/goputioarr/internal/services/putio"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("/downloads")

	if cfg.RPCVersion != "18" {
		t.Errorf("expected RPCVersion '18', got '%s'", cfg.RPCVersion)
	}
	if cfg.Version != "14.0.0" {
		t.Errorf("expected Version '14.0.0', got '%s'", cfg.Version)
	}
	if cfg.DownloadDir != "/downloads" {
		t.Errorf("expected DownloadDir '/downloads', got '%s'", cfg.DownloadDir)
	}
	if cfg.SeedRatioLimit != 1.0 {
		t.Errorf("expected SeedRatioLimit 1.0, got %f", cfg.SeedRatioLimit)
	}
	if !cfg.SeedRatioLimited {
		t.Error("expected SeedRatioLimited to be true")
	}
	if cfg.IdleSeedingLimit != 100 {
		t.Errorf("expected IdleSeedingLimit 100, got %d", cfg.IdleSeedingLimit)
	}
	if cfg.IdleSeedingLimitEnabled {
		t.Error("expected IdleSeedingLimitEnabled to be false")
	}
}

func TestConfigJSON(t *testing.T) {
	cfg := DefaultConfig("/my/downloads")

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	// Check JSON field names
	jsonStr := string(data)

	expectedFields := []string{
		`"rpc-version"`,
		`"version"`,
		`"download-dir"`,
		`"seedRatioLimit"`,
		`"seedRatioLimited"`,
		`"idle-seeding-limit"`,
		`"idle-seeding-limit-enabled"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("expected JSON to contain %s", field)
		}
	}
}

func TestStatusFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected TorrentStatus
	}{
		{"STOPPED", StatusStopped},
		{"COMPLETED", StatusStopped},
		{"ERROR", StatusStopped},
		{"CHECKWAIT", StatusCheckWait},
		{"PREPARING_DOWNLOAD", StatusCheckWait},
		{"CHECK", StatusCheck},
		{"COMPLETING", StatusCheck},
		{"QUEUED", StatusQueued},
		{"IN_QUEUE", StatusQueued},
		{"DOWNLOADING", StatusDownloading},
		{"SEEDINGWAIT", StatusSeedingWait},
		{"SEEDING", StatusSeeding},
		{"UNKNOWN_STATUS", StatusCheckWait}, // default
		{"", StatusCheckWait},               // empty string
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := StatusFromString(tt.input)
			if result != tt.expected {
				t.Errorf("StatusFromString(%s) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTorrentStatusValues(t *testing.T) {
	if StatusStopped != 0 {
		t.Errorf("expected StatusStopped = 0, got %d", StatusStopped)
	}
	if StatusCheckWait != 1 {
		t.Errorf("expected StatusCheckWait = 1, got %d", StatusCheckWait)
	}
	if StatusCheck != 2 {
		t.Errorf("expected StatusCheck = 2, got %d", StatusCheck)
	}
	if StatusQueued != 3 {
		t.Errorf("expected StatusQueued = 3, got %d", StatusQueued)
	}
	if StatusDownloading != 4 {
		t.Errorf("expected StatusDownloading = 4, got %d", StatusDownloading)
	}
	if StatusSeedingWait != 5 {
		t.Errorf("expected StatusSeedingWait = 5, got %d", StatusSeedingWait)
	}
	if StatusSeeding != 6 {
		t.Errorf("expected StatusSeeding = 6, got %d", StatusSeeding)
	}
}

func TestTorrentFromPutIOTransfer(t *testing.T) {
	hash := "abc123"
	name := "Test Movie"
	size := int64(1000000)
	downloaded := int64(500000)
	estimatedTime := int64(120)
	startedAt := "2024-01-15T10:00:00"
	errorMsg := "Some error"
	fileID := int64(999)

	transfer := &putio.Transfer{
		ID:            123,
		Hash:          &hash,
		Name:          &name,
		Size:          &size,
		Downloaded:    &downloaded,
		EstimatedTime: &estimatedTime,
		Status:        "DOWNLOADING",
		StartedAt:     &startedAt,
		ErrorMessage:  &errorMsg,
		FileID:        &fileID,
	}

	torrent := TorrentFromPutIOTransfer(transfer, "/downloads")

	if torrent.ID != 123 {
		t.Errorf("expected ID 123, got %d", torrent.ID)
	}
	if torrent.HashString == nil || *torrent.HashString != "abc123" {
		t.Errorf("unexpected HashString: %v", torrent.HashString)
	}
	if torrent.Name != "Test Movie" {
		t.Errorf("expected Name 'Test Movie', got '%s'", torrent.Name)
	}
	if torrent.DownloadDir != "/downloads" {
		t.Errorf("expected DownloadDir '/downloads', got '%s'", torrent.DownloadDir)
	}
	if torrent.TotalSize != 1000000 {
		t.Errorf("expected TotalSize 1000000, got %d", torrent.TotalSize)
	}
	if torrent.LeftUntilDone != 500000 {
		t.Errorf("expected LeftUntilDone 500000, got %d", torrent.LeftUntilDone)
	}
	if torrent.IsFinished {
		t.Error("expected IsFinished to be false")
	}
	if torrent.ETA != 120 {
		t.Errorf("expected ETA 120, got %d", torrent.ETA)
	}
	if torrent.Status != StatusDownloading {
		t.Errorf("expected Status %d (Downloading), got %d", StatusDownloading, torrent.Status)
	}
	if torrent.ErrorString == nil || *torrent.ErrorString != "Some error" {
		t.Errorf("unexpected ErrorString: %v", torrent.ErrorString)
	}
	if torrent.DownloadedEver != 500000 {
		t.Errorf("expected DownloadedEver 500000, got %d", torrent.DownloadedEver)
	}
	if torrent.FileCount != 1 {
		t.Errorf("expected FileCount 1, got %d", torrent.FileCount)
	}
}

func TestTorrentFromPutIOTransferWithNilFields(t *testing.T) {
	transfer := &putio.Transfer{
		ID:     456,
		Status: "QUEUED",
	}

	torrent := TorrentFromPutIOTransfer(transfer, "/downloads")

	if torrent.ID != 456 {
		t.Errorf("expected ID 456, got %d", torrent.ID)
	}
	if torrent.HashString != nil {
		t.Errorf("expected nil HashString, got %v", torrent.HashString)
	}
	if torrent.Name != "Unknown" {
		t.Errorf("expected Name 'Unknown', got '%s'", torrent.Name)
	}
	if torrent.TotalSize != 0 {
		t.Errorf("expected TotalSize 0, got %d", torrent.TotalSize)
	}
	if torrent.LeftUntilDone != 0 {
		t.Errorf("expected LeftUntilDone 0, got %d", torrent.LeftUntilDone)
	}
	if torrent.ETA != 0 {
		t.Errorf("expected ETA 0, got %d", torrent.ETA)
	}
	if torrent.DownloadedEver != 0 {
		t.Errorf("expected DownloadedEver 0, got %d", torrent.DownloadedEver)
	}
	if torrent.Status != StatusQueued {
		t.Errorf("expected Status %d (Queued), got %d", StatusQueued, torrent.Status)
	}
}

func TestTorrentFromPutIOTransferFinished(t *testing.T) {
	finishedAt := "2024-01-15T12:00:00"
	transfer := &putio.Transfer{
		ID:         789,
		Status:     "COMPLETED",
		FinishedAt: &finishedAt,
	}

	torrent := TorrentFromPutIOTransfer(transfer, "/downloads")

	if !torrent.IsFinished {
		t.Error("expected IsFinished to be true when FinishedAt is set")
	}
}

func TestTorrentFromPutIOTransferLeftUntilDoneNegative(t *testing.T) {
	size := int64(1000)
	downloaded := int64(2000) // More than size (shouldn't happen, but handle it)

	transfer := &putio.Transfer{
		ID:         100,
		Status:     "DOWNLOADING",
		Size:       &size,
		Downloaded: &downloaded,
	}

	torrent := TorrentFromPutIOTransfer(transfer, "/downloads")

	if torrent.LeftUntilDone != 0 {
		t.Errorf("expected LeftUntilDone to be 0 when downloaded > size, got %d", torrent.LeftUntilDone)
	}
}

func TestTorrentJSON(t *testing.T) {
	hash := "testhash"
	torrent := &Torrent{
		ID:                 1,
		HashString:         &hash,
		Name:               "Test",
		DownloadDir:        "/downloads",
		TotalSize:          1000,
		LeftUntilDone:      500,
		IsFinished:         false,
		ETA:                60,
		Status:             StatusDownloading,
		SecondsDownloading: 120,
		DownloadedEver:     500,
		SeedRatioLimit:     2.0,
		SeedRatioMode:      1,
		SeedIdleLimit:      30,
		SeedIdleMode:       1,
		FileCount:          5,
	}

	data, err := json.Marshal(torrent)
	if err != nil {
		t.Fatalf("failed to marshal torrent: %v", err)
	}

	// Check JSON field names (camelCase)
	jsonStr := string(data)
	expectedFields := []string{
		`"id"`,
		`"hashString"`,
		`"name"`,
		`"downloadDir"`,
		`"totalSize"`,
		`"leftUntilDone"`,
		`"isFinished"`,
		`"eta"`,
		`"status"`,
		`"secondsDownloading"`,
		`"downloadedEver"`,
		`"seedRatioLimit"`,
		`"seedRatioMode"`,
		`"seedIdleLimit"`,
		`"seedIdleMode"`,
		`"fileCount"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("expected JSON to contain %s, got: %s", field, jsonStr)
		}
	}
}

func TestResponseJSON(t *testing.T) {
	resp := Response{
		Result:    "success",
		Arguments: map[string]string{"key": "value"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"result"`) {
		t.Error("expected JSON to contain 'result'")
	}
	if !contains(jsonStr, `"success"`) {
		t.Error("expected JSON to contain 'success'")
	}
}

func TestResponseJSONOmitEmpty(t *testing.T) {
	resp := Response{
		Result: "success",
		// Arguments is nil
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	jsonStr := string(data)
	if contains(jsonStr, `"arguments"`) {
		t.Error("expected JSON to omit 'arguments' when nil")
	}
}

func TestRequestJSON(t *testing.T) {
	req := Request{
		Method:    "torrent-get",
		Arguments: map[string]interface{}{"fields": []string{"id", "name"}},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"method"`) {
		t.Error("expected JSON to contain 'method'")
	}
	if !contains(jsonStr, `"torrent-get"`) {
		t.Error("expected JSON to contain 'torrent-get'")
	}
}

func TestTorrentAddArguments(t *testing.T) {
	args := TorrentAddArguments{
		Metainfo: "base64data",
		Filename: "",
	}

	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"metainfo"`) {
		t.Error("expected JSON to contain 'metainfo'")
	}
}

func TestTorrentRemoveArguments(t *testing.T) {
	args := TorrentRemoveArguments{
		IDs:             []string{"hash1", "hash2"},
		DeleteLocalData: true,
	}

	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"ids"`) {
		t.Error("expected JSON to contain 'ids'")
	}
	if !contains(jsonStr, `"delete-local-data"`) {
		t.Error("expected JSON to contain 'delete-local-data'")
	}
	if !contains(jsonStr, "true") {
		t.Error("expected JSON to contain 'true' for delete-local-data")
	}
}

func TestTorrentGetResponse(t *testing.T) {
	hash1 := "hash1"
	hash2 := "hash2"
	resp := TorrentGetResponse{
		Torrents: []*Torrent{
			{ID: 1, HashString: &hash1, Name: "Torrent 1"},
			{ID: 2, HashString: &hash2, Name: "Torrent 2"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"torrents"`) {
		t.Error("expected JSON to contain 'torrents'")
	}
	if !contains(jsonStr, `"Torrent 1"`) {
		t.Error("expected JSON to contain 'Torrent 1'")
	}
	if !contains(jsonStr, `"Torrent 2"`) {
		t.Error("expected JSON to contain 'Torrent 2'")
	}
}

func TestTorrentFromPutIOTransferSecondsDownloading(t *testing.T) {
	// Test with valid startedAt
	startedAt := "2024-01-15T10:00:00"
	transfer := &putio.Transfer{
		ID:        1,
		Status:    "DOWNLOADING",
		StartedAt: &startedAt,
	}

	torrent := TorrentFromPutIOTransfer(transfer, "/downloads")

	// SecondsDownloading should be positive (time since startedAt)
	if torrent.SecondsDownloading < 0 {
		t.Errorf("expected SecondsDownloading >= 0, got %d", torrent.SecondsDownloading)
	}
}

func TestTorrentFromPutIOTransferInvalidStartedAt(t *testing.T) {
	// Test with invalid date format
	invalidDate := "invalid-date"
	transfer := &putio.Transfer{
		ID:        1,
		Status:    "DOWNLOADING",
		StartedAt: &invalidDate,
	}

	// Should not panic, should use current time as fallback
	torrent := TorrentFromPutIOTransfer(transfer, "/downloads")

	if torrent.SecondsDownloading < 0 {
		t.Errorf("expected SecondsDownloading >= 0, got %d", torrent.SecondsDownloading)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDefaultConfigDifferentDownloadDirs(t *testing.T) {
	testDirs := []string{
		"/downloads",
		"/home/user/media",
		"/mnt/storage/torrents",
		"",
	}

	for _, dir := range testDirs {
		t.Run(dir, func(t *testing.T) {
			cfg := DefaultConfig(dir)
			if cfg.DownloadDir != dir {
				t.Errorf("expected DownloadDir '%s', got '%s'", dir, cfg.DownloadDir)
			}
		})
	}
}

func TestConfigSeedRatioSettings(t *testing.T) {
	cfg := DefaultConfig("/downloads")

	if cfg.SeedRatioLimit != 1.0 {
		t.Errorf("expected SeedRatioLimit 1.0, got %f", cfg.SeedRatioLimit)
	}
	if !cfg.SeedRatioLimited {
		t.Error("expected SeedRatioLimited to be true")
	}
}

func TestConfigIdleSeedingSettings(t *testing.T) {
	cfg := DefaultConfig("/downloads")

	if cfg.IdleSeedingLimit != 100 {
		t.Errorf("expected IdleSeedingLimit 100, got %d", cfg.IdleSeedingLimit)
	}
	if cfg.IdleSeedingLimitEnabled {
		t.Error("expected IdleSeedingLimitEnabled to be false")
	}
}

func TestTorrentAllFields(t *testing.T) {
	hash := "abc123def456"
	errorStr := "Test error"

	torrent := &Torrent{
		ID:                 123,
		HashString:         &hash,
		Name:               "Test Torrent",
		DownloadDir:        "/downloads",
		TotalSize:          1000000000,
		LeftUntilDone:      500000000,
		IsFinished:         false,
		ETA:                3600,
		Status:             StatusDownloading,
		SecondsDownloading: 7200,
		ErrorString:        &errorStr,
		DownloadedEver:     500000000,
		SeedRatioLimit:     2.0,
		SeedRatioMode:      1,
		SeedIdleLimit:      30,
		SeedIdleMode:       1,
		FileCount:          10,
	}

	if torrent.ID != 123 {
		t.Errorf("expected ID 123, got %d", torrent.ID)
	}
	if *torrent.HashString != hash {
		t.Errorf("expected HashString '%s', got '%s'", hash, *torrent.HashString)
	}
	if torrent.TotalSize != 1000000000 {
		t.Errorf("expected TotalSize 1000000000, got %d", torrent.TotalSize)
	}
	if *torrent.ErrorString != errorStr {
		t.Errorf("expected ErrorString '%s', got '%s'", errorStr, *torrent.ErrorString)
	}
}

func TestTorrentWithNilHashString(t *testing.T) {
	torrent := &Torrent{
		ID:         1,
		HashString: nil,
		Name:       "No Hash Torrent",
	}

	if torrent.HashString != nil {
		t.Error("expected nil HashString")
	}
}

func TestTorrentWithNilErrorString(t *testing.T) {
	torrent := &Torrent{
		ID:          1,
		Name:        "No Error Torrent",
		ErrorString: nil,
	}

	if torrent.ErrorString != nil {
		t.Error("expected nil ErrorString")
	}
}

func TestStatusFromStringAllStatuses(t *testing.T) {
	allCases := map[string]TorrentStatus{
		"STOPPED":            StatusStopped,
		"COMPLETED":          StatusStopped,
		"ERROR":              StatusStopped,
		"CHECKWAIT":          StatusCheckWait,
		"PREPARING_DOWNLOAD": StatusCheckWait,
		"CHECK":              StatusCheck,
		"COMPLETING":         StatusCheck,
		"QUEUED":             StatusQueued,
		"IN_QUEUE":           StatusQueued,
		"DOWNLOADING":        StatusDownloading,
		"SEEDINGWAIT":        StatusSeedingWait,
		"SEEDING":            StatusSeeding,
	}

	for input, expected := range allCases {
		result := StatusFromString(input)
		if result != expected {
			t.Errorf("StatusFromString(%s) = %d, expected %d", input, result, expected)
		}
	}
}

func TestTorrentFromPutIOTransferCompletedDownload(t *testing.T) {
	hash := "completed123"
	name := "Completed Movie"
	size := int64(2000000000)
	downloaded := int64(2000000000)
	finishedAt := "2024-01-15T14:00:00"

	transfer := &putio.Transfer{
		ID:         100,
		Hash:       &hash,
		Name:       &name,
		Size:       &size,
		Downloaded: &downloaded,
		Status:     "COMPLETED",
		FinishedAt: &finishedAt,
	}

	torrent := TorrentFromPutIOTransfer(transfer, "/downloads")

	if torrent.LeftUntilDone != 0 {
		t.Errorf("expected LeftUntilDone 0 for completed, got %d", torrent.LeftUntilDone)
	}
	if !torrent.IsFinished {
		t.Error("expected IsFinished to be true")
	}
	if torrent.Status != StatusStopped {
		t.Errorf("expected Status StatusStopped, got %d", torrent.Status)
	}
}

func TestTorrentFromPutIOTransferSeeding(t *testing.T) {
	transfer := &putio.Transfer{
		ID:     200,
		Status: "SEEDING",
	}

	torrent := TorrentFromPutIOTransfer(transfer, "/downloads")

	if torrent.Status != StatusSeeding {
		t.Errorf("expected Status StatusSeeding, got %d", torrent.Status)
	}
}

func TestTorrentFromPutIOTransferError(t *testing.T) {
	errorMsg := "Download failed: connection refused"
	transfer := &putio.Transfer{
		ID:           300,
		Status:       "ERROR",
		ErrorMessage: &errorMsg,
	}

	torrent := TorrentFromPutIOTransfer(transfer, "/downloads")

	if torrent.Status != StatusStopped {
		t.Errorf("expected Status StatusStopped for error, got %d", torrent.Status)
	}
	if torrent.ErrorString == nil || *torrent.ErrorString != errorMsg {
		t.Errorf("expected ErrorString '%s', got %v", errorMsg, torrent.ErrorString)
	}
}

func TestRequestWithArguments(t *testing.T) {
	args := map[string]interface{}{
		"fields": []string{"id", "name", "status"},
		"ids":    []int{1, 2, 3},
	}

	req := Request{
		Method:    "torrent-get",
		Arguments: args,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"method"`) || !contains(jsonStr, `"torrent-get"`) {
		t.Error("expected JSON to contain method and torrent-get")
	}
	if !contains(jsonStr, `"arguments"`) {
		t.Error("expected JSON to contain arguments")
	}
}

func TestRequestWithNilArguments(t *testing.T) {
	req := Request{
		Method:    "session-get",
		Arguments: nil,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"method"`) {
		t.Error("expected JSON to contain method")
	}
}

func TestResponseResultTypes(t *testing.T) {
	results := []string{"success", "error", "invalid argument"}

	for _, result := range results {
		t.Run(result, func(t *testing.T) {
			resp := Response{
				Result: result,
			}

			data, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			if !contains(string(data), result) {
				t.Errorf("expected JSON to contain '%s'", result)
			}
		})
	}
}

func TestTorrentAddArgumentsMetainfo(t *testing.T) {
	args := TorrentAddArguments{
		Metainfo: "base64encodedtorrentdata",
	}

	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"metainfo"`) {
		t.Error("expected JSON to contain metainfo")
	}
	if !contains(jsonStr, "base64encodedtorrentdata") {
		t.Error("expected JSON to contain metainfo value")
	}
}

func TestTorrentAddArgumentsFilename(t *testing.T) {
	args := TorrentAddArguments{
		Filename: "magnet:?xt=urn:btih:abc123",
	}

	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"filename"`) {
		t.Error("expected JSON to contain filename")
	}
}

func TestTorrentRemoveArgumentsDeleteLocalData(t *testing.T) {
	tests := []struct {
		name            string
		deleteLocalData bool
	}{
		{"delete true", true},
		{"delete false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := TorrentRemoveArguments{
				IDs:             []string{"hash1"},
				DeleteLocalData: tt.deleteLocalData,
			}

			data, err := json.Marshal(args)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			jsonStr := string(data)
			if !contains(jsonStr, `"delete-local-data"`) {
				t.Error("expected JSON to contain delete-local-data")
			}
		})
	}
}

func TestTorrentGetResponseEmptyTorrents(t *testing.T) {
	resp := TorrentGetResponse{
		Torrents: []*Torrent{},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"torrents":[]`) {
		t.Error("expected JSON to contain empty torrents array")
	}
}

func TestTorrentGetResponseNilTorrents(t *testing.T) {
	resp := TorrentGetResponse{
		Torrents: nil,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// nil slice should marshal as null
	jsonStr := string(data)
	if !contains(jsonStr, `"torrents"`) {
		t.Error("expected JSON to contain torrents key")
	}
}

func TestTorrentFromPutIOTransferZeroValues(t *testing.T) {
	size := int64(0)
	downloaded := int64(0)
	estimatedTime := int64(0)

	transfer := &putio.Transfer{
		ID:            1,
		Status:        "QUEUED",
		Size:          &size,
		Downloaded:    &downloaded,
		EstimatedTime: &estimatedTime,
	}

	torrent := TorrentFromPutIOTransfer(transfer, "/downloads")

	if torrent.TotalSize != 0 {
		t.Errorf("expected TotalSize 0, got %d", torrent.TotalSize)
	}
	if torrent.DownloadedEver != 0 {
		t.Errorf("expected DownloadedEver 0, got %d", torrent.DownloadedEver)
	}
	if torrent.ETA != 0 {
		t.Errorf("expected ETA 0, got %d", torrent.ETA)
	}
}
