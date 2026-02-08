package repository

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ryuju-aono/pavlok-llm-manager/internal/model"
)

const (
	schedulesCollection      = "schedules"
	punishmentLogsCollection = "punishment_logs"
	dailyStatsCollection     = "daily_stats"
	counterCollection        = "counters"
)

// FirestoreRepository implements Repository interface using Firestore.
type FirestoreRepository struct {
	client *firestore.Client
	ctx    context.Context
}

// NewFirestoreRepository creates a new Firestore repository.
func NewFirestoreRepository(ctx context.Context, projectID string) (*FirestoreRepository, error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %w", err)
	}

	return &FirestoreRepository{
		client: client,
		ctx:    ctx,
	}, nil
}

// Close closes the Firestore client.
func (r *FirestoreRepository) Close() error {
	return r.client.Close()
}

// Schedule document structure for Firestore
type scheduleDoc struct {
	ID          int64     `firestore:"id"`
	TaskName    string    `firestore:"task_name"`
	Deadline    time.Time `firestore:"deadline"`
	Priority    string    `firestore:"priority"`
	Status      string    `firestore:"status"`
	CompletedAt *time.Time `firestore:"completed_at,omitempty"`
	CreatedAt   time.Time `firestore:"created_at"`
}

func (r *FirestoreRepository) CreateSchedule(s *model.Schedule) error {
	id, err := r.getNextID(schedulesCollection)
	if err != nil {
		return err
	}
	s.ID = id

	doc := scheduleDoc{
		ID:        s.ID,
		TaskName:  s.TaskName,
		Deadline:  s.Deadline,
		Priority:  string(s.Priority),
		Status:    string(s.Status),
		CreatedAt: s.CreatedAt,
	}

	docRef := r.client.Collection(schedulesCollection).Doc(fmt.Sprintf("%d", s.ID))
	_, err = docRef.Set(r.ctx, doc)
	return err
}

func (r *FirestoreRepository) GetTodaySchedules() ([]*model.Schedule, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	iter := r.client.Collection(schedulesCollection).
		Where("created_at", ">=", startOfDay).
		Where("created_at", "<", endOfDay).
		OrderBy("deadline", firestore.Asc).
		Documents(r.ctx)

	return r.iterateSchedules(iter)
}

func (r *FirestoreRepository) GetPendingSchedules() ([]*model.Schedule, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	iter := r.client.Collection(schedulesCollection).
		Where("status", "==", string(model.StatusPending)).
		Where("created_at", ">=", startOfDay).
		Where("created_at", "<", endOfDay).
		OrderBy("deadline", firestore.Asc).
		Documents(r.ctx)

	return r.iterateSchedules(iter)
}

func (r *FirestoreRepository) GetScheduleByID(id int64) (*model.Schedule, error) {
	docRef := r.client.Collection(schedulesCollection).Doc(fmt.Sprintf("%d", id))
	doc, err := docRef.Get(r.ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("schedule not found: %d", id)
		}
		return nil, err
	}

	var schedDoc scheduleDoc
	if err := doc.DataTo(&schedDoc); err != nil {
		return nil, err
	}

	return r.docToSchedule(&schedDoc), nil
}

func (r *FirestoreRepository) UpdateScheduleStatus(id int64, status model.Status, completedAt *time.Time) error {
	docRef := r.client.Collection(schedulesCollection).Doc(fmt.Sprintf("%d", id))

	updates := []firestore.Update{
		{Path: "status", Value: string(status)},
	}
	if completedAt != nil {
		updates = append(updates, firestore.Update{Path: "completed_at", Value: completedAt})
	}

	_, err := docRef.Update(r.ctx, updates)
	return err
}

func (r *FirestoreRepository) iterateSchedules(iter *firestore.DocumentIterator) ([]*model.Schedule, error) {
	defer iter.Stop()

	var schedules []*model.Schedule
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var schedDoc scheduleDoc
		if err := doc.DataTo(&schedDoc); err != nil {
			return nil, err
		}

		schedules = append(schedules, r.docToSchedule(&schedDoc))
	}

	return schedules, nil
}

func (r *FirestoreRepository) docToSchedule(doc *scheduleDoc) *model.Schedule {
	s := &model.Schedule{
		ID:        doc.ID,
		TaskName:  doc.TaskName,
		Deadline:  doc.Deadline,
		Priority:  model.Priority(doc.Priority),
		Status:    model.Status(doc.Status),
		CreatedAt: doc.CreatedAt,
	}
	if doc.CompletedAt != nil {
		s.CompletedAt.Valid = true
		s.CompletedAt.Time = *doc.CompletedAt
	}
	return s
}

