package main

import (
	"log"
	"os"

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
	voucherCtrl := controllers.NewVoucherController(userRepo)

	proxmoxClient, err := proxmox.NewClient()
	if err != nil {
		log.Fatalf("Failed to initialize Proxmox client: %v", err)
	}
	proxmoxService := services.NewProxmoxService(proxmoxClient)
	proxmoxCtrl := controllers.NewProxmoxController(proxmoxService)

	orderRepo := repositories.NewOrderRepository(database.DB)
	orderCtrl := controllers.NewOrderController(orderRepo, userRepo, emailService, proxmoxService)

	// Register Routes
	routes.RegisterRoutes(app, authCtrl, ssoCtrl, proxmoxCtrl, voucherCtrl, orderCtrl)

	// Start server
	port := config.Env.Port
	log.Printf("Starting CBT Core API (Go) on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
