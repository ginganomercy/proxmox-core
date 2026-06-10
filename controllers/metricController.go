package controllers

import (
	"github.com/gofiber/fiber/v2"
)

// GetInstanceRrdData fetches CPU and RAM history for charts
func (ctrl *ProxmoxController) GetInstanceRrdData(c *fiber.Ctx) error {
	node := c.Params("node")
	vmid := c.Params("vmid")
	type_ := c.Params("type") // qemu or lxc

	timeframe := c.Query("timeframe", "hour") // default to hour

	data, err := ctrl.proxmoxService.GetInstanceRrdData(node, type_, vmid, timeframe)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}

// GetNodeRrdData fetches CPU/RAM history for the whole host node
func (ctrl *ProxmoxController) GetNodeRrdData(c *fiber.Ctx) error {
	node := c.Params("node")
	timeframe := c.Query("timeframe", "hour")

	data, err := ctrl.proxmoxService.GetNodeRrdData(node, timeframe)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(data)
}
