package services

import (
	"errors"
	"time"

	"cbt-core-api/config"
	"cbt-core-api/models"
	"cbt-core-api/repositories"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(username, password string) error
	Login(username, password string) (string, error)
	GetMe(id string) (*models.User, error)
	EnsureAdminExists() error
	RequestPasswordReset(username string) error
	ResetPassword(token, newPassword string) error
}

type authServiceImpl struct {
	userRepo     repositories.UserRepository
	emailService EmailService
}

func NewAuthService(userRepo repositories.UserRepository, emailService EmailService) AuthService {
	return &authServiceImpl{userRepo: userRepo, emailService: emailService}
}

func (s *authServiceImpl) EnsureAdminExists() error {
	count, err := s.userRepo.Count()
	if err != nil {
		return err
	}
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte(config.Env.AdminPassword), bcrypt.DefaultCost)
		admin := models.User{
			Username:     config.Env.AdminUsername,
			PasswordHash: string(hash),
			Role:         "ADMIN",
		}
		return s.userRepo.Create(&admin)
	}
	return nil
}

func (s *authServiceImpl) Register(username, password string) error {
	s.EnsureAdminExists()

	// Check if username already exists
	_, err := s.userRepo.FindByUsername(username)
	if err == nil {
		return errors.New("username already taken")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("failed to hash password")
	}

	user := models.User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         "USER", // Default role for new signups
		Balance:      0.0,
	}

	return s.userRepo.Create(&user)
}

func (s *authServiceImpl) Login(username, password string) (string, error) {
	s.EnsureAdminExists()

	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(config.Env.JWTSecret))
	if err != nil {
		return "", errors.New("could not generate token")
	}

	return tokenString, nil
}

func (s *authServiceImpl) GetMe(id string) (*models.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *authServiceImpl) RequestPasswordReset(username string) error {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		// Don't leak whether the user exists or not
		return nil
	}

	token := uuid.NewString()
	expiry := time.Now().Add(1 * time.Hour)
	
	user.ResetToken = &token
	user.ResetTokenExpiry = &expiry

	if err := s.userRepo.Update(user); err != nil {
		return errors.New("failed to generate reset token")
	}

	return s.emailService.SendPasswordReset(user.Username, user.Username, token)
}

func (s *authServiceImpl) ResetPassword(token, newPassword string) error {
	user, err := s.userRepo.FindByResetToken(token)
	if err != nil {
		return errors.New("invalid or expired token")
	}

	if user.ResetTokenExpiry == nil || time.Now().After(*user.ResetTokenExpiry) {
		return errors.New("token has expired")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("failed to hash password")
	}

	user.PasswordHash = string(hash)
	user.ResetToken = nil
	user.ResetTokenExpiry = nil

	if err := s.userRepo.Update(user); err != nil {
		return errors.New("failed to reset password")
	}

	return nil
}
