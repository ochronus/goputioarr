# goputioarr

Proxy that allows put.io to be used as a download client for sonarr/radarr/whisparr. The proxy uses the Transmission protocol.

This is a Go port of the original Rust implementation.

## Installation

### From Source

Make sure you have Go 1.21 or later installed.

```bash
# Clone the repository
git clone https://github.com/ochronus/goputioarr.git
cd goputioarr

# Build
go build -o goputioarr ./cmd

# Or use Makefile
make build

# Install (optional)
go install ./cmd
```

### Usage

First, generate a config using `goputioarr generate-config`. This will generate a config file in `~/.config/putioarr/config.toml`. Use `-c` to override the configuration file location.

Edit the configuration file and make sure you configure the username and password, as well as the sonarr/radarr/whisparr details.

- Run the proxy: `goputioarr run`
- Configure the Transmission download client in sonarr/radarr/whisparr:
    - Url Base: /transmission
    - Username: <configured username>
    - Password: <configured password>

## Commands

```bash
# Run the proxy
goputioarr run

# Run with custom config path
goputioarr run -c /path/to/config.toml

# Generate a put.io API token
goputioarr get-token

# Generate a config file (will prompt for put.io authentication)
goputioarr generate-config

# Generate config at a specific path
goputioarr generate-config -c /path/to/config.toml

# Show version
goputioarr version
```

## Configuration

A configuration file can be specified using `-c`, but the default configuration file location is:
- Linux: ~/.config/putioarr/config.toml
- macOS: ~/.config/putioarr/config.toml

TOML is used as the configuration format:

```toml
# Required. Username and password that sonarr/radarr/whisparr use to connect to the proxy
username = "myusername"
password = "mypassword"

# Required. Directory where the proxy will download files to. This directory has to be readable by
# sonarr/radarr/whisparr in order to import downloads
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
# Required. Putio API key. You can generate one using `goputioarr get-token`
api_key = "MYPUTIOKEY"

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
```

## Behavior

The proxy will upload torrents or magnet links to put.io. It will then continue to monitor transfers. When a transfer is completed, all files belonging to the transfer will be downloaded to the specified download directory. The proxy will remove the files after sonarr/radarr/whisparr has imported them and put.io is done seeding. The proxy will skip directories named "Sample".

## Project Structure

```
.
├── cmd/
│   └── main.go              # CLI entry point
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration types and loading
│   ├── download/
│   │   ├── manager.go       # Download orchestration
│   │   └── types.go         # Transfer and target types
│   ├── http/
│   │   ├── handlers.go      # Transmission RPC handlers
│   │   └── server.go        # HTTP server setup
│   ├── services/
│   │   ├── arr/
│   │   │   └── client.go    # Sonarr/Radarr/Whisparr API client
│   │   ├── putio/
│   │   │   └── client.go    # Put.io API client
│   │   └── transmission/
│   │       └── types.go     # Transmission protocol types
│   └── utils/
│       └── utils.go         # Utility functions
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Dependencies

- [gin-gonic/gin](https://github.com/gin-gonic/gin) - HTTP web framework
- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework
- [sirupsen/logrus](https://github.com/sirupsen/logrus) - Structured logging
- [BurntSushi/toml](https://github.com/BurntSushi/toml) - TOML parser

## License

MIT