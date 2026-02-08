// Package repository provides data access interfaces and implementations.
package repository

import (
	"time"

	"github.com/ryuju-aono/pavlok-llm-manager/internal/model"
)

// Repository defines the interface for data persistence operations.
type Repository interface {
	// Schedule operations
	CreateSchedule(s *model.Schedule) error
	GetTodaySchedules() ([]*model.Schedule, error)
	GetPendingSchedules() ([]*model.Schedule, error)
	GetScheduleByID(id int64) (*model.Schedule, error)
	UpdateScheduleStatus(id int64, status model.Status, completedAt *time.Time) error

	// PunishmentLog operations
	CreatePunishmentLog(log *model.PunishmentLog) error
	GetTodayShockCount() (int, error)
	GetHourlyShockCount() (int, error)

	// DailyStat operations
	GetRecentStats(days int) ([]*model.DailyStat, error)
	UpsertDailyStat(stat *model.DailyStat) error
	GetTodayStat() (*model.DailyStat, error)

	// Lifecycle
	Close() error
}
