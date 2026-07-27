package libraryrootsyncrepo

import (
	"context"
	"database/sql"
	"errors"
	"path"
	"strings"
	"time"

	"github.com/uptrace/bun"

	domain "xiadown/internal/domain/libraryrootsync"
)

type SQLiteRepository struct{ db *bun.DB }

func NewSQLiteRepository(db *bun.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

type stateRow struct {
	bun.BaseModel    `bun:"table:library_storage_root_sync_states"`
	RootID           string       `bun:"root_id,pk"`
	Status           string       `bun:"status"`
	Generation       int64        `bun:"generation"`
	FullScan         bool         `bun:"full_scan"`
	DiscoveredCount  int          `bun:"discovered_count"`
	ProcessedCount   int          `bun:"processed_count"`
	UnchangedCount   int          `bun:"unchanged_count"`
	DuplicateCount   int          `bun:"duplicate_count"`
	MissingCount     int          `bun:"missing_count"`
	FailedCount      int          `bun:"failed_count"`
	ProcessedBytes   int64        `bun:"processed_bytes"`
	CancelRequested  bool         `bun:"cancel_requested"`
	WatcherCursor    uint64       `bun:"watcher_cursor"`
	LastErrorCode    string       `bun:"last_error_code"`
	LastError        string       `bun:"last_error"`
	StartedAt        sql.NullTime `bun:"started_at"`
	FinishedAt       sql.NullTime `bun:"finished_at"`
	LastReconciledAt sql.NullTime `bun:"last_reconciled_at"`
	CreatedAt        time.Time    `bun:"created_at"`
	UpdatedAt        time.Time    `bun:"updated_at"`
}

type entryRow struct {
	bun.BaseModel      `bun:"table:library_storage_root_sync_entries"`
	RootID             string         `bun:"root_id,pk"`
	RelativePath       string         `bun:"relative_path,pk"`
	SizeBytes          int64          `bun:"size_bytes"`
	ModifiedUnixNano   int64          `bun:"modified_unix_nano"`
	ContentHash        string         `bun:"content_hash"`
	FileID             sql.NullString `bun:"file_id"`
	Status             string         `bun:"status"`
	LastSeenGeneration int64          `bun:"last_seen_generation"`
	LastError          string         `bun:"last_error"`
	CreatedAt          time.Time      `bun:"created_at"`
	UpdatedAt          time.Time      `bun:"updated_at"`
}

func (repo *SQLiteRepository) ListStates(ctx context.Context) ([]domain.State, error) {
	rows := []stateRow{}
	if err := repo.db.NewSelect().Model(&rows).
		OrderExpr("updated_at DESC, root_id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]domain.State, 0, len(rows))
	for _, row := range rows {
		item, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteRepository) GetState(ctx context.Context, rootID string) (domain.State, error) {
	row := new(stateRow)
	if err := repo.db.NewSelect().Model(row).
		Where("root_id = ?", strings.TrimSpace(rootID)).
		Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.State{}, domain.ErrStateNotFound
		}
		return domain.State{}, err
	}
	return row.toDomain()
}

func (repo *SQLiteRepository) SaveState(ctx context.Context, state domain.State) error {
	validated, err := domain.NewState(state)
	if err != nil {
		return err
	}
	row := stateRowFromDomain(validated)
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(root_id) DO UPDATE").
		Set("status = EXCLUDED.status").
		Set("generation = EXCLUDED.generation").
		Set("full_scan = EXCLUDED.full_scan").
		Set("discovered_count = EXCLUDED.discovered_count").
		Set("processed_count = EXCLUDED.processed_count").
		Set("unchanged_count = EXCLUDED.unchanged_count").
		Set("duplicate_count = EXCLUDED.duplicate_count").
		Set("missing_count = EXCLUDED.missing_count").
		Set("failed_count = EXCLUDED.failed_count").
		Set("processed_bytes = EXCLUDED.processed_bytes").
		Set("cancel_requested = EXCLUDED.cancel_requested").
		Set("watcher_cursor = CASE WHEN watcher_cursor < EXCLUDED.watcher_cursor THEN EXCLUDED.watcher_cursor ELSE watcher_cursor END").
		Set("last_error_code = EXCLUDED.last_error_code").
		Set("last_error = EXCLUDED.last_error").
		Set("started_at = EXCLUDED.started_at").
		Set("finished_at = EXCLUDED.finished_at").
		Set("last_reconciled_at = EXCLUDED.last_reconciled_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteRepository) MarkActiveStatesInterrupted(ctx context.Context) error {
	now := time.Now().UTC()
	_, err := repo.db.NewUpdate().Model((*stateRow)(nil)).
		Set("status = ?", string(domain.StatusInterrupted)).
		Set("cancel_requested = FALSE").
		Set("finished_at = ?", now).
		Set("updated_at = ?", now).
		Where("status IN (?)", bun.In([]string{
			string(domain.StatusQueued),
			string(domain.StatusScanning),
			string(domain.StatusCancelling),
		})).
		Exec(ctx)
	return err
}

func (repo *SQLiteRepository) AdvanceWatcherCursor(
	ctx context.Context,
	rootID string,
	cursor uint64,
) error {
	if cursor == 0 {
		return nil
	}
	_, err := repo.db.NewUpdate().Model((*stateRow)(nil)).
		Set("watcher_cursor = CASE WHEN watcher_cursor < ? THEN ? ELSE watcher_cursor END", cursor, cursor).
		Set("updated_at = ?", time.Now().UTC()).
		Where("root_id = ?", strings.TrimSpace(rootID)).
		Exec(ctx)
	return err
}

func (repo *SQLiteRepository) GetEntry(
	ctx context.Context,
	rootID string,
	relativePath string,
) (domain.Entry, error) {
	row := new(entryRow)
	if err := repo.db.NewSelect().Model(row).
		Where("root_id = ?", strings.TrimSpace(rootID)).
		Where("relative_path = ?", normalizeRelativePath(relativePath)).
		Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Entry{}, domain.ErrEntryNotFound
		}
		return domain.Entry{}, err
	}
	return row.toDomain()
}

