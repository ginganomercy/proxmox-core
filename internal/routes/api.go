package routes

import (
	"cbt-core-api/internal/controllers"
	"cbt-core-api/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(
	app *fiber.App,
	authCtrl *controllers.AuthController,
	ssoCtrl *controllers.SSOController,
	proxmoxCtrl *controllers.ProxmoxController,
	orderCtrl *controllers.OrderController,
	adminCtrl *controllers.AdminController,
	monitorCtrl *controllers.MonitorController,
) {
	api := app.Group("/api")

	// Public Routes
	auth := api.Group("/auth")
	// auth.Post("/register", authCtrl.Register) // Disabled: No self-service registration in personal admin dashboard
	auth.Post("/login", authCtrl.Login)
	auth.Post("/logout", authCtrl.Logout)
	// auth.Post("/forgot-password", authCtrl.ForgotPassword) // Disabled: No self-service password reset
	// auth.Post("/reset-password", authCtrl.ResetPassword) // Disabled
	// auth.Get("/google", ssoCtrl.GoogleLogin) // Disabled: Google SSO disabled
	// auth.Get("/google/callback", ssoCtrl.GoogleCallback) // Disabled

	// Protected Routes
	protected := api.Group("/", middleware.Protected())

	// Auth verification
	protected.Get("/auth/me", authCtrl.Me)
	protected.Get("/auth/vnc-token", authCtrl.GetVncToken) // Needed for external noVNC websocket proxy

	// Admin Order Routes
	adminOrders := protected.Group("/orders", middleware.AdminOnly())
	adminOrders.Post("/", orderCtrl.CreateOrder)
	adminOrders.Get("/all", orderCtrl.GetAllOrders)

	// Admin Summary Route
	protected.Get("/admin/summary", middleware.AdminOnly(), adminCtrl.GetDashboardSummary)

	// Uptime Monitors
	monitors := protected.Group("/monitors", middleware.AdminOnly())
	monitors.Get("/", monitorCtrl.GetMonitors)
	monitors.Post("/", monitorCtrl.AddTarget)
	monitors.Delete("/:id", monitorCtrl.DeleteTarget)
	monitors.Get("/:id/logs", monitorCtrl.GetMonitorLogs)

	// Proxmox Nodes & Instances
	proxmox := protected.Group("/proxmox")
	proxmox.Get("/nodes", proxmoxCtrl.GetNodes)
	proxmox.Get("/cluster/logs", middleware.AdminOnly(), proxmoxCtrl.GetClusterLogs)
	proxmox.Get("/cluster/tasks", middleware.AdminOnly(), proxmoxCtrl.GetClusterTasks)
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
