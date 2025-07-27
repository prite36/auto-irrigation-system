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
	"github.com/prite36/auto-irrigation-system/internal/slack"
	slackclient "github.com/slack-go/slack"
	"gorm.io/gorm"
)

// TaskDefinition represents the structure of a task JSON file.
type TaskDefinition struct {
	Payload        json.RawMessage `json:"payload"`
	TimeoutMinutes int             `json:"timeoutMinutes"`
}

// Scheduler manages the scheduling of irrigation tasks.
type Scheduler struct {
	scheduler   *gocron.Scheduler
	cfg         *config.Config
	mqttClient  *mqtt.Client
	db          *gorm.DB
	slackClient *slack.Client
}

// NewScheduler creates a new scheduler instance.
func NewScheduler(cfg *config.Config, mqttClient *mqtt.Client, db *gorm.DB, slackClient *slack.Client) *Scheduler {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		log.Fatalf("Failed to load location: %v", err)
	}

	s := gocron.NewScheduler(loc)
	return &Scheduler{
		scheduler:   s,
		cfg:         cfg,
		mqttClient:  mqttClient,
		db:          db,
		slackClient: slackClient,
	}
}

// Start begins the scheduler's job execution.
func (s *Scheduler) Start() {
	log.Println("Scheduling jobs based on device configurations...")

	for _, device := range s.cfg.Devices {
		for _, scheduleTime := range device.ScheduleTimes {
			trimmedTime := strings.TrimSpace(scheduleTime)
			if trimmedTime == "" {
				continue
			}

			// Capture device for the closure
			deviceToSchedule := device

			log.Printf("Scheduling job for device '%s' at %s", deviceToSchedule.ID, trimmedTime)
			_, err := s.scheduler.Every(1).Day().At(trimmedTime).Do(func() {
				s.runDeviceJob(deviceToSchedule)
			})
			if err != nil {
				log.Fatalf("Failed to schedule job for device '%s' at %s: %v", deviceToSchedule.ID, trimmedTime, err)
			}
		}
	}

	s.scheduler.StartAsync()
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler...")
	s.scheduler.Stop()
}

// RunJobForDevice runs the job for a specific device ID.
func (s *Scheduler) RunJobForDevice(deviceID string) error {
	log.Printf("Starting manual run for device: %s...", deviceID)
	s.notifySlackRich(slack.NewInfoMessage(fmt.Sprintf("🚀 Manual Run Started for %s", deviceID), fmt.Sprintf("Manual run for device %s has commenced.", deviceID)))

	for _, device := range s.cfg.Devices {
		if device.ID == deviceID {
			s.runDeviceJob(device)
			log.Printf("Manual run for device %s finished.", deviceID)
			s.notifySlackRich(slack.NewSuccessMessage(fmt.Sprintf("✅ Manual Run Completed for %s", deviceID), fmt.Sprintf("Finished processing device %s for the manual run.", deviceID)))
			return nil
		}
	}

	log.Printf("Manual run for device %s failed: device not found.", deviceID)
	s.notifySlackRich(slack.NewErrorMessage(fmt.Sprintf("🚨 Manual Run Failed for %s", deviceID), fmt.Sprintf("Device with ID '%s' not found.", deviceID)))
	return fmt.Errorf("device with ID '%s' not found", deviceID)
}

// RunAllJobsOnce is a debug function to run all device jobs immediately.
func (s *Scheduler) RunAllJobsOnce() {
	log.Println("Starting manual run for all devices...")
	s.notifySlackRich(slack.NewInfoMessage("🚀 Manual Run Started", "Manual run for all devices has commenced."))

	for _, device := range s.cfg.Devices {
		s.runDeviceJob(device)
	}

	log.Println("Manual run for all devices finished.")
	s.notifySlackRich(slack.NewSuccessMessage("✅ Manual Run Completed", "Finished processing all devices for the manual run."))
}

