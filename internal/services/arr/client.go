package arr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ochronus/goputioarr/internal/services/retry"
)

const (
	timeout     = 30 * time.Second
	maxRetries  = 3
	backoffBase = 200 * time.Millisecond
)

// Client represents an Arr (Sonarr/Radarr/Whisparr) API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	sleeper    func(time.Duration)
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
		sleeper: time.Sleep,
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

type HTTPError struct {
	URL        string
	StatusCode int
	Status     string
	RetryAfter string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("url: %s, status: %s", e.URL, e.Status)
}

// doRequest executes an HTTP request with the API key header and retries with backoff on 5xx/429
func (c *Client) doRequest(method, url string) (*http.Response, error) {
	var respOut *http.Response

	err := retry.Do(nil, retry.Config{
		MaxRetries: maxRetries,
		BaseDelay:  backoffBase,
		ShouldRetry: func(err error) bool {
			if err == nil {
				return false
			}
			var httpErr *HTTPError
			if errors.As(err, &httpErr) {
				return httpErr.StatusCode == http.StatusTooManyRequests || httpErr.StatusCode >= 500
			}
			return true
		},
		DelayFunc: func(attempt int, err error) time.Duration {
			fallback := backoffBase * time.Duration(1<<attempt)
			var httpErr *HTTPError
			if errors.As(err, &httpErr) && httpErr.RetryAfter != "" {
				return retry.RetryAfterDelay(httpErr.RetryAfter, fallback)
			}
			return fallback
		},
		Sleeper: c.sleeper,
	}, func(attempt int) error {
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			return err
		}

		req.Header.Set("X-Api-Key", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			retryAfter := resp.Header.Get("Retry-After")
			resp.Body.Close()
			return &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status, RetryAfter: retryAfter}
		}

		respOut = resp
		return nil
	})

	if err != nil {
		return nil, err
	}

	return respOut, nil
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

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return false, &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
		}

		var historyResponse HistoryResponse
		if err := json.NewDecoder(resp.Body).Decode(&historyResponse); err != nil {
			resp.Body.Close()
			return false, fmt.Errorf("url: %s, error decoding response: %w", url, err)
		}
		resp.Body.Close()

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
