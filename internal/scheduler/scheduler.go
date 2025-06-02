package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/prite36/auto-irrigation-system/internal/mqtt"
	"github.com/prite36/auto-irrigation-system/internal/models"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type Scheduler struct {
	cron       *cron.Cron
	db         *gorm.DB
	mqttClient *mqtt.Client
	duration   time.Duration
}

func NewScheduler(db *gorm.DB, mqttClient *mqtt.Client, scheduleTime string, durationMinutes int) (*Scheduler, error) {
	s := &Scheduler{
		cron:       cron.New(cron.WithSeconds()),
		db:         db,
		mqttClient: mqttClient,
		duration:   time.Duration(durationMinutes) * time.Minute,
	}

	// Add the schedule
	_, err := s.cron.AddFunc(scheduleTime, s.runIrrigation)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule time: %w", err)
	}

	return s, nil
}

func (s *Scheduler) Start() {
	s.cron.Start()
	log.Printf("Scheduler started with duration: %v\n", s.duration)
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) runIrrigation() {
	ctx := context.Background()
	now := time.Now()

	// Create a new irrigation record
	record := &models.IrrigationHistory{
		ScheduledAt: now,
		Status:      models.StatusStarted,
		Duration:    int(s.duration.Minutes()),
	}


	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		log.Printf("Error creating irrigation record: %v\n", err)
		return
	}

	// Turn on the sprinkler
	if err := s.mqttClient.PublishSprinklerControl(true); err != nil {
		s.handleError(record, fmt.Errorf("failed to turn on sprinkler: %w", err))
		return
	}

	// Wait for the specified duration
	time.Sleep(s.duration)

	// Turn off the sprinkler
	if err := s.mqttClient.PublishSprinklerControl(false); err != nil {
		s.handleError(record, fmt.Errorf("failed to turn off sprinkler: %w", err))
		return
	}

	// Update the record
	now = time.Now()
	record.EndedAt = &now
	record.Status = models.StatusCompleted
	if err := s.db.WithContext(ctx).Save(record).Error; err != nil {
		log.Printf("Error updating irrigation record: %v\n", err)
	}

	log.Printf("Irrigation completed successfully at %s\n", now.Format(time.RFC3339))
}

func (s *Scheduler) handleError(record *models.IrrigationHistory, err error) {
	record.Status = models.StatusFailed
	record.Notes = err.Error()
	now := time.Now()
	record.EndedAt = &now

	if dbErr := s.db.Save(record).Error; dbErr != nil {
		log.Printf("Error saving failed irrigation record: %v\n", dbErr)
	}

	log.Printf("Irrigation failed: %v\n", err)
}
