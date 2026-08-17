package controllers

import (
	"time"
	"cbt-core-api/internal/services"

	"github.com/gofiber/fiber/v2"
)

type AuthController struct {
	authService services.AuthService
}

func NewAuthController(authService services.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (ctrl *AuthController) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Username and password cannot be empty"})
	}

	err := ctrl.authService.Register(req.Username, req.Password)
	if err != nil {
		if err.Error() == "username already taken" {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "User registered successfully"})
}

func (ctrl *AuthController) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	tokenString, err := ctrl.authService.Login(req.Username, req.Password)
	if err != nil {
		if err.Error() == "invalid credentials" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Enterprise Hardening: Set HttpOnly Cookie to prevent XSS
	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    tokenString,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   true, // Should be true in production (HTTPS)
		SameSite: "None",
		Domain:   ".pbjt.web.id", // Allow cookie sharing across subdomains
	})

	return c.JSON(LoginResponse{Token: tokenString}) // Still returning token for backward compatibility during transition
}

func (ctrl *AuthController) Logout(c *fiber.Ctx) error {
	// Clear the HttpOnly cookie by setting its expiration to the past
	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: true,
		Secure:   true,
		SameSite: "None",
		Domain:   ".pbjt.web.id",
	})
	return c.JSON(fiber.Map{"message": "Logged out successfully"})
}

// GetVncToken allows the frontend to retrieve the JWT token temporarily to authenticate
// the external noVNC WebSocket proxy, since the token is stored in an HttpOnly cookie and
// cannot be read directly by JavaScript.
func (ctrl *AuthController) GetVncToken(c *fiber.Ctx) error {
	tokenString := c.Cookies("token")
	if tokenString == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "No token found in cookie"})
	}
	return c.JSON(fiber.Map{"token": tokenString})
}

func (ctrl *AuthController) Me(c *fiber.Ctx) error {
	userID := c.Locals("userId").(string)

	user, err := ctrl.authService.GetMe(userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(user)
}

func (ctrl *AuthController) ForgotPassword(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Username is required"})
	}

	// Always return success to prevent username enumeration
	_ = ctrl.authService.RequestPasswordReset(req.Username)

	return c.JSON(fiber.Map{"message": "If that username exists, a password reset link has been sent."})
}

func (ctrl *AuthController) ResetPassword(c *fiber.Ctx) error {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Token == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Token and newPassword are required"})
	}

	err := ctrl.authService.ResetPassword(req.Token, req.NewPassword)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Password successfully reset"})
}
