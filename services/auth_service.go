package services

import (
	"errors"
	"time"

	"cbt-core-api/config"
	"cbt-core-api/models"
	"cbt-core-api/repositories"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Login(username, password string) (string, error)
	GetMe(id string) (*models.User, error)
	EnsureAdminExists() error
}

type authServiceImpl struct {
	userRepo repositories.UserRepository
}

func NewAuthService(userRepo repositories.UserRepository) AuthService {
	return &authServiceImpl{userRepo: userRepo}
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
