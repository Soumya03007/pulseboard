package repository

import (
	"time"

	"github.com/Soumya03007/pulseboard/internal/models"
	"gorm.io/gorm"
)

type BoardRepository struct{ db *gorm.DB }

func NewBoardRepository(db *gorm.DB) *BoardRepository { return &BoardRepository{db: db} }

func (r *BoardRepository) Create(board *models.Board) error {
	return r.db.Create(board).Error
}

func (r *BoardRepository) ListByOwner(ownerID uint) ([]models.Board, error) {
	var boards []models.Board
	err := r.db.Where("owner_id = ? AND deleted_at IS NULL", ownerID).Order("created_at DESC").Find(&boards).Error
	return boards, err
}

func (r *BoardRepository) GetByOwner(ownerID, boardID uint) (*models.Board, error) {
	var board models.Board
	result := r.db.Where("id = ? AND owner_id = ? AND deleted_at IS NULL", boardID, ownerID).First(&board)
	return &board, result.Error
}

func (r *BoardRepository) Update(board *models.Board) error {
	return r.db.Save(board).Error
}

func (r *BoardRepository) SoftDelete(ownerID, boardID uint) error {
	now := time.Now()
	result := r.db.Model(&models.Board{}).
		Where("id = ? AND owner_id = ? AND deleted_at IS NULL", boardID, ownerID).
		Updates(map[string]interface{}{"deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
