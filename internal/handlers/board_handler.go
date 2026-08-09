package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Soumya03007/pulseboard/internal/models"
	"github.com/Soumya03007/pulseboard/internal/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BoardHandler struct{ boards *repository.BoardRepository }

type CreateBoardRequest struct {
	Title       string  `json:"title" binding:"required"`
	Description *string `json:"description"`
}

type UpdateBoardRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

func NewBoardHandler(boards *repository.BoardRepository) *BoardHandler {
	return &BoardHandler{boards: boards}
}

func (h *BoardHandler) Create(c *gin.Context) {
	ownerID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req CreateBoardRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Title) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	board := models.Board{OwnerID: ownerID, Title: strings.TrimSpace(req.Title), Description: cleanOptionalString(req.Description)}
	if err := h.boards.Create(&board); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusCreated, board.Response())
}

func (h *BoardHandler) List(c *gin.Context) {
	ownerID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	boards, err := h.boards.ListByOwner(ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	responses := make([]models.BoardResponse, 0, len(boards))
	for _, board := range boards {
		responses = append(responses, board.Response())
	}
	c.JSON(http.StatusOK, responses)
}

func (h *BoardHandler) Get(c *gin.Context) {
	ownerID, boardID, ok := ownerAndBoardID(c)
	if !ok {
		return
	}
	board, err := h.boards.GetByOwner(ownerID, boardID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "board not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, board.Response())
}

func (h *BoardHandler) Update(c *gin.Context) {
	ownerID, boardID, ok := ownerAndBoardID(c)
	if !ok {
		return
	}
	var req UpdateBoardRequest
	if c.ShouldBindJSON(&req) != nil || (req.Title == nil && req.Description == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	board, err := h.boards.GetByOwner(ownerID, boardID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "board not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		board.Title = title
	}
	if req.Description != nil {
		board.Description = cleanOptionalString(req.Description)
	}
	board.UpdatedAt = time.Now()
	if err := h.boards.Update(board); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, board.Response())
}

func (h *BoardHandler) Delete(c *gin.Context) {
	ownerID, boardID, ok := ownerAndBoardID(c)
	if !ok {
		return
	}
	if err := h.boards.SoftDelete(ownerID, boardID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "board not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func ownerAndBoardID(c *gin.Context) (uint, uint, bool) {
	ownerID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, 0, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 0)
	if err != nil || id == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "board not found"})
		return 0, 0, false
	}
	return ownerID, uint(id), true
}

func currentUserID(c *gin.Context) (uint, bool) {
	userID, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	id, ok := userID.(uint)
	return id, ok
}

func cleanOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	if cleaned == "" {
		return nil
	}
	return &cleaned
}
