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

// Request performs an HTTP request to the Proxmox API
func (c *clientImpl) Request(method, endpoint string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("proxmox API error: %d - %s", res.StatusCode, string(resBody))
	}

	return resBody, nil
}

// Get performs a GET request
func (c *clientImpl) Get(endpoint string) ([]byte, error) {
	return c.Request(http.MethodGet, endpoint, nil)
}

// Post performs a POST request
func (c *clientImpl) Post(endpoint string, body interface{}) ([]byte, error) {
	return c.Request(http.MethodPost, endpoint, body)
}

// Delete performs a DELETE request
func (c *clientImpl) Delete(endpoint string) ([]byte, error) {
	return c.Request(http.MethodDelete, endpoint, nil)
}
