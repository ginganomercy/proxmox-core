package controllers

import (
	"cbt-core-api/services"
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
	data, err := ctrl.proxmoxService.GetNodeStatus(node)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}

func (ctrl *ProxmoxController) GetInstances(c *fiber.Ctx) error {
	node := c.Params("node")
	data, err := ctrl.proxmoxService.GetInstances(node)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}
