package handlers

import (
	"github.com/Soumya03007/pulseboard/internal/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

type AuthRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}
type AuthHandler struct{ auth *services.AuthService }

func NewAuthHandler(auth *services.AuthService) *AuthHandler { return &AuthHandler{auth: auth} }
func (h *AuthHandler) Register(c *gin.Context) {
	var req AuthRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.auth.RegisterUser(req.Email, req.Password); err != nil {
		if err == services.ErrUserAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "user registered successfully"})
}
func (h *AuthHandler) Login(c *gin.Context) {
	var req AuthRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	token, err := h.auth.LoginUser(req.Email, req.Password)
	if err != nil {
		if err == services.ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}
