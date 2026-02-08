// Package service provides business logic services.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ryuju-aono/pavlok-llm-manager/internal/config"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/model"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/repository"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/safety"
)

// SchedulerService handles schedule management and punishment logic.
type SchedulerService struct {
	cfg           *config.Config
	repo          repository.Repository
	gemini        *GeminiService
	pavlok        *PavlokService
	line          *LineService
	safetyChecker *safety.SafetyChecker
	logger        *slog.Logger
}

// NewSchedulerService creates a new SchedulerService.
func NewSchedulerService(
	cfg *config.Config,
	repo repository.Repository,
	gemini *GeminiService,
	pavlok *PavlokService,
	line *LineService,
	safetyChecker *safety.SafetyChecker,
	logger *slog.Logger,
) *SchedulerService {
	return &SchedulerService{
		cfg:           cfg,
		repo:          repo,
		gemini:        gemini,
		pavlok:        pavlok,
		line:          line,
		safetyChecker: safetyChecker,
		logger:        logger,
	}
}

// ProcessResult represents the result of message processing.
type ProcessResult struct {
	ReplyMessage string
	ShockSent    bool
}

// ProcessMessage processes a user message and returns the result.
func (s *SchedulerService) ProcessMessage(ctx context.Context, userMessage string) (*ProcessResult, error) {
	pendingTasks, err := s.repo.GetPendingSchedules()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending schedules: %w", err)
	}

	parsed, err := s.gemini.ParseMessage(ctx, userMessage, pendingTasks)
	if err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	s.logger.Info("parsed message",
		slog.String("intent", string(parsed.Intent)),
		slog.Int("tasks_count", len(parsed.Tasks)),
	)

	switch parsed.Intent {
	case model.IntentScheduleRegister:
		return s.handleScheduleRegister(parsed)
	case model.IntentProgressReport:
		return s.handleProgressReport(ctx, parsed, userMessage)
	case model.IntentStatusCheck:
		return s.handleStatusCheck()
	case model.IntentReviewResponse:
		return s.ProcessReviewResponse(ctx, userMessage)
	default:
		return s.handleOther(ctx, userMessage)
	}
}

func (s *SchedulerService) handleScheduleRegister(parsed *model.ParsedMessage) (*ProcessResult, error) {
	if len(parsed.Tasks) == 0 {
		return &ProcessResult{
			ReplyMessage: "スケジュールを認識できませんでした。「9時までに起床」のように具体的な時間とタスクを教えてください。",
		}, nil
	}

	var registeredTasks []string
	now := time.Now()

	for _, task := range parsed.Tasks {
		schedule := &model.Schedule{
			TaskName:  task.Name,
			Deadline:  task.Deadline,
			Priority:  task.Priority,
			Status:    model.StatusPending,
			CreatedAt: now,
		}

		if err := s.repo.CreateSchedule(schedule); err != nil {
			s.logger.Error("failed to create schedule", slog.String("error", err.Error()))
			continue
		}

		priorityStr := "中"
		switch task.Priority {
		case model.PriorityHigh:
			priorityStr = "高"
		case model.PriorityLow:
			priorityStr = "低"
		}

		registeredTasks = append(registeredTasks, fmt.Sprintf("・%s - %sまで (重要度: %s)",
			task.Name,
			task.Deadline.Format("15:04"),
			priorityStr,
		))
	}

	message := fmt.Sprintf("✅ 以下のスケジュールを登録しました：\n%s\n\n頑張りましょう！💪",
		strings.Join(registeredTasks, "\n"))

	return &ProcessResult{
		ReplyMessage: message,
	}, nil
}

