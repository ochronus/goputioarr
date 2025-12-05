package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ochronus/goputioarr/internal/services/putio"
)

const configTemplate = `# Required. Username and password that sonarr/radarr use to connect to the proxy
username = "myusername"
password = "mypassword"

# Required. Directory where the proxy will download files to. This directory has to be readable by
# sonarr/radarr in order to import downloads
download_directory = "/path/to/downloads"

# Optional bind address, default "0.0.0.0"
bind_address = "0.0.0.0"

# Optional TCP port, default 9091
port = 9091

# Optional log level, default "info"
loglevel = "info"

# Optional UID, default 1000. Change the owner of the downloaded files to this UID. Requires root.
uid = 1000

# Optional polling interval in secs, default 10.
polling_interval = 10

# Optional skip directories when downloding, default ["sample", "extras"]
skip_directories = ["sample", "extras"]

# Optional number of orchestration workers, default 10. Unless there are many changes coming from
# put.io, you shouldn't have to touch this number. 10 is already overkill.
orchestration_workers = 10

# Optional number of download workers, default 4. This controls how many downloads we run in parallel.
download_workers = 4

[putio]
# Required. Putio API key. You can generate one using 'putioarr get-token'
api_key = "{{PUTIO_API_KEY}}"

# Both [sonarr] and [radarr] are optional, but you'll need at least one of them
[sonarr]
url = "http://mysonarrhost:8989/sonarr"
# Can be found in Settings -> General
api_key = "MYSONARRAPIKEY"

[radarr]
url = "http://myradarrhost:7878/radarr"
# Can be found in Settings -> General
api_key = "MYRADARRAPIKEY"

[whisparr]
url = "http://mywhisparrhost:6969/radarr"
# Can be found in Settings -> General
api_key = "MYWHISPARRAPIKEY"
`

// GetToken obtains a new Put.io API token through OOB authentication
func GetToken() (string, error) {
	fmt.Println()

	// Get OOB code
	oobCode, err := putio.GetOOB()
	if err != nil {
		return "", fmt.Errorf("failed to get OOB code: %w", err)
	}

	fmt.Printf("Go to https://put.io/link and enter the code: %s\n", oobCode)
	fmt.Println("Waiting for token...")

	// Poll for token every 3 seconds
	for {
		time.Sleep(3 * time.Second)

		token, err := putio.CheckOOB(oobCode)
		if err != nil {
			// Not linked yet, continue waiting
			continue
		}

		fmt.Printf("Put.io API token: %s\n", token)
		return token, nil
	}
}

// GenerateConfig generates a configuration file with the Put.io API token
func GenerateConfig(configPath string) error {
	fmt.Printf("Generating config %s\n", configPath)

	// Get Put.io token
	putioAPIKey, err := GetToken()
	if err != nil {
		return err
	}

	// Replace placeholder with actual API key
	config := strings.Replace(configTemplate, "{{PUTIO_API_KEY}}", putioAPIKey, 1)

	// Check if config file already exists and back it up
	if _, err := os.Stat(configPath); err == nil {
		backupPath := configPath + ".bak"
		fmt.Printf("Backing up config %s\n", configPath)
		if err := os.Rename(configPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup config: %w", err)
		}
	}

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	fmt.Printf("Writing %s\n", configPath)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
