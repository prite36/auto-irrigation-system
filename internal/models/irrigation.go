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
