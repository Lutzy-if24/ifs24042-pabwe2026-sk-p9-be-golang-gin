// Command server menjalankan Delcom Blog REST API.
package main

import (
	"log"
	"net/http"
	"time"

	"app/internal/config"
	"app/internal/database"
	"app/internal/server"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("gagal terhubung ke database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("gagal migrasi database: %v", err)
	}
	if err := database.Seed(db, cfg); err != nil {
		log.Fatalf("gagal seeding database: %v", err)
	}

	router := server.NewRouter(cfg, db, &http.Client{Timeout: 90 * time.Second})
	log.Printf("Delcom Blog API berjalan di %s (Swagger UI: /swagger/index.html)", cfg.Addr())
	log.Fatal(router.Run(cfg.Addr()))
}
