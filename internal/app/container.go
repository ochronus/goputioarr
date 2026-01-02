package app

import (
	"fmt"

	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/services/arr"
	"github.com/ochronus/goputioarr/internal/services/putio"
	"github.com/sirupsen/logrus"
)

// Container centralizes the core dependencies used across the application.
// It is intentionally small and uses interfaces so callers (and tests) can
// substitute implementations easily.
type Container struct {
	Config        *config.Config
	Logger        *logrus.Logger
	PutioClient   putio.ClientAPI
	ArrClients    []ArrServiceClient
	ValidatePutio bool
}

// ArrServiceClient couples a service name with its Arr client interface.
type ArrServiceClient struct {
	Name   string
	Client arr.ClientAPI
}

// Option allows customizing the container during construction.
type Option func(*Container) error

// WithLogger overrides the default logger.
func WithLogger(logger *logrus.Logger) Option {
	return func(c *Container) error {
		if logger == nil {
			return fmt.Errorf("logger cannot be nil")
		}
		c.Logger = logger
		return nil
	}
}

// WithPutioClient overrides the default put.io client.
func WithPutioClient(client putio.ClientAPI) Option {
	return func(c *Container) error {
		if client == nil {
			return fmt.Errorf("put.io client cannot be nil")
		}
		c.PutioClient = client
		return nil
	}
}

// WithPutioValidation enables or disables put.io API key validation (default: enabled).
func WithPutioValidation(validate bool) Option {
	return func(c *Container) error {
		c.ValidatePutio = validate
		return nil
	}
}

// WithArrClients overrides the default Arr clients.
func WithArrClients(clients []ArrServiceClient) Option {
	return func(c *Container) error {
		c.ArrClients = clients
		return nil
	}
}

// NewContainer builds a Container with sensible defaults derived from cfg.
// Options can be supplied to override specific dependencies (useful in tests).
func NewContainer(cfg *config.Config, opts ...Option) (*Container, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	container := &Container{
		Config:        cfg,
		Logger:        buildDefaultLogger(cfg.Loglevel),
		ValidatePutio: true,
	}

	// Apply options early so tests can inject mocks before defaults are created.
	for _, opt := range opts {
		if err := opt(container); err != nil {
			return nil, err
		}
	}

	if container.PutioClient == nil {
		container.PutioClient = putio.NewClient(cfg.Putio.APIKey)
	}

	if container.ArrClients == nil {
		container.ArrClients = buildArrClients(cfg)
	}

	if container.ValidatePutio {
		if _, err := container.PutioClient.GetAccountInfo(); err != nil {
			return nil, fmt.Errorf("failed to verify put.io API key: %w", err)
		}
	}

	return container, nil
}

func buildDefaultLogger(levelStr string) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	level, err := logrus.ParseLevel(levelStr)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	return logger
}

func buildArrClients(cfg *config.Config) []ArrServiceClient {
	arrConfigs := cfg.GetArrConfigs()
	arrClients := make([]ArrServiceClient, 0, len(arrConfigs))
	for _, svc := range arrConfigs {
		arrClients = append(arrClients, ArrServiceClient{
			Name:   svc.Name,
			Client: arr.NewClient(svc.URL, svc.APIKey),
		})
	}
	return arrClients
}
