package libraryrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

type SQLiteOperationRepository struct{ db *bun.DB }
type SQLiteOperationChunkRepository struct{ db *bun.DB }
type SQLiteHistoryRepository struct{ db *bun.DB }
type SQLiteWorkspaceStateRepository struct{ db *bun.DB }
type SQLiteFileEventRepository struct{ db *bun.DB }
type SQLiteSubtitleDocumentRepository struct{ db *bun.DB }

type operationRow struct {
	bun.BaseModel   `bun:"table:library_operations"`
	ID              string         `bun:"id,pk"`
	LibraryID       string         `bun:"library_id"`
	Kind            string         `bun:"kind"`
	Status          string         `bun:"status"`
	DisplayName     string         `bun:"display_name"`
	CorrelationJSON string         `bun:"correlation_json"`
	InputJSON       string         `bun:"input_json"`
	OutputJSON      string         `bun:"output_json"`
	MetaJSON        sql.NullString `bun:"meta_json"`
	ProgressJSON    sql.NullString `bun:"progress_json"`
	SourceDomain    sql.NullString `bun:"source_domain"`
	SourceIcon      sql.NullString `bun:"source_icon"`
	FileCount       int            `bun:"file_count"`
	TotalSizeBytes  sql.NullInt64  `bun:"total_size_bytes"`
	DurationMs      sql.NullInt64  `bun:"duration_ms"`
	ErrorCode       sql.NullString `bun:"error_code"`
	ErrorMessage    sql.NullString `bun:"error_message"`
	CreatedAt       time.Time      `bun:"created_at"`
	StartedAt       sql.NullTime   `bun:"started_at"`
	FinishedAt      sql.NullTime   `bun:"finished_at"`
}

type operationOutputRow struct {
	bun.BaseModel `bun:"table:library_operation_outputs"`
	ID            string    `bun:"id,pk"`
	OperationID   string    `bun:"operation_id"`
	LibraryID     string    `bun:"library_id"`
	FileID        string    `bun:"file_id"`
	IsPrimary     bool      `bun:"is_primary"`
	CreatedAt     time.Time `bun:"created_at"`
}

type operationChunkRow struct {
	bun.BaseModel `bun:"table:library_operation_chunks"`
	ID            string         `bun:"id,pk"`
	OperationID   string         `bun:"operation_id"`
	LibraryID     string         `bun:"library_id"`
	ChunkIndex    int            `bun:"chunk_index"`
	Status        string         `bun:"status"`
	SourceRange   sql.NullString `bun:"source_range"`
	InputHash     sql.NullString `bun:"input_hash"`
	RequestHash   sql.NullString `bun:"request_hash"`
	PromptHash    sql.NullString `bun:"prompt_hash"`
	ResponseHash  sql.NullString `bun:"response_hash"`
	ResultJSON    sql.NullString `bun:"result_json"`
	UsageJSON     sql.NullString `bun:"usage_json"`
	RetryCount    int            `bun:"retry_count"`
	ErrorMessage  sql.NullString `bun:"error_message"`
	StartedAt     sql.NullTime   `bun:"started_at"`
	FinishedAt    sql.NullTime   `bun:"finished_at"`
	CreatedAt     time.Time      `bun:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at"`
}

type historyRow struct {
	bun.BaseModel  `bun:"table:library_history_records"`
	ID             string         `bun:"id,pk"`
	LibraryID      string         `bun:"library_id"`
	Category       string         `bun:"category"`
	Action         string         `bun:"action"`
	DisplayName    string         `bun:"display_name"`
	Status         string         `bun:"status"`
	SourceKind     string         `bun:"source_kind"`
	SourceCaller   sql.NullString `bun:"source_caller"`
	SourceRunID    sql.NullString `bun:"source_run_id"`
	SourceActor    sql.NullString `bun:"source_actor"`
	OperationID    sql.NullString `bun:"operation_id"`
	ImportBatchID  sql.NullString `bun:"import_batch_id"`
	FileCount      int            `bun:"file_count"`
	TotalSizeBytes sql.NullInt64  `bun:"total_size_bytes"`
	DurationMs     sql.NullInt64  `bun:"duration_ms"`
	ImportPath     sql.NullString `bun:"import_path"`
	KeepSourceFile sql.NullBool   `bun:"keep_source_file"`
	ErrorCode      sql.NullString `bun:"error_code"`
	ErrorMessage   sql.NullString `bun:"error_message"`
	OccurredAt     time.Time      `bun:"occurred_at"`
	CreatedAt      time.Time      `bun:"created_at"`
	UpdatedAt      time.Time      `bun:"updated_at"`
}

