package libraryimportrepo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/uptrace/bun"

	importdomain "xiadown/internal/domain/libraryimport"
)

type SQLiteRepository struct{ db *bun.DB }

func NewSQLiteRepository(db *bun.DB) *SQLiteRepository { return &SQLiteRepository{db: db} }

type batchRow struct {
	bun.BaseModel   `bun:"table:library_import_batches"`
	ID              string       `bun:"id,pk"`
	RequestKey      string       `bun:"request_key"`
	LibraryID       string       `bun:"library_id"`
	Mode            string       `bun:"mode"`
	ManagedRoot     string       `bun:"managed_root"`
	HiddenPolicy    string       `bun:"hidden_policy"`
	SymlinkPolicy   string       `bun:"symlink_policy"`
	Status          string       `bun:"status"`
	TotalCount      int          `bun:"total_count"`
	ReadyCount      int          `bun:"ready_count"`
	DuplicateCount  int          `bun:"duplicate_count"`
	SkippedCount    int          `bun:"skipped_count"`
	SucceededCount  int          `bun:"succeeded_count"`
	FailedCount     int          `bun:"failed_count"`
	TotalBytes      int64        `bun:"total_bytes"`
	LastErrorCode   string       `bun:"last_error_code"`
	LastError       string       `bun:"last_error"`
	CancelRequested bool         `bun:"cancel_requested"`
	StartedAt       sql.NullTime `bun:"started_at"`
	FinishedAt      sql.NullTime `bun:"finished_at"`
	CreatedAt       time.Time    `bun:"created_at"`
	UpdatedAt       time.Time    `bun:"updated_at"`
}

type candidateRow struct {
	bun.BaseModel        `bun:"table:library_import_candidates"`
	ID                   string         `bun:"id,pk"`
	BatchID              string         `bun:"batch_id"`
	SourcePath           string         `bun:"source_path"`
	RelativePath         string         `bun:"relative_path"`
	DisplayName          string         `bun:"display_name"`
	Extension            string         `bun:"extension"`
	Category             string         `bun:"category"`
	MIMEType             string         `bun:"mime_type"`
	MediaProbed          bool           `bun:"media_probed"`
	WasSymlink           bool           `bun:"was_symlink"`
	SizeBytes            int64          `bun:"size_bytes"`
	ModifiedAt           sql.NullTime   `bun:"modified_at"`
	HashAlgorithm        string         `bun:"hash_algorithm"`
	ContentHash          string         `bun:"content_hash"`
	Status               string         `bun:"status"`
	DuplicateFileID      sql.NullString `bun:"duplicate_file_id"`
	DuplicateCandidateID sql.NullString `bun:"duplicate_candidate_id"`
	ManagedPath          string         `bun:"managed_path"`
	FileID               sql.NullString `bun:"file_id"`
	ErrorCode            string         `bun:"error_code"`
	ErrorMessage         string         `bun:"error_message"`
	Attempts             int            `bun:"attempts"`
	CreatedAt            time.Time      `bun:"created_at"`
	UpdatedAt            time.Time      `bun:"updated_at"`
}

func (repo *SQLiteRepository) CreateBatch(ctx context.Context, batch importdomain.Batch) (importdomain.Batch, bool, error) {
	validated, err := importdomain.NewBatch(batch)
	if err != nil {
		return importdomain.Batch{}, false, err
	}
	row := toBatchRow(validated)
	result, err := repo.db.NewInsert().Model(&row).On("CONFLICT(request_key) DO NOTHING").Exec(ctx)
	if err != nil {
		return importdomain.Batch{}, false, err
	}
	affected, _ := result.RowsAffected()
	if affected > 0 {
		return validated, true, nil
	}
	existing, err := repo.GetBatchByRequestKey(ctx, validated.RequestKey)
	return existing, false, err
}

