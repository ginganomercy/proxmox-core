package controllers

import (
	"cbt-core-api/database"
	"cbt-core-api/models"

	"github.com/gofiber/fiber/v2"
)

type AdminController struct{}

func NewAdminController() *AdminController {
	return &AdminController{}
}

func (ctrl *AdminController) GetDashboardSummary(c *fiber.Ctx) error {
	var totalOrders int64
	var pendingOrders int64
	var totalVouchers int64
	var activeVouchers int64

	// Count Orders
	database.DB.Model(&models.Order{}).Count(&totalOrders)
	database.DB.Model(&models.Order{}).Where("status = ?", "PENDING").Count(&pendingOrders)

	// Count Vouchers
	database.DB.Model(&models.Voucher{}).Count(&totalVouchers)
	database.DB.Model(&models.Voucher{}).Where("is_used = ?", false).Count(&activeVouchers)

	return c.JSON(fiber.Map{
		"total_orders":    totalOrders,
		"pending_orders":  pendingOrders,
		"total_vouchers":  totalVouchers,
		"active_vouchers": activeVouchers,
	})
}
