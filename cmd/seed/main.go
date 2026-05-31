package main

import (
	"log"

	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	if err := db.Bootstrap(); err != nil {
		log.Fatalf("Failed to bootstrap database: %v", err)
	}

	if err := db.Seed(); err != nil {
		log.Fatalf("Failed to seed database: %v", err)
	}
}
