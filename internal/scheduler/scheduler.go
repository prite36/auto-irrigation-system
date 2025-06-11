package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/prite36/auto-irrigation-system/internal/config"
	"github.com/prite36/auto-irrigation-system/internal/models"
	"github.com/prite36/auto-irrigation-system/internal/mqtt"
	"gorm.io/gorm"
)

const (
	calibrationTimeout = 30 * time.Second
	publishDelay       = 3 * time.Second
)

// TaskDefinition represents the structure of a task JSON file.
type TaskDefinition struct {
	Payload        string `json:"payload"`
	TimeoutMinutes int    `json:"timeoutMinutes"`
}

// Scheduler manages the scheduling of irrigation tasks.
type Scheduler struct {
	scheduler  *gocron.Scheduler
	cfg        *config.Config
	mqttClient *mqtt.Client
	db         *gorm.DB
}

// NewScheduler creates a new scheduler instance.
func NewScheduler(cfg *config.Config, mqttClient *mqtt.Client, db *gorm.DB) *Scheduler {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		log.Fatalf("Failed to load location: %v", err)
	}
	s := gocron.NewScheduler(loc)
	return &Scheduler{
		scheduler:  s,
		cfg:        cfg,
		mqttClient: mqttClient,
		db:         db,
	}
}

// Start begins the scheduler's job execution.
func (s *Scheduler) Start() {
	times := strings.Split(s.cfg.Schedule.Times, ",")
	for _, scheduleTime := range times {
		trimmedTime := strings.TrimSpace(scheduleTime)
		if trimmedTime == "" {
			continue
		}
		log.Printf("Scheduling job at %s", trimmedTime)
		_, err := s.scheduler.Every(1).Day().At(trimmedTime).Do(s.RunJob)
		if err != nil {
			log.Fatalf("Failed to schedule job at %s: %v", trimmedTime, err)
		}
	}
	s.scheduler.StartAsync()
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler...")
	s.scheduler.Stop()
}

// RunJob is the main function executed by the scheduler.
// It can also be called directly for debugging purposes.
func (s *Scheduler) RunJob() {
	log.Println("Starting scheduled irrigation run...")

	for _, device := range s.cfg.Devices {
		if err := s.processDevice(device); err != nil {
			log.Printf("Error processing device %s: %v. Halting further processing in this run.", device.ID, err)
			break
		}
	}

	log.Println("Scheduled irrigation run finished.")
}

// processDevice handles the full workflow for a single device.
func (s *Scheduler) processDevice(device config.DeviceConfig) error {
	log.Printf("Processing device: %s", device.ID)
	now := time.Now()
	history := &models.IrrigationHistory{
		ScheduledAt: now,
		StartedAt:   &now,
		Status:      models.StatusStarted,
		Notes:       fmt.Sprintf("Processing device: %s", device.ID),
	}
	s.db.Create(history)

	// 1. Calibration Phase
	if err := s.runCalibration(device, history); err != nil {
		return err // Error is already logged and saved in runCalibration
	}

	// 2. Task Execution Phase
	if err := s.runDeviceTasks(device, history); err != nil {
		return err // Error is already logged and saved in runDeviceTasks
	}

	// If all went well
	endedAt := time.Now()
	history.Status = models.StatusCompleted
	history.EndedAt = &endedAt
	history.Notes = "All tasks completed successfully."
	s.db.Save(history)
	log.Printf("Successfully completed all tasks for device %s", device.ID)
	return nil
}

