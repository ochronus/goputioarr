package putio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

const (
	baseURL   = "https://api.put.io/v2"
	uploadURL = "https://upload.put.io/v2"
	timeout   = 10 * time.Second
)

// Client represents a Put.io API client
type Client struct {
	apiToken   string
	httpClient *http.Client
}

var _ ClientAPI = (*Client)(nil)

// NewClient creates a new Put.io client
func NewClient(apiToken string) *Client {
	return &Client{
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// AccountInfo represents put.io account information
type AccountInfo struct {
	Username      string `json:"username"`
	Mail          string `json:"mail"`
	AccountActive bool   `json:"account_active"`
}

// AccountInfoResponse represents the API response for account info
type AccountInfoResponse struct {
	Info AccountInfo `json:"info"`
}

// Transfer represents a put.io transfer
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

// IsDownloadable returns true if the transfer has a file_id
func (t *Transfer) IsDownloadable() bool {
	return t.FileID != nil
}

// ListTransferResponse represents the API response for list transfers
type ListTransferResponse struct {
	Transfers []Transfer `json:"transfers"`
}

// GetTransferResponse represents the API response for get transfer
type GetTransferResponse struct {
	Transfer Transfer `json:"transfer"`
}

// FileResponse represents a file from put.io
type FileResponse struct {
	ContentType string `json:"content_type"`
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	FileType    string `json:"file_type"`
}

// ListFileResponse represents the API response for list files
type ListFileResponse struct {
	Files  []FileResponse `json:"files"`
	Parent FileResponse   `json:"parent"`
}

// URLResponse represents the API response for getting a file URL
type URLResponse struct {
	URL string `json:"url"`
}

// doRequest executes an HTTP request with authorization
func (c *Client) doRequest(method, url string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return c.httpClient.Do(req)
}

// GetAccountInfo retrieves account information
func (c *Client) GetAccountInfo() (*AccountInfoResponse, error) {
	resp, err := c.doRequest("GET", baseURL+"/account/info", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting put.io account info: %s", resp.Status)
	}

	var result AccountInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListTransfers returns the user's transfers
func (c *Client) ListTransfers() (*ListTransferResponse, error) {
	resp, err := c.doRequest("GET", baseURL+"/transfers/list", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting put.io transfers: %s", resp.Status)
	}

	var result ListTransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetTransfer returns a specific transfer
func (c *Client) GetTransfer(transferID uint64) (*GetTransferResponse, error) {
	url := fmt.Sprintf("%s/transfers/%d", baseURL, transferID)
	resp, err := c.doRequest("GET", url, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting put.io transfer id:%d: %s", transferID, resp.Status)
	}

	var result GetTransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// RemoveTransfer removes a transfer
func (c *Client) RemoveTransfer(transferID uint64) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("transfer_ids", strconv.FormatUint(transferID, 10))
	writer.Close()

	resp, err := c.doRequest("POST", baseURL+"/transfers/remove", &buf, writer.FormDataContentType())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error removing put.io transfer id:%d: %s", transferID, resp.Status)
	}

	return nil
}

// DeleteFile deletes a file or directory
func (c *Client) DeleteFile(fileID int64) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("file_ids", strconv.FormatInt(fileID, 10))
	writer.Close()

	resp, err := c.doRequest("POST", baseURL+"/files/delete", &buf, writer.FormDataContentType())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error removing put.io file/directory id:%d: %s", fileID, resp.Status)
	}

	return nil
}

// AddTransfer adds a new transfer from a URL or magnet link
func (c *Client) AddTransfer(url string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("url", url)
	writer.Close()

	resp, err := c.doRequest("POST", baseURL+"/transfers/add", &buf, writer.FormDataContentType())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error adding url: %s to put.io: %s", url, resp.Status)
	}

	return nil
}

// UploadFile uploads a torrent file
func (c *Client) UploadFile(data []byte) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "foo.torrent")
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}

	_ = writer.WriteField("filename", "foo.torrent")
	writer.Close()

	resp, err := c.doRequest("POST", uploadURL+"/files/upload", &buf, writer.FormDataContentType())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error uploading file to put.io: %s", resp.Status)
	}

	return nil
}

// ListFiles lists files in a directory
func (c *Client) ListFiles(fileID int64) (*ListFileResponse, error) {
	url := fmt.Sprintf("%s/files/list?parent_id=%d", baseURL, fileID)
	resp, err := c.doRequest("GET", url, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error listing put.io file/directory id:%d: %s", fileID, resp.Status)
	}

	var result ListFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetFileURL returns the download URL for a file
func (c *Client) GetFileURL(fileID int64) (string, error) {
	url := fmt.Sprintf("%s/files/%d/url", baseURL, fileID)
	resp, err := c.doRequest("GET", url, nil, "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error getting url for put.io file id:%d: %s", fileID, resp.Status)
	}

	var result URLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.URL, nil
}

// GetOOB returns a new OOB (out-of-band) code for authentication
func GetOOB() (string, error) {
	resp, err := http.Get("https://api.put.io/v2/oauth2/oob/code?app_id=6487")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error getting put.io OOB: %s", resp.Status)
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

// CheckOOB checks if the OOB code has been linked and returns the OAuth token
func CheckOOB(oobCode string) (string, error) {
	url := fmt.Sprintf("https://api.put.io/v2/oauth2/oob/code/%s", oobCode)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error checking put.io OOB %s: %s", oobCode, resp.Status)
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
