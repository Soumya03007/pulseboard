package models

import "time"

type User struct {
	ID            uint       `json:"id"`
	Email         string     `json:"email"`
	PasswordHash  string     `json:"-"`
	DisplayName   string     `json:"display_name"`
	StatusMessage string     `json:"status_message"`
	LastActiveAt  *time.Time `json:"last_active_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type UserProfile struct {
	ID            uint       `json:"id"`
	Email         string     `json:"email"`
	DisplayName   string     `json:"display_name"`
	StatusMessage string     `json:"status_message"`
	LastActiveAt  *time.Time `json:"last_active_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (u User) Profile() UserProfile {
	return UserProfile{
		ID:            u.ID,
		Email:         u.Email,
		DisplayName:   u.DisplayName,
		StatusMessage: u.StatusMessage,
		LastActiveAt:  u.LastActiveAt,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}
