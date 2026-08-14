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