func (repo *SQLiteRepository) ReplaceScan(ctx context.Context, batch importdomain.Batch, candidates []importdomain.Candidate) error {
	validated, err := importdomain.NewBatch(batch)
	if err != nil {
		return err
	}
	validatedCandidates := make([]importdomain.Candidate, len(candidates))
	for index, candidate := range candidates {
		candidate, err = importdomain.NewCandidate(candidate)
		if err != nil {
			return err
		}
		if candidate.BatchID != validated.ID {
			return importdomain.ErrInvalidCandidate
		}
		validatedCandidates[index] = candidate
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().Model((*candidateRow)(nil)).Where("batch_id = ?", validated.ID).Exec(ctx); err != nil {
			return err
		}
		for _, candidate := range validatedCandidates {
			row := toCandidateRow(candidate)
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				return err
			}
		}
		row := toBatchRow(validated)
		_, err := tx.NewUpdate().Model(&row).WherePK().
			Column("library_id", "mode", "managed_root", "hidden_policy", "symlink_policy", "status",
				"total_count", "ready_count", "duplicate_count", "skipped_count", "succeeded_count", "failed_count",
				"total_bytes", "last_error_code", "last_error", "cancel_requested", "started_at", "finished_at", "updated_at").Exec(ctx)
		return err
	})
}

func (repo *SQLiteRepository) GetBatch(ctx context.Context, id string) (importdomain.Batch, error) {
	row := new(batchRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return importdomain.Batch{}, importdomain.ErrBatchNotFound
		}
		return importdomain.Batch{}, err
	}
	return row.toDomain()
}

func (repo *SQLiteRepository) GetBatchByRequestKey(ctx context.Context, requestKey string) (importdomain.Batch, error) {
	row := new(batchRow)
	if err := repo.db.NewSelect().Model(row).Where("request_key = ?", strings.TrimSpace(requestKey)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return importdomain.Batch{}, importdomain.ErrBatchNotFound
		}
		return importdomain.Batch{}, err
	}
	return row.toDomain()
}

func (repo *SQLiteRepository) ListBatches(ctx context.Context, limit int) ([]importdomain.Batch, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows := make([]batchRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Order("updated_at DESC", "id ASC").Limit(limit).Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]importdomain.Batch, 0, len(rows))
	for _, row := range rows {
		item, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteRepository) ListCandidates(ctx context.Context, batchID string) ([]importdomain.Candidate, error) {
	rows := make([]candidateRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Where("batch_id = ?", strings.TrimSpace(batchID)).Order("source_path ASC", "id ASC").Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]importdomain.Candidate, 0, len(rows))
	for _, row := range rows {
		item, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteRepository) SaveBatch(ctx context.Context, batch importdomain.Batch) error {
	validated, err := importdomain.NewBatch(batch)
	if err != nil {
		return err
	}
	row := toBatchRow(validated)
	result, err := repo.db.NewUpdate().Model(&row).WherePK().
		Column("library_id", "mode", "managed_root", "hidden_policy", "symlink_policy", "status",
			"total_count", "ready_count", "duplicate_count", "skipped_count", "succeeded_count", "failed_count",
			"total_bytes", "last_error_code", "last_error", "cancel_requested", "started_at", "finished_at", "updated_at").Exec(ctx)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return importdomain.ErrBatchNotFound
	}
	return nil
}

func (repo *SQLiteRepository) SaveCandidate(ctx context.Context, candidate importdomain.Candidate) error {
	validated, err := importdomain.NewCandidate(candidate)
	if err != nil {
		return err
	}
	row := toCandidateRow(validated)
	result, err := repo.db.NewUpdate().Model(&row).WherePK().
		Column("status", "managed_path", "file_id", "error_code", "error_message", "attempts", "updated_at").Exec(ctx)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return importdomain.ErrCandidateNotFound
	}
	return nil
}

