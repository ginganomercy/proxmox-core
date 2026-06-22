package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all the environment variables
type Config struct {
	Port               string
	NodeEnv            string
	AdminUsername      string
	AdminPassword      string
	JWTSecret          string
	JWTExpiresIn       string
	ProxmoxURL         string
	ProxmoxTokenID     string
	ProxmoxTokenSecret string
	ProxmoxNode        string
	DatabaseURL        string
	SMTPHost           string
	SMTPPort           string
	SMTPUser           string
	SMTPPass           string
}

// Env global variable holding config
var Env *Config

// LoadConfig loads environment variables from .env file
func LoadConfig() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	Env = &Config{
		Port:               getEnv("PORT", "3001"),
		NodeEnv:            getEnv("NODE_ENV", "development"),
		AdminUsername:      getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:      getEnv("ADMIN_PASSWORD", "securepassword123"),
		JWTSecret:          getEnv("JWT_SECRET", "supersecretjwtkeythatshouldbechanged"),
		JWTExpiresIn:       getEnv("JWT_EXPIRES_IN", "24h"),
		ProxmoxURL:         getEnv("PROXMOX_URL", ""),
		ProxmoxTokenID:     getEnv("PROXMOX_TOKEN_ID", ""),
		ProxmoxTokenSecret: getEnv("PROXMOX_TOKEN_SECRET", ""),
		ProxmoxNode:        getEnv("PROXMOX_NODE", "pve"),
		DatabaseURL:        getEnv("DATABASE_URL", "file:./dev.db"),
		SMTPHost:           getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:           getEnv("SMTP_PORT", "587"),
		SMTPUser:           getEnv("SMTP_USER", ""),
		SMTPPass:           getEnv("SMTP_PASS", ""),
	}
}

// getEnv is a helper to read an environment variable or return a fallback value
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
