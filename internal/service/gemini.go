package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/config"
	"github.com/ryuju-aono/pavlok-llm-manager/internal/model"
	"google.golang.org/api/option"
)

type GeminiService struct {
	client *genai.Client
	model  *genai.GenerativeModel
	logger *slog.Logger
}

func NewGeminiService(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*GeminiService, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.GeminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	geminiModel := client.GenerativeModel("gemini-1.5-flash")
	geminiModel.SetTemperature(0.3)

	return &GeminiService{
		client: client,
		model:  geminiModel,
		logger: logger,
	}, nil
}

func (s *GeminiService) Close() error {
	return s.client.Close()
}

func (s *GeminiService) ParseMessage(ctx context.Context, message string, pendingTasks []*model.Schedule) (*model.ParsedMessage, error) {
	now := time.Now()

	taskListStr := ""
	if len(pendingTasks) > 0 {
		var tasks []string
		for _, t := range pendingTasks {
			tasks = append(tasks, fmt.Sprintf("- %s (期限: %s)", t.TaskName, t.Deadline.Format("15:04")))
		}
		taskListStr = "現在の未完了タスク:\n" + strings.Join(tasks, "\n")
	}

	prompt := fmt.Sprintf(`あなたはスケジュール管理アシスタントです。
ユーザーのメッセージを解析し、以下のいずれかの意図を判定してください。

1. schedule_register: スケジュールや予定の登録
2. progress_report: タスクの完了報告
3. status_check: 今日の状況確認
4. review_response: 1日の振り返りへの回答（ダラダラした時間の報告、反省、振り返りなど）
5. other: その他

現在時刻: %s
%s

ユーザーメッセージ: %s

以下のJSON形式で回答してください:
{
  "intent": "意図（schedule_register/progress_report/status_check/review_response/other）",
  "tasks": [
    {
      "name": "タスク名",
      "deadline": "HH:MM形式の期限時刻",
      "priority": "high/medium/low"
    }
  ],
  "task_name": "進捗報告の場合の該当タスク名",
  "response_message": "ユーザーへの応答メッセージ"
}

注意:
- schedule_registerの場合、tasksに抽出したタスクをすべて含めてください
- progress_reportの場合、task_nameに該当するタスク名を設定してください（未完了タスクリストから最も近いものを選んでください）
- review_responseの場合: ダラダラした時間、サボった、YouTubeを見た、SNSに時間を使った、などの反省や振り返りの報告
- 期限が明示されていない場合は、常識的な時間を設定してください
- 重要度は文脈から判断してください（起床、仕事などは高、趣味は低など）
- JSONのみを出力し、他のテキストは含めないでください`,
		now.Format("15:04"),
		taskListStr,
		message,
	)

	resp, err := s.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	s.logger.Debug("gemini response", slog.String("response", responseText))

	var result struct {
		Intent          string `json:"intent"`
		Tasks           []struct {
			Name     string `json:"name"`
			Deadline string `json:"deadline"`
			Priority string `json:"priority"`
		} `json:"tasks"`
		TaskName        string `json:"task_name"`
		ResponseMessage string `json:"response_message"`
	}

	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	parsed := &model.ParsedMessage{
		Intent:   model.MessageIntent(result.Intent),
		TaskName: result.TaskName,
		RawText:  message,
	}

	for _, t := range result.Tasks {
		deadline, err := parseDeadline(t.Deadline, now)
		if err != nil {
			s.logger.Warn("failed to parse deadline", slog.String("deadline", t.Deadline), slog.String("error", err.Error()))
			continue
		}

		parsed.Tasks = append(parsed.Tasks, model.TaskInfo{
			Name:     t.Name,
			Deadline: deadline,
			Priority: model.Priority(t.Priority),
		})
	}

	return parsed, nil
}

