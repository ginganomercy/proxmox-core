package controllers

import (
	"math/rand"
	"strings"
	"time"

	"cbt-core-api/database"
	"cbt-core-api/models"
	"cbt-core-api/repositories"

	"github.com/gofiber/fiber/v2"
)

type VoucherController struct {
	userRepo repositories.UserRepository
}

func NewVoucherController(userRepo repositories.UserRepository) *VoucherController {
	return &VoucherController{userRepo: userRepo}
}

// Admin Only
func (ctrl *VoucherController) GenerateVoucher(c *fiber.Ctx) error {
	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid payload"})
	}

	if req.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "amount must be greater than 0"})
	}

	// Generate random code PBJT-XXXX-XXXX
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	code := "PBJT-" + string(b[:4]) + "-" + string(b[4:])

	voucher := models.Voucher{
		Code:   code,
		Amount: req.Amount,
		IsUsed: false,
	}

	if err := database.DB.Create(&voucher).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to generate voucher"})
	}

	return c.JSON(fiber.Map{"status": "success", "voucher": voucher})
}

// Admin Only
func (ctrl *VoucherController) GetVouchers(c *fiber.Ctx) error {
	var vouchers []models.Voucher
	if err := database.DB.Order("created_at desc").Find(&vouchers).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch vouchers"})
	}
	return c.JSON(fiber.Map{"data": vouchers})
}

// Client
func (ctrl *VoucherController) RedeemVoucher(c *fiber.Ctx) error {
	userId := c.Locals("userId").(string)

	var req struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid payload"})
	}

	code := strings.TrimSpace(strings.ToUpper(req.Code))
	if code == "" {
		return c.Status(400).JSON(fiber.Map{"error": "code cannot be empty"})
	}

	voucher, err := ctrl.userRepo.RedeemVoucher(code, userId)
	if err != nil {
		if err.Error() == "record not found" {
			return c.Status(404).JSON(fiber.Map{"error": "voucher code not found"})
		}
		return c.Status(400).JSON(fiber.Map{"error": "voucher already used or invalid"})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"message": "Voucher redeemed successfully",
		"addedAmount": voucher.Amount,
	})
}
