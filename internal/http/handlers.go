package http

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/gin-gonic/gin/binding"
	"github.com/ochronus/goputioarr/internal/app"
	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/services/putio"
	"github.com/ochronus/goputioarr/internal/services/transmission"
	"github.com/sirupsen/logrus"
)

const sessionID = "useless-session-id"

// Handler contains the HTTP handlers for the Transmission RPC protocol.
type Handler struct {
	container   *app.Container
	config      *config.Config
	putioClient putio.ClientAPI
	logger      *logrus.Logger
}

// NewHandler creates a new HTTP handler.
func NewHandler(container *app.Container) *Handler {
	return &Handler{
		container:   container,
		config:      container.Config,
		putioClient: container.PutioClient,
		logger:      container.Logger,
	}
}

// RPCPost handles POST requests to the Transmission RPC endpoint.
func (h *Handler) RPCPost(c *gin.Context) {
	// Validate user
	if !h.validateUser(c) {
		c.Header("X-Transmission-Session-Id", sessionID)
		c.Status(http.StatusConflict)
		return
	}

	var req transmission.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if strings.TrimSpace(req.Method) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "method is required"})
		return
	}

	var (
		arguments interface{}
		err       error
	)

	switch req.Method {
	case "session-get":
		arguments = transmission.DefaultConfig(h.config.DownloadDirectory)

	case "torrent-get":
		arguments, err = h.handleTorrentGet()
		if err != nil {
			h.logger.Errorf("torrent-get error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "torrent-set":
		// Nothing to do here
		arguments = nil

	case "queue-move-top":
		// Nothing to do here
		arguments = nil

	case "torrent-remove":
		err = h.handleTorrentRemove(&req)
		if err != nil {
			h.logger.Errorf("torrent-remove error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		arguments = nil

	case "torrent-add":
		err = h.handleTorrentAdd(&req)
		if err != nil {
			h.logger.Errorf("torrent-add error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		arguments = nil

	default:
		h.logger.Warnf("Unknown method: %s", req.Method)
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown method"})
		return
	}

	response := transmission.Response{
		Result:    "success",
		Arguments: arguments,
	}

	c.JSON(http.StatusOK, response)
}

// RPCGet handles GET requests to the Transmission RPC endpoint (for authentication).
func (h *Handler) RPCGet(c *gin.Context) {
	if !h.validateUser(c) {
		c.Status(http.StatusForbidden)
		return
	}

	c.Header("X-Transmission-Session-Id", sessionID)
	c.Status(http.StatusConflict)
}

// validateUser validates the Basic Auth credentials.
func (h *Handler) validateUser(c *gin.Context) bool {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return false
	}

	if !strings.HasPrefix(authHeader, "Basic ") {
		return false
	}

	encoded := strings.TrimPrefix(authHeader, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return false
	}

	username := parts[0]
	password := parts[1]

	return username == h.config.Username && password == h.config.Password
}

// handleTorrentGet handles the torrent-get RPC method.
func (h *Handler) handleTorrentGet() (*transmission.TorrentGetResponse, error) {
	transfers, err := h.putioClient.ListTransfers()
	if err != nil {
		return nil, err
	}

	var torrents []*transmission.Torrent
	for _, t := range transfers.Transfers {
		torrent := transmission.TorrentFromPutIOTransfer(&t, h.config.DownloadDirectory)
		torrents = append(torrents, torrent)
	}

	return &transmission.TorrentGetResponse{
		Torrents: torrents,
	}, nil
}

// handleTorrentAdd handles the torrent-add RPC method.
func (h *Handler) handleTorrentAdd(req *transmission.Request) error {
	var args transmission.TorrentAddArguments
	if err := bindArguments(req, &args); err != nil {
		return err
	}

	if args.Metainfo != "" {
		data, err := base64.StdEncoding.DecodeString(args.Metainfo)
		if err != nil {
			return err
		}

		return h.putioClient.UploadFile(data)
	}

	if args.Filename == "" {
		return nil
	}

	if err := h.putioClient.AddTransfer(args.Filename); err != nil {
		return err
	}

	name := "unknown"
	if strings.HasPrefix(args.Filename, "magnet:") {
		if parsed, err := url.Parse(args.Filename); err == nil {
			if dn := parsed.Query().Get("dn"); dn != "" {
				if decoded, err := url.QueryUnescape(dn); err == nil {
					name = decoded
				}
			}
		}
	}

	h.logger.Infof("[ffff: %s]: magnet link uploaded", name)
	return nil
}

// handleTorrentRemove handles the torrent-remove RPC method.
func (h *Handler) handleTorrentRemove(req *transmission.Request) error {
	var args transmission.TorrentRemoveArguments
	if err := bindArguments(req, &args); err != nil {
		return err
	}
	if len(args.IDs) == 0 {
		return nil
	}

	// Get all transfers to match by hash
	transfers, err := h.putioClient.ListTransfers()
	if err != nil {
		return err
	}

	// Build a set of hashes to remove
	hashSet := make(map[string]bool)
	for _, id := range args.IDs {
		hashSet[id] = true
	}

	// Find and remove matching transfers
	for _, t := range transfers.Transfers {
		if t.Hash == nil {
			continue
		}

		if hashSet[*t.Hash] {
			if err := h.putioClient.RemoveTransfer(t.ID); err != nil {
				h.logger.Errorf("Failed to remove transfer %d: %v", t.ID, err)
				continue
			}

			if t.UserfileExists && args.DeleteLocalData && t.FileID != nil {
				if err := h.putioClient.DeleteFile(*t.FileID); err != nil {
					h.logger.Errorf("Failed to delete file %d: %v", *t.FileID, err)
				}
			}
		}
	}

	return nil
}

func bindArguments[T any](req *transmission.Request, dest *T) error {
	if len(req.Arguments) == 0 {
		return nil
	}
	return json.Unmarshal(req.Arguments, dest)
}
