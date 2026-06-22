package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cbt-core-api/config"
	"cbt-core-api/controllers"
	"cbt-core-api/database"
	"cbt-core-api/proxmox"
	"cbt-core-api/repositories"
	"cbt-core-api/routes"
	"cbt-core-api/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Load environment variables
	config.LoadConfig()

	// Initialize Database and Cache
	database.ConnectDB()
	proxmox.InitCache()

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName: "Cloud Baja Tegal - Core API",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path}\n",
	}))
	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "https://cloud-dashboard.pbjt.web.id, http://localhost:5173"
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// Initialize Dependencies (Clean Architecture)
	userRepo := repositories.NewUserRepository(database.DB)
	emailService := services.NewEmailService()
	authService := services.NewAuthService(userRepo, emailService)
	authCtrl := controllers.NewAuthController(authService)
	ssoCtrl := controllers.NewSSOController(authService)

	proxmoxClient, err := proxmox.NewClient()
	if err != nil {
		log.Fatalf("Failed to initialize Proxmox client: %v", err)
	}
	proxmoxService := services.NewProxmoxService(proxmoxClient)
	proxmoxCtrl := controllers.NewProxmoxController(proxmoxService)

	orderRepo := repositories.NewOrderRepository(database.DB)
	orderCtrl := controllers.NewOrderController(orderRepo, userRepo, emailService, proxmoxService)
	adminCtrl := controllers.NewAdminController()

	// Register Routes
	routes.RegisterRoutes(app, authCtrl, ssoCtrl, proxmoxCtrl, orderCtrl, adminCtrl)

	// Start server in a separate goroutine
	port := config.Env.Port
	go func() {
		log.Printf("Starting CBT Core API (Go) on port %s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	// Graceful Shutdown Setup
	quit := make(chan os.Signal, 1)
	// Catch SIGINT (Ctrl+C) and SIGTERM (Docker stop)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	<-quit
	log.Println("Gracefully shutting down server...")

	// Create a deadline to wait for currently serving requests to finish
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Close database connection
	sqlDB, err := database.DB.DB()
	if err == nil {
		sqlDB.Close()
		log.Println("Database connection closed.")
	}

	log.Println("Server exiting")
}
