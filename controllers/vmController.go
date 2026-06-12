package controllers

import (
	"github.com/gofiber/fiber/v2"
)

func (ctrl *ProxmoxController) VMPowerAction(c *fiber.Ctx) error {
	node := c.Params("node")
	vmid := c.Params("vmid")
	
	var req struct {
		Action string `json:"action"` // start, stop, shutdown, reboot
	}
	
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}

	if req.Action != "start" && req.Action != "stop" && req.Action != "shutdown" && req.Action != "reboot" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid action"})
	}

	err := ctrl.proxmoxService.VMPowerAction(node, "qemu", vmid, req.Action)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

func (ctrl *ProxmoxController) GetVncProxy(c *fiber.Ctx) error {
	node := c.Params("node")
	vmid := c.Params("vmid")
	type_ := c.Params("type") // qemu or lxc

	data, err := ctrl.proxmoxService.GetVncProxy(node, type_, vmid)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(data)
}

type VMConfigRequest struct {
	Memory     *int    `json:"memory,omitempty"`
	Cores      *int    `json:"cores,omitempty"`
	CIUser     *string `json:"ciuser,omitempty"`
	CIPassword *string `json:"cipassword,omitempty"`
	IPConfig0  *string `json:"ipconfig0,omitempty"`
	SSHKeys    *string `json:"sshkeys,omitempty"`
}

func (ctrl *ProxmoxController) UpdateVMConfig(c *fiber.Ctx) error {
	node := c.Params("node")
	vmid := c.Params("vmid")

	var req VMConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON payload"})
	}

	// Security Hardening: Validate constraints
	if req.Memory != nil && (*req.Memory < 512 || *req.Memory > 32768) {
		return c.Status(400).JSON(fiber.Map{"error": "memory must be between 512MB and 32768MB"})
	}
	if req.Cores != nil && (*req.Cores < 1 || *req.Cores > 32) {
		return c.Status(400).JSON(fiber.Map{"error": "cores must be between 1 and 32"})
	}

	err := ctrl.proxmoxService.UpdateVMConfig(node, "qemu", vmid, req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Proxmox API Error: " + err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

func (ctrl *ProxmoxController) GetInstanceIP(c *fiber.Ctx) error {
	node := c.Params("node")
	type_ := c.Params("type")
	vmid := c.Params("vmid")

	ip, err := ctrl.proxmoxService.GetInstanceIP(node, type_, vmid)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "IP not found or agent not running"})
	}

	return c.JSON(fiber.Map{"ip": ip})
}
