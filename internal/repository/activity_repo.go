package repository

import (
	"errors"
	"time"

	"github.com/Soumya03007/pulseboard/internal/models"
	"gorm.io/gorm"
)

type ActivityRepository struct{ db *gorm.DB }

func NewActivityRepository(db *gorm.DB) *ActivityRepository { return &ActivityRepository{db: db} }

func (r *ActivityRepository) CreateForUser(userID uint, title string) (*models.Activity, error) {
	var active models.Activity
	result := r.db.Where("user_id = ? AND status = ?", userID, "active").First(&active)
	if result.Error == nil {
		active.Status = "completed"
		active.CompletedAt = pointerToTime(time.Now())
		active.UpdatedAt = time.Now()
		if err := r.db.Save(&active).Error; err != nil {
			return nil, err
		}
	} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}
	activity := &models.Activity{UserID: userID, Title: title, Status: "active", StartedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := r.db.Create(activity).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.User{}).Where("id = ?", userID).Update("current_activity_id", activity.ID).Error; err != nil {
		return nil, err
	}
	return activity, nil
}

func (r *ActivityRepository) CurrentForUser(userID uint) (*models.Activity, error) {
	var user models.User
	if err := r.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	if user.CurrentActivityID == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var activity models.Activity
	if err := r.db.Where("id = ?", *user.CurrentActivityID).First(&activity).Error; err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *ActivityRepository) ListForUser(userID uint) ([]models.Activity, error) {
	var activities []models.Activity
	if err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&activities).Error; err != nil {
		return nil, err
	}
	return activities, nil
}

func (r *ActivityRepository) CompleteCurrentForUser(userID uint) (*models.Activity, error) {
	activity, err := r.CurrentForUser(userID)
	if err != nil {
		return nil, err
	}
	activity.Status = "completed"
	activity.CompletedAt = pointerToTime(time.Now())
	activity.UpdatedAt = time.Now()
	if err := r.db.Save(activity).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.User{}).Where("id = ?", userID).Update("current_activity_id", nil).Error; err != nil {
		return nil, err
	}
	return activity, nil
}

func pointerToTime(value time.Time) *time.Time {
	return &value
}
