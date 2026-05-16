package libraryrepo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

type SQLiteExternalProcessRepository struct{ db *bun.DB }

type externalProcessRow struct {
	bun.BaseModel `bun:"table:library_external_processes"`

	ID             string        `bun:"id,pk"`
	OperationID    string        `bun:"operation_id"`
	Kind           string        `bun:"kind"`
	Tool           string        `bun:"tool"`
	PID            int           `bun:"pid"`
	ProcessGroupID sql.NullInt64 `bun:"process_group_id"`
	CreatedAt      time.Time     `bun:"created_at"`
	UpdatedAt      time.Time     `bun:"updated_at"`
}

func NewSQLiteExternalProcessRepository(db *bun.DB) *SQLiteExternalProcessRepository {
	return &SQLiteExternalProcessRepository{db: db}
}

func (repo *SQLiteExternalProcessRepository) List(ctx context.Context) ([]library.ExternalProcess, error) {
	rows := make([]externalProcessRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Order("created_at ASC").Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	result := make([]library.ExternalProcess, 0, len(rows))
	for _, row := range rows {
		result = append(result, externalProcessFromRow(row))
	}
	return result, nil
}

func (repo *SQLiteExternalProcessRepository) Save(ctx context.Context, item library.ExternalProcess) error {
	row := externalProcessRow{
		ID:             strings.TrimSpace(item.ID),
		OperationID:    strings.TrimSpace(item.OperationID),
		Kind:           strings.TrimSpace(item.Kind),
		Tool:           strings.TrimSpace(item.Tool),
		PID:            item.PID,
		ProcessGroupID: nullProcessID(item.ProcessGroupID),
		CreatedAt:      item.CreatedAt.UTC(),
		UpdatedAt:      item.UpdatedAt.UTC(),
	}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("operation_id = EXCLUDED.operation_id").
		Set("kind = EXCLUDED.kind").
		Set("tool = EXCLUDED.tool").
		Set("pid = EXCLUDED.pid").
		Set("process_group_id = EXCLUDED.process_group_id").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteExternalProcessRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().
		Model((*externalProcessRow)(nil)).
		Where("id = ?", strings.TrimSpace(id)).
		Exec(ctx)
	return err
}

func externalProcessFromRow(row externalProcessRow) library.ExternalProcess {
	return library.ExternalProcess{
		ID:             row.ID,
		OperationID:    row.OperationID,
		Kind:           row.Kind,
		Tool:           row.Tool,
		PID:            row.PID,
		ProcessGroupID: intFromNullInt64(row.ProcessGroupID),
		CreatedAt:      row.CreatedAt.UTC(),
		UpdatedAt:      row.UpdatedAt.UTC(),
	}
}

func nullProcessID(value int) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(value), Valid: true}
}

func intFromNullInt64(value sql.NullInt64) int {
	if !value.Valid {
		return 0
	}
	return int(value.Int64)
}
