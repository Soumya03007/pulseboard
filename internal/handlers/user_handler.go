package handlers

import (
	"github.com/Soumya03007/pulseboard/internal/repository"
	"github.com/gin-gonic/gin"
	"net/http"
)

type UserHandler struct{ users *repository.UserRepository }

func NewUserHandler(users *repository.UserRepository) *UserHandler { return &UserHandler{users: users} }
func (h *UserHandler) Me(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.users.GetByID(id)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, user.Profile())
}
