package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found or it could not be loaded")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("failed to connect to Neon: %v", err)
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS playing_with_neon (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			value REAL
		)
	`); err != nil {
		log.Fatalf("failed to create table: %v", err)
	}

	if _, err := conn.Exec(context.Background(), `
		INSERT INTO playing_with_neon (name, value)
		SELECT LEFT(md5(i::TEXT), 10), random()
		FROM generate_series(1, 10) AS s(i)
	`); err != nil {
		log.Fatalf("failed to insert sample data: %v", err)
	}

	rows, err := conn.Query(context.Background(), "SELECT id, name, value FROM playing_with_neon ORDER BY id")
	if err != nil {
		log.Fatalf("failed to query rows: %v", err)
	}
	defer rows.Close()

	fmt.Println("Neon connection successful. Rows:")
	for rows.Next() {
		var id int32
		var name string
		var value float32
		if err := rows.Scan(&id, &name, &value); err != nil {
			log.Fatalf("failed to scan row: %v", err)
		}
		fmt.Printf("%d | %s | %f\n", id, name, value)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("row iteration failed: %v", err)
	}
}
