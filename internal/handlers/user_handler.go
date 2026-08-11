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
	Presence      *string `json:"presence"`
	Availability  *string `json:"availability"`
}

type CreateActivityRequest struct {
	Title string `json:"title" binding:"required"`
}

type UserHandler struct {
	users      *repository.UserRepository
	activities *repository.ActivityRepository
}

func NewUserHandler(users *repository.UserRepository, activities *repository.ActivityRepository) *UserHandler {
	return &UserHandler{users: users, activities: activities}
}

func (h *UserHandler) Me(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	activity, err := h.activities.CurrentForUser(user.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, user.Profile(activity))
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	var req UpdateProfileRequest
	if c.ShouldBindJSON(&req) != nil || (req.DisplayName == nil && req.StatusMessage == nil && req.Presence == nil && req.Availability == nil) {
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
	if req.Presence != nil {
		presence := strings.ToLower(strings.TrimSpace(*req.Presence))
		if presence != "online" && presence != "away" && presence != "offline" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		user.Presence = presence
	}
	if req.Availability != nil {
		availability := strings.ToLower(strings.TrimSpace(*req.Availability))
		if availability != "available" && availability != "busy" && availability != "in_meeting" && availability != "do_not_disturb" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		user.Availability = availability
	}
	user.LastActiveAt = pointerToTime(time.Now())
	user.UpdatedAt = time.Now()
	if err := h.users.Update(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	activity, err := h.activities.CurrentForUser(user.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, user.Profile(activity))
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

func (h *UserHandler) ListActivities(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	activities, err := h.activities.ListForUser(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	responses := make([]models.ActivityResponse, 0, len(activities))
	for _, activity := range activities {
		responses = append(responses, activity.Response())
	}
	c.JSON(http.StatusOK, responses)
}

func (h *UserHandler) CreateActivity(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	var req CreateActivityRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	activity, err := h.activities.CreateForUser(user.ID, title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusCreated, activity.Response())
}

func (h *UserHandler) CompleteActivity(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	if _, err := h.activities.CompleteCurrentForUser(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "completed"})
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
