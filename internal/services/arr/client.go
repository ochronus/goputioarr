package arr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const timeout = 30 * time.Second

// Client represents an Arr (Sonarr/Radarr/Whisparr) API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

var _ ClientAPI = (*Client)(nil)

// NewClient creates a new Arr client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// HistoryResponse represents the API response for history
type HistoryResponse struct {
	TotalRecords int             `json:"totalRecords"`
	Records      []HistoryRecord `json:"records"`
}

// HistoryRecord represents a single history record
type HistoryRecord struct {
	EventType string            `json:"eventType"`
	Data      map[string]string `json:"data"`
}

// doRequest executes an HTTP request with the API key header
func (c *Client) doRequest(method, url string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", c.apiKey)

	return c.httpClient.Do(req)
}

// CheckImported checks if a file has been imported by checking the history
func (c *Client) CheckImported(targetPath string) (bool, error) {
	inspected := 0
	page := 0

	for {
		url := fmt.Sprintf("%s/api/v3/history?includeSeries=false&includeEpisode=false&page=%d&pageSize=1000",
			c.baseURL, page)

		resp, err := c.doRequest("GET", url)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("url: %s, status: %s", url, resp.Status)
		}

		var historyResponse HistoryResponse
		if err := json.NewDecoder(resp.Body).Decode(&historyResponse); err != nil {
			return false, fmt.Errorf("url: %s, error decoding response: %w", url, err)
		}

		for _, record := range historyResponse.Records {
			if record.EventType == "downloadFolderImported" {
				droppedPath, ok := record.Data["droppedPath"]
				if ok && droppedPath == targetPath {
					return true, nil
				}
			}
			inspected++
		}

		if historyResponse.TotalRecords > inspected {
			page++
		} else {
			return false, nil
		}
	}
}

// CheckImportedMultiService checks if a file has been imported by any of the configured services
func CheckImportedMultiService(targetPath string, services []struct {
	Name   string
	URL    string
	APIKey string
}) (bool, string, error) {
	for _, svc := range services {
		client := NewClient(svc.URL, svc.APIKey)
		imported, err := client.CheckImported(targetPath)
		if err != nil {
			// Log the error but continue checking other services
			continue
		}
		if imported {
			return true, svc.Name, nil
		}
	}
	return false, "", nil
}
