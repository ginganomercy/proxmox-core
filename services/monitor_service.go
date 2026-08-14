package services

import (
	"crypto/tls"
	"net/http"
	"time"
)

type MonitorResult struct {
	Status      string // UP or DOWN
	LatencyMs   int64
	ErrorReason string
}

func PingURL(url string) MonitorResult {
	// Create a custom HTTP client with a strict timeout to prevent goroutine leaks
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			// Ignore TLS certificate errors for monitoring purposes (optional, depending on environment)
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	start := time.Now()
	resp, err := client.Get(url)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return MonitorResult{
			Status:      "DOWN",
			LatencyMs:   latency,
			ErrorReason: err.Error(),
		}
	}
	defer resp.Body.Close()

	// Consider any 2xx or 3xx status code as UP. 4xx or 5xx is DOWN.
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return MonitorResult{
			Status:      "UP",
			LatencyMs:   latency,
			ErrorReason: "",
		}
	}

	return MonitorResult{
		Status:      "DOWN",
		LatencyMs:   latency,
		ErrorReason: "HTTP Status: " + resp.Status,
	}
}
