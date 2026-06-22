package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"

	"cbt-core-api/database"
	"cbt-core-api/models"
	"cbt-core-api/repositories"
	"cbt-core-api/services"

	"github.com/gofiber/fiber/v2"
)

type OrderController struct {
	orderRepo      repositories.OrderRepository
	userRepo       repositories.UserRepository
	emailService   services.EmailService
	proxmoxService services.ProxmoxService
}

func NewOrderController(
	orderRepo repositories.OrderRepository,
	userRepo repositories.UserRepository,
	emailService services.EmailService,
	proxmoxService services.ProxmoxService,
) *OrderController {
	return &OrderController{
		orderRepo:      orderRepo,
		userRepo:       userRepo,
		emailService:   emailService,
		proxmoxService: proxmoxService,
	}
}

func generateRandomCode(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "123456" // fallback
	}
	return hex.EncodeToString(bytes)
}

func (ctrl *OrderController) CreateOrder(c *fiber.Ctx) error {
	userID := c.Locals("userId").(string)

	var req models.Order
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	// Basic validation
	if req.UserEmail == "" || req.Name == "" || req.Cores == 0 || req.Memory == 0 || req.Storage == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing required fields"})
	}

	// Cek apakah pengguna sudah memiliki pesanan / VM sebelumnya
	existingOrders, err := ctrl.orderRepo.FindByUserID(userID)
	if err == nil && len(existingOrders) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Anda telah mencapai batas maksimal 1 Server Virtual per akun."})
	}

	// Calculate cost
	totalCost := float64(req.Cores*10000 + req.Memory*10 + req.Storage*5000)

	order := models.Order{
		UserID:     userID,
		UserEmail:  req.UserEmail,
		Node:       req.Node,
		Name:       req.Name,
		Cores:      req.Cores,
		Memory:     req.Memory,
		Storage:    req.Storage,
		Ciuser:     req.Ciuser,
		Cipassword: req.Cipassword,
		Ipconfig0:  req.Ipconfig0,
		TotalCost:  totalCost,
		Status:     "PENDING",
	}

	if err := ctrl.orderRepo.Create(&order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create order"})
	}

	user, _ := ctrl.userRepo.FindByID(userID)
	username := "Customer"
	if user != nil {
		username = user.Username
	}

	go func() {
		if err := ctrl.emailService.SendOrderInvoice(order.UserEmail, username, order.Name, order.Cores, order.Memory, order.Storage, order.TotalCost); err != nil {
			log.Printf("[ERROR] Failed to send order invoice email to %s: %v", order.UserEmail, err)
		}
	}()

	return c.Status(fiber.StatusCreated).JSON(order)
}

func (ctrl *OrderController) GetMyOrders(c *fiber.Ctx) error {
	userID := c.Locals("userId").(string)
	orders, err := ctrl.orderRepo.FindByUserID(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch orders"})
	}
	return c.JSON(orders)
}

func (ctrl *OrderController) GetAllOrders(c *fiber.Ctx) error {
	orders, err := ctrl.orderRepo.FindAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch orders"})
	}
	return c.JSON(orders)
}

func (ctrl *OrderController) GenerateCode(c *fiber.Ctx) error {
	orderID := c.Params("id")
	order, err := ctrl.orderRepo.FindByID(orderID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	if order.Status != "PENDING" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Order is not PENDING"})
	}

	// Get user info for the email (Toleran terhadap error misal user sudah terhapus)
	user, _ := ctrl.userRepo.FindByID(order.UserID)
	username := "Customer"
	if user != nil && user.Username != "" {
		username = user.Username
	}

	// Generate 6-char random code using cryptographically secure source
	code := generateRandomCode(6)
	order.ActivationCode = code
	order.Status = "READY_TO_ACTIVATE"

	if err := ctrl.orderRepo.Update(order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update order"})
	}

	// Send Email asynchronously — fire-and-forget with structured error log
	go func() {
		if err := ctrl.emailService.SendActivationCode(order.UserEmail, username, order.Name, code); err != nil {
			log.Printf("[ERROR] Failed to send activation email to %s: %v", order.UserEmail, err)
		}
	}()

	return c.JSON(fiber.Map{"message": "Code generated and email queued", "code": code})
}

type ActivateRequest struct {
	Code string `json:"code"`
}

