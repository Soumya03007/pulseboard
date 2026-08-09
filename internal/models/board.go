package models

import "time"

type Board struct {
	ID          uint       `json:"id"`
	OwnerID     uint       `json:"owner_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"-"`
}

type BoardResponse struct {
	ID          uint      `json:"id"`
	OwnerID     uint      `json:"owner_id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (b Board) Response() BoardResponse {
	return BoardResponse{
		ID:          b.ID,
		OwnerID:     b.OwnerID,
		Title:       b.Title,
		Description: b.Description,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}
