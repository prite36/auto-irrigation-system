package service

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/prite36/auto-irrigation-system/internal/config"
	"github.com/prite36/auto-irrigation-system/internal/mqtt"
	"github.com/prite36/auto-irrigation-system/internal/models"
	"github.com/prite36/auto-irrigation-system/internal/scheduler"
)

type App struct {
	cfg        *config.Config
	db         *gorm.DB
	mqttClient *mqtt.Client
	scheduler  *scheduler.Scheduler
}

func NewApp(cfg *config.Config) (*App, error) {
	// Initialize PostgreSQL database
	dsn := cfg.DSN()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&models.IrrigationHistory{}); err != nil {
		return nil, err
	}

	// Initialize MQTT client
	mqttClient, err := mqtt.NewClient(
		cfg.MQTT.Broker,
		cfg.MQTT.ClientID,
		cfg.MQTT.Username,
		cfg.MQTT.Password,
	)
	if err != nil {
		return nil, err
	}

	// Initialize scheduler
	scheduler, err := scheduler.NewScheduler(
		db,
		mqttClient,
		cfg.Schedule.Time,
		cfg.Schedule.Duration,
	)
	if err != nil {
		mqttClient.Close()
		return nil, err
	}

	return &App{
		cfg:        cfg,
		db:         db,
		mqttClient: mqttClient,
		scheduler:  scheduler,
	}, nil
}

func (a *App) Start() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the scheduler
	a.scheduler.Start()

	log.Println("Irrigation system started. Press Ctrl+C to stop.")


	// Wait for interrupt signal
	<-sigChan

	// Cleanup
	a.Stop()
	return nil
}

func (a *App) Stop() {
	log.Println("Shutting down...")

	if a.scheduler != nil {
		a.scheduler.Stop()
	}

	if a.mqttClient != nil {
		a.mqttClient.Close()
	}

	log.Println("Irrigation system stopped")
}
