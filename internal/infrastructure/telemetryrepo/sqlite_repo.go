package telemetryrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	apptelemetry "xiadown/internal/application/telemetry"
	"xiadown/internal/infrastructure/persistence/sqlitedto"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type SQLiteStateRepository struct {
	db           *bun.DB
	newInstallID func() string
}

type telemetryStateRow = sqlitedto.TelemetryStateRow

func NewSQLiteStateRepository(db *bun.DB) *SQLiteStateRepository {
	return &SQLiteStateRepository{
		db:           db,
		newInstallID: uuid.NewString,
	}
}

func (repo *SQLiteStateRepository) Ensure(ctx context.Context) (apptelemetry.State, error) {
	var state apptelemetry.State
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		row, err := repo.ensureRow(ctx, tx)
		if err != nil {
			return err
		}
		state = toState(row)
		return nil
	})
	return state, err
}

func (repo *SQLiteStateRepository) IncrementLaunchCount(ctx context.Context, at time.Time) (apptelemetry.State, error) {
	var state apptelemetry.State
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		row, err := repo.ensureRow(ctx, tx)
		if err != nil {
			return err
		}
		row.LaunchCount++
		if err := repo.recordSessionDay(ctx, tx, at); err != nil {
			return err
		}
		if err := repo.updateSessionDayCounts(ctx, tx, &row, at); err != nil {
			return err
		}
		row.UpdatedAt = at.UTC()
		if err := repo.saveRow(ctx, tx, row); err != nil {
			return err
		}
		state = toState(row)
		return nil
	})
	return state, err
}

func (repo *SQLiteStateRepository) RecordSessionSummary(ctx context.Context, endedAt time.Time, durationSeconds float64) (apptelemetry.State, error) {
	var state apptelemetry.State
	if durationSeconds < 0 {
		durationSeconds = 0
	}
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		row, err := repo.ensureRow(ctx, tx)
		if err != nil {
			return err
		}
		row.CompletedSessionCount++
		row.TotalSessionSeconds += durationSeconds
		row.PreviousSessionSeconds = sql.NullFloat64{Float64: durationSeconds, Valid: true}
		row.UpdatedAt = endedAt.UTC()
		if err := repo.saveRow(ctx, tx, row); err != nil {
			return err
		}
		state = toState(row)
		return nil
	})
	return state, err
}

func (repo *SQLiteStateRepository) MarkFirstLibraryCompleted(ctx context.Context, at time.Time) (apptelemetry.State, bool, error) {
	return repo.markFirstTime(ctx, at, func(row *telemetryStateRow) *sql.NullTime {
		return &row.FirstLibraryCompletedAt
	})
}

func (repo *SQLiteStateRepository) markFirstTime(
	ctx context.Context,
	at time.Time,
	field func(row *telemetryStateRow) *sql.NullTime,
) (apptelemetry.State, bool, error) {
	var (
		state apptelemetry.State
		first bool
	)
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		row, err := repo.ensureRow(ctx, tx)
		if err != nil {
			return err
		}
		target := field(&row)
		if target != nil && !target.Valid {
			target.Valid = true
			target.Time = at.UTC()
			row.UpdatedAt = at.UTC()
			if err := repo.saveRow(ctx, tx, row); err != nil {
				return err
			}
			first = true
		}
		state = toState(row)
		return nil
	})
	return state, first, err
}

func (repo *SQLiteStateRepository) ensureRow(ctx context.Context, tx bun.Tx) (telemetryStateRow, error) {
	row := telemetryStateRow{}
	if err := tx.NewSelect().Model(&row).Where("id = 1").Scan(ctx); err == nil {
		return row, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return telemetryStateRow{}, err
	}

	now := time.Now().UTC()
	row = telemetryStateRow{
		ID:                        1,
		InstallID:                 repo.newInstallID(),
		InstallCreatedAt:          now,
		LaunchCount:               0,
		DistinctDaysUsed:          0,
		DistinctDaysUsedLastMonth: 0,
		CompletedSessionCount:     0,
		TotalSessionSeconds:       0,
		UpdatedAt:                 now,
	}
	if err := repo.saveRow(ctx, tx, row); err != nil {
		return telemetryStateRow{}, err
	}
	return row, nil
}

func (repo *SQLiteStateRepository) saveRow(ctx context.Context, tx bun.Tx, row telemetryStateRow) error {
	_, err := tx.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("install_id = EXCLUDED.install_id").
		Set("install_created_at = EXCLUDED.install_created_at").
		Set("launch_count = EXCLUDED.launch_count").
		Set("distinct_days_used = EXCLUDED.distinct_days_used").
		Set("distinct_days_used_last_month = EXCLUDED.distinct_days_used_last_month").
		Set("completed_session_count = EXCLUDED.completed_session_count").
		Set("total_session_seconds = EXCLUDED.total_session_seconds").
		Set("previous_session_seconds = EXCLUDED.previous_session_seconds").
		Set("first_chat_completed_at = EXCLUDED.first_chat_completed_at").
		Set("first_library_completed_at = EXCLUDED.first_library_completed_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func toState(row telemetryStateRow) apptelemetry.State {
	return apptelemetry.State{
		InstallID:                 row.InstallID,
		InstallCreatedAt:          row.InstallCreatedAt,
		LaunchCount:               row.LaunchCount,
		DistinctDaysUsed:          row.DistinctDaysUsed,
		DistinctDaysUsedLastMonth: row.DistinctDaysUsedLastMonth,
		CompletedSessionCount:     row.CompletedSessionCount,
		TotalSessionSeconds:       row.TotalSessionSeconds,
		PreviousSessionSeconds:    nullFloatPtr(row.PreviousSessionSeconds),
		FirstLibraryCompletedAt:   nullTimePtr(row.FirstLibraryCompletedAt),
	}
}

func (repo *SQLiteStateRepository) recordSessionDay(ctx context.Context, tx bun.Tx, at time.Time) error {
	day := sessionDay(at)
	if day == "" {
		return nil
	}
	_, err := tx.NewRaw(
		"INSERT OR IGNORE INTO telemetry_session_days(day, first_seen_at) VALUES (?, ?)",
		day,
		at.UTC(),
	).Exec(ctx)
	return err
}

func (repo *SQLiteStateRepository) updateSessionDayCounts(ctx context.Context, tx bun.Tx, row *telemetryStateRow, at time.Time) error {
	if row == nil {
		return nil
	}
	var total int
	if err := tx.NewRaw("SELECT COUNT(*) FROM telemetry_session_days").Scan(ctx, &total); err != nil {
		return err
	}
	var lastMonth int
	if err := tx.NewRaw(
		"SELECT COUNT(*) FROM telemetry_session_days WHERE day >= ?",
		sessionDay(at.AddDate(0, -1, 0)),
	).Scan(ctx, &lastMonth); err != nil {
		return err
	}
	row.DistinctDaysUsed = total
	row.DistinctDaysUsedLastMonth = lastMonth
	return nil
}

func sessionDay(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.Format("2006-01-02")
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	timestamp := value.Time
	return &timestamp
}

func nullFloatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	number := value.Float64
	return &number
}
