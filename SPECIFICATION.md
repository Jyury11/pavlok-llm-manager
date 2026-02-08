# Pavlok LLM Manager 仕様書

## 概要

LINE公式アカウントを通じて受信したメッセージをもとに、朝設定したスケジュールの達成状況をGemini APIで判定し、未達成の場合にPavlok APIで電気ショックを送信するシステム。

---

## システム構成図

```text
┌─────────────┐      ┌──────────────┐      ┌─────────────────┐
│   LINE      │      │   Webhook    │      │   Server        │
│   User      │─────▶│   (LINE)     │─────▶│   (Go)          │
└─────────────┘      └──────────────┘      └────────┬────────┘
                                                     │
                     ┌───────────────────────────────┼───────────────────────────────┐
                     │                               │                               │
                     ▼                               ▼                               ▼
              ┌─────────────┐              ┌─────────────────┐              ┌─────────────┐
              │  Gemini API │              │   Database      │              │ Pavlok API  │
              │  (判定/管理) │              │   (SQLite)      │              │ (電気ショック)│
              └─────────────┘              └─────────────────┘              └─────────────┘
```

---

## 機能要件

### 1. LINE Webhook受信機能

| 項目 | 内容 |
| ---- | ---- |
| エンドポイント | `POST /webhook/line` |
| 認証 | LINE署名検証 (X-Line-Signature) |
| フィルタ | 特定UserIDからのメッセージのみ処理 |
| 対応イベント | テキストメッセージ |

#### フィルタリング仕様

- 環境変数 `ALLOWED_LINE_USER_ID` に設定されたUserIDからのメッセージのみ処理
- それ以外は無視（200 OKを返すが処理しない）

---

### 2. スケジュール管理機能

#### 2.1 スケジュール登録

**トリガー**: 「今日のスケジュール」「予定登録」などのメッセージ

**処理フロー**:

1. Gemini APIでメッセージを解析
2. 以下の情報を抽出:
   - タスク名
   - 期限時刻
   - 重要度（高/中/低）
3. DBに保存
4. LINEに確認メッセージを返信

**例**:

```text
User: 「今日は9時までに起床、12時までにジム、18時までに仕事終わり」

System:
  ✅ 以下のスケジュールを登録しました：
  1. 起床 - 9:00まで (重要度: 高)
  2. ジム - 12:00まで (重要度: 中)
  3. 仕事終わり - 18:00まで (重要度: 高)
```

#### 2.2 進捗報告

**トリガー**: 「起きた」「ジム完了」「終わった」などの報告メッセージ

**処理フロー**:

1. Gemini APIでどのタスクの完了報告か判定
2. 現在時刻と期限を比較
3. DBのタスクステータスを更新
4. 結果に応じたメッセージを返信

---

### 3. 罰則判定機能（Gemini API）

#### 3.1 判定ロジック

Gemini APIに以下のコンテキストを渡して判定:

```json
{
  "task": {
    "name": "起床",
    "deadline": "09:00",
    "priority": "high",
    "completed_at": "09:15"
  },
  "user_message": "すみません、寝坊しました",
  "history": ["過去3日間の達成率: 60%"]
}
```

#### 3.2 判定結果

Gemini APIは以下の形式で返答:

```json
{
  "should_punish": true,
  "reason": "15分の遅刻。重要度が高いタスクのため罰則適用",
  "message_to_user": "15分の遅刻ですね。電気ショックを送信します。明日は頑張りましょう！"
}
```

**注意**: ショックレベルは安全性の観点から **固定値50** とし、Gemini APIでは制御しない。

#### 3.3 判定基準（Geminiへのプロンプト指示）

- **遅刻時間**: 5分以内は警告のみ、それ以上は罰則
- **重要度**: 高→厳しく、低→寛容に
- **履歴**: 連続達成中は寛容に、連続未達成は厳しく
- **言い訳**: 正当な理由（体調不良等）は考慮

---

### 4. Pavlok API連携

#### 4.1 電気ショック送信

| 項目 | 内容 |
| ---- | ---- |
| エンドポイント | Pavlok API (OAuth2認証) |
| ショックレベル | **固定値: 50**（安全性のため） |
| タイムアウト | 10秒 |
| リトライ | 最大3回 |

#### 4.2 安全機能

- 1日の最大ショック回数: 10回
- 1時間の最大ショック回数: 3回
- ショックレベル: **50固定**（変更不可）
- 深夜帯（23:00-6:00）は無効化

---

### 5. LINE返信機能

#### 5.1 返信パターン

| シーン | メッセージ例 |
| ------ | ------------ |
| スケジュール登録完了 | ✅ スケジュールを登録しました |
| タスク完了（期限内） | 🎉 素晴らしい！「{タスク名}」を時間内に完了しました |
| タスク完了（期限超過） | ⚡ 「{タスク名}」が{X分}遅れました。電気ショックを送信します |
| リマインダー | ⏰ 「{タスク名}」の期限まであと15分です |
| 今日の状況確認 | 📊 今日の進捗: 2/5タスク完了 |

