package database

import (
	"log"
	"os"

	"cbt-core-api/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	var err error

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "proxmox.db"
	}

	// Open SQLite database file
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Enterprise Hardening: Enable WAL Mode to prevent SQLite "Database is Locked" errors
	// This allows concurrent reads and writes which is critical for the audit logs worker
	if err := DB.Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
		log.Printf("Failed to set WAL mode: %v", err)
	}
	if err := DB.Exec("PRAGMA synchronous=NORMAL;").Error; err != nil {
		log.Printf("Failed to set synchronous mode: %v", err)
	}

	log.Println("Database connection successfully opened (WAL mode enabled).")

	// Run AutoMigrate to build tables based on models
	err = DB.AutoMigrate(&models.User{}, &models.Server{}, &models.Order{}, &models.MonitorTarget{}, &models.MonitorLog{})
	if err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Database migration completed.")
}
