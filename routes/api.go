package routes

import (
	"cbt-core-api/controllers"
	"cbt-core-api/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(
	app *fiber.App,
	authCtrl *controllers.AuthController,
	ssoCtrl *controllers.SSOController,
	proxmoxCtrl *controllers.ProxmoxController,
	orderCtrl *controllers.OrderController,
	adminCtrl *controllers.AdminController,
) {
	api := app.Group("/api")

	// Public Routes
	auth := api.Group("/auth")
	auth.Post("/register", authCtrl.Register)
	auth.Post("/login", authCtrl.Login)
	auth.Post("/forgot-password", authCtrl.ForgotPassword)
	auth.Post("/reset-password", authCtrl.ResetPassword)
	auth.Get("/google", ssoCtrl.GoogleLogin)
	auth.Get("/google/callback", ssoCtrl.GoogleCallback)

	// Protected Routes
	protected := api.Group("/", middleware.Protected())

	// Auth verification
	protected.Get("/auth/me", authCtrl.Me)

	// Order Routes
	orders := protected.Group("/orders")
	orders.Post("/", orderCtrl.CreateOrder)
	orders.Get("/me", orderCtrl.GetMyOrders)
	orders.Post("/:id/activate", orderCtrl.ActivateOrder)
	orders.Delete("/:id", orderCtrl.DeleteOrder)
	
	// Admin Order Routes
	adminOrders := protected.Group("/admin/orders", middleware.AdminOnly())
	adminOrders.Get("/", orderCtrl.GetAllOrders)
	adminOrders.Post("/:id/generate", orderCtrl.GenerateCode)

	// Admin Summary Route
	protected.Get("/admin/summary", middleware.AdminOnly(), adminCtrl.GetDashboardSummary)

	// Proxmox Nodes & Instances
	proxmox := protected.Group("/proxmox")
	proxmox.Get("/nodes", proxmoxCtrl.GetNodes)
	proxmox.Get("/cluster/logs", middleware.AdminOnly(), proxmoxCtrl.GetClusterLogs)
	proxmox.Get("/nodes/:node/status", proxmoxCtrl.GetNodeStatus)
	proxmox.Get("/nodes/:node/instances", proxmoxCtrl.GetInstances)
	proxmox.Get("/nodes/:node/:type/:vmid/ip", proxmoxCtrl.GetInstanceIP)

	// Proxmox VM Actions
	proxmox.Post("/vms", proxmoxCtrl.CreateVM)
	proxmox.Post("/nodes/:node/qemu/:vmid/power", proxmoxCtrl.VMPowerAction)
	proxmox.Post("/nodes/:node/qemu/:vmid/config", proxmoxCtrl.UpdateVMConfig)
	proxmox.Post("/nodes/:node/:type/:vmid/vncproxy", proxmoxCtrl.GetVncProxy)
	proxmox.Delete("/nodes/:node/:type/:vmid", proxmoxCtrl.DeleteInstance)

	// Advanced Operations (Sprint 3)
	proxmox.Get("/nodes/:node/:type/:vmid/snapshots", proxmoxCtrl.GetSnapshots)
	proxmox.Post("/nodes/:node/:type/:vmid/snapshots", proxmoxCtrl.CreateSnapshot)
	proxmox.Post("/nodes/:node/:type/:vmid/snapshots/:snapname/rollback", proxmoxCtrl.RollbackSnapshot)
	proxmox.Delete("/nodes/:node/:type/:vmid/snapshots/:snapname", proxmoxCtrl.DeleteSnapshot)
	proxmox.Post("/nodes/:node/:type/:vmid/rebuild", proxmoxCtrl.RebuildInstance)

	// Metrics & Telemetry (Sprint 4)
	proxmox.Get("/nodes/:node/:type/:vmid/rrddata", proxmoxCtrl.GetInstanceRrdData)
	proxmox.Get("/nodes/:node/rrddata", proxmoxCtrl.GetNodeRrdData)
}