---

## 非機能要件

### セキュリティ

- LINE Webhook署名検証必須
- 環境変数で機密情報管理
- HTTPS必須

### パフォーマンス

- Webhook応答: 3秒以内
- Gemini API呼び出し: goroutineで非同期処理

### 可用性

- エラー時はLINEに通知
- Pavlok API失敗時はログ記録＋再試行

---

## データベース設計（SQLite）

### schedules テーブル

| カラム | 型 | 説明 |
| ------ | -- | ---- |
| id | INTEGER | PRIMARY KEY |
| task_name | TEXT | タスク名 |
| deadline | DATETIME | 期限 |
| priority | TEXT | 重要度 (high/medium/low) |
| status | TEXT | ステータス (pending/completed/failed) |
| completed_at | DATETIME | 完了日時 |
| created_at | DATETIME | 作成日時 |

### punishment_logs テーブル

| カラム | 型 | 説明 |
| ------ | -- | ---- |
| id | INTEGER | PRIMARY KEY |
| schedule_id | INTEGER | 関連スケジュールID |
| shock_level | INTEGER | ショックレベル（常に50） |
| reason | TEXT | 理由 |
| executed_at | DATETIME | 実行日時 |

### daily_stats テーブル

| カラム | 型 | 説明 |
| ------ | -- | ---- |
| id | INTEGER | PRIMARY KEY |
| date | DATE | 日付 |
| total_tasks | INTEGER | 総タスク数 |
| completed_on_time | INTEGER | 期限内完了数 |
| total_shocks | INTEGER | ショック回数 |

---

## 環境変数

```env
# LINE
LINE_CHANNEL_SECRET=xxx
LINE_CHANNEL_ACCESS_TOKEN=xxx
ALLOWED_LINE_USER_ID=Uxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# Gemini
GEMINI_API_KEY=xxx

# Pavlok
PAVLOK_ACCESS_TOKEN=xxx

# Server
PORT=8080

# Safety
MAX_DAILY_SHOCKS=10
MAX_HOURLY_SHOCKS=3
SHOCK_LEVEL=50
QUIET_HOURS_START=23
QUIET_HOURS_END=6
```

---

## 技術スタック

| 項目 | 技術 |
| ---- | ---- |
| 言語 | Go 1.22+ |
| HTTPルーター | net/http (標準ライブラリ) |
| データベース | SQLite (modernc.org/sqlite) |
| LINE SDK | github.com/line/line-bot-sdk-go/v8 |
| Gemini | github.com/google/generative-ai-go |
| 設定管理 | github.com/caarlos0/env/v10 |
| ログ | log/slog (標準ライブラリ) |

---

## API エンドポイント

| メソッド | パス | 説明 |
| -------- | ---- | ---- |
| POST | /webhook/line | LINE Webhook受信 |
| GET | /health | ヘルスチェック |
| GET | /api/schedules/today | 今日のスケジュール取得 |
| GET | /api/stats | 統計情報取得 |

---

## ディレクトリ構成

```text
pavlok-llm-manager/
├── cmd/
│   └── server/
│       └── main.go           # エントリーポイント
├── internal/
│   ├── config/
│   │   └── config.go         # 環境変数設定
│   ├── handler/
│   │   ├── webhook.go        # LINE Webhookハンドラ
│   │   └── api.go            # APIハンドラ
│   ├── service/
│   │   ├── line.go           # LINE連携
│   │   ├── gemini.go         # Gemini API連携
│   │   ├── pavlok.go         # Pavlok API連携
│   │   └── scheduler.go      # スケジュール管理
│   ├── repository/
│   │   └── sqlite.go         # DBアクセス
│   ├── model/
│   │   └── model.go          # データモデル
│   └── safety/
│       └── safety.go         # 安全機能
├── go.mod
├── go.sum
├── .env.example
└── README.md
```

---

## 処理フロー詳細

### メッセージ受信→罰則判定フロー

```text
1. LINE Webhookでメッセージ受信
   ↓
2. 署名検証 & UserIDフィルタ
   ↓
3. Gemini APIでメッセージ意図を解析
   ├─ スケジュール登録 → DBに保存 → 確認返信
   ├─ 進捗報告 → 該当タスク特定 → 完了処理
   └─ その他 → 適切な返答生成
   ↓
4. 進捗報告の場合、期限チェック
   ↓
5. 期限超過の場合、Gemini APIで罰則判定（should_punishのみ）
   ↓
6. 罰則適用の場合
   ├─ 安全チェック（回数制限、深夜制限）
   ├─ Pavlok API呼び出し（レベル50固定）
   └─ ログ記録
   ↓
7. LINEに結果を返信
```

---

## 今後の拡張案

1. **リマインダー機能**: 期限15分前に自動通知
2. **週次レポート**: 達成率のサマリーを送信
3. **目標設定**: 長期目標とマイルストーン管理
4. **ご褒美機能**: 連続達成時にポジティブフィードバック
5. **Slack連携**: LINE以外のチャネル対応
