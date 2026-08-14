package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"

	"cbt-core-api/database"
	"cbt-core-api/internal/models"
	"cbt-core-api/internal/repositories"
	"cbt-core-api/internal/services"

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
		// Ipconfig0 will be auto-generated during provisioning based on VMID
		Ipconfig0:  "",
		Status:     "PROVISIONING",
	}

	if err := ctrl.orderRepo.Create(&order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create order"})
	}

	user, _ := ctrl.userRepo.FindByID(userID)
	username := "Customer"
	if user != nil {
		username = user.Username
	}

	// Launch the provisioning pipeline immediately
	orderSnapshot := order
	go func() {
		ctrl.runProvisioningPipeline(orderSnapshot, userID)
	}()

	go func() {
		if err := ctrl.emailService.SendVMProvisioningNotification(order.UserEmail, username, order.Name, order.Cores, order.Memory, order.Storage); err != nil {
			log.Printf("[ERROR] Failed to send notification email to %s: %v", order.UserEmail, err)
		}
	}()

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "VM provisioning started.",
		"orderId": order.ID,
	})
}

func (ctrl *OrderController) reconcileOrdersWithProxmox(orders []models.Order) []models.Order {
	// 1. Fetch active instances from Proxmox across nodes (or default node 'pve' / first node)
	nodes, err := ctrl.proxmoxService.GetNodes()
	if err != nil || len(nodes) == 0 {
		return orders // fallback to DB state if Proxmox API is unreachable
	}

	activeVMNames := make(map[string]bool)
	for _, n := range nodes {
		if nodeMap, ok := n.(map[string]interface{}); ok {
			nodeName, _ := nodeMap["node"].(string)
			if nodeName != "" {
				instances, _ := ctrl.proxmoxService.GetInstances(nodeName)
				for _, inst := range instances {
					if instMap, ok := inst.(map[string]interface{}); ok {
						vmName, _ := instMap["name"].(string)
						if vmName != "" {
							activeVMNames[vmName] = true
						}
					}
				}
			}
		}
	}

	// 2. Reconcile COMPLETED orders against active VM names
	for i := range orders {
		if orders[i].Status == "COMPLETED" {
			// If the VM name no longer exists in Proxmox VE at all ("sesuai dengan proxmox aslinya")
			if !activeVMNames[orders[i].Name] {
				log.Printf("[INFO] Reconciling obsolete COMPLETED order %s (%s) -> DELETED (not found in Proxmox VE)", orders[i].ID, orders[i].Name)
				orders[i].Status = "DELETED"
				ctrl.orderRepo.Update(&orders[i])
				// Also clean up obsolete server record if any
				database.DB.Where("name = ?", orders[i].Name).Delete(&models.Server{})
			}
		}
	}

	return orders
}

func (ctrl *OrderController) GetMyOrders(c *fiber.Ctx) error {
	userID := c.Locals("userId").(string)
	orders, err := ctrl.orderRepo.FindByUserID(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch orders"})
	}
	reconciled := ctrl.reconcileOrdersWithProxmox(orders)
	return c.JSON(reconciled)
}

func (ctrl *OrderController) GetAllOrders(c *fiber.Ctx) error {
	orders, err := ctrl.orderRepo.FindAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch orders"})
	}
	reconciled := ctrl.reconcileOrdersWithProxmox(orders)
	return c.JSON(reconciled)
}

func (ctrl *OrderController) DeleteOrder(c *fiber.Ctx) error {
	orderID := c.Params("id")
	userId := c.Locals("userId").(string)
	role, _ := c.Locals("role").(string)

	order, err := ctrl.orderRepo.FindByID(orderID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	// Only admin or owner can delete, and only if it's FAILED (to prevent accidental deletion of provisioning/active orders)
	if role != "ADMIN" && order.UserID != userId {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	if order.Status != "FAILED" && role != "ADMIN" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Only FAILED orders can be deleted by user"})
	}

	if err := ctrl.orderRepo.Delete(orderID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete order"})
	}

	return c.JSON(fiber.Map{"message": "Order deleted successfully"})
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
	// Auto-generate static IP based on VMID (e.g., 104 -> 172.17.2.104/24)
	autoIpconfig0 := fmt.Sprintf("ip=172.17.2.%s/24,gw=172.17.2.1", newVmid)
	
	ciConfig := VMConfigRequest{
		Cores:      &order.Cores,
		Memory:     &order.Memory,
		CIUser:     &order.Ciuser,
		CIPassword: &order.Cipassword,
		IPConfig0:  &autoIpconfig0,
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