func (s *SchedulerService) handleProgressReport(ctx context.Context, parsed *model.ParsedMessage, userMessage string) (*ProcessResult, error) {
	pendingTasks, err := s.repo.GetPendingSchedules()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending schedules: %w", err)
	}

	if len(pendingTasks) == 0 {
		return &ProcessResult{
			ReplyMessage: "現在進行中のタスクはありません。新しいスケジュールを登録しますか？",
		}, nil
	}

	var matchedTask *model.Schedule
	taskNameLower := strings.ToLower(parsed.TaskName)

	for _, task := range pendingTasks {
		if strings.Contains(strings.ToLower(task.TaskName), taskNameLower) ||
			strings.Contains(taskNameLower, strings.ToLower(task.TaskName)) {
			matchedTask = task
			break
		}
	}

	if matchedTask == nil && len(pendingTasks) == 1 {
		matchedTask = pendingTasks[0]
	}

	if matchedTask == nil {
		var taskList []string
		for _, t := range pendingTasks {
			taskList = append(taskList, fmt.Sprintf("・%s (%sまで)", t.TaskName, t.Deadline.Format("15:04")))
		}
		return &ProcessResult{
			ReplyMessage: fmt.Sprintf("どのタスクの完了報告ですか？\n%s", strings.Join(taskList, "\n")),
		}, nil
	}

	now := time.Now()
	if err := s.repo.UpdateScheduleStatus(matchedTask.ID, model.StatusCompleted, &now); err != nil {
		return nil, fmt.Errorf("failed to update schedule status: %w", err)
	}

	if now.Before(matchedTask.Deadline) || now.Equal(matchedTask.Deadline) {
		return &ProcessResult{
			ReplyMessage: fmt.Sprintf("🎉 素晴らしい！「%s」を時間内に完了しました！\nこの調子で頑張りましょう！", matchedTask.TaskName),
		}, nil
	}

	delayMinutes := int(now.Sub(matchedTask.Deadline).Minutes())

	judgmentCtx := &model.JudgmentContext{
		Task:        matchedTask,
		UserMessage: userMessage,
		History:     s.getRecentHistory(),
	}

	decision, err := s.gemini.JudgePunishment(ctx, judgmentCtx)
	if err != nil {
		s.logger.Error("failed to judge punishment", slog.String("error", err.Error()))
		return &ProcessResult{
			ReplyMessage: fmt.Sprintf("⚠️ 「%s」が%d分遅れました。次は時間内に完了しましょう！", matchedTask.TaskName, delayMinutes),
		}, nil
	}

	if !decision.ShouldPunish {
		return &ProcessResult{
			ReplyMessage: decision.MessageToUser,
		}, nil
	}

	safetyResult, err := s.safetyChecker.CanExecuteShock()
	if err != nil {
		s.logger.Error("failed to check safety", slog.String("error", err.Error()))
		return &ProcessResult{
			ReplyMessage: fmt.Sprintf("⚠️ 「%s」が%d分遅れました。%s", matchedTask.TaskName, delayMinutes, decision.MessageToUser),
		}, nil
	}

	if !safetyResult.Allowed {
		return &ProcessResult{
			ReplyMessage: fmt.Sprintf("⚠️ 「%s」が%d分遅れましたが、%s\n%s",
				matchedTask.TaskName, delayMinutes, safetyResult.Reason, decision.MessageToUser),
		}, nil
	}

	shockLevel := s.safetyChecker.GetShockLevel()

	if err := s.pavlok.SendShock(ctx, shockLevel); err != nil {
		s.logger.Error("failed to send shock", slog.String("error", err.Error()))
		return &ProcessResult{
			ReplyMessage: fmt.Sprintf("⚠️ 「%s」が%d分遅れました。電気ショックの送信に失敗しました。\n%s",
				matchedTask.TaskName, delayMinutes, decision.MessageToUser),
		}, nil
	}

	punishmentLog := &model.PunishmentLog{
		ScheduleID: matchedTask.ID,
		ShockLevel: shockLevel,
		Reason:     decision.Reason,
		ExecutedAt: time.Now(),
	}
	if err := s.repo.CreatePunishmentLog(punishmentLog); err != nil {
		s.logger.Error("failed to create punishment log", slog.String("error", err.Error()))
	}

	return &ProcessResult{
		ReplyMessage: fmt.Sprintf("⚡ 「%s」が%d分遅れました。電気ショックを送信しました。\n%s",
			matchedTask.TaskName, delayMinutes, decision.MessageToUser),
		ShockSent: true,
	}, nil
}

func (s *SchedulerService) handleStatusCheck() (*ProcessResult, error) {
	schedules, err := s.repo.GetTodaySchedules()
	if err != nil {
		return nil, fmt.Errorf("failed to get today schedules: %w", err)
	}

	if len(schedules) == 0 {
		return &ProcessResult{
			ReplyMessage: "📋 今日はまだスケジュールが登録されていません。\n「9時までに起床」のように予定を教えてください！",
		}, nil
	}

	completed := 0
	var taskList []string

	for _, schedule := range schedules {
		status := "⏳"
		switch schedule.Status {
		case model.StatusCompleted:
			status = "✅"
			completed++
		case model.StatusFailed:
			status = "❌"
		}

		taskList = append(taskList, fmt.Sprintf("%s %s (%sまで)",
			status, schedule.TaskName, schedule.Deadline.Format("15:04")))
	}

	shockCount, _ := s.repo.GetTodayShockCount()

	message := fmt.Sprintf("📊 今日の進捗: %d/%d タスク完了\n\n%s",
		completed, len(schedules), strings.Join(taskList, "\n"))

	if shockCount > 0 {
		message += fmt.Sprintf("\n\n⚡ 本日の電気ショック: %d回", shockCount)
	}

	return &ProcessResult{
		ReplyMessage: message,
	}, nil
}

