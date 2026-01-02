package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ochronus/goputioarr/internal/app"
	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/download"
	"github.com/ochronus/goputioarr/internal/http"
	"github.com/ochronus/goputioarr/internal/utils"
	"github.com/spf13/cobra"
)

const version = "0.5.38"

var configPath string

func main() {
	// Get default config path
	defaultConfigPath, err := config.DefaultConfigPath()
	if err != nil {
		defaultConfigPath = "./config.toml"
	}

	// Root command
	rootCmd := &cobra.Command{
		Use:   "goputioarr",
		Short: "put.io to sonarr/radarr/whisparr proxy",
		Long:  "Proxy that allows put.io to be used as a download client for sonarr/radarr/whisparr. The proxy uses the Transmission protocol.",
	}

	// Run command
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the proxy",
		RunE:  runProxy,
	}
	runCmd.Flags().StringVarP(&configPath, "config", "c", defaultConfigPath, "Path to config file")

	// Get-token command
	getTokenCmd := &cobra.Command{
		Use:   "get-token",
		Short: "Generate a put.io API token",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := utils.GetToken()
			return err
		},
	}

	// Generate-config command
	generateConfigCmd := &cobra.Command{
		Use:   "generate-config",
		Short: "Generate config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return utils.GenerateConfig(configPath)
		},
	}
	generateConfigCmd.Flags().StringVarP(&configPath, "config", "c", defaultConfigPath, "Path to config file")

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("goputioarr version %s\n", version)
		},
	}

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(getTokenCmd)
	rootCmd.AddCommand(generateConfigCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runProxy(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Build container with shared dependencies
	container, err := app.NewContainer(cfg)
	if err != nil {
		return fmt.Errorf("failed to build container: %w", err)
	}

	container.Logger.Infof("Starting goputioarr, version %s", version)

	// Start download manager
	downloadManager := download.NewManager(container)
	if err := downloadManager.StartWithContext(ctx); err != nil {
		return fmt.Errorf("failed to start download manager: %w", err)
	}
	defer downloadManager.Stop()

	// Start HTTP server
	server := http.NewServer(container)
	return server.StartWithContext(ctx)
}
