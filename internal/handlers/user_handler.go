package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Soumya03007/pulseboard/internal/models"
	"github.com/Soumya03007/pulseboard/internal/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UpdateProfileRequest struct {
	DisplayName   *string `json:"display_name"`
	StatusMessage *string `json:"status_message"`
}

type UserHandler struct{ users *repository.UserRepository }

func NewUserHandler(users *repository.UserRepository) *UserHandler { return &UserHandler{users: users} }

func (h *UserHandler) Me(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, user.Profile())
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	var req UpdateProfileRequest
	if c.ShouldBindJSON(&req) != nil || (req.DisplayName == nil && req.StatusMessage == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.DisplayName != nil {
		displayName := strings.TrimSpace(*req.DisplayName)
		if displayName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		user.DisplayName = displayName
	}
	if req.StatusMessage != nil {
		statusMessage := strings.TrimSpace(*req.StatusMessage)
		if statusMessage == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		user.StatusMessage = statusMessage
	}
	user.LastActiveAt = pointerToTime(time.Now())
	user.UpdatedAt = time.Now()
	if err := h.users.Update(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, user.Profile())
}

func (h *UserHandler) DeleteProfile(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	if err := h.users.Delete(user.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UserHandler) currentUser(c *gin.Context) (*models.User, bool) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	id, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	user, err := h.users.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return nil, false
	}
	return user, true
}

func pointerToTime(value time.Time) *time.Time {
	return &value
}