func (s *SchedulerService) handleOther(ctx context.Context, userMessage string) (*ProcessResult, error) {
	schedules, err := s.repo.GetTodaySchedules()
	if err != nil {
		schedules = []*model.Schedule{}
	}

	response, err := s.gemini.GenerateResponse(ctx, userMessage, schedules)
	if err != nil {
		s.logger.Error("failed to generate response", slog.String("error", err.Error()))
		return &ProcessResult{
			ReplyMessage: "申し訳ありません、メッセージを処理できませんでした。スケジュールの登録や進捗報告をお試しください。",
		}, nil
	}

	return &ProcessResult{
		ReplyMessage: response,
	}, nil
}

func (s *SchedulerService) getRecentHistory() []string {
	stats, err := s.repo.GetRecentStats(3)
	if err != nil || len(stats) == 0 {
		return nil
	}

	var history []string
	totalTasks := 0
	completedOnTime := 0

	for _, stat := range stats {
		totalTasks += stat.TotalTasks
		completedOnTime += stat.CompletedOnTime
	}

	if totalTasks > 0 {
		rate := float64(completedOnTime) / float64(totalTasks) * 100
		history = append(history, fmt.Sprintf("過去3日間の達成率: %.0f%%", rate))
	}

	return history
}

// FindMatchingTask finds a task matching the given name.
func (s *SchedulerService) FindMatchingTask(taskName string) (*model.Schedule, error) {
	pendingTasks, err := s.repo.GetPendingSchedules()
	if err != nil {
		return nil, err
	}

	taskNameLower := strings.ToLower(taskName)
	for _, task := range pendingTasks {
		if strings.Contains(strings.ToLower(task.TaskName), taskNameLower) {
			return task, nil
		}
	}

	return nil, sql.ErrNoRows
}

// SendDailyReview sends daily review message to the user.
func (s *SchedulerService) SendDailyReview(ctx context.Context) error {
	schedules, err := s.repo.GetTodaySchedules()
	if err != nil {
		return fmt.Errorf("failed to get today schedules: %w", err)
	}

	shockCount, err := s.repo.GetTodayShockCount()
	if err != nil {
		s.logger.Warn("failed to get shock count", slog.String("error", err.Error()))
		shockCount = 0
	}

	reviewMessage, err := s.gemini.GenerateDailyReview(ctx, schedules, shockCount)
	if err != nil {
		return fmt.Errorf("failed to generate daily review: %w", err)
	}

	if err := s.line.PushMessage(s.cfg.AllowedLineUserID, reviewMessage); err != nil {
		return fmt.Errorf("failed to send review message: %w", err)
	}

	s.logger.Info("sent daily review message")
	return nil
}

// ProcessReviewResponse processes the user's response to daily review.
func (s *SchedulerService) ProcessReviewResponse(ctx context.Context, userMessage string) (*ProcessResult, error) {
	schedules, err := s.repo.GetTodaySchedules()
	if err != nil {
		schedules = []*model.Schedule{}
	}

	decision, err := s.gemini.ProcessReviewResponse(ctx, userMessage, schedules)
	if err != nil {
		s.logger.Error("failed to process review response", slog.String("error", err.Error()))
		return &ProcessResult{
			ReplyMessage: "振り返りの回答を処理できませんでした。もう一度お試しください。",
		}, nil
	}

	if !decision.ShouldPunish {
		return &ProcessResult{
			ReplyMessage: decision.MessageToUser,
		}, nil
	}

	// Check safety before sending shock
	safetyResult, err := s.safetyChecker.CanExecuteShock()
	if err != nil {
		s.logger.Error("failed to check safety", slog.String("error", err.Error()))
		return &ProcessResult{
			ReplyMessage: decision.MessageToUser,
		}, nil
	}

	if !safetyResult.Allowed {
		return &ProcessResult{
			ReplyMessage: fmt.Sprintf("%s\n\n（⚠️ %s）", decision.MessageToUser, safetyResult.Reason),
		}, nil
	}

	shockLevel := s.safetyChecker.GetShockLevel()

	if err := s.pavlok.SendShock(ctx, shockLevel); err != nil {
		s.logger.Error("failed to send shock", slog.String("error", err.Error()))
		return &ProcessResult{
			ReplyMessage: fmt.Sprintf("%s\n\n（電気ショックの送信に失敗しました）", decision.MessageToUser),
		}, nil
	}

	// Log punishment for review
	punishmentLog := &model.PunishmentLog{
		ScheduleID: 0, // No specific schedule, this is for daily review
		ShockLevel: shockLevel,
		Reason:     fmt.Sprintf("振り返り: %s", decision.PunishmentReason),
		ExecutedAt: time.Now(),
	}
	if err := s.repo.CreatePunishmentLog(punishmentLog); err != nil {
		s.logger.Error("failed to create punishment log", slog.String("error", err.Error()))
	}

	return &ProcessResult{
		ReplyMessage: fmt.Sprintf("⚡ %s", decision.MessageToUser),
		ShockSent:    true,
	}, nil
}
