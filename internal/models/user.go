package models

import "time"

type User struct {
	ID                uint       `json:"id"`
	Email             string     `json:"email"`
	PasswordHash      string     `json:"-"`
	DisplayName       string     `json:"display_name"`
	StatusMessage     string     `json:"status_message"`
	Presence          string     `json:"presence"`
	Availability      string     `json:"availability"`
	CurrentActivityID *uint      `json:"-"`
	LastActiveAt      *time.Time `json:"last_active_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type UserProfile struct {
	ID              uint              `json:"id"`
	Email           string            `json:"email"`
	DisplayName     string            `json:"display_name"`
	StatusMessage   string            `json:"status_message"`
	Presence        string            `json:"presence"`
	Availability    string            `json:"availability"`
	CurrentActivity *ActivityResponse `json:"current_activity,omitempty"`
	LastActiveAt    *time.Time        `json:"last_active_at,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

func (u User) Profile(currentActivity *Activity) UserProfile {
	var activity *ActivityResponse
	if currentActivity != nil {
		response := currentActivity.Response()
		activity = &response
	}
	return UserProfile{
		ID:              u.ID,
		Email:           u.Email,
		DisplayName:     u.DisplayName,
		StatusMessage:   u.StatusMessage,
		Presence:        u.Presence,
		Availability:    u.Availability,
		CurrentActivity: activity,
		LastActiveAt:    u.LastActiveAt,
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
	}
}
