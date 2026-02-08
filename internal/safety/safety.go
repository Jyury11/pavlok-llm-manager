// Package safety provides safety checks for shock execution.
package safety

import (
	"fmt"
	"time"

	"github.com/ryuju-aono/pavlok-llm-manager/internal/config"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/repository"
)

// SafetyChecker verifies if shock execution is safe.
type SafetyChecker struct {
	cfg  *config.Config
	repo repository.Repository
}

// New creates a new SafetyChecker.
func New(cfg *config.Config, repo repository.Repository) *SafetyChecker {
	return &SafetyChecker{
		cfg:  cfg,
		repo: repo,
	}
}

// CheckResult represents the result of a safety check.
type CheckResult struct {
	Allowed bool
	Reason  string
}

// CanExecuteShock checks if shock execution is allowed.
func (s *SafetyChecker) CanExecuteShock() (*CheckResult, error) {
	// 深夜帯チェック
	if s.isQuietHours() {
		return &CheckResult{
			Allowed: false,
			Reason:  fmt.Sprintf("深夜帯（%d:00-%d:00）のため電気ショックは無効化されています", s.cfg.QuietHoursStart, s.cfg.QuietHoursEnd),
		}, nil
	}

	// 1日の最大回数チェック
	dailyCount, err := s.repo.GetTodayShockCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get daily shock count: %w", err)
	}
	if dailyCount >= s.cfg.MaxDailyShocks {
		return &CheckResult{
			Allowed: false,
			Reason:  fmt.Sprintf("本日の電気ショック回数が上限（%d回）に達しました", s.cfg.MaxDailyShocks),
		}, nil
	}

	// 1時間の最大回数チェック
	hourlyCount, err := s.repo.GetHourlyShockCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get hourly shock count: %w", err)
	}
	if hourlyCount >= s.cfg.MaxHourlyShocks {
		return &CheckResult{
			Allowed: false,
			Reason:  fmt.Sprintf("直近1時間の電気ショック回数が上限（%d回）に達しました", s.cfg.MaxHourlyShocks),
		}, nil
	}

	return &CheckResult{
		Allowed: true,
		Reason:  "",
	}, nil
}

func (s *SafetyChecker) isQuietHours() bool {
	now := time.Now()
	hour := now.Hour()

	start := s.cfg.QuietHoursStart
	end := s.cfg.QuietHoursEnd

	// 例: 23:00 - 6:00 の場合
	if start > end {
		// 23時以降 または 6時未満
		return hour >= start || hour < end
	}
	// 例: 1:00 - 6:00 の場合
	return hour >= start && hour < end
}

// GetShockLevel returns the fixed shock level.
func (s *SafetyChecker) GetShockLevel() int {
	return s.cfg.ShockLevel // 常に50を返す
}
