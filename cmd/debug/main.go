package main

import (
	"fmt"
	"log"

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

	// Initialize Scheduler
	scheduler := scheduler.NewScheduler(cfg, mqttClient, db)

	// Run the job directly
	log.Println("Executing RunJob directly...")
	scheduler.RunJob()

	log.Println("Debug run finished.")
}
