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
	if !c.cfg.ReviewEnable {
		c.logger.Info("daily review is disabled")
		return
	}

	go c.runReviewScheduler()
	c.logger.Info("cron service started",
		slog.Int("review_hour", c.cfg.ReviewHour),
		slog.Int("review_minute", c.cfg.ReviewMinute),
	)
}

// Stop stops the cron service.
func (c *CronService) Stop() {
	close(c.stopCh)
	c.logger.Info("cron service stopped")
}

func (c *CronService) runReviewScheduler() {
	for {
		now := time.Now()
		nextRun := c.calculateNextReviewTime(now)
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

func (c *CronService) calculateNextReviewTime(now time.Time) time.Time {
	// Calculate today's review time
	reviewTime := time.Date(
		now.Year(), now.Month(), now.Day(),
		c.cfg.ReviewHour, c.cfg.ReviewMinute, 0, 0,
		now.Location(),
	)

	// If today's review time has passed, schedule for tomorrow
	if now.After(reviewTime) {
		reviewTime = reviewTime.Add(24 * time.Hour)
	}

	return reviewTime
}

func (c *CronService) executeDailyReview() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.logger.Info("executing daily review")

	if err := c.scheduler.SendDailyReview(ctx); err != nil {
		c.logger.Error("failed to send daily review", slog.String("error", err.Error()))
	}
}
