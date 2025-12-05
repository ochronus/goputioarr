package http

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/ochronus/goputioarr/internal/config"
	"github.com/sirupsen/logrus"
)

// Server represents the HTTP server
type Server struct {
	config  *config.Config
	handler *Handler
	logger  *logrus.Logger
	router  *gin.Engine
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, logger *logrus.Logger) *Server {
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

	handler := NewHandler(cfg, logger)

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

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.Port)
	s.logger.Infof("Starting web server at http://%s", addr)
	return s.router.Run(addr)
}

// GetRouter returns the underlying gin router (useful for testing)
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}