// runDeviceJob selects the appropriate processor for a given device and executes it.
func (s *Scheduler) runDeviceJob(device config.DeviceConfig) {
	log.Printf("Starting job for device %s of type %s", device.ID, device.Type)
	var err error
	switch device.Type {
	case "iot_sprinkler":
		err = s.processSprinklerDevice(device)
	case "iot_plant_pot":
		err = s.processPlantPotDevice(device)
	default:
		log.Printf("Warning: Unknown device type '%s' for device '%s'. Skipping.", device.Type, device.ID)
	}

	if err != nil {
		log.Printf("Error processing device %s: %v.", device.ID, err)
		s.notifySlackRich(slack.NewErrorMessage(fmt.Sprintf("🚨 ERROR: Device %s", device.ID), fmt.Sprintf("Error processing device: %v", err)))
	}
}

// processPlantPotDevice handles the logic for a single iot_plant_pot device.
func (s *Scheduler) processPlantPotDevice(device config.DeviceConfig) error {
	log.Printf("Processing plant pot device: %s", device.ID)
	s.notifySlackRich(slack.NewInfoMessage(fmt.Sprintf("🪴 Plant Pot Job Started: %s", device.ID), "Starting health check and watering process."))

	// 1. Check health_check
	status := s.mqttClient.GetDeviceStatus(device.ID)
	if !status.HealthCheck {
		errMsg := fmt.Sprintf("Health check failed for plant pot %s. Aborting job for this device.", device.ID)
		log.Println(errMsg)
		s.notifySlackRich(slack.NewErrorMessage(fmt.Sprintf("🚨 ERROR: Plant Pot %s", device.ID), errMsg))
		return fmt.Errorf("%s", errMsg)
	}

	log.Printf("Health check passed for %s.", device.ID)

	// 2. Publish trigger command
	topic := fmt.Sprintf("%s/cmd/trigger_solenoid_valve", device.ID)
	payload := fmt.Sprintf("%d", device.ScheduleDuration)
	log.Printf("Publishing to %s with payload '%s' for %d seconds", topic, payload, device.ScheduleDuration)
	s.mqttClient.Publish(topic, payload)

	// 3. Send success notification
	successMsg := fmt.Sprintf("Successfully triggered solenoid valve for plant pot %s.", device.ID)
	log.Println(successMsg)
	s.notifySlackRich(slack.NewSuccessMessage(fmt.Sprintf("✅ Plant Pot Job Completed: %s", device.ID), successMsg))

	return nil
}

// processSprinklerDevice handles the full workflow for a single sprinkler device.
func (s *Scheduler) processSprinklerDevice(device config.DeviceConfig) error {
	log.Printf("Processing sprinkler device: %s", device.ID)
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
	log.Printf("Successfully completed all tasks")

	// Send success notification
	successMsg := fmt.Sprintf("Successfully completed all tasks for device %s.", device.ID)
	s.notifySlackRich(slack.NewSuccessMessage(fmt.Sprintf("✅ Sprinkler Job Completed: %s", device.ID), successMsg))

	return nil
}

