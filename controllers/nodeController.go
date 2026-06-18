package controllers

import (
	"cbt-core-api/database"
	"cbt-core-api/models"
	"cbt-core-api/services"
	"cbt-core-api/utils"

	"github.com/gofiber/fiber/v2"
)

type ProxmoxController struct {
	proxmoxService services.ProxmoxService
}

func NewProxmoxController(proxmoxService services.ProxmoxService) *ProxmoxController {
	return &ProxmoxController{proxmoxService: proxmoxService}
}

func (ctrl *ProxmoxController) GetNodes(c *fiber.Ctx) error {
	data, err := ctrl.proxmoxService.GetNodes()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}

func (ctrl *ProxmoxController) GetNodeStatus(c *fiber.Ctx) error {
	node := c.Params("node")

	if !utils.IsValidNode(node) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	data, err := ctrl.proxmoxService.GetNodeStatus(node)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}

func (ctrl *ProxmoxController) GetInstances(c *fiber.Ctx) error {
	node := c.Params("node")
	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)

	if !utils.IsValidNode(node) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	data, err := ctrl.proxmoxService.GetInstances(node)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Filter based on role
	if role == "ADMIN" {
		return c.JSON(data)
	}

	// For USER, fetch their servers
	var servers []models.Server
	if err := database.DB.Where("user_id = ?", userId).Find(&servers).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to verify ownership"})
	}

	// Map owned VMIDs
	ownedMap := make(map[int]bool)
	for _, s := range servers {
		ownedMap[s.VMID] = true
	}

	// Filter Proxmox API data
	var filtered []interface{}
	for _, v := range data {
		if m, ok := v.(map[string]interface{}); ok {
			vmidFloat, ok := m["vmid"].(float64)
			if ok && ownedMap[int(vmidFloat)] {
				filtered = append(filtered, m)
			}
		}
	}

	return c.JSON(filtered)
}

// CheckOwnership verifies if a user owns a VM or is an ADMIN
func (ctrl *ProxmoxController) CheckOwnership(userId, role, vmid string) bool {
	if role == "ADMIN" {
		return true
	}
	var count int64
	database.DB.Model(&models.Server{}).Where("user_id = ? AND vmid = ?", userId, vmid).Count(&count)
	return count > 0
}
