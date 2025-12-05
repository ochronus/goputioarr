package download

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/services/putio"
)

// TargetType represents the type of download target
type TargetType int

const (
	TargetTypeDirectory TargetType = iota
	TargetTypeFile
)

// DownloadTarget represents a file or directory to be downloaded
type DownloadTarget struct {
	From         string     `json:"from,omitempty"`
	To           string     `json:"to"`
	TargetType   TargetType `json:"target_type"`
	TopLevel     bool       `json:"top_level"`
	TransferHash string     `json:"transfer_hash"`
}

// String returns a formatted string representation of the download target
func (dt *DownloadTarget) String() string {
	hash := dt.TransferHash
	if len(hash) > 4 {
		hash = hash[:4]
	}
	return fmt.Sprintf("[%s: %s]", hash, dt.To)
}

// Transfer represents a put.io transfer being processed
type Transfer struct {
	Name       string
	FileID     *int64
	Hash       *string
	TransferID uint64
	Targets    []DownloadTarget
	Config     *config.Config
	mu         sync.RWMutex
}

// NewTransfer creates a new Transfer from a put.io transfer
func NewTransfer(cfg *config.Config, pt *putio.Transfer) *Transfer {
	name := "Unknown"
	if pt.Name != nil {
		name = *pt.Name
	}

	return &Transfer{
		TransferID: pt.ID,
		Name:       name,
		FileID:     pt.FileID,
		Hash:       pt.Hash,
		Targets:    nil,
		Config:     cfg,
	}
}

// String returns a formatted string representation of the transfer
func (t *Transfer) String() string {
	hash := "0000"
	if t.Hash != nil && len(*t.Hash) >= 4 {
		hash = (*t.Hash)[:4]
	}
	return fmt.Sprintf("[%s: %s]", hash, t.Name)
}

// GetHash returns the hash or a default value
func (t *Transfer) GetHash() string {
	if t.Hash != nil {
		return *t.Hash
	}
	return "0000"
}

// SetTargets sets the download targets for this transfer
func (t *Transfer) SetTargets(targets []DownloadTarget) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Targets = targets
}

// GetTargets returns the download targets for this transfer
func (t *Transfer) GetTargets() []DownloadTarget {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Targets
}

// GetTopLevel returns the top-level download target
func (t *Transfer) GetTopLevel() *DownloadTarget {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, target := range t.Targets {
		if target.TopLevel {
			return &target
		}
	}
	return nil
}

// GetFileTargets returns only file targets (not directories)
func (t *Transfer) GetFileTargets() []DownloadTarget {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var fileTargets []DownloadTarget
	for _, target := range t.Targets {
		if target.TargetType == TargetTypeFile {
			fileTargets = append(fileTargets, target)
		}
	}
	return fileTargets
}

// TransferMessage represents a message about transfer state changes
type TransferMessage struct {
	Type     TransferMessageType
	Transfer *Transfer
}

// TransferMessageType represents the type of transfer message
type TransferMessageType int

const (
	// MessageQueuedForDownload indicates the transfer is ready to be downloaded
	MessageQueuedForDownload TransferMessageType = iota
	// MessageDownloaded indicates the transfer has been downloaded
	MessageDownloaded
	// MessageImported indicates the transfer has been imported by arr services
	MessageImported
)

// String returns a string representation of the message type
func (t TransferMessageType) String() string {
	switch t {
	case MessageQueuedForDownload:
		return "QueuedForDownload"
	case MessageDownloaded:
		return "Downloaded"
	case MessageImported:
		return "Imported"
	default:
		return "Unknown"
	}
}

// DownloadTargetMessage represents a message to download a specific target
type DownloadTargetMessage struct {
	Target   DownloadTarget
	DoneChan chan DownloadDoneStatus
}

// DownloadDoneStatus represents the result of a download operation
type DownloadDoneStatus int

const (
	DownloadStatusSuccess DownloadDoneStatus = iota
	DownloadStatusFailed
)

// ShouldSkipDirectory checks if a directory should be skipped based on configuration
func ShouldSkipDirectory(name string, skipDirs []string) bool {
	lowerName := strings.ToLower(name)
	for _, skipDir := range skipDirs {
		if strings.ToLower(skipDir) == lowerName {
			return true
		}
	}
	return false
}
