package models

import (
	"time"

	"gorm.io/gorm"
)

type IrrigationStatus string

const (
	StatusScheduled IrrigationStatus = "scheduled"
	StatusStarted  IrrigationStatus = "started"
	StatusCompleted IrrigationStatus = "completed"
	StatusFailed   IrrigationStatus = "failed"
)

type IrrigationHistory struct {
	gorm.Model
	ScheduledAt time.Time       `gorm:"not null"`
	StartedAt   *time.Time
	EndedAt     *time.Time
	Status      IrrigationStatus `gorm:"type:varchar(20);not null"`
	Duration    int             `gorm:"not null"` // in minutes
	Notes       string
}

func (IrrigationHistory) TableName() string {
	return "irrigation_history"
}

// SprinklerStatus holds the most recent status from a sprinkler device.
// This data is updated via MQTT messages.
type SprinklerStatus struct {
	DeviceID                      string `json:"deviceId"`
	IsAtTargetPosition            bool   `json:"isAtTargetPosition"`
	WaterValveCalibrationComplete bool   `json:"waterValveCalibrationComplete"`
	SprinklerCalibrationComplete  bool   `json:"sprinklerCalibrationComplete"`
	TasksArray                    string `json:"tasksArray"` // Storing as raw JSON string
	AllTasksComplete              bool   `json:"allTasksComplete"`
}
