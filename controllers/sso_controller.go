package controllers

import (
	"context"
	"encoding/json"
	"net/http"

	"cbt-core-api/config"
	"cbt-core-api/services"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type SSOController struct {
	authService services.AuthService
	oauthConfig *oauth2.Config
}

func NewSSOController(authService services.AuthService) *SSOController {
	return &SSOController{
		authService: authService,
	}
}

func (c *SSOController) getOAuthConfig() *oauth2.Config {
	if c.oauthConfig == nil {
		c.oauthConfig = &oauth2.Config{
			ClientID:     config.Env.GoogleClientID,
			ClientSecret: config.Env.GoogleClientSecret,
			RedirectURL:  config.Env.GoogleRedirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		}
	}
	return c.oauthConfig
}

func (c *SSOController) GoogleLogin(ctx *fiber.Ctx) error {
	oauthConfig := c.getOAuthConfig()
	if oauthConfig.ClientID == "" {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Google SSO is not configured on the server",
		})
	}

	// State should ideally be a random string saved in a cookie/session to prevent CSRF,
	// but for simplicity in this stateless API, we can use a static or simple state.
	url := oauthConfig.AuthCodeURL("cbt-sso-state")
	return ctx.Redirect(url, fiber.StatusTemporaryRedirect)
}

func (c *SSOController) GoogleCallback(ctx *fiber.Ctx) error {
	state := ctx.Query("state")
	if state != "cbt-sso-state" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid state"})
	}

	code := ctx.Query("code")
	if code == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Code not found"})
	}

	oauthConfig := c.getOAuthConfig()
	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Failed to exchange token"})
	}

	// Get user info from Google
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Failed to get user info"})
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse user info"})
	}

	if userInfo.Email == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email not provided by Google"})
	}

	// Authenticate or Register User
	jwtToken, err := c.authService.LoginWithGoogle(userInfo.Email)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Redirect back to frontend with token
	// Assuming frontend runs on same domain or we redirect to the main frontend URL
	// We'll redirect to a special SSO callback page on the frontend
	frontendURL := "https://cloud-dashboard.pbjt.web.id/sso-callback?token=" + jwtToken
	if config.Env.NodeEnv == "development" {
		frontendURL = "http://localhost:5173/sso-callback?token=" + jwtToken
	}

	return ctx.Redirect(frontendURL, fiber.StatusTemporaryRedirect)
}
