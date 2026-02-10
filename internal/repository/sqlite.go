package repository

import (
	"database/sql"
	"time"

	"github.com/ryuju-aono/pavlok-llm-manager/internal/model"
	_ "modernc.org/sqlite"
)

// SQLiteRepository implements Repository interface using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository.
func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	repo := &SQLiteRepository{db: db}
	if err := repo.initTables(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *SQLiteRepository) initTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS schedules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_name TEXT NOT NULL,
			deadline DATETIME NOT NULL,
			priority TEXT NOT NULL DEFAULT 'medium',
			status TEXT NOT NULL DEFAULT 'pending',
			completed_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS punishment_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			schedule_id INTEGER,
			shock_level INTEGER NOT NULL,
			reason TEXT NOT NULL,
			executed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (schedule_id) REFERENCES schedules(id)
		)`,
		`CREATE TABLE IF NOT EXISTS daily_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date DATE NOT NULL UNIQUE,
			total_tasks INTEGER NOT NULL DEFAULT 0,
			completed_on_time INTEGER NOT NULL DEFAULT 0,
			total_shocks INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS conversation_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			date DATE NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_created_at ON schedules(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_status ON schedules(status)`,
		`CREATE INDEX IF NOT EXISTS idx_punishment_logs_executed_at ON punishment_logs(executed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_conversation_history_date ON conversation_history(date)`,
	}

	for _, query := range queries {
		if _, err := r.db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the database connection.
func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

// CreateSchedule creates a new schedule.
func (r *SQLiteRepository) CreateSchedule(s *model.Schedule) error {
	result, err := r.db.Exec(
		`INSERT INTO schedules (task_name, deadline, priority, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		s.TaskName, s.Deadline, s.Priority, s.Status, s.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = id
	return nil
}

// GetTodaySchedules returns all schedules created today.
func (r *SQLiteRepository) GetTodaySchedules() ([]*model.Schedule, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	rows, err := r.db.Query(
		`SELECT id, task_name, deadline, priority, status, completed_at, created_at
		 FROM schedules
		 WHERE created_at >= ? AND created_at < ?
		 ORDER BY deadline ASC`,
		startOfDay, endOfDay,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []*model.Schedule
	for rows.Next() {
		s := &model.Schedule{}
		err := rows.Scan(&s.ID, &s.TaskName, &s.Deadline, &s.Priority, &s.Status, &s.CompletedAt, &s.CreatedAt)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, rows.Err()
}

// GetPendingSchedules returns all pending schedules for today.
func (r *SQLiteRepository) GetPendingSchedules() ([]*model.Schedule, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	rows, err := r.db.Query(
		`SELECT id, task_name, deadline, priority, status, completed_at, created_at
		 FROM schedules
		 WHERE status = 'pending' AND created_at >= ? AND created_at < ?
		 ORDER BY deadline ASC`,
		startOfDay, endOfDay,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []*model.Schedule
	for rows.Next() {
		s := &model.Schedule{}
		err := rows.Scan(&s.ID, &s.TaskName, &s.Deadline, &s.Priority, &s.Status, &s.CompletedAt, &s.CreatedAt)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, rows.Err()
}

// GetScheduleByID returns a schedule by its ID.
func (r *SQLiteRepository) GetScheduleByID(id int64) (*model.Schedule, error) {
	s := &model.Schedule{}
	err := r.db.QueryRow(
		`SELECT id, task_name, deadline, priority, status, completed_at, created_at FROM schedules WHERE id = ?`,
		id,
	).Scan(&s.ID, &s.TaskName, &s.Deadline, &s.Priority, &s.Status, &s.CompletedAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// UpdateScheduleStatus updates the status of a schedule.
func (r *SQLiteRepository) UpdateScheduleStatus(id int64, status model.Status, completedAt *time.Time) error {
	var err error
	if completedAt != nil {
		_, err = r.db.Exec(
			`UPDATE schedules SET status = ?, completed_at = ? WHERE id = ?`,
			status, completedAt, id,
		)
	} else {
		_, err = r.db.Exec(
			`UPDATE schedules SET status = ? WHERE id = ?`,
			status, id,
		)
	}
	return err
}

// CreatePunishmentLog creates a new punishment log.
func (r *SQLiteRepository) CreatePunishmentLog(log *model.PunishmentLog) error {
	result, err := r.db.Exec(
		`INSERT INTO punishment_logs (schedule_id, shock_level, reason, executed_at) VALUES (?, ?, ?, ?)`,
		log.ScheduleID, log.ShockLevel, log.Reason, log.ExecutedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	log.ID = id
	return nil
}

// GetTodayShockCount returns the number of shocks executed today.
func (r *SQLiteRepository) GetTodayShockCount() (int, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM punishment_logs WHERE executed_at >= ?`,
		startOfDay,
	).Scan(&count)
	return count, err
}

// GetHourlyShockCount returns the number of shocks in the last hour.
func (r *SQLiteRepository) GetHourlyShockCount() (int, error) {
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM punishment_logs WHERE executed_at >= ?`,
		oneHourAgo,
	).Scan(&count)
	return count, err
}

// GetRecentStats returns daily stats for the specified number of days.
func (r *SQLiteRepository) GetRecentStats(days int) ([]*model.DailyStat, error) {
	startDate := time.Now().AddDate(0, 0, -days)

	rows, err := r.db.Query(
		`SELECT id, date, total_tasks, completed_on_time, total_shocks
		 FROM daily_stats
		 WHERE date >= ?
		 ORDER BY date DESC`,
		startDate,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*model.DailyStat
	for rows.Next() {
		s := &model.DailyStat{}
		err := rows.Scan(&s.ID, &s.Date, &s.TotalTasks, &s.CompletedOnTime, &s.TotalShocks)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// UpsertDailyStat creates or updates a daily stat.
func (r *SQLiteRepository) UpsertDailyStat(stat *model.DailyStat) error {
	_, err := r.db.Exec(
		`INSERT INTO daily_stats (date, total_tasks, completed_on_time, total_shocks)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(date) DO UPDATE SET
		 total_tasks = excluded.total_tasks,
		 completed_on_time = excluded.completed_on_time,
		 total_shocks = excluded.total_shocks`,
		stat.Date, stat.TotalTasks, stat.CompletedOnTime, stat.TotalShocks,
	)
	return err
}

// GetTodayStat returns today's daily stat.
func (r *SQLiteRepository) GetTodayStat() (*model.DailyStat, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	stat := &model.DailyStat{}
	err := r.db.QueryRow(
		`SELECT id, date, total_tasks, completed_on_time, total_shocks FROM daily_stats WHERE date = ?`,
		today,
	).Scan(&stat.ID, &stat.Date, &stat.TotalTasks, &stat.CompletedOnTime, &stat.TotalShocks)

	if err == sql.ErrNoRows {
		return &model.DailyStat{
			Date:            today,
			TotalTasks:      0,
			CompletedOnTime: 0,
			TotalShocks:     0,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return stat, nil
}

// AddConversationMessage adds a message to conversation history.
func (r *SQLiteRepository) AddConversationMessage(msg *model.ConversationMessage) error {
	result, err := r.db.Exec(
		`INSERT INTO conversation_history (role, content, date, created_at) VALUES (?, ?, ?, ?)`,
		msg.Role, msg.Content, msg.Date, msg.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	msg.ID = id
	return nil
}

// GetTodayConversationHistory returns today's conversation history.
func (r *SQLiteRepository) GetTodayConversationHistory() ([]*model.ConversationMessage, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	rows, err := r.db.Query(
		`SELECT id, role, content, date, created_at
		 FROM conversation_history
		 WHERE date = ?
		 ORDER BY created_at ASC`,
		today,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*model.ConversationMessage
	for rows.Next() {
		m := &model.ConversationMessage{}
		err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.Date, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// ClearOldConversationHistory deletes conversation history older than today.
func (r *SQLiteRepository) ClearOldConversationHistory() error {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	_, err := r.db.Exec(`DELETE FROM conversation_history WHERE date < ?`, today)
	return err
}