// runCalibration handles the calibration sequence for a device.
func (s *Scheduler) runCalibration(device config.DeviceConfig, history *models.IrrigationHistory) error {
	log.Printf("Starting calibration for device %s...", device.ID)
	s.mqttClient.ResetDeviceStatus(device.ID)

	// Calibrate Sprinkler
	s.mqttClient.Publish(fmt.Sprintf("%s/cmd/sprinkler/home", device.ID), "1")
	if err := s.waitForFlag(device.ID, 5*time.Minute, func(status *models.SprinklerStatus) bool {
		if status == nil {
			return false
		}
		return status.SprinklerCalibComplete
	}); err != nil {
		history.Status = "CALIB_TIMEOUT"
		history.Notes = "Sprinkler calibration timed out."
		s.db.Save(history)
		return fmt.Errorf("sprinkler calibration for device %s timed out: %w", device.ID, err)
	}

	// Calibrate Valve
	s.mqttClient.Publish(fmt.Sprintf("%s/cmd/valve/home", device.ID), "1")
	if err := s.waitForFlag(device.ID, 5*time.Minute, func(status *models.SprinklerStatus) bool {
		if status == nil {
			return false
		}
		return status.ValveCalibComplete
	}); err != nil {
		history.Status = "CALIB_TIMEOUT"
		history.Notes = "Valve calibration timed out."
		s.db.Save(history)
		return fmt.Errorf("valve calibration for device %s timed out: %w", device.ID, err)
	}

	log.Printf("Calibration completed for device %s.", device.ID)
	return nil
}

// runDeviceTasks handles executing all JSON-defined tasks for a device based on TaskIDs.
func (s *Scheduler) runDeviceTasks(device config.DeviceConfig, history *models.IrrigationHistory) error {
	log.Printf("Starting tasks for device %s...", device.ID)

	for _, taskID := range device.TaskIDs {
		// Reset device status for the new task to ensure a clean state.
		s.mqttClient.ResetDeviceStatus(device.ID)

		taskFilePath := fmt.Sprintf("tasks/%s_%s.json", device.ID, taskID)
		log.Printf("Processing task ID '%s' for device '%s' from file: %s", taskID, device.ID, taskFilePath)

		// 1. Read and parse the task JSON file
		taskData, err := os.ReadFile(taskFilePath)
		if err != nil {
			errMsg := fmt.Sprintf("failed to read task file %s", taskFilePath)
			history.Status = "TASK_ERROR"
			history.Notes = errMsg
			s.db.Save(history)
			return fmt.Errorf("%s: %w", errMsg, err)
		}

		var taskDef TaskDefinition
		if err := json.Unmarshal(taskData, &taskDef); err != nil {
			errMsg := fmt.Sprintf("failed to parse task JSON from %s", taskFilePath)
			history.Status = "TASK_ERROR"
			history.Notes = errMsg
			s.db.Save(history)
			return fmt.Errorf("%s: %w", errMsg, err)
		}

		// 2.1 Publish task payload and wait
		topic := fmt.Sprintf("%s/cmd/task/set", device.ID)
		log.Printf("Publishing task payload to %s", topic)
		s.mqttClient.Publish(topic, taskDef.Payload)

		log.Printf("Waiting 3 seconds after publishing task...")
		time.Sleep(3 * time.Second)

		// 2.2 Wait for task completion with timeout
		log.Printf("Waiting for task completion flag with timeout: %d minutes", taskDef.TimeoutMinutes)
		timeout := time.Duration(taskDef.TimeoutMinutes) * time.Minute
		if err := s.waitForFlag(device.ID, timeout, func(status *models.SprinklerStatus) bool {
			if status == nil {
				return false
			}
			return status.TaskAllComplete
		}); err != nil {
			history.Status = "TASK_TIMEOUT"
			history.Notes = fmt.Sprintf("Task '%s' for device '%s' timed out after %d minutes.", taskID, device.ID, taskDef.TimeoutMinutes)
			s.db.Save(history)
			return fmt.Errorf("task '%s' for device '%s' timed out: %w", taskID, device.ID, err)
		}

		log.Printf("Task '%s' completed successfully for device '%s'.", taskID, device.ID)
	}

	log.Printf("All tasks for device %s completed successfully.", device.ID)
	return nil
}

// waitForFlag is a helper function to poll for a status change with a timeout.
func (s *Scheduler) waitForFlag(deviceID string, timeout time.Duration, checkFunc func(status *models.SprinklerStatus) bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for flag for device %s", deviceID)
		case <-ticker.C:
			status := s.mqttClient.GetDeviceStatus(deviceID)
			if status != nil && checkFunc(status) {
				log.Printf("Flag condition met for device %s.", deviceID)
				return nil
			}
			log.Printf("Waiting for flag condition for device %s...", deviceID)
		}
	}
}
