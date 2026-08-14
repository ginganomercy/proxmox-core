package workers

import (
	"context"
	"log"
	"sync"
	"time"

	"cbt-core-api/database"
	"cbt-core-api/models"
	"cbt-core-api/services"
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

	// Strategy: Only log if status changes, OR if it's the very first check (PENDING)
	statusChanged := target.Status != result.Status

	if statusChanged {
		log.Printf("[MonitorWorker] Target %s status changed from %s to %s\n", target.Domain, target.Status, result.Status)
		
		// 1. Insert into MonitorLog
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

// InjectInitialTargets is a helper to seed the database with required domains
func InjectInitialTargets() {
	domains := []string{
		"https://finger.pbjt.web.id",
		"https://finger-be.pbjt.web.id",
	}

	for _, domain := range domains {
		var existing models.MonitorTarget
		if err := database.DB.Where("domain = ?", domain).First(&existing).Error; err != nil {
			// Not found, create it
			newTarget := models.MonitorTarget{
				Domain: domain,
				Status: "PENDING",
			}
			database.DB.Create(&newTarget)
			log.Printf("Injected initial monitoring target: %s\n", domain)
		}
	}
}
