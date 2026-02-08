package service

import (
	"fmt"
	"log/slog"

	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/config"
)

type LineService struct {
	bot    *messaging_api.MessagingApiAPI
	cfg    *config.Config
	logger *slog.Logger
}

func NewLineService(cfg *config.Config, logger *slog.Logger) (*LineService, error) {
	bot, err := messaging_api.NewMessagingApiAPI(cfg.LineChannelAccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create line bot: %w", err)
	}

	return &LineService{
		bot:    bot,
		cfg:    cfg,
		logger: logger,
	}, nil
}

func (s *LineService) ReplyMessage(replyToken string, message string) error {
	_, err := s.bot.ReplyMessage(&messaging_api.ReplyMessageRequest{
		ReplyToken: replyToken,
		Messages: []messaging_api.MessageInterface{
			&messaging_api.TextMessage{
				Text: message,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to reply message: %w", err)
	}

	s.logger.Info("replied message", slog.String("message", message))
	return nil
}

func (s *LineService) PushMessage(userID string, message string) error {
	_, err := s.bot.PushMessage(&messaging_api.PushMessageRequest{
		To: userID,
		Messages: []messaging_api.MessageInterface{
			&messaging_api.TextMessage{
				Text: message,
			},
		},
	}, "")
	if err != nil {
		return fmt.Errorf("failed to push message: %w", err)
	}

	s.logger.Info("pushed message", slog.String("userID", userID), slog.String("message", message))
	return nil
}

func (s *LineService) IsAllowedUser(userID string) bool {
	return userID == s.cfg.AllowedLineUserID
}

func (s *LineService) GetChannelSecret() string {
	return s.cfg.LineChannelSecret
}
