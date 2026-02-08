// Package handler provides HTTP handlers for the application.
package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/line/line-bot-sdk-go/v8/linebot/webhook"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/service"
)

// WebhookHandler handles LINE webhook events.
type WebhookHandler struct {
	lineService      *service.LineService
	schedulerService *service.SchedulerService
	logger           *slog.Logger
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(
	lineService *service.LineService,
	schedulerService *service.SchedulerService,
	logger *slog.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		lineService:      lineService,
		schedulerService: schedulerService,
		logger:           logger,
	}
}

// HandleLineWebhook handles LINE webhook requests.
func (h *WebhookHandler) HandleLineWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read request body", slog.String("error", err.Error()))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("X-Line-Signature")
	if !h.validateSignature(body, signature) {
		h.logger.Warn("invalid signature")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var callbackRequest webhook.CallbackRequest
	if err := json.Unmarshal(body, &callbackRequest); err != nil {
		h.logger.Error("failed to unmarshal request", slog.String("error", err.Error()))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	for _, event := range callbackRequest.Events {
		h.handleEvent(event)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) validateSignature(body []byte, signature string) bool {
	decoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	hash := hmac.New(sha256.New, []byte(h.lineService.GetChannelSecret()))
	hash.Write(body)

	return hmac.Equal(decoded, hash.Sum(nil))
}

func (h *WebhookHandler) handleEvent(event webhook.EventInterface) {
	switch e := event.(type) {
	case webhook.MessageEvent:
		h.handleMessageEvent(e)
	default:
		h.logger.Debug("unhandled event type")
	}
}

func (h *WebhookHandler) handleMessageEvent(event webhook.MessageEvent) {
	source := event.Source
	var userID string

	switch s := source.(type) {
	case webhook.UserSource:
		userID = s.UserId
	default:
		h.logger.Debug("non-user source, ignoring")
		return
	}

	if !h.lineService.IsAllowedUser(userID) {
		h.logger.Info("message from non-allowed user", slog.String("userID", userID))
		return
	}

	var messageText string
	switch msg := event.Message.(type) {
	case webhook.TextMessageContent:
		messageText = msg.Text
	default:
		h.logger.Debug("non-text message, ignoring")
		return
	}

	h.logger.Info("processing message",
		slog.String("userID", userID),
		slog.String("message", messageText),
	)

	go h.processMessageAsync(messageText, event.ReplyToken)
}

func (h *WebhookHandler) processMessageAsync(message string, replyToken string) {
	ctx := context.Background()

	result, err := h.schedulerService.ProcessMessage(ctx, message)
	if err != nil {
		h.logger.Error("failed to process message", slog.String("error", err.Error()))
		if err := h.lineService.ReplyMessage(replyToken, "申し訳ありません、エラーが発生しました。"); err != nil {
			h.logger.Error("failed to reply error message", slog.String("error", err.Error()))
		}
		return
	}

	if err := h.lineService.ReplyMessage(replyToken, result.ReplyMessage); err != nil {
		h.logger.Error("failed to reply message", slog.String("error", err.Error()))
	}
}