func (s *GeminiService) JudgePunishment(ctx context.Context, judgmentCtx *model.JudgmentContext) (*model.PunishmentDecision, error) {
	historyStr := ""
	if len(judgmentCtx.History) > 0 {
		historyStr = "履歴情報:\n" + strings.Join(judgmentCtx.History, "\n")
	}

	delayMinutes := int(time.Since(judgmentCtx.Task.Deadline).Minutes())

	prompt := fmt.Sprintf(`あなたはスケジュール管理の罰則判定を行うアシスタントです。
以下の情報をもとに、電気ショックによる罰則を与えるべきか判定してください。

タスク情報:
- タスク名: %s
- 期限: %s
- 重要度: %s
- 完了報告時刻: %s
- 遅延時間: %d分

ユーザーからのメッセージ: %s

%s

判定基準:
- 5分以内の遅刻は警告のみ（罰則なし）
- 重要度が高いタスクは厳しく判定
- 重要度が低いタスクは寛容に判定
- 体調不良など正当な理由がある場合は考慮
- 連続で達成できている場合は寛容に

以下のJSON形式で回答してください:
{
  "should_punish": true/false,
  "reason": "判定理由",
  "message_to_user": "ユーザーへのメッセージ"
}

注意:
- JSONのみを出力し、他のテキストは含めないでください
- message_to_userは日本語で、励ましの言葉を含めてください`,
		judgmentCtx.Task.TaskName,
		judgmentCtx.Task.Deadline.Format("15:04"),
		judgmentCtx.Task.Priority,
		time.Now().Format("15:04"),
		delayMinutes,
		judgmentCtx.UserMessage,
		historyStr,
	)

	resp, err := s.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	s.logger.Debug("gemini punishment response", slog.String("response", responseText))

	var result struct {
		ShouldPunish  bool   `json:"should_punish"`
		Reason        string `json:"reason"`
		MessageToUser string `json:"message_to_user"`
	}

	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	return &model.PunishmentDecision{
		ShouldPunish:  result.ShouldPunish,
		Reason:        result.Reason,
		MessageToUser: result.MessageToUser,
	}, nil
}

