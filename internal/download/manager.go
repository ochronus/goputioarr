package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/services/arr"
	"github.com/ochronus/goputioarr/internal/services/putio"
	"github.com/sirupsen/logrus"
)

// Manager handles the download orchestration
type ArrServiceClient struct {
	Name   string
	Client *arr.Client
}

type Manager struct {
	config       *config.Config
	putioClient  *putio.Client
	arrClients   []ArrServiceClient
	transferChan chan TransferMessage
	downloadChan chan DownloadTargetMessage
	seen         map[uint64]bool
	seenMu       sync.RWMutex
	logger       *logrus.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new download manager
func NewManager(cfg *config.Config, logger *logrus.Logger, putioClient *putio.Client, arrClients []ArrServiceClient) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:       cfg,
		putioClient:  putioClient,
		arrClients:   arrClients,
		transferChan: make(chan TransferMessage, 100),
		downloadChan: make(chan DownloadTargetMessage, 100),
		seen:         make(map[uint64]bool),
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins the download manager's operations with a background context.
func (m *Manager) Start() error {
	return m.StartWithContext(context.Background())
}

// StartWithContext begins the download manager's operations using the provided parent context.
func (m *Manager) StartWithContext(ctx context.Context) error {
	// derive a cancellable context from the provided parent
	m.ctx, m.cancel = context.WithCancel(ctx)

	// Start orchestration workers
	for i := 0; i < m.config.OrchestrationWorkers; i++ {
		m.wg.Add(1)
		go m.orchestrationWorker(i)
	}

	// Start download workers
	for i := 0; i < m.config.DownloadWorkers; i++ {
		m.wg.Add(1)
		go m.downloadWorker(i)
	}

	// Start the transfer producer
	m.wg.Add(1)
	go m.produceTransfers()

	return nil
}

// Stop signals all workers to exit and waits for them to finish.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

// orchestrationWorker handles transfer state transitions
func (m *Manager) orchestrationWorker(id int) {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case msg := <-m.transferChan:
			switch msg.Type {
			case MessageQueuedForDownload:
				m.handleQueuedForDownload(msg.Transfer)
			case MessageDownloaded:
				m.wg.Add(1)
				go m.watchForImport(msg.Transfer)
			case MessageImported:
				m.wg.Add(1)
				go m.watchSeeding(msg.Transfer)
			}
		}
	}
}

// downloadWorker handles file downloads
func (m *Manager) downloadWorker(id int) {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case msg := <-m.downloadChan:
			status := m.downloadTarget(&msg.Target)
			select {
			case <-m.ctx.Done():
				return
			case msg.DoneChan <- status:
			}
		}
	}
}

// handleQueuedForDownload processes a transfer that's ready for download
func (m *Manager) handleQueuedForDownload(transfer *Transfer) {
	m.logger.Infof("%s: download started", transfer)

	targets, err := m.getDownloadTargets(transfer)
	if err != nil {
		m.logger.Errorf("%s: failed to get download targets: %v", transfer, err)
		return
	}

	// Create channels for each target
	doneChans := make([]chan DownloadDoneStatus, len(targets))
	for i, target := range targets {
		doneChans[i] = make(chan DownloadDoneStatus, 1)
		select {
		case <-m.ctx.Done():
			return
		case m.downloadChan <- DownloadTargetMessage{
			Target:   target,
			DoneChan: doneChans[i],
		}:
		}
	}

	// Wait for all downloads to complete
	allSuccess := true
	for _, doneChan := range doneChans {
		select {
		case <-m.ctx.Done():
			return
		case status := <-doneChan:
			if status != DownloadStatusSuccess {
				allSuccess = false
			}
		}
	}

	if allSuccess {
		m.logger.Infof("%s: download done", transfer)
		transfer.SetTargets(targets)
		select {
		case <-m.ctx.Done():
			return
		case m.transferChan <- TransferMessage{
			Type:     MessageDownloaded,
			Transfer: transfer,
		}:
		}
	} else {
		m.logger.Warnf("%s: not all targets downloaded", transfer)
	}
}

// downloadTarget downloads a single target (file or directory)
func (m *Manager) downloadTarget(target *DownloadTarget) DownloadDoneStatus {
	switch target.TargetType {
	case TargetTypeDirectory:
		if _, err := os.Stat(target.To); os.IsNotExist(err) {
			if err := os.MkdirAll(target.To, 0755); err != nil {
				m.logger.Errorf("%s: failed to create directory: %v", target, err)
				return DownloadStatusFailed
			}
			// Change ownership if running as root
			if os.Getuid() == 0 {
				if err := os.Chown(target.To, m.config.UID, -1); err != nil {
					m.logger.Warnf("%s: failed to change ownership: %v", target, err)
				}
			}
			m.logger.Infof("%s: directory created", target)
		}
		return DownloadStatusSuccess

	case TargetTypeFile:
		if _, err := os.Stat(target.To); err == nil {
			m.logger.Infof("%s: already exists", target)
			return DownloadStatusSuccess
		}

		m.logger.Infof("%s: download started", target)
		if err := m.fetchFile(target); err != nil {
			m.logger.Errorf("%s: download failed: %v", target, err)
			return DownloadStatusFailed
		}
		m.logger.Infof("%s: download succeeded", target)
		return DownloadStatusSuccess
	}

	return DownloadStatusFailed
}