// runCalibration handles the calibration sequence for a device.
func (s *Scheduler) runCalibration(device config.DeviceConfig, history *models.IrrigationHistory) error {
	log.Printf("Starting calibration check for device %s...", device.ID)

	// Get current device status
	currentStatus := s.mqttClient.GetDeviceStatus(device.ID)

	// --- Calibrate Sprinkler ---
	if currentStatus != nil && currentStatus.SprinklerCalibComplete {
		log.Printf("Sprinkler for device %s is already calibrated. Skipping.", device.ID)
	} else {
		log.Printf("Calibrating sprinkler for device %s...", device.ID)
		s.mqttClient.Publish(fmt.Sprintf("%s/cmd/sprinkler/home", device.ID), "1")
		if err := s.waitForFlag(device.ID, 2*time.Minute, func(status *models.DeviceStatus) bool {
			return status != nil && status.SprinklerCalibComplete
		}); err != nil {
			history.Status = "SPRINKLER_CALIB_TIMEOUT"
			history.Notes = "Sprinkler calibration timed out."
			s.db.Save(history)
			errMsg := fmt.Sprintf("Timeout waiting for sprinkler calibration on device %s", device.ID)
			log.Println(errMsg)
			s.notifySlackRich(slack.NewErrorMessage("🚨 Calibration Timeout", errMsg))
			return fmt.Errorf("sprinkler calibration timed out: %w", err)
		}
		log.Printf("Sprinkler calibration completed for device %s", device.ID)
	}

	// --- Calibrate Water Valve ---
	// Re-fetch status in case it was updated during sprinkler calibration
	currentStatus = s.mqttClient.GetDeviceStatus(device.ID)
	if currentStatus != nil && currentStatus.ValveCalibComplete {
		log.Printf("Water valve for device %s is already calibrated. Skipping.", device.ID)
	} else {
		log.Printf("Calibrating water valve for device %s...", device.ID)
		s.mqttClient.Publish(fmt.Sprintf("%s/cmd/valve/home", device.ID), "1")
		if err := s.waitForFlag(device.ID, 2*time.Minute, func(status *models.DeviceStatus) bool {
			return status != nil && status.ValveCalibComplete
		}); err != nil {
			history.Status = "VALVE_CALIB_TIMEOUT"
			history.Notes = "Water valve calibration timed out."
			s.db.Save(history)
			errMsg := fmt.Sprintf("Timeout waiting for water valve calibration on device %s", device.ID)
			log.Println(errMsg)
			s.notifySlackRich(slack.NewErrorMessage("🚨 Calibration Timeout", errMsg))
			return fmt.Errorf("water valve calibration timed out: %w", err)
		}
		log.Printf("Water valve calibration completed for device %s", device.ID)
	}

	log.Printf("Calibration phase completed for device %s", device.ID)
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
			s.notifySlackRich(slack.NewErrorMessage("🚨 Task Error", errMsg))
			return fmt.Errorf("%s: %w", errMsg, err)
		}

		var taskDef TaskDefinition
		if err := json.Unmarshal(taskData, &taskDef); err != nil {
			errMsg := fmt.Sprintf("failed to parse task JSON from %s", taskFilePath)
			history.Status = "TASK_ERROR"
			history.Notes = errMsg
			s.db.Save(history)
			s.notifySlackRich(slack.NewErrorMessage("🚨 Task Error", errMsg))
			return fmt.Errorf("%s: %w", errMsg, err)
		}

		// 2.1 Publish task payload and wait
		topic := fmt.Sprintf("%s/cmd/task/set", device.ID)
		log.Printf("Publishing task payload to %s", topic)
		s.mqttClient.Publish(topic, string(taskDef.Payload))

		log.Printf("Waiting 3 seconds after publishing task...")
		time.Sleep(3 * time.Second)

		// 2.2 Wait for task completion with timeout
		log.Printf("Waiting for task completion flag with timeout: %d minutes", taskDef.TimeoutMinutes)
		timeout := time.Duration(taskDef.TimeoutMinutes) * time.Minute
		if err := s.waitForFlag(device.ID, timeout, func(status *models.DeviceStatus) bool {
			if status == nil {
				return false
			}
			return status.TaskAllComplete
		}); err != nil {
			history.Status = "TASK_TIMEOUT"
			history.Notes = fmt.Sprintf("Task '%s' for device '%s' timed out after %d minutes.", taskID, device.ID, taskDef.TimeoutMinutes)
			s.db.Save(history)
			errMsg := fmt.Sprintf("Device %s, Task %s: Timeout waiting for completion", device.ID, taskID)
			log.Println(errMsg)
			s.notifySlackRich(slack.NewErrorMessage("🚨 Task Timeout", errMsg))
			return fmt.Errorf("task '%s' timed out: %w", taskID, err)
		}

		log.Printf("Task '%s' completed successfully for device '%s'.", taskID, device.ID)
	}

	log.Printf("All tasks for device %s completed successfully.", device.ID)
	return nil
}

// waitForFlag is a helper function to poll for a status change with a timeout.
func (s *Scheduler) waitForFlag(deviceID string, timeout time.Duration, checkFunc func(status *models.DeviceStatus) bool) error {
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

// notifySlackRich sends a rich message to Slack if the client is configured and not rate limited.
func (s *Scheduler) notifySlackRich(options slackclient.MsgOption) {
	if s.slackClient != nil {
		if !s.slackClient.SendRichMessageSafe(options) {
			log.Println("Slack message skipped due to rate limiting")
		}
	}
}
