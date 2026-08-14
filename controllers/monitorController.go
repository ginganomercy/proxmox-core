package controllers

import (
	"time"
	"cbt-core-api/database"
	"cbt-core-api/models"
	"github.com/gofiber/fiber/v2"
)

type MonitorController struct{}

func NewMonitorController() *MonitorController {
	return &MonitorController{}
}

func (c *MonitorController) GetMonitors(ctx *fiber.Ctx) error {
	var targets []models.MonitorTarget
	if err := database.DB.Find(&targets).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch monitor targets",
		})
	}
	return ctx.JSON(targets)
}

func (c *MonitorController) GetMonitorLogs(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Target ID is required",
		})
	}

	// Calculate time 24 hours ago
	yesterday := time.Now().Add(-24 * time.Hour)

	var logs []models.MonitorLog
	if err := database.DB.Where("monitor_target_id = ? AND created_at >= ?", id, yesterday).
		Order("created_at asc").
		Find(&logs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch monitor logs",
		})
	}

	return ctx.JSON(logs)
}

func (c *MonitorController) AddTarget(ctx *fiber.Ctx) error {
	var payload struct {
		Domain string `json:"domain"`
	}

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	domain := payload.Domain
	if domain == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Domain is required"})
	}

	// Auto-prefix with https:// if no scheme is provided
	if len(domain) < 4 || (domain[:4] != "http" && domain[:5] != "https") {
		domain = "https://" + domain
	}

	// Check if exists
	var count int64
	database.DB.Model(&models.MonitorTarget{}).Where("domain = ?", domain).Count(&count)
	if count > 0 {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Domain already being monitored"})
	}

	target := models.MonitorTarget{
		Domain: domain,
		Status: "PENDING",
	}

	if err := database.DB.Create(&target).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to add domain"})
	}

	return ctx.JSON(target)
}

func (c *MonitorController) DeleteTarget(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Target ID is required"})
	}

	// Delete from MonitorTarget and cascade manually to MonitorLog if constraints aren't set
	database.DB.Where("monitor_target_id = ?", id).Delete(&models.MonitorLog{})
	
	if err := database.DB.Where("id = ?", id).Delete(&models.MonitorTarget{}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete target"})
	}

	return ctx.JSON(fiber.Map{"message": "Target deleted successfully"})
}
