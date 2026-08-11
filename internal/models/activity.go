package models

import "time"

type Activity struct {
	ID          uint       `json:"id"`
	UserID      uint       `json:"user_id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ActivityResponse struct {
	ID          uint       `json:"id"`
	UserID      uint       `json:"user_id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (a Activity) Response() ActivityResponse {
	return ActivityResponse{
		ID:          a.ID,
		UserID:      a.UserID,
		Title:       a.Title,
		Status:      a.Status,
		StartedAt:   a.StartedAt,
		CompletedAt: a.CompletedAt,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}
