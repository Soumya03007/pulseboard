package main

import (
	"github.com/Soumya03007/pulseboard/internal/config"
	"github.com/Soumya03007/pulseboard/internal/migrations"
	"github.com/Soumya03007/pulseboard/internal/routes"
	"github.com/joho/godotenv"
	"log/slog"
	"os"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	settings, err := config.LoadSettings()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	db, err := config.OpenDatabase(settings.DatabaseURL)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	if err := migrations.Apply(db); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	slog.Info("pulseboard server starting", "port", port)
	if err := routes.NewRouter(db, settings.JWTSecret).Run(":" + port); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
