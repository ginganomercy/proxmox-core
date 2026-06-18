package database

import (
	"log"

	"cbt-core-api/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	var err error

	// Open SQLite database file (it will create proxmox.db if it doesn't exist)
	DB, err = gorm.Open(sqlite.Open("proxmox.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connection successfully opened.")

	// Run AutoMigrate to build tables based on models
	err = DB.AutoMigrate(&models.User{}, &models.Server{}, &models.Voucher{})
	if err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Database migration completed.")
}