func (repo *SQLiteRepository) ListEntriesByStatus(
	ctx context.Context,
	rootID string,
	status domain.EntryStatus,
) ([]domain.Entry, error) {
	rows := []entryRow{}
	if err := repo.db.NewSelect().Model(&rows).
		Where("root_id = ?", strings.TrimSpace(rootID)).
		Where("status = ?", string(status)).
		OrderExpr("relative_path ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]domain.Entry, 0, len(rows))
	for _, row := range rows {
		item, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteRepository) ListActiveEntriesBySize(
	ctx context.Context,
	rootID string,
	sizeBytes int64,
) ([]domain.Entry, error) {
	rows := []entryRow{}
	if err := repo.db.NewSelect().Model(&rows).
		Where("root_id = ?", strings.TrimSpace(rootID)).
		Where("size_bytes = ?", sizeBytes).
		Where("status = ?", string(domain.EntryActive)).
		OrderExpr("relative_path ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]domain.Entry, 0, len(rows))
	for _, row := range rows {
		item, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteRepository) FindActiveEntryByDigest(
	ctx context.Context,
	rootID string,
	sizeBytes int64,
	contentHash string,
) (domain.Entry, error) {
	row := new(entryRow)
	if err := repo.db.NewSelect().Model(row).
		Where("root_id = ?", strings.TrimSpace(rootID)).
		Where("size_bytes = ?", sizeBytes).
		Where("content_hash = ?", strings.ToLower(strings.TrimSpace(contentHash))).
		Where("status = ?", string(domain.EntryActive)).
		OrderExpr("relative_path ASC").
		Limit(1).
		Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Entry{}, domain.ErrEntryNotFound
		}
		return domain.Entry{}, err
	}
	return row.toDomain()
}

func (repo *SQLiteRepository) UpsertEntry(ctx context.Context, entry domain.Entry) error {
	validated, err := domain.NewEntry(entry)
	if err != nil {
		return err
	}
	row := entryRowFromDomain(validated)
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(root_id, relative_path) DO UPDATE").
		Set("size_bytes = EXCLUDED.size_bytes").
		Set("modified_unix_nano = EXCLUDED.modified_unix_nano").
		Set("content_hash = EXCLUDED.content_hash").
		Set("file_id = EXCLUDED.file_id").
		Set("status = EXCLUDED.status").
		Set("last_seen_generation = EXCLUDED.last_seen_generation").
		Set("last_error = EXCLUDED.last_error").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteRepository) MarkUnseenEntriesMissing(
	ctx context.Context,
	rootID string,
	generation int64,
) (int, error) {
	result, err := repo.db.NewUpdate().Model((*entryRow)(nil)).
		Set("status = ?", string(domain.EntryMissing)).
		Set("updated_at = ?", time.Now().UTC()).
		Where("root_id = ?", strings.TrimSpace(rootID)).
		Where("last_seen_generation < ?", generation).
		Where("status <> ?", string(domain.EntryMissing)).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func (repo *SQLiteRepository) MarkPathMissing(
	ctx context.Context,
	rootID string,
	relativePath string,
	recursive bool,
	generation int64,
) (int, error) {
	query := repo.db.NewUpdate().Model((*entryRow)(nil)).
		Set("status = ?", string(domain.EntryMissing)).
		Set("last_seen_generation = ?", generation).
		Set("updated_at = ?", time.Now().UTC()).
		Where("root_id = ?", strings.TrimSpace(rootID)).
		Where("status <> ?", string(domain.EntryMissing))
	relativePath = normalizeRelativePath(relativePath)
	if recursive {
		query = query.Where(
			"(relative_path = ? OR relative_path LIKE ? ESCAPE '\\')",
			relativePath,
			escapeLike(relativePath)+"/%",
		)
	} else {
		query = query.Where("relative_path = ?", relativePath)
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func stateRowFromDomain(item domain.State) stateRow {
	return stateRow{
		RootID: item.RootID, Status: string(item.Status), Generation: item.Generation,
		FullScan: item.FullScan, DiscoveredCount: item.DiscoveredCount,
		ProcessedCount: item.ProcessedCount, UnchangedCount: item.UnchangedCount,
		DuplicateCount: item.DuplicateCount, MissingCount: item.MissingCount,
		FailedCount: item.FailedCount, ProcessedBytes: item.ProcessedBytes,
		CancelRequested: item.CancelRequested, WatcherCursor: item.WatcherCursor,
		LastErrorCode: item.LastErrorCode, LastError: item.LastError,
		StartedAt: nullTime(item.StartedAt), FinishedAt: nullTime(item.FinishedAt),
		LastReconciledAt: nullTime(item.LastReconciledAt),
		CreatedAt:        item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (row stateRow) toDomain() (domain.State, error) {
	return domain.NewState(domain.State{
		RootID: row.RootID, Status: domain.Status(row.Status), Generation: row.Generation,
		FullScan: row.FullScan, DiscoveredCount: row.DiscoveredCount,
		ProcessedCount: row.ProcessedCount, UnchangedCount: row.UnchangedCount,
		DuplicateCount: row.DuplicateCount, MissingCount: row.MissingCount,
		FailedCount: row.FailedCount, ProcessedBytes: row.ProcessedBytes,
		CancelRequested: row.CancelRequested, WatcherCursor: row.WatcherCursor,
		LastErrorCode: row.LastErrorCode, LastError: row.LastError,
		StartedAt: timePointer(row.StartedAt), FinishedAt: timePointer(row.FinishedAt),
		LastReconciledAt: timePointer(row.LastReconciledAt),
		CreatedAt:        row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
}

func entryRowFromDomain(item domain.Entry) entryRow {
	return entryRow{
		RootID: item.RootID, RelativePath: item.RelativePath,
		SizeBytes: item.SizeBytes, ModifiedUnixNano: item.ModifiedUnixNano,
		ContentHash: item.ContentHash, FileID: nullString(item.FileID),
		Status: string(item.Status), LastSeenGeneration: item.LastSeenGeneration,
		LastError: item.LastError, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (row entryRow) toDomain() (domain.Entry, error) {
	return domain.NewEntry(domain.Entry{
		RootID: row.RootID, RelativePath: row.RelativePath,
		SizeBytes: row.SizeBytes, ModifiedUnixNano: row.ModifiedUnixNano,
		ContentHash: row.ContentHash, FileID: row.FileID.String,
		Status: domain.EntryStatus(row.Status), LastSeenGeneration: row.LastSeenGeneration,
		LastError: row.LastError, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
}

func nullTime(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
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

func nullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func normalizeRelativePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	if value == "" {
		return ""
	}
	value = path.Clean(value)
	if value == "." || value == ".." || path.IsAbs(value) ||
		strings.HasPrefix(value, "../") {
		return ""
	}
	return value
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
