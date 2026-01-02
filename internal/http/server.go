package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/services/putio"
	"github.com/sirupsen/logrus"
)

// Server represents the HTTP server
type Server struct {
	config  *config.Config
	handler *Handler
	logger  *logrus.Logger
	router  *gin.Engine
	srv     *http.Server
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, logger *logrus.Logger, putioClient *putio.Client) *Server {
	// Set gin mode based on log level
	if cfg.Loglevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Add logging middleware
	router.Use(func(c *gin.Context) {
		c.Next()
	})

	handler := NewHandler(cfg, logger, putioClient)

	// Register routes
	router.POST("/transmission/rpc", handler.RPCPost)
	router.GET("/transmission/rpc", handler.RPCGet)

	return &Server{
		config:  cfg,
		handler: handler,
		logger:  logger,
		router:  router,
	}
}

// Start starts the HTTP server with a background context.
func (s *Server) Start() error {
	return s.StartWithContext(context.Background())
}

// StartWithContext starts the HTTP server and shuts down gracefully when the context is canceled.
func (s *Server) StartWithContext(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.Port)
	s.logger.Infof("Starting web server at http://%s", addr)

	s.srv = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		<-errCh
		return nil
	case err := <-errCh:
		return err
	}
}

// GetRouter returns the underlying gin router (useful for testing)
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}
