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

// ActivateOrder is the VM provisioning entry point.
// It validates the request synchronously (fast) then IMMEDIATELY returns HTTP 202 Accepted.
// All heavy Proxmox work (Clone → WaitForTask → Resize → CloudInit → PowerOn)
// runs in a background goroutine. The frontend must poll GET /orders/me to track
// the status transition: READY_TO_ACTIVATE → PROVISIONING → COMPLETED | FAILED.
//
// Why async? The full pipeline can take 2-5 minutes, far exceeding any reverse proxy
// (Traefik/Nginx) timeout or browser keepalive window.
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

	if order.Status == "PROVISIONING" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "VM provisioning is already in progress. Please wait."})
	}

	if order.Status != "READY_TO_ACTIVATE" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Order is not ready or already activated"})
	}

	if order.ActivationCode != req.Code {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid activation code"})
	}

	// ── Mark as PROVISIONING immediately so frontend/admin can track state ──────
	order.Status = "PROVISIONING"
	order.ProvisionError = ""
	if err := ctrl.orderRepo.Update(order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start provisioning"})
	}

	// ── Launch the long-running pipeline in the background ──────────────────────
	// MUST NOT reference fiber.Ctx after this point (context will be freed).
	// Capture all needed values by value before the goroutine.
	orderSnapshot := *order // copy to avoid data race
	go func() {
		ctrl.runProvisioningPipeline(orderSnapshot, userID)
	}()

	// ── Return 202 Accepted immediately — client must poll /orders/me ────────────
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "VM provisioning started. Poll GET /orders/me for status updates.",
		"orderId": orderID,
	})
}

// runProvisioningPipeline is the background worker for VM provisioning.
// It updates the order status to COMPLETED or FAILED with a descriptive error.
// This function MUST be called as a goroutine.
func (ctrl *OrderController) runProvisioningPipeline(order models.Order, userID string) {
	// Helper: mark order as FAILED with a human-readable error
	failOrder := func(msg string, err error) {
		fullErr := msg
		if err != nil {
			fullErr = fmt.Sprintf("%s: %v", msg, err)
		}
		log.Printf("[ERROR] Provisioning FAILED for order %s — %s", order.ID, fullErr)
		order.Status = "FAILED"
		order.ProvisionError = fullErr
		if updateErr := ctrl.orderRepo.Update(&order); updateErr != nil {
			log.Printf("[CRITICAL] Failed to save FAILED status for order %s: %v", order.ID, updateErr)
		}
	}

	// ── Step 1: Get guaranteed-unique VMID ──────────────────────────────────────
	newVmid, err := ctrl.proxmoxService.GetNextVMID()
	if err != nil {
		failOrder("Failed to allocate VMID from cluster", err)
		return
	}

	const BASE_TEMPLATE_VMID = "100"
	log.Printf("[INFO] Provisioning VM for order %s: NewVMID=%s template=%s node=%s",
		order.ID, newVmid, BASE_TEMPLATE_VMID, order.Node)

	// ── Step 2: Clone VM template ────────────────────────────────────────────────
	cloneUpid, err := ctrl.proxmoxService.CloneVM(order.Node, BASE_TEMPLATE_VMID, newVmid, order.Name)
	if err != nil {
		failOrder("Failed to clone VM template", err)
		return
	}
	log.Printf("[INFO] Clone task started, UPID: %s", cloneUpid)

	// ── Step 3: Poll until clone is fully done (WaitForTask) ────────────────────
	// ── Rollback helper — defined BEFORE WaitForTask so it covers ALL post-clone failures ──
	rollback := func(reason string) {
		log.Printf("[WARN] Rollback triggered for VMID %s: %s", newVmid, reason)
		if rbErr := ctrl.proxmoxService.DeleteVM(order.Node, newVmid); rbErr != nil {
			log.Printf("[ERROR] Rollback failed for VMID %s: %v — manual cleanup required.", newVmid, rbErr)
		}
	}

	// ── Step 3: Poll until clone is fully done ───────────────────────────────────
	if err := ctrl.proxmoxService.WaitForTask(order.Node, cloneUpid); err != nil {
		log.Printf("[ERROR] Clone task %s failed or timed out: %v", cloneUpid, err)
		// Attempt rollback — the clone may or may not have finished; best-effort delete.
		rollback(fmt.Sprintf("WaitForTask failed: %v", err))
		failOrder("VM clone task failed", err)
		return
	}
	log.Printf("[INFO] Clone task %s done. VM %s unlocked.", cloneUpid, newVmid)

	// ── Step 4: Resize disk ──────────────────────────────────────────────────────
	const BASE_DISK_SIZE_GB = 3
	if order.Storage > BASE_DISK_SIZE_GB {
		addSize := order.Storage - BASE_DISK_SIZE_GB
		sizeStr := fmt.Sprintf("+%dG", addSize)
		log.Printf("[INFO] Resizing disk for VMID %s by %s", newVmid, sizeStr)
		if err := ctrl.proxmoxService.ResizeDisk(order.Node, "qemu", newVmid, "scsi0", sizeStr); err != nil {
			rollback(fmt.Sprintf("ResizeDisk failed: %v", err))
			failOrder("Failed to resize VM disk", err)
			return
		}
	}

	// ── Step 5: Apply Cloud-Init config ─────────────────────────────────────────
	ciConfig := VMConfigRequest{
		Cores:      &order.Cores,
		Memory:     &order.Memory,
		CIUser:     &order.Ciuser,
		CIPassword: &order.Cipassword,
		IPConfig0:  &order.Ipconfig0,
	}
	if err := ctrl.proxmoxService.UpdateVMConfig(order.Node, "qemu", newVmid, ciConfig); err != nil {
		rollback(fmt.Sprintf("UpdateVMConfig failed: %v", err))
		failOrder("Failed to apply VM configuration", err)
		return
	}
	log.Printf("[INFO] Config applied to VMID %s. Powering on...", newVmid)

	// ── Step 6: Power on ─────────────────────────────────────────────────────────
	if err := ctrl.proxmoxService.VMPowerAction(order.Node, "qemu", newVmid, "start"); err != nil {
		log.Printf("[WARN] VM %s configured but failed to power on: %v", newVmid, err)
		// Not fatal — admin can start manually
	}

	// ── Step 7: Persist VM record to DB ─────────────────────────────────────────
	newVmidInt, _ := strconv.Atoi(newVmid)
	server := models.Server{
		VMID:   newVmidInt,
		Node:   order.Node,
		Type:   "qemu",
		Name:   order.Name,
		UserID: userID,
	}
	if err := database.DB.Create(&server).Error; err != nil {
		// VM IS running — do NOT rollback. Log for manual reconciliation.
		log.Printf("[CRITICAL] VMID %s running but failed to save to DB: %v. Manual reconciliation required.", newVmid, err)
		failOrder("VM provisioned but DB registration failed. Please contact support.", err)
		return
	}

	// ── Step 8: Mark order COMPLETED ─────────────────────────────────────────────
	order.Status = "COMPLETED"
	order.ProvisionError = ""
	if err := ctrl.orderRepo.Update(&order); err != nil {
		log.Printf("[ERROR] Failed to update order %s to COMPLETED: %v", order.ID, err)
	}

	log.Printf("[INFO] ✅ VM provisioning complete. OrderID=%s, VMID=%s, Node=%s", order.ID, newVmid, order.Node)
}

