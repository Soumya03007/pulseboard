package services

import (
	"errors"
	"github.com/Soumya03007/pulseboard/internal/models"
	"github.com/Soumya03007/pulseboard/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"strings"
	"time"
)

var (
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type AuthService struct {
	users     *repository.UserRepository
	jwtSecret []byte
}

func NewAuthService(users *repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{users: users, jwtSecret: []byte(jwtSecret)}
}
func (s *AuthService) RegisterUser(email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	_, err := s.users.GetByEmail(email)
	if err == nil {
		return ErrUserAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.users.Create(&models.User{Email: email, PasswordHash: string(hash)})
}
func (s *AuthService) LoginUser(email, password string) (string, error) {
	user, err := s.users.GetByEmail(strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return "", ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", ErrInvalidCredentials
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user.ID, "exp": time.Now().Add(24 * time.Hour).Unix()})
	return token.SignedString(s.jwtSecret)
}