// fetchFile downloads a file from a URL
func (m *Manager) fetchFile(target *DownloadTarget) error {
	if target.From == "" {
		return fmt.Errorf("no URL found for target")
	}

	tmpPath := target.To + ".downloading"

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(target.To), 0755); err != nil {
		return err
	}

	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.From, nil)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpPath)
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	tmpFile.Close()

	// Change ownership if running as root
	if os.Getuid() == 0 {
		if err := os.Chown(tmpPath, m.config.UID, -1); err != nil {
			m.logger.Warnf("%s: failed to change ownership: %v", target, err)
		}
	}

	// Rename to final location
	return os.Rename(tmpPath, target.To)
}

// getDownloadTargets recursively builds the list of download targets for a transfer
func (m *Manager) getDownloadTargets(transfer *Transfer) ([]DownloadTarget, error) {
	m.logger.Infof("%s: generating targets", transfer)

	if transfer.FileID == nil {
		return nil, fmt.Errorf("no file ID for transfer")
	}

	return m.recurseDownloadTargets(*transfer.FileID, transfer.GetHash(), "", true)
}

// recurseDownloadTargets recursively builds download targets
func (m *Manager) recurseDownloadTargets(fileID int64, hash string, basePath string, topLevel bool) ([]DownloadTarget, error) {
	if basePath == "" {
		basePath = m.config.DownloadDirectory
	}

	var targets []DownloadTarget

	response, err := m.putioClient.ListFiles(fileID)
	if err != nil {
		return nil, err
	}

	to := filepath.Join(basePath, response.Parent.Name)

	switch response.Parent.FileType {
	case "FOLDER":
		if !ShouldSkipDirectory(response.Parent.Name, m.config.SkipDirectories) {
			targets = append(targets, DownloadTarget{
				From:         "",
				To:           to,
				TargetType:   TargetTypeDirectory,
				TopLevel:     topLevel,
				TransferHash: hash,
			})

			for _, file := range response.Files {
				childTargets, err := m.recurseDownloadTargets(file.ID, hash, to, false)
				if err != nil {
					return nil, err
				}
				targets = append(targets, childTargets...)
			}
		}

	case "VIDEO":
		url, err := m.putioClient.GetFileURL(response.Parent.ID)
		if err != nil {
			return nil, err
		}
		targets = append(targets, DownloadTarget{
			From:         url,
			To:           to,
			TargetType:   TargetTypeFile,
			TopLevel:     topLevel,
			TransferHash: hash,
		})
	}

	return targets, nil
}

// watchForImport watches for a transfer to be imported by arr services
func (m *Manager) watchForImport(transfer *Transfer) {
	defer m.wg.Done()
	m.logger.Infof("%s: watching imports", transfer)

	ticker := time.NewTicker(time.Duration(m.config.PollingInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if m.isImported(transfer) {
				m.logger.Infof("%s: imported", transfer)

				// Clean up downloaded files
				topLevel := transfer.GetTopLevel()
				if topLevel != nil {
					info, err := os.Stat(topLevel.To)
					if err == nil {
						if info.IsDir() {
							os.RemoveAll(topLevel.To)
						} else {
							os.Remove(topLevel.To)
						}
						m.logger.Infof("%s: deleted", topLevel)
					}
				}

				select {
				case <-m.ctx.Done():
					return
				case m.transferChan <- TransferMessage{
					Type:     MessageImported,
					Transfer: transfer,
				}:
				}
				return
			}
		}
	}
}

// isImported checks if all file targets have been imported by arr services
func (m *Manager) isImported(transfer *Transfer) bool {
	fileTargets := transfer.GetFileTargets()
	if len(fileTargets) == 0 {
		return false
	}

	if len(m.arrClients) == 0 {
		return false
	}

	for _, target := range fileTargets {
		imported := false
		for _, svc := range m.arrClients {
			isImported, err := svc.Client.CheckImported(target.To)
			if err != nil {
				m.logger.Errorf("Error checking import from %s: %v", svc.Name, err)
				continue
			}
			if isImported {
				m.logger.Infof("%s: found imported by %s", &target, svc.Name)
				imported = true
				break
			}
		}
		if !imported {
			return false
		}
	}

	return true
}