// ActivateOrder is the core VM provisioning pipeline.
// It follows a strict sequential flow with UPID polling at each async step:
// 1. Get safe unique VMID from cluster (anti-collision)
// 2. Clone VM from template → wait for task completion (WaitForTask)
// 3. Resize disk to ordered size
// 4. Apply cloud-init config (CPU, RAM, user, password, IP)
// 5. Power on the VM
// 6. On any failure after clone: auto-rollback by deleting the cloned VM
func (ctrl *OrderController) ActivateOrder(c *fiber.Ctx) error {
	orderID := c.Params("id")
	userID := c.Locals("userId").(string)

	var req ActivateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	order, err := ctrl.orderRepo.FindByID(orderID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	// Authorization: ensure the requesting user owns this order
	if order.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not own this order"})
	}

	if order.Status != "READY_TO_ACTIVATE" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Order is not ready or already activated"})
	}

	if order.ActivationCode != req.Code {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid activation code"})
	}

	// ── Step 1: Get a guaranteed-unique VMID from the Proxmox cluster ──────────
	// Replaces unsafe math/rand approach that could collide with existing VMs.
	newVmid, err := ctrl.proxmoxService.GetNextVMID()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to allocate a new VMID from cluster",
			"details": err.Error(),
		})
	}

	// The golden image (base template) must exist at VMID 100 in Proxmox with Cloud-Init configured.
	const BASE_TEMPLATE_VMID = "100"
	log.Printf("[INFO] Provisioning VM for order %s: NewVMID=%s from template %s on node %s",
		orderID, newVmid, BASE_TEMPLATE_VMID, order.Node)

	// ── Step 2: Clone the template and obtain Proxmox UPID task token ──────────
	cloneUpid, err := ctrl.proxmoxService.CloneVM(order.Node, BASE_TEMPLATE_VMID, newVmid, order.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to clone VM template",
			"details": err.Error(),
		})
	}
	log.Printf("[INFO] Clone task started. Waiting for UPID: %s", cloneUpid)

	// ── Step 3: WAIT for clone task to fully complete ───────────────────────────
	// Polls Proxmox every 3s until task is "OK". Replaces unreliable time.Sleep(3s).
	if err := ctrl.proxmoxService.WaitForTask(order.Node, cloneUpid); err != nil {
		log.Printf("[ERROR] Clone task %s failed: %v", cloneUpid, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "VM clone task failed",
			"details": err.Error(),
		})
	}
	log.Printf("[INFO] Clone task %s completed. VM %s is fully unlocked and ready.", cloneUpid, newVmid)

	// From here, any failure must trigger a rollback to avoid orphaned VMs.
	rollback := func(reason string) {
		log.Printf("[WARN] Rollback triggered for VMID %s. Reason: %s", newVmid, reason)
		if rbErr := ctrl.proxmoxService.DeleteVM(order.Node, newVmid); rbErr != nil {
			log.Printf("[ERROR] Rollback failed for VMID %s: %v — manual cleanup required.", newVmid, rbErr)
		}
	}

	// ── Step 4: Resize disk if ordered storage exceeds base template size ───────
	const BASE_DISK_SIZE_GB = 3
	if order.Storage > BASE_DISK_SIZE_GB {
		addSize := order.Storage - BASE_DISK_SIZE_GB
		sizeStr := fmt.Sprintf("+%dG", addSize)
		log.Printf("[INFO] Resizing disk for VMID %s by %s", newVmid, sizeStr)

		if err := ctrl.proxmoxService.ResizeDisk(order.Node, "qemu", newVmid, "scsi0", sizeStr); err != nil {
			rollback(fmt.Sprintf("ResizeDisk failed: %v", err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to resize VM disk",
				"details": err.Error(),
			})
		}
	}

	// ── Step 5: Apply Cloud-Init configuration ──────────────────────────────────
	// VM is now fully unlocked post-WaitForTask, so config will be applied correctly.
	ciConfig := VMConfigRequest{
		Cores:      &order.Cores,
		Memory:     &order.Memory,
		CIUser:     &order.Ciuser,
		CIPassword: &order.Cipassword,
		IPConfig0:  &order.Ipconfig0,
	}
	if err := ctrl.proxmoxService.UpdateVMConfig(order.Node, "qemu", newVmid, ciConfig); err != nil {
		rollback(fmt.Sprintf("UpdateVMConfig failed: %v", err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to apply VM configuration",
			"details": err.Error(),
		})
	}
	log.Printf("[INFO] Config applied to VMID %s. Powering on...", newVmid)

	// ── Step 6: Power on the VM ─────────────────────────────────────────────────
	if err := ctrl.proxmoxService.VMPowerAction(order.Node, "qemu", newVmid, "start"); err != nil {
		// VM is configured but failed to start — recoverable state, admin can start manually.
		log.Printf("[WARN] VM %s configured but failed to power on: %v", newVmid, err)
	}

	// ── Step 7: Persist VM ownership record to database ─────────────────────────
	newVmidInt, _ := strconv.Atoi(newVmid)
	server := models.Server{
		VMID:   newVmidInt,
		Node:   order.Node,
		Type:   "qemu",
		Name:   order.Name,
		UserID: userID,
	}

	if err := database.DB.Create(&server).Error; err != nil {
		// VM is running. Do NOT rollback — log for manual reconciliation instead.
		log.Printf("[CRITICAL] VMID %s provisioned but failed to save to DB: %v. Manual reconciliation required.", newVmid, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "VM provisioned but failed to register in database. Please contact support.",
		})
	}

	// ── Step 8: Mark order as completed ─────────────────────────────────────────
	order.Status = "COMPLETED"
	if err := ctrl.orderRepo.Update(order); err != nil {
		log.Printf("[ERROR] Failed to update order %s status to COMPLETED: %v", orderID, err)
	}

	log.Printf("[INFO] VM provisioning successful. OrderID=%s, VMID=%s, Node=%s", orderID, newVmid, order.Node)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "VM provisioned successfully",
		"vmid":    newVmidInt,
		"node":    order.Node,
	})
}
