package putio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/ochronus/goputioarr/internal/services/retry"
)

const (
	defaultBaseURL   = "https://api.put.io/v2"
	defaultUploadURL = "https://upload.put.io/v2"
	defaultTimeout   = 10 * time.Second

	maxRetries  = 3
	backoffBase = 200 * time.Millisecond
)

type HTTPError struct {
	URL        string
	StatusCode int
	Status     string
	RetryAfter string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("url: %s, status: %s", e.URL, e.Status)
}

// Client represents a Put.io API client.
type Client struct {
	apiToken   string
	baseURL    string
	uploadURL  string
	httpClient *http.Client
	sleeper    func(time.Duration)
}

var _ ClientAPI = (*Client)(nil)

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithBaseURLs overrides the API and upload base URLs (useful for tests).
func WithBaseURLs(apiBaseURL, uploadBaseURL string) ClientOption {
	return func(c *Client) {
		if apiBaseURL != "" {
			c.baseURL = apiBaseURL
		}
		if uploadBaseURL != "" {
			c.uploadURL = uploadBaseURL
		}
	}
}

// WithHTTPClient overrides the HTTP client (useful for tests).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// NewClient creates a new Put.io client.
func NewClient(apiToken string, opts ...ClientOption) *Client {
	c := &Client{
		apiToken:  apiToken,
		baseURL:   defaultBaseURL,
		uploadURL: defaultUploadURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		sleeper: time.Sleep,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// AccountInfo represents put.io account information.
type AccountInfo struct {
	Username      string `json:"username"`
	Mail          string `json:"mail"`
	AccountActive bool   `json:"account_active"`
}

// AccountInfoResponse represents the API response for account info.
type AccountInfoResponse struct {
	Info AccountInfo `json:"info"`
}

// Transfer represents a put.io transfer.
type Transfer struct {
	ID             uint64  `json:"id"`
	Hash           *string `json:"hash"`
	Name           *string `json:"name"`
	Size           *int64  `json:"size"`
	Downloaded     *int64  `json:"downloaded"`
	FinishedAt     *string `json:"finished_at"`
	EstimatedTime  *int64  `json:"estimated_time"`
	Status         string  `json:"status"`
	StartedAt      *string `json:"started_at"`
	ErrorMessage   *string `json:"error_message"`
	FileID         *int64  `json:"file_id"`
	UserfileExists bool    `json:"userfile_exists"`
}

// IsDownloadable returns true if the transfer has a file_id.
func (t *Transfer) IsDownloadable() bool {
	return t.FileID != nil
}

// ListTransferResponse represents the API response for list transfers.
type ListTransferResponse struct {
	Transfers []Transfer `json:"transfers"`
}

// GetTransferResponse represents the API response for get transfer.
type GetTransferResponse struct {
	Transfer Transfer `json:"transfer"`
}

// FileResponse represents a file from put.io.
type FileResponse struct {
	ContentType string `json:"content_type"`
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	FileType    string `json:"file_type"`
}

// ListFileResponse represents the API response for list files.
type ListFileResponse struct {
	Files  []FileResponse `json:"files"`
	Parent FileResponse   `json:"parent"`
}

// URLResponse represents the API response for getting a file URL.
type URLResponse struct {
	URL string `json:"url"`
}

type requestFactory func() (io.ReadCloser, string, error)

// doRequest executes an HTTP request with authorization and retries with backoff on 5xx/429.
func (c *Client) doRequest(method, url string, factory requestFactory) (*http.Response, error) {
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
		body, contentType, err := factory()
		if err != nil {
			return err
		}

		req, err := http.NewRequest(method, url, body)
		if err != nil {
			return err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

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

// GetAccountInfo retrieves account information.
func (c *Client) GetAccountInfo() (*AccountInfoResponse, error) {
	url := c.baseURL + "/account/info"
	resp, err := c.doRequest(http.MethodGet, url, func() (io.ReadCloser, string, error) {
		return nil, "", nil
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	var result AccountInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListTransfers returns the user's transfers.
func (c *Client) ListTransfers() (*ListTransferResponse, error) {
	url := c.baseURL + "/transfers/list"

	resp, err := c.doRequest(http.MethodGet, url, func() (io.ReadCloser, string, error) {
		return nil, "", nil
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	var result ListTransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetTransfer returns a specific transfer.
func (c *Client) GetTransfer(transferID uint64) (*GetTransferResponse, error) {
	url := fmt.Sprintf("%s/transfers/%d", c.baseURL, transferID)
	resp, err := c.doRequest(http.MethodGet, url, func() (io.ReadCloser, string, error) {
		return nil, "", nil
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	var result GetTransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// RemoveTransfer removes a transfer.
func (c *Client) RemoveTransfer(transferID uint64) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("transfer_ids", strconv.FormatUint(transferID, 10))
	writer.Close()
	url := c.baseURL + "/transfers/remove"

	resp, err := c.doRequest(http.MethodPost, url, func() (io.ReadCloser, string, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), writer.FormDataContentType(), nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	return nil
}

// DeleteFile deletes a file or directory.
func (c *Client) DeleteFile(fileID int64) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("file_ids", strconv.FormatInt(fileID, 10))
	writer.Close()
	url := c.baseURL + "/files/delete"

	resp, err := c.doRequest(http.MethodPost, url, func() (io.ReadCloser, string, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), writer.FormDataContentType(), nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	return nil
}

// AddTransfer adds a new transfer from a URL or magnet link.
func (c *Client) AddTransfer(url string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("url", url)
	writer.Close()
	requestURL := c.baseURL + "/transfers/add"

	resp, err := c.doRequest(http.MethodPost, requestURL, func() (io.ReadCloser, string, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), writer.FormDataContentType(), nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HTTPError{URL: requestURL, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	return nil
}

// UploadFile uploads a torrent file.
func (c *Client) UploadFile(data []byte) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}

	_ = writer.WriteField("filename", "upload.torrent")
	writer.Close()

	url := c.uploadURL + "/files/upload"

	resp, err := c.doRequest(http.MethodPost, url, func() (io.ReadCloser, string, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), writer.FormDataContentType(), nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	return nil
}

// ListFiles lists files in a directory.
func (c *Client) ListFiles(fileID int64) (*ListFileResponse, error) {
	url := fmt.Sprintf("%s/files/list?parent_id=%d", c.baseURL, fileID)
	resp, err := c.doRequest(http.MethodGet, url, func() (io.ReadCloser, string, error) {
		return nil, "", nil
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	var result ListFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetFileURL returns the download URL for a file.
func (c *Client) GetFileURL(fileID int64) (string, error) {
	url := fmt.Sprintf("%s/files/%d/url", c.baseURL, fileID)
	resp, err := c.doRequest(http.MethodGet, url, func() (io.ReadCloser, string, error) {
		return nil, "", nil
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	var result URLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.URL, nil
}

// GetOOB returns a new OOB (out-of-band) code for authentication.
func GetOOB() (string, error) {
	url := "https://api.put.io/v2/oauth2/oob/code?app_id=6487"
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	code, ok := result["code"]
	if !ok {
		return "", fmt.Errorf("OOB code not found in response")
	}

	return code, nil
}

// CheckOOB checks if the OOB code has been linked and returns the OAuth token.
func CheckOOB(oobCode string) (string, error) {
	url := fmt.Sprintf("https://api.put.io/v2/oauth2/oob/code/%s", oobCode)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &HTTPError{URL: url, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	token, ok := result["oauth_token"]
	if !ok {
		return "", fmt.Errorf("OAuth token not found in response")
	}

	return token, nil
}
