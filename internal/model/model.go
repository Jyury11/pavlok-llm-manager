package model

import (
	"database/sql"
	"time"
)

type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Schedule struct {
	ID          int64
	TaskName    string
	Deadline    time.Time
	Priority    Priority
	Status      Status
	CompletedAt sql.NullTime
	CreatedAt   time.Time
}

type PunishmentLog struct {
	ID         int64
	ScheduleID int64
	ShockLevel int
	Reason     string
	ExecutedAt time.Time
}

type DailyStat struct {
	ID              int64
	Date            time.Time
	TotalTasks      int
	CompletedOnTime int
	TotalShocks     int
}

// Gemini API用の構造体

type MessageIntent string

const (
	IntentScheduleRegister MessageIntent = "schedule_register"
	IntentProgressReport   MessageIntent = "progress_report"
	IntentStatusCheck      MessageIntent = "status_check"
	IntentReviewResponse   MessageIntent = "review_response"
	IntentOther            MessageIntent = "other"
)

type ParsedMessage struct {
	Intent    MessageIntent
	Tasks     []TaskInfo
	TaskName  string // 進捗報告の場合のタスク名
	RawText   string
}

type TaskInfo struct {
	Name     string
	Deadline time.Time
	Priority Priority
}

type PunishmentDecision struct {
	ShouldPunish  bool
	Reason        string
	MessageToUser string
}

type JudgmentContext struct {
	Task        *Schedule
	UserMessage string
	History     []string
}

// ReviewDecision represents the decision made after analyzing user's daily review response.
type ReviewDecision struct {
	HadWastedTime    bool
	WastedMinutes    int
	WastedActivities []string
	ShouldPunish     bool
	PunishmentReason string
	MessageToUser    string
}
