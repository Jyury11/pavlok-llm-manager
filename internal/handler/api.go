package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ryuju-aono/pavlok-llm-manager/internal/repository"
)

// APIHandler handles API requests.
type APIHandler struct {
	repo   repository.Repository
	logger *slog.Logger
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(repo repository.Repository, logger *slog.Logger) *APIHandler {
	return &APIHandler{
		repo:   repo,
		logger: logger,
	}
}

// HandleHealth handles health check requests.
func (h *APIHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleGetTodaySchedules returns today's schedules.
func (h *APIHandler) HandleGetTodaySchedules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	schedules, err := h.repo.GetTodaySchedules()
	if err != nil {
		h.logger.Error("failed to get today schedules", slog.String("error", err.Error()))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	type ScheduleResponse struct {
		ID          int64  `json:"id"`
		TaskName    string `json:"task_name"`
		Deadline    string `json:"deadline"`
		Priority    string `json:"priority"`
		Status      string `json:"status"`
		CompletedAt string `json:"completed_at,omitempty"`
		CreatedAt   string `json:"created_at"`
	}

	var response []ScheduleResponse
	for _, s := range schedules {
		item := ScheduleResponse{
			ID:        s.ID,
			TaskName:  s.TaskName,
			Deadline:  s.Deadline.Format("15:04"),
			Priority:  string(s.Priority),
			Status:    string(s.Status),
			CreatedAt: s.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if s.CompletedAt.Valid {
			item.CompletedAt = s.CompletedAt.Time.Format("15:04")
		}
		response = append(response, item)
	}

	if response == nil {
		response = []ScheduleResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetStats returns statistics.
func (h *APIHandler) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	todayStat, err := h.repo.GetTodayStat()
	if err != nil {
		h.logger.Error("failed to get today stat", slog.String("error", err.Error()))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	dailyShocks, _ := h.repo.GetTodayShockCount()
	hourlyShocks, _ := h.repo.GetHourlyShockCount()

	recentStats, err := h.repo.GetRecentStats(7)
	if err != nil {
		h.logger.Error("failed to get recent stats", slog.String("error", err.Error()))
	}

	type DailyStatResponse struct {
		Date            string `json:"date"`
		TotalTasks      int    `json:"total_tasks"`
		CompletedOnTime int    `json:"completed_on_time"`
		TotalShocks     int    `json:"total_shocks"`
	}

	var recentStatsResponse []DailyStatResponse
	for _, s := range recentStats {
		recentStatsResponse = append(recentStatsResponse, DailyStatResponse{
			Date:            s.Date.Format("2006-01-02"),
			TotalTasks:      s.TotalTasks,
			CompletedOnTime: s.CompletedOnTime,
			TotalShocks:     s.TotalShocks,
		})
	}

	if recentStatsResponse == nil {
		recentStatsResponse = []DailyStatResponse{}
	}

	response := map[string]interface{}{
		"today": map[string]interface{}{
			"total_tasks":       todayStat.TotalTasks,
			"completed_on_time": todayStat.CompletedOnTime,
			"total_shocks":      dailyShocks,
			"hourly_shocks":     hourlyShocks,
		},
		"recent_stats": recentStatsResponse,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
