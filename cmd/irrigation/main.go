package main

import (
	"log"

	"github.com/prite36/auto-irrigation-system/internal/config"
	"github.com/prite36/auto-irrigation-system/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create and start the application
	app, err := service.NewApp(cfg)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	// Start the application
	if err := app.Start(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