// PunishmentLog document structure
type punishmentLogDoc struct {
	ID         int64     `firestore:"id"`
	ScheduleID int64     `firestore:"schedule_id"`
	ShockLevel int       `firestore:"shock_level"`
	Reason     string    `firestore:"reason"`
	ExecutedAt time.Time `firestore:"executed_at"`
}

func (r *FirestoreRepository) CreatePunishmentLog(log *model.PunishmentLog) error {
	id, err := r.getNextID(punishmentLogsCollection)
	if err != nil {
		return err
	}
	log.ID = id

	doc := punishmentLogDoc{
		ID:         log.ID,
		ScheduleID: log.ScheduleID,
		ShockLevel: log.ShockLevel,
		Reason:     log.Reason,
		ExecutedAt: log.ExecutedAt,
	}

	docRef := r.client.Collection(punishmentLogsCollection).Doc(fmt.Sprintf("%d", log.ID))
	_, err = docRef.Set(r.ctx, doc)
	return err
}

func (r *FirestoreRepository) GetTodayShockCount() (int, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	iter := r.client.Collection(punishmentLogsCollection).
		Where("executed_at", ">=", startOfDay).
		Documents(r.ctx)
	defer iter.Stop()

	count := 0
	for {
		_, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}
		count++
	}

	return count, nil
}

func (r *FirestoreRepository) GetHourlyShockCount() (int, error) {
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	iter := r.client.Collection(punishmentLogsCollection).
		Where("executed_at", ">=", oneHourAgo).
		Documents(r.ctx)
	defer iter.Stop()

	count := 0
	for {
		_, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}
		count++
	}

	return count, nil
}

// DailyStat document structure
type dailyStatDoc struct {
	Date            time.Time `firestore:"date"`
	TotalTasks      int       `firestore:"total_tasks"`
	CompletedOnTime int       `firestore:"completed_on_time"`
	TotalShocks     int       `firestore:"total_shocks"`
}

func (r *FirestoreRepository) GetRecentStats(days int) ([]*model.DailyStat, error) {
	startDate := time.Now().AddDate(0, 0, -days)

	iter := r.client.Collection(dailyStatsCollection).
		Where("date", ">=", startDate).
		OrderBy("date", firestore.Desc).
		Documents(r.ctx)
	defer iter.Stop()

	var stats []*model.DailyStat
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var statDoc dailyStatDoc
		if err := doc.DataTo(&statDoc); err != nil {
			return nil, err
		}

		stats = append(stats, &model.DailyStat{
			Date:            statDoc.Date,
			TotalTasks:      statDoc.TotalTasks,
			CompletedOnTime: statDoc.CompletedOnTime,
			TotalShocks:     statDoc.TotalShocks,
		})
	}

	return stats, nil
}

func (r *FirestoreRepository) UpsertDailyStat(stat *model.DailyStat) error {
	dateKey := stat.Date.Format("2006-01-02")
	docRef := r.client.Collection(dailyStatsCollection).Doc(dateKey)

	doc := dailyStatDoc{
		Date:            stat.Date,
		TotalTasks:      stat.TotalTasks,
		CompletedOnTime: stat.CompletedOnTime,
		TotalShocks:     stat.TotalShocks,
	}

	_, err := docRef.Set(r.ctx, doc)
	return err
}

func (r *FirestoreRepository) GetTodayStat() (*model.DailyStat, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dateKey := today.Format("2006-01-02")

	docRef := r.client.Collection(dailyStatsCollection).Doc(dateKey)
	doc, err := docRef.Get(r.ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return &model.DailyStat{
				Date:            today,
				TotalTasks:      0,
				CompletedOnTime: 0,
				TotalShocks:     0,
			}, nil
		}
		return nil, err
	}

	var statDoc dailyStatDoc
	if err := doc.DataTo(&statDoc); err != nil {
		return nil, err
	}

	return &model.DailyStat{
		Date:            statDoc.Date,
		TotalTasks:      statDoc.TotalTasks,
		CompletedOnTime: statDoc.CompletedOnTime,
		TotalShocks:     statDoc.TotalShocks,
	}, nil
}

// getNextID generates auto-incrementing IDs using a counter document
func (r *FirestoreRepository) getNextID(collection string) (int64, error) {
	counterRef := r.client.Collection(counterCollection).Doc(collection)

	var newID int64
	err := r.client.RunTransaction(r.ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(counterRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				newID = 1
				return tx.Set(counterRef, map[string]int64{"value": 1})
			}
			return err
		}

		currentValue, err := doc.DataAt("value")
		if err != nil {
			return err
		}

		newID = currentValue.(int64) + 1
		return tx.Update(counterRef, []firestore.Update{
			{Path: "value", Value: newID},
		})
	})

	return newID, err
}
