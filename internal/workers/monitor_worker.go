package workers

import (
	"context"
	"log"
	"sync"
	"time"

	"cbt-core-api/database"
	"cbt-core-api/internal/models"
	"cbt-core-api/internal/services"
)

type MonitorWorker struct {
	ticker *time.Ticker
	quit   chan struct{}
}

func NewMonitorWorker() *MonitorWorker {
	return &MonitorWorker{
		// Set interval to 2 minutes as requested
		ticker: time.NewTicker(2 * time.Minute),
		quit:   make(chan struct{}),
	}
}

func (w *MonitorWorker) Start(ctx context.Context) {
	log.Println("Uptime Monitor Worker started. Interval: 2 minutes.")
	// Run immediately on startup
	w.runChecks()

	for {
		select {
		case <-w.ticker.C:
			w.runChecks()
		case <-w.quit:
			w.ticker.Stop()
			log.Println("Uptime Monitor Worker stopped.")
			return
		case <-ctx.Done():
			w.ticker.Stop()
			log.Println("Uptime Monitor Worker stopped by context.")
			return
		}
	}
}

func (w *MonitorWorker) Stop() {
	close(w.quit)
}

func (w *MonitorWorker) runChecks() {
	var targets []models.MonitorTarget
	if err := database.DB.Find(&targets).Error; err != nil {
		log.Printf("[MonitorWorker] Error fetching targets: %v\n", err)
		return
	}

	if len(targets) == 0 {
		return
	}

	// Garbage Collection: Delete logs older than 7 days
	threshold := time.Now().Add(-7 * 24 * time.Hour)
	if err := database.DB.Where("created_at < ?", threshold).Delete(&models.MonitorLog{}).Error; err != nil {
		log.Printf("[MonitorWorker] Error cleaning up old logs: %v\n", err)
	}

	log.Printf("[MonitorWorker] Running checks for %d targets...\n", len(targets))

	var wg sync.WaitGroup
	for _, target := range targets {
		wg.Add(1)
		go func(t models.MonitorTarget) {
			defer wg.Done()
			w.checkTarget(t)
		}(target)
	}
	wg.Wait()
}

func (w *MonitorWorker) checkTarget(target models.MonitorTarget) {
	result := services.PingURL(target.Domain)
	
	now := time.Now()

	// 1. ALWAYS Insert into MonitorLog to track latency over time for the APM chart
	logEntry := models.MonitorLog{
		MonitorTargetID: target.ID,
		Status:          result.Status,
		LatencyMs:       result.LatencyMs,
		ErrorReason:     result.ErrorReason,
		CreatedAt:       now,
	}
	if err := database.DB.Create(&logEntry).Error; err != nil {
		log.Printf("[MonitorWorker] Error saving log for %s: %v\n", target.Domain, err)
	}

	if target.Status != result.Status {
		log.Printf("[MonitorWorker] Target %s status changed from %s to %s\n", target.Domain, target.Status, result.Status)
	}

	// 2. Update MonitorTarget (we always update LastPing and Status)
	target.Status = result.Status
	target.LastStatusCode = result.StatusCode
	target.SslValid = result.SslValid
	target.SslExpiryDays = result.SslExpiryDays
	target.LastPing = now
	target.UpdatedAt = now

	if err := database.DB.Save(&target).Error; err != nil {
		log.Printf("[MonitorWorker] Error updating target %s: %v\n", target.Domain, err)
	}
}
