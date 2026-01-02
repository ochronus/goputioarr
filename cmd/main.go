package main

import (
	"fmt"
	"os"

	"github.com/ochronus/goputioarr/internal/config"
	"github.com/ochronus/goputioarr/internal/download"
	"github.com/ochronus/goputioarr/internal/http"
	"github.com/ochronus/goputioarr/internal/services/arr"
	"github.com/ochronus/goputioarr/internal/services/putio"
	"github.com/ochronus/goputioarr/internal/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const version = "0.5.37"

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
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set up logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Parse log level
	level, err := logrus.ParseLevel(cfg.Loglevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	logger.Infof("Starting goputioarr, version %s", version)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Build shared clients
	putioClient := putio.NewClient(cfg.Putio.APIKey)
	if _, err := putioClient.GetAccountInfo(); err != nil {
		return fmt.Errorf("failed to verify put.io API key: %w", err)
	}

	var arrClients []download.ArrServiceClient
	for _, svc := range cfg.GetArrConfigs() {
		arrClients = append(arrClients, download.ArrServiceClient{
			Name:   svc.Name,
			Client: arr.NewClient(svc.URL, svc.APIKey),
		})
	}

	// Start download manager
	downloadManager := download.NewManager(cfg, logger, putioClient, arrClients)
	if err := downloadManager.Start(); err != nil {
		return fmt.Errorf("failed to start download manager: %w", err)
	}

	// Start HTTP server
	server := http.NewServer(cfg, logger, putioClient)
	return server.Start()
}
