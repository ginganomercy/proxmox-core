package proxmox

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"cbt-core-api/config"
)

// ProxmoxClient defines the interface for Proxmox API calls
type ProxmoxClient interface {
	Get(endpoint string) ([]byte, error)
	Post(endpoint string, body interface{}) ([]byte, error)
	Put(endpoint string, body interface{}) ([]byte, error)
	Delete(endpoint string) ([]byte, error)
}

// clientImpl is the actual HTTP implementation
type clientImpl struct {
	httpClient *http.Client
	baseURL    string
	authHeader string
}

// NewClient initializes a new Proxmox API client
func NewClient() (ProxmoxClient, error) {
	if config.Env.ProxmoxURL == "" || config.Env.ProxmoxTokenID == "" || config.Env.ProxmoxTokenSecret == "" {
		return nil, errors.New("proxmox credentials not fully configured in .env")
	}

	// Ignore self-signed certificates
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", config.Env.ProxmoxTokenID, config.Env.ProxmoxTokenSecret)

	return &clientImpl{
		httpClient: httpClient,
		baseURL:    config.Env.ProxmoxURL,
		authHeader: authHeader,
	}, nil
}

// Request performs an HTTP request to the Proxmox API with Retry & Exponential Backoff
func (c *clientImpl) Request(method, endpoint string, body interface{}) ([]byte, error) {
	var jsonBody []byte
	var err error

	if body != nil {
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	maxRetries := 3
	baseDelay := 500 * time.Millisecond
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewBuffer(jsonBody)
		}

		req, err := http.NewRequest(method, c.baseURL+endpoint, bodyReader)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", c.authHeader)
		req.Header.Set("Content-Type", "application/json")

		res, err := c.httpClient.Do(req)
		if err == nil {
			defer res.Body.Close()
			resBody, err := io.ReadAll(res.Body)
			if err != nil {
				return nil, err
			}

			// Don't retry on 4xx client errors (e.g. Unauthorized, Not Found)
			if res.StatusCode < 500 {
				if res.StatusCode >= 400 {
					return nil, fmt.Errorf("proxmox API error: %d - %s", res.StatusCode, string(resBody))
				}
				return resBody, nil
			}
			lastErr = fmt.Errorf("proxmox server error: %d - %s", res.StatusCode, string(resBody))
		} else {
			lastErr = err
		}

		// Retry delay (Exponential backoff)
		time.Sleep(baseDelay * time.Duration(1<<attempt))
	}

	return nil, fmt.Errorf("failed after %d attempts, last error: %v", maxRetries, lastErr)
}

// Get performs a GET request
func (c *clientImpl) Get(endpoint string) ([]byte, error) {
	return c.Request(http.MethodGet, endpoint, nil)
}

// Post performs a POST request
func (c *clientImpl) Post(endpoint string, body interface{}) ([]byte, error) {
	return c.Request(http.MethodPost, endpoint, body)
}

// Put performs a PUT request
func (c *clientImpl) Put(endpoint string, body interface{}) ([]byte, error) {
	return c.Request(http.MethodPut, endpoint, body)
}

// Delete performs a DELETE request
func (c *clientImpl) Delete(endpoint string) ([]byte, error) {
	return c.Request(http.MethodDelete, endpoint, nil)
}
