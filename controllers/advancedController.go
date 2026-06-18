package controllers

import (
	"cbt-core-api/utils"

	"github.com/gofiber/fiber/v2"
)

// Snapshots Operations

func (ctrl *ProxmoxController) GetSnapshots(c *fiber.Ctx) error {
	node := c.Params("node")
	vmid := c.Params("vmid")
	type_ := c.Params("type")

	if !utils.IsValidNode(node) || !utils.IsValidVMID(vmid) || !utils.IsValidVMType(type_) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)
	if !ctrl.CheckOwnership(userId, role, vmid) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden: You do not own this instance"})
	}

	data, err := ctrl.proxmoxService.GetSnapshots(node, type_, vmid)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(data)
}

func (ctrl *ProxmoxController) CreateSnapshot(c *fiber.Ctx) error {
	node := c.Params("node")
	vmid := c.Params("vmid")
	type_ := c.Params("type")

	if !utils.IsValidNode(node) || !utils.IsValidVMID(vmid) || !utils.IsValidVMType(type_) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)
	if !ctrl.CheckOwnership(userId, role, vmid) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden: You do not own this instance"})
	}

	var req struct {
		Snapname    string `json:"snapname"`
		Description string `json:"description"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid payload"})
	}

	err := ctrl.proxmoxService.CreateSnapshot(node, type_, vmid, req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

func (ctrl *ProxmoxController) RollbackSnapshot(c *fiber.Ctx) error {
	node := c.Params("node")
	vmid := c.Params("vmid")
	type_ := c.Params("type")
	snapname := c.Params("snapname")

	if !utils.IsValidNode(node) || !utils.IsValidVMID(vmid) || !utils.IsValidVMType(type_) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)
	if !ctrl.CheckOwnership(userId, role, vmid) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden: You do not own this instance"})
	}

	err := ctrl.proxmoxService.RollbackSnapshot(node, type_, vmid, snapname)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

func (ctrl *ProxmoxController) DeleteSnapshot(c *fiber.Ctx) error {
	node := c.Params("node")
	vmid := c.Params("vmid")
	type_ := c.Params("type")
	snapname := c.Params("snapname")

	if !utils.IsValidNode(node) || !utils.IsValidVMID(vmid) || !utils.IsValidVMType(type_) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)
	if !ctrl.CheckOwnership(userId, role, vmid) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden: You do not own this instance"})
	}

	err := ctrl.proxmoxService.DeleteSnapshot(node, type_, vmid, snapname)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

// Rebuild OS

func (ctrl *ProxmoxController) RebuildInstance(c *fiber.Ctx) error {
	node := c.Params("node")
	vmid := c.Params("vmid")
	type_ := c.Params("type")

	if !utils.IsValidNode(node) || !utils.IsValidVMID(vmid) || !utils.IsValidVMType(type_) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)
	if !ctrl.CheckOwnership(userId, role, vmid) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden: You do not own this instance"})
	}

	err := ctrl.proxmoxService.RebuildInstance(node, type_, vmid)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Rebuild OS operation completed. Golden image applied."})
}
