package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
	"strconv"
	"time"

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
		UserID:    userID,
		UserEmail: req.UserEmail,
		Node:      req.Node,
		Name:      req.Name,
		Cores:     req.Cores,
		Memory:    req.Memory,
		Storage:   req.Storage,
		Ciuser:    req.Ciuser,
		Cipassword: req.Cipassword,
		Ipconfig0: req.Ipconfig0,
		TotalCost: totalCost,
		Status:    "PENDING",
	}

	if err := ctrl.orderRepo.Create(&order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create order"})
	}

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

	// Get user info for the email
	user, err := ctrl.userRepo.FindByID(order.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch user data"})
	}

	// Generate 6-char random code
	code := generateRandomCode(6)
	order.ActivationCode = code
	order.Status = "READY_TO_ACTIVATE"

	if err := ctrl.orderRepo.Update(order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update order"})
	}

	// Send Email
	go func() {
		err := ctrl.emailService.SendActivationCode(order.UserEmail, user.Username, order.Name, code)
		if err != nil {
			fmt.Println("Failed to send email to", order.UserEmail, err)
		}
	}()

	return c.JSON(fiber.Map{"message": "Code generated and email queued", "code": code})
}

type ActivateRequest struct {
	Code string `json:"code"`
}

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

	if order.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not own this order"})
	}

	if order.Status != "READY_TO_ACTIVATE" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Order is not ready or already activated"})
	}

	if order.ActivationCode != req.Code {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid activation code"})
	}

	// Generate a new unique VMID (e.g. 500-900)
	seededRand := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	newVmidInt := 500 + seededRand.Intn(400)
	newVmid := strconv.Itoa(newVmidInt)
	baseVmid := "100"

	// 1. Clone VM
	err = ctrl.proxmoxService.CloneVM(order.Node, baseVmid, newVmid, order.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to clone VM", "details": err.Error()})
	}

	time.Sleep(3 * time.Second)

	// 2. Resize Disk
	if order.Storage > 3 {
		addSize := order.Storage - 3
		_ = ctrl.proxmoxService.ResizeDisk(order.Node, "qemu", newVmid, "scsi0", fmt.Sprintf("+%dG", addSize))
	}

	// 3. Update Config
	ciConfig := VMConfigRequest{
		Cores:      &order.Cores,
		Memory:     &order.Memory,
		CIUser:     &order.Ciuser,
		CIPassword: &order.Cipassword,
		IPConfig0:  &order.Ipconfig0,
	}
	_ = ctrl.proxmoxService.UpdateVMConfig(order.Node, "qemu", newVmid, ciConfig)

	// 4. Register server to DB
	server := models.Server{
		VMID:   newVmidInt,
		Node:   order.Node,
		Type:   "qemu",
		Name:   order.Name,
		UserID: userID,
	}
	
	if err := database.DB.Create(&server).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save server to database"})
	}

	order.Status = "COMPLETED"
	ctrl.orderRepo.Update(order)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "VM provisioned successfully", "vmid": newVmidInt})
}
