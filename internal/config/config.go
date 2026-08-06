package config

import (
	"fmt"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Settings struct {
	DatabaseURL string
	JWTSecret   string
}

func LoadSettings() (Settings, error) {
	settings := Settings{DatabaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")), JWTSecret: strings.TrimSpace(os.Getenv("JWT_SECRET"))}
	if settings.DatabaseURL == "" {
		return Settings{}, fmt.Errorf("DATABASE_URL is required")
	}
	if settings.JWTSecret == "" {
		return Settings{}, fmt.Errorf("JWT_SECRET is required")
	}
	return settings, nil
}

func OpenDatabase(databaseURL string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
}