type historyFileRow struct {
	bun.BaseModel `bun:"table:library_history_files"`
	ID            string         `bun:"id,pk"`
	HistoryID     string         `bun:"history_id"`
	FileID        string         `bun:"file_id"`
	Kind          string         `bun:"kind"`
	Format        sql.NullString `bun:"format"`
	SizeBytes     sql.NullInt64  `bun:"size_bytes"`
	Deleted       bool           `bun:"deleted"`
	CreatedAt     time.Time      `bun:"created_at"`
}

type workspaceStateRow struct {
	bun.BaseModel `bun:"table:library_workspace_states"`
	ID            string         `bun:"id,pk"`
	LibraryID     string         `bun:"library_id"`
	StateVersion  int            `bun:"state_version"`
	StateJSON     string         `bun:"state_json"`
	OperationID   sql.NullString `bun:"operation_id"`
	CreatedAt     time.Time      `bun:"created_at"`
}

type workspaceHeadRow struct {
	bun.BaseModel `bun:"table:library_workspace_state_head"`
	LibraryID     string    `bun:"library_id,pk"`
	LatestStateID string    `bun:"latest_state_id"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

type fileEventRow struct {
	bun.BaseModel `bun:"table:library_file_events"`
	ID            string         `bun:"id,pk"`
	LibraryID     string         `bun:"library_id"`
	FileID        string         `bun:"file_id"`
	OperationID   sql.NullString `bun:"operation_id"`
	EventType     string         `bun:"event_type"`
	DetailJSON    string         `bun:"detail_json"`
	CreatedAt     time.Time      `bun:"created_at"`
}

type subtitleDocumentRow struct {
	bun.BaseModel   `bun:"table:library_subtitle_documents"`
	ID              string    `bun:"id,pk"`
	FileID          string    `bun:"file_id"`
	LibraryID       string    `bun:"library_id"`
	Format          string    `bun:"format"`
	OriginalContent string    `bun:"original_content"`
	WorkingContent  string    `bun:"working_content"`
	CreatedAt       time.Time `bun:"created_at"`
	UpdatedAt       time.Time `bun:"updated_at"`
}

func NewSQLiteOperationRepository(db *bun.DB) *SQLiteOperationRepository {
	return &SQLiteOperationRepository{db: db}
}
func NewSQLiteOperationChunkRepository(db *bun.DB) *SQLiteOperationChunkRepository {
	return &SQLiteOperationChunkRepository{db: db}
}
func NewSQLiteHistoryRepository(db *bun.DB) *SQLiteHistoryRepository {
	return &SQLiteHistoryRepository{db: db}
}
func NewSQLiteWorkspaceStateRepository(db *bun.DB) *SQLiteWorkspaceStateRepository {
	return &SQLiteWorkspaceStateRepository{db: db}
}
func NewSQLiteFileEventRepository(db *bun.DB) *SQLiteFileEventRepository {
	return &SQLiteFileEventRepository{db: db}
}
func NewSQLiteSubtitleDocumentRepository(db *bun.DB) *SQLiteSubtitleDocumentRepository {
	return &SQLiteSubtitleDocumentRepository{db: db}
}

func (repo *SQLiteOperationRepository) List(ctx context.Context) ([]library.LibraryOperation, error) {
	rows := make([]operationRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	return repo.mapOperations(ctx, rows)
}

func (repo *SQLiteOperationRepository) ListByLibraryID(ctx context.Context, libraryID string) ([]library.LibraryOperation, error) {
	rows := make([]operationRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Where("library_id = ?", strings.TrimSpace(libraryID)).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	return repo.mapOperations(ctx, rows)
}

func (repo *SQLiteOperationRepository) Get(ctx context.Context, id string) (library.LibraryOperation, error) {
	row := new(operationRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.LibraryOperation{}, library.ErrOperationNotFound
		}
		return library.LibraryOperation{}, err
	}
	items, err := repo.mapOperations(ctx, []operationRow{*row})
	if err != nil {
		return library.LibraryOperation{}, err
	}
	if len(items) == 0 {
		return library.LibraryOperation{}, library.ErrOperationNotFound
	}
	return items[0], nil
}

func (repo *SQLiteOperationRepository) Save(ctx context.Context, item library.LibraryOperation) error {
	correlationJSON, err := json.Marshal(item.Correlation)
	if err != nil {
		return err
	}
	metaJSON := sql.NullString{}
	if payload, err := json.Marshal(item.Meta); err == nil && string(payload) != "{}" {
		metaJSON = nullString(string(payload))
	} else if err != nil {
		return err
	}
	progressJSON := sql.NullString{}
	if item.Progress != nil {
		payload, err := json.Marshal(item.Progress)
		if err != nil {
			return err
		}
		progressJSON = nullString(string(payload))
	}
	row := operationRow{
		ID: item.ID, LibraryID: item.LibraryID, Kind: item.Kind, Status: string(item.Status), DisplayName: item.DisplayName,
		CorrelationJSON: string(correlationJSON), InputJSON: item.InputJSON, OutputJSON: item.OutputJSON,
		MetaJSON: metaJSON, ProgressJSON: progressJSON, SourceDomain: nullString(item.SourceDomain), SourceIcon: nullString(item.SourceIcon),
		FileCount: item.Metrics.FileCount, TotalSizeBytes: nullInt64Ptr(item.Metrics.TotalSizeBytes), DurationMs: nullInt64Ptr(item.Metrics.DurationMs),
		ErrorCode: nullString(item.ErrorCode), ErrorMessage: nullString(item.ErrorMessage), CreatedAt: item.CreatedAt, StartedAt: nullTime(item.StartedAt), FinishedAt: nullTime(item.FinishedAt),
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&row).
			On("CONFLICT(id) DO UPDATE").
			Set("library_id = EXCLUDED.library_id").
			Set("kind = EXCLUDED.kind").
			Set("status = EXCLUDED.status").
			Set("display_name = EXCLUDED.display_name").
			Set("correlation_json = EXCLUDED.correlation_json").
			Set("input_json = EXCLUDED.input_json").
			Set("output_json = EXCLUDED.output_json").
			Set("meta_json = EXCLUDED.meta_json").
			Set("progress_json = EXCLUDED.progress_json").
			Set("source_domain = EXCLUDED.source_domain").
			Set("source_icon = EXCLUDED.source_icon").
			Set("file_count = EXCLUDED.file_count").
			Set("total_size_bytes = EXCLUDED.total_size_bytes").
			Set("duration_ms = EXCLUDED.duration_ms").
			Set("error_code = EXCLUDED.error_code").
			Set("error_message = EXCLUDED.error_message").
			Set("started_at = EXCLUDED.started_at").
			Set("finished_at = EXCLUDED.finished_at").
			Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*operationOutputRow)(nil)).Where("operation_id = ?", item.ID).Exec(ctx); err != nil {
			return err
		}
		for index, output := range item.OutputFiles {
			fileLibraryID := ""
			if err := tx.NewSelect().Model((*fileRow)(nil)).Column("library_id").Where("id = ?", strings.TrimSpace(output.FileID)).Scan(ctx, &fileLibraryID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return library.ErrInvalidOperationOutput
				}
				return err
			}
			if strings.TrimSpace(fileLibraryID) != item.LibraryID {
				return library.ErrInvalidOperationOutput
			}
			isPrimary := output.IsPrimary
			if !isPrimary && index == 0 {
				isPrimary = true
			}
			outRow := operationOutputRow{ID: uuid.NewString(), OperationID: item.ID, LibraryID: item.LibraryID, FileID: output.FileID, IsPrimary: isPrimary, CreatedAt: item.CreatedAt}
			if _, err := tx.NewInsert().Model(&outRow).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repo *SQLiteOperationRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*operationRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func (repo *SQLiteOperationChunkRepository) ListByOperationID(ctx context.Context, operationID string) ([]library.OperationChunk, error) {
	rows := make([]operationChunkRow, 0)
	if err := repo.db.NewSelect().
		Model(&rows).
		Where("operation_id = ?", strings.TrimSpace(operationID)).
		Order("chunk_index ASC").
		Scan(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	result := make([]library.OperationChunk, 0, len(rows))
	for _, row := range rows {
		item, err := library.NewOperationChunk(library.OperationChunkParams{
			ID:           row.ID,
			OperationID:  row.OperationID,
			LibraryID:    row.LibraryID,
			ChunkIndex:   row.ChunkIndex,
			Status:       row.Status,
			SourceRange:  row.SourceRange.String,
			InputHash:    row.InputHash.String,
			RequestHash:  row.RequestHash.String,
			PromptHash:   row.PromptHash.String,
			ResponseHash: row.ResponseHash.String,
			ResultJSON:   row.ResultJSON.String,
			UsageJSON:    row.UsageJSON.String,
			RetryCount:   row.RetryCount,
			ErrorMessage: row.ErrorMessage.String,
			StartedAt:    timeOrNil(row.StartedAt),
			FinishedAt:   timeOrNil(row.FinishedAt),
			CreatedAt:    &row.CreatedAt,
			UpdatedAt:    &row.UpdatedAt,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteOperationChunkRepository) Save(ctx context.Context, item library.OperationChunk) error {
	row := operationChunkRow{
		ID:           item.ID,
		OperationID:  item.OperationID,
		LibraryID:    item.LibraryID,
		ChunkIndex:   item.ChunkIndex,
		Status:       string(item.Status),
		SourceRange:  nullString(item.SourceRange),
		InputHash:    nullString(item.InputHash),
		RequestHash:  nullString(item.RequestHash),
		PromptHash:   nullString(item.PromptHash),
		ResponseHash: nullString(item.ResponseHash),
		ResultJSON:   nullString(item.ResultJSON),
		UsageJSON:    nullString(item.UsageJSON),
		RetryCount:   item.RetryCount,
		ErrorMessage: nullString(item.ErrorMessage),
		StartedAt:    nullTime(item.StartedAt),
		FinishedAt:   nullTime(item.FinishedAt),
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
	_, err := repo.db.NewInsert().
		Model(&row).
		On("CONFLICT(operation_id, chunk_index) DO UPDATE").
		Set("library_id = EXCLUDED.library_id").
		Set("status = EXCLUDED.status").
		Set("source_range = EXCLUDED.source_range").
		Set("input_hash = EXCLUDED.input_hash").
		Set("request_hash = EXCLUDED.request_hash").
		Set("prompt_hash = EXCLUDED.prompt_hash").
		Set("response_hash = EXCLUDED.response_hash").
		Set("result_json = EXCLUDED.result_json").
		Set("usage_json = EXCLUDED.usage_json").
		Set("retry_count = EXCLUDED.retry_count").
		Set("error_message = EXCLUDED.error_message").
		Set("started_at = EXCLUDED.started_at").
		Set("finished_at = EXCLUDED.finished_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteOperationChunkRepository) DeleteByOperationID(ctx context.Context, operationID string) error {
	_, err := repo.db.NewDelete().Model((*operationChunkRow)(nil)).Where("operation_id = ?", strings.TrimSpace(operationID)).Exec(ctx)
	return err
}

func (repo *SQLiteOperationRepository) mapOperations(ctx context.Context, rows []operationRow) ([]library.LibraryOperation, error) {
	if len(rows) == 0 {
		return []library.LibraryOperation{}, nil
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	outputRows := make([]operationOutputRow, 0)
	if err := repo.db.NewSelect().Model(&outputRows).Where("operation_id IN (?)", bun.In(ids)).Order("created_at ASC").Scan(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	fileRows := make([]fileRow, 0)
	if err := repo.db.NewSelect().Model(&fileRows).Where("id IN (?)", bun.In(extractFileIDs(outputRows))).Scan(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	fileByID := make(map[string]fileRow, len(fileRows))
	for _, row := range fileRows {
		fileByID[row.ID] = row
	}
	outputsByOperation := make(map[string][]library.OperationOutputFile)
	for _, row := range outputRows {
		fileRow, ok := fileByID[row.FileID]
		if !ok {
			continue
		}
		outputsByOperation[row.OperationID] = append(outputsByOperation[row.OperationID], library.OperationOutputFile{
			FileID:    row.FileID,
			Kind:      fileRow.Kind,
			Format:    mediaFormat(fileRow),
			SizeBytes: mediaSize(fileRow),
			IsPrimary: row.IsPrimary,
			Deleted:   decodeDeleted(fileRow),
		})
	}
	result := make([]library.LibraryOperation, 0, len(rows))
	for _, row := range rows {
		correlation := library.OperationCorrelation{}
		if err := json.Unmarshal([]byte(row.CorrelationJSON), &correlation); err != nil {
			return nil, err
		}
		meta := library.OperationMeta{}
		if row.MetaJSON.Valid && strings.TrimSpace(row.MetaJSON.String) != "" {
			if err := json.Unmarshal([]byte(row.MetaJSON.String), &meta); err != nil {
				return nil, err
			}
		}
		var progress *library.OperationProgress
		if row.ProgressJSON.Valid && strings.TrimSpace(row.ProgressJSON.String) != "" {
			decoded := new(library.OperationProgress)
			if err := json.Unmarshal([]byte(row.ProgressJSON.String), decoded); err != nil {
				return nil, err
			}
			progress = decoded
		}
		outputs := outputsByOperation[row.ID]
		metrics := library.OperationMetrics{FileCount: row.FileCount, TotalSizeBytes: int64Ptr(row.TotalSizeBytes), DurationMs: int64Ptr(row.DurationMs)}
		item, err := library.NewLibraryOperation(library.LibraryOperationParams{
			ID: row.ID, LibraryID: row.LibraryID, Kind: row.Kind, Status: row.Status, DisplayName: row.DisplayName,
			Correlation: correlation, InputJSON: row.InputJSON, OutputJSON: row.OutputJSON, Meta: meta, Progress: progress,
			OutputFiles: outputs, Metrics: metrics, SourceDomain: stringOrEmpty(row.SourceDomain), SourceIcon: stringOrEmpty(row.SourceIcon),
			ErrorCode: stringOrEmpty(row.ErrorCode), ErrorMessage: stringOrEmpty(row.ErrorMessage), CreatedAt: &row.CreatedAt, StartedAt: timeOrNil(row.StartedAt), FinishedAt: timeOrNil(row.FinishedAt),
		})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteHistoryRepository) ListByLibraryID(ctx context.Context, libraryID string) ([]library.HistoryRecord, error) {
	rows := make([]historyRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Where("library_id = ?", strings.TrimSpace(libraryID)).Order("occurred_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	return repo.mapHistory(ctx, rows)
}

func (repo *SQLiteHistoryRepository) Get(ctx context.Context, id string) (library.HistoryRecord, error) {
	row := new(historyRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.HistoryRecord{}, library.ErrHistoryRecordNotFound
		}
		return library.HistoryRecord{}, err
	}
	items, err := repo.mapHistory(ctx, []historyRow{*row})
	if err != nil {
		return library.HistoryRecord{}, err
	}
	if len(items) == 0 {
		return library.HistoryRecord{}, library.ErrHistoryRecordNotFound
	}
	return items[0], nil
}

func (repo *SQLiteHistoryRepository) Save(ctx context.Context, item library.HistoryRecord) error {
	row := historyRow{
		ID: item.ID, LibraryID: item.LibraryID, Category: item.Category, Action: item.Action, DisplayName: item.DisplayName, Status: item.Status,
		SourceKind: item.Source.Kind, SourceCaller: nullString(item.Source.Caller), SourceRunID: nullString(item.Source.RunID), SourceActor: nullString(item.Source.Actor),
		OperationID: nullString(item.Refs.OperationID), ImportBatchID: nullString(item.Refs.ImportBatchID), FileCount: item.Metrics.FileCount,
		TotalSizeBytes: nullInt64Ptr(item.Metrics.TotalSizeBytes), DurationMs: nullInt64Ptr(item.Metrics.DurationMs),
		OccurredAt: item.OccurredAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	if item.ImportMeta != nil {
		row.ImportPath = nullString(item.ImportMeta.ImportPath)
		row.KeepSourceFile = sql.NullBool{Bool: item.ImportMeta.KeepSourceFile, Valid: true}
	}
	if item.OperationMeta != nil {
		row.ErrorCode = nullString(item.OperationMeta.ErrorCode)
		row.ErrorMessage = nullString(item.OperationMeta.ErrorMessage)
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&row).
			On("CONFLICT(id) DO UPDATE").
			Set("library_id = EXCLUDED.library_id").Set("category = EXCLUDED.category").Set("action = EXCLUDED.action").Set("display_name = EXCLUDED.display_name").
			Set("status = EXCLUDED.status").Set("source_kind = EXCLUDED.source_kind").Set("source_caller = EXCLUDED.source_caller").Set("source_run_id = EXCLUDED.source_run_id").
			Set("source_actor = EXCLUDED.source_actor").Set("operation_id = EXCLUDED.operation_id").Set("import_batch_id = EXCLUDED.import_batch_id").
			Set("file_count = EXCLUDED.file_count").Set("total_size_bytes = EXCLUDED.total_size_bytes").Set("duration_ms = EXCLUDED.duration_ms").
			Set("import_path = EXCLUDED.import_path").Set("keep_source_file = EXCLUDED.keep_source_file").Set("error_code = EXCLUDED.error_code").Set("error_message = EXCLUDED.error_message").
			Set("occurred_at = EXCLUDED.occurred_at").Set("updated_at = EXCLUDED.updated_at").Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*historyFileRow)(nil)).Where("history_id = ?", item.ID).Exec(ctx); err != nil {
			return err
		}
		for _, file := range item.Files {
			row := historyFileRow{ID: uuid.NewString(), HistoryID: item.ID, FileID: file.FileID, Kind: file.Kind, Format: nullString(file.Format), SizeBytes: nullInt64Ptr(file.SizeBytes), Deleted: file.Deleted, CreatedAt: item.CreatedAt}
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repo *SQLiteHistoryRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*historyRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func (repo *SQLiteHistoryRepository) DeleteByOperationID(ctx context.Context, operationID string) error {
	_, err := repo.db.NewDelete().Model((*historyRow)(nil)).Where("operation_id = ?", strings.TrimSpace(operationID)).Exec(ctx)
	return err
}

func (repo *SQLiteHistoryRepository) mapHistory(ctx context.Context, rows []historyRow) ([]library.HistoryRecord, error) {
	if len(rows) == 0 {
		return []library.HistoryRecord{}, nil
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	fileRows := make([]historyFileRow, 0)
	if err := repo.db.NewSelect().Model(&fileRows).Where("history_id IN (?)", bun.In(ids)).Order("created_at ASC").Scan(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	filesByHistory := make(map[string][]library.OperationOutputFile)
	for _, row := range fileRows {
		filesByHistory[row.HistoryID] = append(filesByHistory[row.HistoryID], library.OperationOutputFile{FileID: row.FileID, Kind: row.Kind, Format: stringOrEmpty(row.Format), SizeBytes: int64Ptr(row.SizeBytes), Deleted: row.Deleted})
	}
	result := make([]library.HistoryRecord, 0, len(rows))
	for _, row := range rows {
		metrics := library.OperationMetrics{FileCount: row.FileCount, TotalSizeBytes: int64Ptr(row.TotalSizeBytes), DurationMs: int64Ptr(row.DurationMs)}
		refs := library.HistoryRecordRefs{OperationID: stringOrEmpty(row.OperationID), ImportBatchID: stringOrEmpty(row.ImportBatchID)}
		meta := (*library.ImportRecordMeta)(nil)
		if row.ImportPath.Valid || row.KeepSourceFile.Valid {
			meta = &library.ImportRecordMeta{ImportPath: stringOrEmpty(row.ImportPath), KeepSourceFile: boolOrFalse(row.KeepSourceFile)}
		}
		opMeta := (*library.OperationRecordMeta)(nil)
		if row.Category == "operation" {
			opMeta = &library.OperationRecordMeta{Kind: row.Action, ErrorCode: stringOrEmpty(row.ErrorCode), ErrorMessage: stringOrEmpty(row.ErrorMessage)}
		}
		item, err := library.NewHistoryRecord(library.HistoryRecordParams{ID: row.ID, LibraryID: row.LibraryID, Category: row.Category, Action: row.Action, DisplayName: row.DisplayName, Status: row.Status, Source: library.HistoryRecordSource{Kind: row.SourceKind, Caller: stringOrEmpty(row.SourceCaller), RunID: stringOrEmpty(row.SourceRunID), Actor: stringOrEmpty(row.SourceActor)}, Refs: refs, Files: filesByHistory[row.ID], Metrics: metrics, ImportMeta: meta, OperationMeta: opMeta, OccurredAt: &row.OccurredAt, CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteWorkspaceStateRepository) ListByLibraryID(ctx context.Context, libraryID string) ([]library.WorkspaceStateRecord, error) {
	rows := make([]workspaceStateRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Where("library_id = ?", strings.TrimSpace(libraryID)).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.WorkspaceStateRecord, 0, len(rows))
	for _, row := range rows {
		item, err := library.NewWorkspaceStateRecord(library.WorkspaceStateRecordParams{ID: row.ID, LibraryID: row.LibraryID, StateVersion: row.StateVersion, StateJSON: row.StateJSON, OperationID: stringOrEmpty(row.OperationID), CreatedAt: &row.CreatedAt})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteWorkspaceStateRepository) GetHeadByLibraryID(ctx context.Context, libraryID string) (library.WorkspaceStateRecord, error) {
	head := new(workspaceHeadRow)
	if err := repo.db.NewSelect().Model(head).Where("library_id = ?", strings.TrimSpace(libraryID)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.WorkspaceStateRecord{}, library.ErrWorkspaceStateNotFound
		}
		return library.WorkspaceStateRecord{}, err
	}
	row := new(workspaceStateRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", head.LatestStateID).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.WorkspaceStateRecord{}, library.ErrWorkspaceStateNotFound
		}
		return library.WorkspaceStateRecord{}, err
	}
	return library.NewWorkspaceStateRecord(library.WorkspaceStateRecordParams{ID: row.ID, LibraryID: row.LibraryID, StateVersion: row.StateVersion, StateJSON: row.StateJSON, OperationID: stringOrEmpty(row.OperationID), CreatedAt: &row.CreatedAt})
}

func (repo *SQLiteWorkspaceStateRepository) Save(ctx context.Context, item library.WorkspaceStateRecord) error {
	row := workspaceStateRow{ID: item.ID, LibraryID: item.LibraryID, StateVersion: item.StateVersion, StateJSON: item.StateJSON, OperationID: nullString(item.OperationID), CreatedAt: item.CreatedAt}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&row).On("CONFLICT(id) DO UPDATE").Set("state_json = EXCLUDED.state_json").Set("operation_id = EXCLUDED.operation_id").Exec(ctx); err != nil {
			return err
		}
		head := workspaceHeadRow{LibraryID: item.LibraryID, LatestStateID: item.ID, UpdatedAt: item.CreatedAt}
		_, err := tx.NewInsert().Model(&head).On("CONFLICT(library_id) DO UPDATE").Set("latest_state_id = EXCLUDED.latest_state_id").Set("updated_at = EXCLUDED.updated_at").Exec(ctx)
		return err
	})
}

func (repo *SQLiteFileEventRepository) ListByLibraryID(ctx context.Context, libraryID string) ([]library.FileEventRecord, error) {
	rows := make([]fileEventRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Where("library_id = ?", strings.TrimSpace(libraryID)).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.FileEventRecord, 0, len(rows))
	for _, row := range rows {
		item, err := library.NewFileEventRecord(library.FileEventRecordParams{ID: row.ID, LibraryID: row.LibraryID, FileID: row.FileID, OperationID: stringOrEmpty(row.OperationID), EventType: row.EventType, DetailJSON: row.DetailJSON, CreatedAt: &row.CreatedAt})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteFileEventRepository) Save(ctx context.Context, item library.FileEventRecord) error {
	row := fileEventRow{ID: item.ID, LibraryID: item.LibraryID, FileID: item.FileID, OperationID: nullString(item.OperationID), EventType: item.EventType, DetailJSON: item.DetailJSON, CreatedAt: item.CreatedAt}
	_, err := repo.db.NewInsert().Model(&row).On("CONFLICT(id) DO UPDATE").Set("detail_json = EXCLUDED.detail_json").Exec(ctx)
	return err
}

func (repo *SQLiteSubtitleDocumentRepository) Get(ctx context.Context, id string) (library.SubtitleDocument, error) {
	row := new(subtitleDocumentRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.SubtitleDocument{}, library.ErrSubtitleDocumentNotFound
		}
		return library.SubtitleDocument{}, err
	}
	return library.NewSubtitleDocument(library.SubtitleDocumentParams{ID: row.ID, FileID: row.FileID, LibraryID: row.LibraryID, Format: row.Format, OriginalContent: row.OriginalContent, WorkingContent: row.WorkingContent, CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt})
}

func (repo *SQLiteSubtitleDocumentRepository) GetByFileID(ctx context.Context, fileID string) (library.SubtitleDocument, error) {
	row := new(subtitleDocumentRow)
	if err := repo.db.NewSelect().Model(row).Where("file_id = ?", strings.TrimSpace(fileID)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.SubtitleDocument{}, library.ErrSubtitleDocumentNotFound
		}
		return library.SubtitleDocument{}, err
	}
	return library.NewSubtitleDocument(library.SubtitleDocumentParams{ID: row.ID, FileID: row.FileID, LibraryID: row.LibraryID, Format: row.Format, OriginalContent: row.OriginalContent, WorkingContent: row.WorkingContent, CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt})
}

func (repo *SQLiteSubtitleDocumentRepository) Save(ctx context.Context, item library.SubtitleDocument) error {
	row := subtitleDocumentRow{ID: item.ID, FileID: item.FileID, LibraryID: item.LibraryID, Format: item.Format, OriginalContent: item.OriginalContent, WorkingContent: item.WorkingContent, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("file_id = EXCLUDED.file_id").Set("library_id = EXCLUDED.library_id").Set("format = EXCLUDED.format").
		Set("original_content = EXCLUDED.original_content").Set("working_content = EXCLUDED.working_content").Set("updated_at = EXCLUDED.updated_at").Exec(ctx)
	return err
}

func (repo *SQLiteSubtitleDocumentRepository) DeleteByFileID(ctx context.Context, fileID string) error {
	_, err := repo.db.NewDelete().Model((*subtitleDocumentRow)(nil)).Where("file_id = ?", strings.TrimSpace(fileID)).Exec(ctx)
	return err
}

func extractFileIDs(rows []operationOutputRow) []string {
	ids := make([]string, 0, len(rows))
	seen := map[string]struct{}{}
	for _, row := range rows {
		if _, ok := seen[row.FileID]; ok {
			continue
		}
		seen[row.FileID] = struct{}{}
		ids = append(ids, row.FileID)
	}
	if len(ids) == 0 {
		return []string{""}
	}
	return ids
}

func mediaFormat(row fileRow) string {
	if row.MediaJSON.Valid {
		decoded := struct {
			Format string `json:"format"`
		}{}
		if json.Unmarshal([]byte(row.MediaJSON.String), &decoded) == nil && strings.TrimSpace(decoded.Format) != "" {
			return decoded.Format
		}
	}
	format := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(pathExt(row.StorageLocalPath.String))), ".")
	if format != "" {
		return format
	}
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(pathExt(row.Name))), ".")
}

func mediaSize(row fileRow) *int64 {
	if row.MediaJSON.Valid {
		decoded := struct {
			SizeBytes *int64 `json:"sizeBytes"`
		}{}
		if json.Unmarshal([]byte(row.MediaJSON.String), &decoded) == nil {
			return decoded.SizeBytes
		}
	}
	return nil
}

func decodeDeleted(row fileRow) bool {
	decoded := struct {
		Deleted bool `json:"deleted"`
	}{}
	if json.Unmarshal([]byte(row.StateJSON), &decoded) == nil {
		return decoded.Deleted
	}
	return false
}

func int64Ptr(value sql.NullInt64) *int64 {
	if value.Valid {
		copied := value.Int64
		return &copied
	}
	return nil
}

func nullInt64Ptr(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func pathExt(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return ""
	}
	return path[idx:]
}
