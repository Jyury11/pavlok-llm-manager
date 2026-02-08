package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ryuju-aono/pavlok-llm-manager/internal/config"
)

const (
	pavlokAPIBaseURL = "https://app.pavlok.com/api/v1"
	defaultTimeout   = 10 * time.Second
	maxRetries       = 3
)

type PavlokService struct {
	cfg        *config.Config
	httpClient *http.Client
	logger     *slog.Logger
}

func NewPavlokService(cfg *config.Config, logger *slog.Logger) *PavlokService {
	return &PavlokService{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		logger: logger,
	}
}

type ShockRequest struct {
	AccessToken string `json:"access_token"`
	Value       int    `json:"value"`
}

type ShockResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func (s *PavlokService) SendShock(ctx context.Context, level int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		s.logger.Info("attempting to send shock",
			slog.Int("attempt", attempt),
			slog.Int("level", level),
		)

		err := s.doSendShock(ctx, level)
		if err == nil {
			s.logger.Info("shock sent successfully",
				slog.Int("level", level),
			)
			return nil
		}

		lastErr = err
		s.logger.Warn("failed to send shock",
			slog.Int("attempt", attempt),
			slog.String("error", err.Error()),
		)

		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return fmt.Errorf("failed to send shock after %d attempts: %w", maxRetries, lastErr)
}

func (s *PavlokService) doSendShock(ctx context.Context, level int) error {
	url := fmt.Sprintf("%s/stimuli/shock", pavlokAPIBaseURL)

	payload := ShockRequest{
		AccessToken: s.cfg.PavlokAccessToken,
		Value:       level,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result ShockResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("shock request failed: %s", result.Message)
	}

	return nil
}

func (s *PavlokService) SendVibration(ctx context.Context, level int) error {
	url := fmt.Sprintf("%s/stimuli/vibration", pavlokAPIBaseURL)

	payload := ShockRequest{
		AccessToken: s.cfg.PavlokAccessToken,
		Value:       level,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
