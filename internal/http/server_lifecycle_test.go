package http

import (
	"context"
	"testing"
	"time"

	"github.com/ochronus/goputioarr/internal/app"
	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/services/putio"
	"github.com/sirupsen/logrus"
)

func TestServerGracefulShutdownWithContext(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		BindAddress:       "127.0.0.1",
		Port:              0, // let the OS pick a free port
		Username:          "user",
		Password:          "pass",
		DownloadDirectory: "/tmp",
		Loglevel:          "error",
		Putio: config.PutioConfig{
			APIKey: "dummy",
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // silence output in tests

	container := &app.Container{
		Config:        cfg,
		Logger:        logger,
		PutioClient:   putio.NewClient(cfg.Putio.APIKey),
		ValidatePutio: false,
	}

	s := NewServer(container)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.StartWithContext(ctx)
	}()

	// Allow the server to start listening.
	time.Sleep(100 * time.Millisecond)

	// Trigger graceful shutdown.
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected graceful shutdown without error, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down after context cancellation")
	}
}
