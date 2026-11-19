package main

import (
	"context"
	"log"

	_ "github.com/joho/godotenv/autoload"

	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/database"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
)

// The deployment runs this command after migrations. It links historical sales
// to real customer accounts so they appear in the admin Users list and in the
// reservation search. It is idempotent: linked rows are skipped.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	db, err := database.Connect(cfg.DatabaseURL, cfg.Environment)
	if err != nil {
		log.Fatalf("failed to connect to the database: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	salesRepo := repository.NewHistoricalSaleRepository(db)
	service := services.NewHistoricalClientService(userRepo, salesRepo)

	result, err := service.Backfill(context.Background(), 200)
	if err != nil {
		log.Fatalf("historical client backfill failed: %v", err)
	}
	for _, message := range result.Messages {
		log.Printf("historical client backfill warning: %s", message)
	}
	log.Printf(
		"historical client backfill finished: linked=%d skipped=%d errors=%d",
		result.Linked,
		result.Skipped,
		result.Errors,
	)
}