// watchSeeding watches for a transfer to stop seeding
func (m *Manager) watchSeeding(transfer *Transfer) {
	defer m.wg.Done()
	m.logger.Infof("%s: watching seeding", transfer)

	ticker := time.NewTicker(time.Duration(m.config.PollingInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			resp, err := m.putioClient.GetTransfer(transfer.TransferID)
			if err != nil {
				m.logger.Warnf("%s: failed to get transfer status: %v", transfer, err)
				continue
			}

			if resp.Transfer.Status != "SEEDING" {
				m.logger.Infof("%s: stopped seeding", transfer)

				// Remove transfer from put.io
				if err := m.putioClient.RemoveTransfer(transfer.TransferID); err != nil {
					m.logger.Warnf("%s: failed to remove transfer: %v", transfer, err)
				} else {
					m.logger.Infof("%s: removed from put.io", transfer)
				}

				// Delete remote files
				if transfer.FileID != nil {
					if err := m.putioClient.DeleteFile(*transfer.FileID); err != nil {
						m.logger.Warnf("%s: unable to delete remote files: %v", transfer, err)
					} else {
						m.logger.Infof("%s: deleted remote files", transfer)
					}
				}

				m.logger.Infof("%s: done seeding", transfer)
				return
			}
		}
	}
}

// produceTransfers monitors put.io for new transfers
func (m *Manager) produceTransfers() {
	defer m.wg.Done()

	m.logger.Info("Checking unfinished transfers")

	// Check existing transfers on startup
	m.checkExistingTransfers()

	m.logger.Info("Done checking for unfinished transfers. Starting to monitor transfers.")

	ticker := time.NewTicker(time.Duration(m.config.PollingInterval) * time.Second)
	defer ticker.Stop()

	lastLogTime := time.Now()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			listResp, err := m.putioClient.ListTransfers()
			if err != nil {
				m.logger.Warnf("List put.io transfers failed. Retrying..: %v", err)
				continue
			}

			for _, pt := range listResp.Transfers {
				if m.isSeen(pt.ID) || !pt.IsDownloadable() {
					continue
				}

				transfer := NewTransfer(m.config, &pt)
				m.logger.Infof("%s: ready for download", transfer)

				select {
				case <-m.ctx.Done():
					return
				case m.transferChan <- TransferMessage{
					Type:     MessageQueuedForDownload,
					Transfer: transfer,
				}:
				}

				m.markSeen(pt.ID)
			}

			// Clean up seen list
			activeIDs := make(map[uint64]bool)
			for _, t := range listResp.Transfers {
				activeIDs[t.ID] = true
			}
			m.cleanupSeen(activeIDs)

			// Log status periodically
			if time.Since(lastLogTime) >= 60*time.Second {
				m.logger.Infof("Active transfers: %d", len(listResp.Transfers))
				for _, pt := range listResp.Transfers {
					transfer := NewTransfer(m.config, &pt)
					m.logger.Infof("  %s", transfer)
				}
				lastLogTime = time.Now()
			}
		}
	}
}

// checkExistingTransfers checks for transfers that may have been imported while we were offline
func (m *Manager) checkExistingTransfers() {
	listResp, err := m.putioClient.ListTransfers()
	if err != nil {
		m.logger.Errorf("Failed to list transfers: %v", err)
		return
	}

	for _, pt := range listResp.Transfers {
		name := "??"
		if pt.Name != nil {
			name = *pt.Name
		}

		transfer := NewTransfer(m.config, &pt)

		if pt.IsDownloadable() {
			m.logger.Infof("Getting download target for %s", name)

			targets, err := m.getDownloadTargets(transfer)
			if err != nil {
				m.logger.Warnf("Could not get target for %s: %v", name, err)
				continue
			}

			transfer.SetTargets(targets)

			if m.isImported(transfer) {
				m.logger.Infof("%s: already imported", transfer)
				m.markSeen(transfer.TransferID)
				select {
				case <-m.ctx.Done():
					return
				case m.transferChan <- TransferMessage{
					Type:     MessageImported,
					Transfer: transfer,
				}:
				}
			} else {
				m.logger.Infof("%s: not imported yet", transfer)
			}
		}
	}
}

// isSeen checks if a transfer ID has been seen
func (m *Manager) isSeen(id uint64) bool {
	m.seenMu.RLock()
	defer m.seenMu.RUnlock()
	return m.seen[id]
}

// markSeen marks a transfer ID as seen
func (m *Manager) markSeen(id uint64) {
	m.seenMu.Lock()
	defer m.seenMu.Unlock()
	m.seen[id] = true
}

// cleanupSeen removes IDs from seen that are no longer in the active list
func (m *Manager) cleanupSeen(activeIDs map[uint64]bool) {
	m.seenMu.Lock()
	defer m.seenMu.Unlock()
	for id := range m.seen {
		if !activeIDs[id] {
			delete(m.seen, id)
		}
	}
}
