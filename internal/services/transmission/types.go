package transmission

import (
	"time"

	"github.com/ochronus/goputioarr/internal/services/putio"
)

// Response represents a Transmission RPC response
type Response struct {
	Result    string      `json:"result"`
	Arguments interface{} `json:"arguments,omitempty"`
}

// Request represents a Transmission RPC request
type Request struct {
	Method    string      `json:"method"`
	Arguments interface{} `json:"arguments,omitempty"`
}

// Config represents Transmission session configuration
type Config struct {
	RPCVersion              string  `json:"rpc-version"`
	Version                 string  `json:"version"`
	DownloadDir             string  `json:"download-dir"`
	SeedRatioLimit          float32 `json:"seedRatioLimit"`
	SeedRatioLimited        bool    `json:"seedRatioLimited"`
	IdleSeedingLimit        uint64  `json:"idle-seeding-limit"`
	IdleSeedingLimitEnabled bool    `json:"idle-seeding-limit-enabled"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig(downloadDir string) *Config {
	return &Config{
		RPCVersion:              "18",
		Version:                 "14.0.0",
		DownloadDir:             downloadDir,
		SeedRatioLimit:          1.0,
		SeedRatioLimited:        true,
		IdleSeedingLimit:        100,
		IdleSeedingLimitEnabled: false,
	}
}

// Torrent represents a Transmission torrent
type Torrent struct {
	ID                 uint64        `json:"id"`
	HashString         *string       `json:"hashString"`
	Name               string        `json:"name"`
	DownloadDir        string        `json:"downloadDir"`
	TotalSize          int64         `json:"totalSize"`
	LeftUntilDone      int64         `json:"leftUntilDone"`
	IsFinished         bool          `json:"isFinished"`
	ETA                int64         `json:"eta"`
	Status             TorrentStatus `json:"status"`
	SecondsDownloading int64         `json:"secondsDownloading"`
	ErrorString        *string       `json:"errorString"`
	DownloadedEver     int64         `json:"downloadedEver"`
	SeedRatioLimit     float32       `json:"seedRatioLimit"`
	SeedRatioMode      uint32        `json:"seedRatioMode"`
	SeedIdleLimit      uint64        `json:"seedIdleLimit"`
	SeedIdleMode       uint32        `json:"seedIdleMode"`
	FileCount          uint32        `json:"fileCount"`
}

// TorrentStatus represents the status of a torrent
type TorrentStatus int

const (
	StatusStopped     TorrentStatus = 0
	StatusCheckWait   TorrentStatus = 1
	StatusCheck       TorrentStatus = 2
	StatusQueued      TorrentStatus = 3
	StatusDownloading TorrentStatus = 4
	StatusSeedingWait TorrentStatus = 5
	StatusSeeding     TorrentStatus = 6
)

// StatusFromString converts a put.io status string to a TorrentStatus
func StatusFromString(status string) TorrentStatus {
	switch status {
	case "STOPPED", "COMPLETED", "ERROR":
		return StatusStopped
	case "CHECKWAIT", "PREPARING_DOWNLOAD":
		return StatusCheckWait
	case "CHECK", "COMPLETING":
		return StatusCheck
	case "QUEUED", "IN_QUEUE":
		return StatusQueued
	case "DOWNLOADING":
		return StatusDownloading
	case "SEEDINGWAIT":
		return StatusSeedingWait
	case "SEEDING":
		return StatusSeeding
	default:
		return StatusCheckWait
	}
}

// TorrentFromPutIOTransfer converts a put.io Transfer to a Transmission Torrent
func TorrentFromPutIOTransfer(t *putio.Transfer, downloadDir string) *Torrent {
	var startedAt time.Time
	if t.StartedAt != nil {
		parsed, err := time.Parse("2006-01-02T15:04:05", *t.StartedAt)
		if err == nil {
			startedAt = parsed
		} else {
			startedAt = time.Now().UTC()
		}
	} else {
		startedAt = time.Now().UTC()
	}

	secondsDownloading := int64(time.Since(startedAt).Seconds())

	name := "Unknown"
	if t.Name != nil {
		name = *t.Name
	}

	var totalSize int64
	if t.Size != nil {
		totalSize = *t.Size
	}

	var downloaded int64
	if t.Downloaded != nil {
		downloaded = *t.Downloaded
	}

	leftUntilDone := totalSize - downloaded
	if leftUntilDone < 0 {
		leftUntilDone = 0
	}

	var eta int64
	if t.EstimatedTime != nil {
		eta = *t.EstimatedTime
	}

	return &Torrent{
		ID:                 t.ID,
		HashString:         t.Hash,
		Name:               name,
		DownloadDir:        downloadDir,
		TotalSize:          totalSize,
		LeftUntilDone:      leftUntilDone,
		IsFinished:         t.FinishedAt != nil,
		ETA:                eta,
		Status:             StatusFromString(t.Status),
		SecondsDownloading: secondsDownloading,
		ErrorString:        t.ErrorMessage,
		DownloadedEver:     downloaded,
		SeedRatioLimit:     0.0,
		SeedRatioMode:      0,
		SeedIdleLimit:      0,
		SeedIdleMode:       0,
		FileCount:          1,
	}
}

// TorrentAddArguments represents arguments for torrent-add method
type TorrentAddArguments struct {
	Metainfo string `json:"metainfo,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// TorrentRemoveArguments represents arguments for torrent-remove method
type TorrentRemoveArguments struct {
	IDs             []string `json:"ids"`
	DeleteLocalData bool     `json:"delete-local-data"`
}

// TorrentGetResponse represents the response for torrent-get method
type TorrentGetResponse struct {
	Torrents []*Torrent `json:"torrents"`
}
