package libraryrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

type SQLiteLibraryRepository struct{ db *bun.DB }
type SQLiteFileRepository struct{ db *bun.DB }

type libraryRow struct {
	bun.BaseModel `bun:"table:library_libraries"`
	ID            string    `bun:"id,pk"`
	Name          string    `bun:"name"`
	CreatedByJSON string    `bun:"created_by_json"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

type fileRow struct {
	bun.BaseModel        `bun:"table:library_files"`
	ID                   string         `bun:"id,pk"`
	LibraryID            string         `bun:"library_id"`
	Kind                 string         `bun:"kind"`
	Name                 string         `bun:"name"`
	DisplayName          sql.NullString `bun:"display_name"`
	MetadataJSON         sql.NullString `bun:"metadata_json"`
	StorageMode          string         `bun:"storage_mode"`
	StorageLocalPath     sql.NullString `bun:"storage_local_path"`
	StorageDocumentID    sql.NullString `bun:"storage_document_id"`
	OriginKind           string         `bun:"origin_kind"`
	OriginOperationID    sql.NullString `bun:"origin_operation_id"`
	OriginImportBatchID  sql.NullString `bun:"origin_import_batch_id"`
	OriginImportPath     sql.NullString `bun:"origin_import_path"`
	OriginImportedAt     sql.NullTime   `bun:"origin_imported_at"`
	OriginKeepSourceFile sql.NullBool   `bun:"origin_keep_source_file"`
	LineageRootFileID    sql.NullString `bun:"lineage_root_file_id"`
	LatestOperationID    sql.NullString `bun:"latest_operation_id"`
	StateJSON            string         `bun:"state_json"`
	MediaJSON            sql.NullString `bun:"media_json"`
	CreatedAt            time.Time      `bun:"created_at"`
	UpdatedAt            time.Time      `bun:"updated_at"`
}

func NewSQLiteLibraryRepository(db *bun.DB) *SQLiteLibraryRepository {
	return &SQLiteLibraryRepository{db: db}
}
func NewSQLiteFileRepository(db *bun.DB) *SQLiteFileRepository { return &SQLiteFileRepository{db: db} }

func (repo *SQLiteLibraryRepository) List(ctx context.Context) ([]library.Library, error) {
	rows := make([]libraryRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Order("updated_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.Library, 0, len(rows))
	for _, row := range rows {
		item, err := repo.loadLibrary(ctx, row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteLibraryRepository) Get(ctx context.Context, id string) (library.Library, error) {
	row := new(libraryRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Library{}, library.ErrLibraryNotFound
		}
		return library.Library{}, err
	}
	return repo.loadLibrary(ctx, *row)
}

func (repo *SQLiteLibraryRepository) loadLibrary(ctx context.Context, row libraryRow) (library.Library, error) {
	meta := library.CreateMeta{}
	if strings.TrimSpace(row.CreatedByJSON) != "" {
		if err := json.Unmarshal([]byte(row.CreatedByJSON), &meta); err != nil {
			return library.Library{}, err
		}
	}
	return library.NewLibrary(library.LibraryParams{
		ID:        row.ID,
		Name:      row.Name,
		CreatedBy: meta,
		CreatedAt: &row.CreatedAt,
		UpdatedAt: &row.UpdatedAt,
	})
}

func (repo *SQLiteLibraryRepository) Save(ctx context.Context, item library.Library) error {
	createdByJSON, err := json.Marshal(item.CreatedBy)
	if err != nil {
		return err
	}
	row := libraryRow{ID: item.ID, Name: item.Name, CreatedByJSON: string(createdByJSON), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("created_by_json = EXCLUDED.created_by_json").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteLibraryRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*libraryRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func (repo *SQLiteFileRepository) List(ctx context.Context) ([]library.LibraryFile, error) {
	rows := make([]fileRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	return mapFiles(rows)
}

func (repo *SQLiteFileRepository) ListByLibraryID(ctx context.Context, libraryID string) ([]library.LibraryFile, error) {
	rows := make([]fileRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("library_id = ?", strings.TrimSpace(libraryID)).
		Order("created_at DESC").
		Scan(ctx); err != nil {
		return nil, err
	}
	return mapFiles(rows)
}

func (repo *SQLiteFileRepository) Get(ctx context.Context, id string) (library.LibraryFile, error) {
	row := new(fileRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.LibraryFile{}, library.ErrFileNotFound
		}
		return library.LibraryFile{}, err
	}
	return toDomainFile(*row)
}

func (repo *SQLiteFileRepository) Save(ctx context.Context, item library.LibraryFile) error {
	stateJSON, err := json.Marshal(item.State)
	if err != nil {
		return err
	}
	metadataJSON := sql.NullString{}
	if item.Metadata != (library.FileMetadata{}) {
		payload, err := json.Marshal(item.Metadata)
		if err != nil {
			return err
		}
		metadataJSON = nullString(string(payload))
	}
	mediaJSON := sql.NullString{}
	if item.Media != nil {
		payload, err := json.Marshal(item.Media)
		if err != nil {
			return err
		}
		mediaJSON = nullString(string(payload))
	}
	row := fileRow{
		ID:                item.ID,
		LibraryID:         item.LibraryID,
		Kind:              string(item.Kind),
		Name:              item.Name,
		DisplayName:       nullString(item.DisplayName),
		MetadataJSON:      metadataJSON,
		StorageMode:       item.Storage.Mode,
		StorageLocalPath:  nullString(item.Storage.LocalPath),
		StorageDocumentID: nullString(item.Storage.DocumentID),
		OriginKind:        item.Origin.Kind,
		OriginOperationID: nullString(item.Origin.OperationID),
		LineageRootFileID: nullString(item.Lineage.RootFileID),
		LatestOperationID: nullString(item.LatestOperationID),
		StateJSON:         string(stateJSON),
		MediaJSON:         mediaJSON,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
	if item.Origin.Import != nil {
		row.OriginImportBatchID = nullString(item.Origin.Import.BatchID)
		row.OriginImportPath = nullString(item.Origin.Import.ImportPath)
		row.OriginImportedAt = nullTime(&item.Origin.Import.ImportedAt)
		row.OriginKeepSourceFile = sql.NullBool{Bool: item.Origin.Import.KeepSourceFile, Valid: true}
	}
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("library_id = EXCLUDED.library_id").
		Set("kind = EXCLUDED.kind").
		Set("name = EXCLUDED.name").
		Set("display_name = EXCLUDED.display_name").
		Set("metadata_json = EXCLUDED.metadata_json").
		Set("storage_mode = EXCLUDED.storage_mode").
		Set("storage_local_path = EXCLUDED.storage_local_path").
		Set("storage_document_id = EXCLUDED.storage_document_id").
		Set("origin_kind = EXCLUDED.origin_kind").
		Set("origin_operation_id = EXCLUDED.origin_operation_id").
		Set("origin_import_batch_id = EXCLUDED.origin_import_batch_id").
		Set("origin_import_path = EXCLUDED.origin_import_path").
		Set("origin_imported_at = EXCLUDED.origin_imported_at").
		Set("origin_keep_source_file = EXCLUDED.origin_keep_source_file").
		Set("lineage_root_file_id = EXCLUDED.lineage_root_file_id").
		Set("latest_operation_id = EXCLUDED.latest_operation_id").
		Set("state_json = EXCLUDED.state_json").
		Set("media_json = EXCLUDED.media_json").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteFileRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*fileRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func mapFiles(rows []fileRow) ([]library.LibraryFile, error) {
	result := make([]library.LibraryFile, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainFile(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func toDomainFile(row fileRow) (library.LibraryFile, error) {
	state := library.FileState{}
	if err := json.Unmarshal([]byte(row.StateJSON), &state); err != nil {
		return library.LibraryFile{}, err
	}
	var media *library.MediaInfo
	if row.MediaJSON.Valid && strings.TrimSpace(row.MediaJSON.String) != "" {
		decoded := new(library.MediaInfo)
		if err := json.Unmarshal([]byte(row.MediaJSON.String), decoded); err != nil {
			return library.LibraryFile{}, err
		}
		media = decoded
	}
	metadata := library.FileMetadata{}
	if row.MetadataJSON.Valid && strings.TrimSpace(row.MetadataJSON.String) != "" {
		if err := json.Unmarshal([]byte(row.MetadataJSON.String), &metadata); err != nil {
			return library.LibraryFile{}, err
		}
	}
	origin := library.FileOrigin{Kind: row.OriginKind, OperationID: stringOrEmpty(row.OriginOperationID)}
	if row.OriginKind == "import" {
		origin.Import = &library.ImportOrigin{
			BatchID:        stringOrEmpty(row.OriginImportBatchID),
			ImportPath:     stringOrEmpty(row.OriginImportPath),
			ImportedAt:     timeOrZero(row.OriginImportedAt),
			KeepSourceFile: boolOrFalse(row.OriginKeepSourceFile),
		}
	}
	return library.NewLibraryFile(library.LibraryFileParams{
		ID:                row.ID,
		LibraryID:         row.LibraryID,
		Kind:              row.Kind,
		Name:              row.Name,
		DisplayName:       stringOrEmpty(row.DisplayName),
		Storage:           library.FileStorage{Mode: row.StorageMode, LocalPath: stringOrEmpty(row.StorageLocalPath), DocumentID: stringOrEmpty(row.StorageDocumentID)},
		Origin:            origin,
		Lineage:           library.FileLineage{RootFileID: stringOrEmpty(row.LineageRootFileID)},
		Metadata:          metadata,
		LatestOperationID: stringOrEmpty(row.LatestOperationID),
		Media:             media,
		State:             state,
		CreatedAt:         &row.CreatedAt,
		UpdatedAt:         &row.UpdatedAt,
	})
}

func stringOrEmpty(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func boolOrFalse(value sql.NullBool) bool {
	return value.Valid && value.Bool
}

func timeOrNil(value sql.NullTime) *time.Time {
	if value.Valid {
		copyValue := value.Time.UTC()
		return &copyValue
	}
	return nil
}

func timeOrZero(value sql.NullTime) time.Time {
	if value.Valid {
		return value.Time.UTC()
	}
	return time.Time{}
}

func nullString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func nullTime(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}
