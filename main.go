package main

import (
	"log"

	"app/internal/config"
	"app/internal/database"
	"app/internal/server"
)

func main() {
	cfg := config.LoadConfig()

	db, err := database.ConnectDatabase(cfg)
	if err != nil {
		log.Fatalf("gagal terhubung ke database: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("gagal migrasi database: %v", err)
	}
	if err := database.Seed(db, cfg); err != nil {
		log.Fatalf("gagal seeding database: %v", err)
	}

	router := server.NewRouter(db, cfg)

	addr := ":" + cfg.AppPort
	log.Printf("Delcom Blog API berjalan di %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("gagal menjalankan server: %v", err)
	}
}
