package controllers

import (
	"cbt-core-api/database"
	"cbt-core-api/models"
	"cbt-core-api/utils"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func (ctrl *ProxmoxController) VMPowerAction(c *fiber.Ctx) error {
	node := c.Params("node")
	vmid := c.Params("vmid")

	if !utils.IsValidNode(node) || !utils.IsValidVMID(vmid) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)
	if !ctrl.CheckOwnership(userId, role, vmid) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden: You do not own this instance"})
	}

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

	if !utils.IsValidNode(node) || !utils.IsValidVMID(vmid) || !utils.IsValidVMType(type_) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)
	if !ctrl.CheckOwnership(userId, role, vmid) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden: You do not own this instance"})
	}

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

	if !utils.IsValidNode(node) || !utils.IsValidVMID(vmid) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)
	if !ctrl.CheckOwnership(userId, role, vmid) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden: You do not own this instance"})
	}

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

	if !utils.IsValidNode(node) || !utils.IsValidVMID(vmid) || !utils.IsValidVMType(type_) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid parameter format (potential path traversal detected)"})
	}

	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)
	if !ctrl.CheckOwnership(userId, role, vmid) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden: You do not own this instance"})
	}

	ip, err := ctrl.proxmoxService.GetInstanceIP(node, type_, vmid)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "IP not found or agent not running"})
	}

	return c.JSON(fiber.Map{"ip": ip})
}

type CreateVMRequest struct {
	Node     string `json:"node"`
	Name     string `json:"name"`
	Cores    int    `json:"cores"`
	Memory   int    `json:"memory"` // MB
	Storage  int    `json:"storage"` // GB
	// Cloud Init
	CIUser     string `json:"ciuser"`
	CIPassword string `json:"cipassword"`
	IPConfig0  string `json:"ipconfig0"`
}

func (ctrl *ProxmoxController) CreateVM(c *fiber.Ctx) error {
	userId := c.Locals("userId").(string)

	var req CreateVMRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid payload"})
	}

	if !utils.IsValidNode(req.Node) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid node format"})
	}

	// Cost calculation:
	// Cores: 10000/core, RAM: 10/MB, Storage: 5000/GB
	cost := float64(req.Cores*10000 + req.Memory*10 + req.Storage*5000)

	var user models.User
	if err := database.DB.Where("id = ?", userId).First(&user).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	}

	if user.Balance < cost {
		return c.Status(400).JSON(fiber.Map{"error": "insufficient balance. Please redeem a voucher."})
	}

	// Generate a new unique VMID (e.g. 500-900)
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	newVmidInt := 500 + seededRand.Intn(400)
	newVmid := strconv.Itoa(newVmidInt)

	// Base Image is debian-golden-image (VMID: 100)
	baseVmid := "100"

	// 1. Clone VM
	err := ctrl.proxmoxService.CloneVM(req.Node, baseVmid, newVmid, req.Name)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to clone VM from Golden Image", "details": err.Error()})
	}

	// Wait a bit for clone to register
	time.Sleep(3 * time.Second)

	// 2. Resize Disk if needed (Golden image base is 3GB)
	if req.Storage > 3 {
		addSize := req.Storage - 3
		_ = ctrl.proxmoxService.ResizeDisk(req.Node, "qemu", newVmid, "scsi0", fmt.Sprintf("+%dG", addSize))
	}

	// 3. Update Hardware & Cloud-Init Specs
	ciConfig := VMConfigRequest{
		Cores:      &req.Cores,
		Memory:     &req.Memory,
		CIUser:     &req.CIUser,
		CIPassword: &req.CIPassword,
		IPConfig0:  &req.IPConfig0,
	}
	_ = ctrl.proxmoxService.UpdateVMConfig(req.Node, "qemu", newVmid, ciConfig)

	// 4. Deduct Balance and Create Ownership Record
	database.DB.Transaction(func(tx *gorm.DB) error {
		tx.Model(&user).Update("balance", gorm.Expr("balance - ?", cost))
		
		server := models.Server{
			VMID:   newVmidInt,
			Node:   req.Node,
			Type:   "qemu",
			Name:   req.Name,
			UserID: userId,
		}
		tx.Create(&server)
		return nil
	})

	return c.JSON(fiber.Map{
		"status": "success",
		"message": "VM Provisioning started successfully",
		"vmid": newVmidInt,
		"cost": cost,
	})
}
