package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/ryuju-aono/pavlok-llm-manager/internal/config"
)

// CronService handles scheduled tasks.
type CronService struct {
	cfg       *config.Config
	scheduler *SchedulerService
	logger    *slog.Logger
	stopCh    chan struct{}
}

// NewCronService creates a new CronService.
func NewCronService(cfg *config.Config, scheduler *SchedulerService, logger *slog.Logger) *CronService {
	return &CronService{
		cfg:       cfg,
		scheduler: scheduler,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

// Start starts the cron service.
func (c *CronService) Start() {
	if c.cfg.MorningPromptEnable {
		go c.runMorningPromptScheduler()
		c.logger.Info("morning prompt scheduler started",
			slog.Int("hour", c.cfg.MorningPromptHour),
			slog.Int("minute", c.cfg.MorningPromptMinute),
		)
	} else {
		c.logger.Info("morning prompt is disabled")
	}

	if c.cfg.ReviewEnable {
		go c.runReviewScheduler()
		c.logger.Info("daily review scheduler started",
			slog.Int("hour", c.cfg.ReviewHour),
			slog.Int("minute", c.cfg.ReviewMinute),
		)
	} else {
		c.logger.Info("daily review is disabled")
	}

	c.logger.Info("cron service started")
}

// Stop stops the cron service.
func (c *CronService) Stop() {
	close(c.stopCh)
	c.logger.Info("cron service stopped")
}

func (c *CronService) runMorningPromptScheduler() {
	for {
		now := time.Now()
		nextRun := c.calculateNextScheduledTime(now, c.cfg.MorningPromptHour, c.cfg.MorningPromptMinute)
		waitDuration := nextRun.Sub(now)

		c.logger.Info("next morning prompt scheduled",
			slog.Time("next_run", nextRun),
			slog.Duration("wait_duration", waitDuration),
		)

		select {
		case <-time.After(waitDuration):
			c.executeMorningPrompt()
		case <-c.stopCh:
			return
		}
	}
}

func (c *CronService) runReviewScheduler() {
	for {
		now := time.Now()
		nextRun := c.calculateNextScheduledTime(now, c.cfg.ReviewHour, c.cfg.ReviewMinute)
		waitDuration := nextRun.Sub(now)

		c.logger.Info("next daily review scheduled",
			slog.Time("next_run", nextRun),
			slog.Duration("wait_duration", waitDuration),
		)

		select {
		case <-time.After(waitDuration):
			c.executeDailyReview()
		case <-c.stopCh:
			return
		}
	}
}

func (c *CronService) calculateNextScheduledTime(now time.Time, hour, minute int) time.Time {
	// Calculate today's scheduled time
	scheduledTime := time.Date(
		now.Year(), now.Month(), now.Day(),
		hour, minute, 0, 0,
		now.Location(),
	)

	// If today's scheduled time has passed, schedule for tomorrow
	if now.After(scheduledTime) {
		scheduledTime = scheduledTime.Add(24 * time.Hour)
	}

	return scheduledTime
}

func (c *CronService) executeMorningPrompt() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.logger.Info("executing morning prompt")

	if err := c.scheduler.SendMorningPrompt(ctx); err != nil {
		c.logger.Error("failed to send morning prompt", slog.String("error", err.Error()))
	}
}

func (c *CronService) executeDailyReview() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.logger.Info("executing daily review")

	if err := c.scheduler.SendDailyReview(ctx); err != nil {
		c.logger.Error("failed to send daily review", slog.String("error", err.Error()))
	}
}
