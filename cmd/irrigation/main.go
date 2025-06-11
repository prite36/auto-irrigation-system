package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prite36/auto-irrigation-system/internal/config"
	"github.com/prite36/auto-irrigation-system/internal/models"
	"github.com/prite36/auto-irrigation-system/internal/mqtt"
	"github.com/prite36/auto-irrigation-system/internal/scheduler"
	"github.com/prite36/auto-irrigation-system/internal/server"
	"github.com/prite36/auto-irrigation-system/internal/slack"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	log.Println("Starting application...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize Database
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.Port,
		cfg.Database.SSLMode,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate the schema
	log.Println("Auto-migrating database schema...")
	if err := db.AutoMigrate(&models.IrrigationHistory{}); err != nil {
		log.Fatalf("Failed to auto-migrate database schema: %v", err)
	}

	// Initialize MQTT Client
	mqttClient, err := mqtt.NewClient(
		cfg.MQTT.Broker,
		cfg.MQTT.ClientID,
		cfg.MQTT.Username,
		cfg.MQTT.Password,
	)
	if err != nil {
		log.Fatalf("Failed to initialize MQTT client: %v", err)
	}
	defer mqttClient.Close()

	// Subscribe to topics for all configured devices
	log.Println("Subscribing to topics for configured devices...")
	for _, device := range cfg.Devices {
		mqttClient.SubscribeToDeviceTopics(device.ID)
	}

	// Initialize Slack Client
	slackClient := slack.NewClient(cfg.Slack.BotToken, cfg.Slack.ChannelID)

	// Initialize Scheduler
	scheduler := scheduler.NewScheduler(cfg, mqttClient, db, slackClient)

	// Initialize the API server
	srv := server.New(cfg, scheduler)

	// Start services in goroutines
	go func() {
		log.Println("Starting scheduler...")
		scheduler.Start()
	}()
	defer scheduler.Stop()

	go func() {
		log.Println("Starting API server on port 3005...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API server listen: %s\n", err)
		}
	}()

	log.Println("Application is running with both Scheduler and API Server. Press CTRL+C to exit.")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down application...")

	// Shutdown API server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Application exiting.")
}
