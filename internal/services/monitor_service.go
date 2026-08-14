package services

import (
	"crypto/tls"
	"net/http"
	"time"
)

type MonitorResult struct {
	Status        string // UP or DOWN
	StatusCode    int
	SslValid      bool
	SslExpiryDays int
	LatencyMs     int64
	ErrorReason   string
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
			StatusCode:  0,
			LatencyMs:   latency,
			ErrorReason: err.Error(),
		}
	}
	defer resp.Body.Close()

	// Extract SSL Information
	sslValid := false
	sslExpiryDays := 0
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		now := time.Now()
		if now.Before(cert.NotAfter) && now.After(cert.NotBefore) {
			sslValid = true
			sslExpiryDays = int(cert.NotAfter.Sub(now).Hours() / 24)
		}
	}

	// Consider any 2xx or 3xx status code as UP. 4xx or 5xx is DOWN.
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return MonitorResult{
			Status:        "UP",
			StatusCode:    resp.StatusCode,
			SslValid:      sslValid,
			SslExpiryDays: sslExpiryDays,
			LatencyMs:     latency,
			ErrorReason:   "",
		}
	}

	return MonitorResult{
		Status:        "DOWN",
		StatusCode:    resp.StatusCode,
		SslValid:      sslValid,
		SslExpiryDays: sslExpiryDays,
		LatencyMs:     latency,
		ErrorReason:   "HTTP Status: " + resp.Status,
	}
}
