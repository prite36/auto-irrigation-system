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

// DeviceStatus holds the most recent status from a device.
// This data is updated via MQTT messages.
type DeviceStatus struct {
	DeviceID                  string  `json:"deviceId"`
	HealthCheck               bool    `json:"healthCheck"`
	SprinklerPosition         float64 `json:"sprinklerPosition"`
	ValvePosition             float64 `json:"valvePosition"`
	SprinklerCalibComplete    bool    `json:"sprinklerCalibComplete"`
	ValveCalibComplete        bool    `json:"valveCalibComplete"`
		ValveIsAtTarget           bool    `json:"valveIsAtTarget"`
	TaskCurrentIndex          int     `json:"taskCurrentIndex"`
	TaskCurrentCount          int     `json:"taskCurrentCount"`
	TaskAllComplete           bool    `json:"taskAllComplete"`
	TaskArray                 string  `json:"taskArray"` // Storing as raw JSON string
}