func (s *GeminiService) GenerateResponse(ctx context.Context, message string, schedules []*model.Schedule) (string, error) {
	schedulesInfo := ""
	if len(schedules) > 0 {
		var items []string
		completed := 0
		for _, s := range schedules {
			status := "⏳"
			if s.Status == model.StatusCompleted {
				status = "✅"
				completed++
			} else if s.Status == model.StatusFailed {
				status = "❌"
			}
			items = append(items, fmt.Sprintf("%s %s (期限: %s)", status, s.TaskName, s.Deadline.Format("15:04")))
		}
		schedulesInfo = fmt.Sprintf("今日のスケジュール (%d/%d完了):\n%s", completed, len(schedules), strings.Join(items, "\n"))
	}

	prompt := fmt.Sprintf(`あなたはフレンドリーなスケジュール管理アシスタントです。
ユーザーのメッセージに対して、簡潔で励ましになる応答を生成してください。

%s

ユーザーメッセージ: %s

注意:
- 応答は日本語で、2-3文程度に収めてください
- 絵文字を適度に使用してください
- 応答テキストのみを出力してください`,
		schedulesInfo,
		message,
	)

	resp, err := s.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini")
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

// GenerateDailyReview generates a daily review message asking about the day and wasted time.
func (s *GeminiService) GenerateDailyReview(ctx context.Context, schedules []*model.Schedule, shockCount int) (string, error) {
	var completedTasks []string
	var failedTasks []string
	var pendingTasks []string

	for _, schedule := range schedules {
		taskInfo := fmt.Sprintf("%s (期限: %s)", schedule.TaskName, schedule.Deadline.Format("15:04"))
		switch schedule.Status {
		case model.StatusCompleted:
			if schedule.CompletedAt.Valid && schedule.CompletedAt.Time.After(schedule.Deadline) {
				taskInfo += " - 遅れて完了"
			}
			completedTasks = append(completedTasks, taskInfo)
		case model.StatusFailed:
			failedTasks = append(failedTasks, taskInfo)
		case model.StatusPending:
			pendingTasks = append(pendingTasks, taskInfo)
		}
	}

	prompt := fmt.Sprintf(`あなたはフレンドリーなスケジュール管理アシスタントです。
1日の終わりに、ユーザーに今日の振り返りを促すメッセージを生成してください。

今日の結果:
- 完了したタスク: %d件
%s
- 未完了のタスク: %d件
%s
- 失敗したタスク: %d件
%s
- 本日の電気ショック回数: %d回

以下の要素を含むメッセージを生成してください:
1. 今日の成果を簡潔に振り返る（良かった点、改善点）
2. 「今日、ダラダラしてしまった時間はありましたか？」と質問する
3. もしダラダラした時間があれば正直に報告するよう促す
4. 明日に向けての励ましの言葉

注意:
- メッセージは日本語で、親しみやすいトーンで
- 絵文字を適度に使用
- 全体で200-300文字程度に収める
- 応答テキストのみを出力してください`,
		len(completedTasks),
		formatTaskList(completedTasks),
		len(pendingTasks),
		formatTaskList(pendingTasks),
		len(failedTasks),
		formatTaskList(failedTasks),
		shockCount,
	)

	resp, err := s.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini")
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

// ProcessReviewResponse processes user's response to daily review and determines if punishment is needed.
func (s *GeminiService) ProcessReviewResponse(ctx context.Context, userMessage string, schedules []*model.Schedule) (*model.ReviewDecision, error) {
	prompt := fmt.Sprintf(`あなたはスケジュール管理アシスタントです。
ユーザーが1日の振り返りに対して返答しました。内容を分析してください。

ユーザーの返答: %s

以下のJSON形式で回答してください:
{
  "had_wasted_time": true/false,
  "wasted_minutes": 推定のダラダラした時間（分）,
  "wasted_activities": ["ダラダラした活動のリスト"],
  "should_punish": true/false,
  "punishment_reason": "罰を与える場合の理由",
  "message_to_user": "ユーザーへの返答メッセージ"
}

判定基準:
- 30分以上ダラダラしたと報告した場合は罰を検討
- 正直に報告している場合は、罰を軽減または免除
- 反省の態度があれば励ましのメッセージを
- 嘘をついていそうな場合は厳しく判定

注意:
- JSONのみを出力し、他のテキストは含めないでください
- message_to_userは日本語で、励ましや改善提案を含めてください`, userMessage)

	resp, err := s.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	s.logger.Debug("gemini review response", slog.String("response", responseText))

	var result struct {
		HadWastedTime    bool     `json:"had_wasted_time"`
		WastedMinutes    int      `json:"wasted_minutes"`
		WastedActivities []string `json:"wasted_activities"`
		ShouldPunish     bool     `json:"should_punish"`
		PunishmentReason string   `json:"punishment_reason"`
		MessageToUser    string   `json:"message_to_user"`
	}

	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	return &model.ReviewDecision{
		HadWastedTime:    result.HadWastedTime,
		WastedMinutes:    result.WastedMinutes,
		WastedActivities: result.WastedActivities,
		ShouldPunish:     result.ShouldPunish,
		PunishmentReason: result.PunishmentReason,
		MessageToUser:    result.MessageToUser,
	}, nil
}

func formatTaskList(tasks []string) string {
	if len(tasks) == 0 {
		return "  なし"
	}
	var result []string
	for _, t := range tasks {
		result = append(result, "  - "+t)
	}
	return strings.Join(result, "\n")
}

func parseDeadline(timeStr string, now time.Time) (time.Time, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
	}

	var hour, minute int
	if _, err := fmt.Sscanf(timeStr, "%d:%d", &hour, &minute); err != nil {
		return time.Time{}, err
	}

	deadline := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	return deadline, nil
}
