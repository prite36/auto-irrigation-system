package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/prite36/auto-irrigation-system/internal/config"
	"github.com/prite36/auto-irrigation-system/internal/models"
	"github.com/prite36/auto-irrigation-system/internal/mqtt"
	"github.com/prite36/auto-irrigation-system/internal/scheduler"
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
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
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

	// Subscribe to topics for all devices
	deviceIDs := []string{"sprinkler_01", "sprinkler_02"} // You can load this from config
	for _, id := range deviceIDs {
		mqttClient.SubscribeToDeviceTopics(id)
	}

	// Initialize Scheduler
	sched, err := scheduler.NewScheduler(db, mqttClient, cfg.Schedule.Time, cfg.Schedule.Duration)
	if err != nil {
		log.Fatalf("Failed to create scheduler: %v", err)
	}

	// Start the scheduler
	sched.Start()
	defer sched.Stop()

	log.Println("Application is running. Press CTRL+C to exit.")

	// Wait for a shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("Application shutting down.")
}