func toBatchRow(item importdomain.Batch) batchRow {
	return batchRow{
		ID: item.ID, RequestKey: item.RequestKey, LibraryID: item.LibraryID, Mode: string(item.Mode),
		ManagedRoot: item.ManagedRoot, HiddenPolicy: string(item.HiddenPolicy), SymlinkPolicy: string(item.SymlinkPolicy),
		Status: string(item.Status), TotalCount: item.Counts.Total, ReadyCount: item.Counts.Ready,
		DuplicateCount: item.Counts.Duplicate, SkippedCount: item.Counts.Skipped,
		SucceededCount: item.Counts.Succeeded, FailedCount: item.Counts.Failed, TotalBytes: item.Counts.TotalBytes,
		LastErrorCode: item.LastErrorCode, LastError: item.LastError, CancelRequested: item.CancelRequested,
		StartedAt: nullTime(item.StartedAt), FinishedAt: nullTime(item.FinishedAt), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (row batchRow) toDomain() (importdomain.Batch, error) {
	return importdomain.NewBatch(importdomain.Batch{
		ID: row.ID, RequestKey: row.RequestKey, LibraryID: row.LibraryID, Mode: importdomain.Mode(row.Mode), ManagedRoot: row.ManagedRoot,
		HiddenPolicy: importdomain.HiddenPolicy(row.HiddenPolicy), SymlinkPolicy: importdomain.SymlinkPolicy(row.SymlinkPolicy),
		Status: importdomain.BatchStatus(row.Status), Counts: importdomain.BatchCounts{
			Total: row.TotalCount, Ready: row.ReadyCount, Duplicate: row.DuplicateCount, Skipped: row.SkippedCount,
			Succeeded: row.SucceededCount, Failed: row.FailedCount, TotalBytes: row.TotalBytes,
		}, LastErrorCode: row.LastErrorCode, LastError: row.LastError, CancelRequested: row.CancelRequested,
		StartedAt: timePointer(row.StartedAt), FinishedAt: timePointer(row.FinishedAt), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
}

func toCandidateRow(item importdomain.Candidate) candidateRow {
	return candidateRow{
		ID: item.ID, BatchID: item.BatchID, SourcePath: item.SourcePath, RelativePath: item.RelativePath,
		DisplayName: item.DisplayName, Extension: item.Extension, Category: string(item.Category), MIMEType: item.MIMEType,
		MediaProbed: item.MediaProbed, WasSymlink: item.WasSymlink, SizeBytes: item.SizeBytes,
		ModifiedAt: nullTimeValue(item.ModifiedAt), HashAlgorithm: item.HashAlgorithm, ContentHash: item.ContentHash,
		Status: string(item.Status), DuplicateFileID: nullString(item.DuplicateFileID),
		DuplicateCandidateID: nullString(item.DuplicateCandidateID), ManagedPath: item.ManagedPath, FileID: nullString(item.FileID),
		ErrorCode: item.ErrorCode, ErrorMessage: item.ErrorMessage, Attempts: item.Attempts,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (row candidateRow) toDomain() (importdomain.Candidate, error) {
	return importdomain.NewCandidate(importdomain.Candidate{
		ID: row.ID, BatchID: row.BatchID, SourcePath: row.SourcePath, RelativePath: row.RelativePath,
		DisplayName: row.DisplayName, Extension: row.Extension, Category: importdomain.Category(row.Category), MIMEType: row.MIMEType,
		MediaProbed: row.MediaProbed, WasSymlink: row.WasSymlink, SizeBytes: row.SizeBytes,
		ModifiedAt: timeValue(row.ModifiedAt), HashAlgorithm: row.HashAlgorithm, ContentHash: row.ContentHash,
		Status: importdomain.CandidateStatus(row.Status), DuplicateFileID: stringValue(row.DuplicateFileID),
		DuplicateCandidateID: stringValue(row.DuplicateCandidateID), ManagedPath: row.ManagedPath, FileID: stringValue(row.FileID),
		ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage, Attempts: row.Attempts,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
}

func nullTime(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}
func nullTimeValue(value time.Time) sql.NullTime {
	if value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}
func timePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
func timeValue(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time.UTC()
}
func nullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}
func stringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
