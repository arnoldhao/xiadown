package libraryrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

const (
	defaultCatalogChangeLimit = 100
	maximumCatalogChangeLimit = 1000
)

type SQLiteUserStateRepository struct{ db *bun.DB }
type SQLiteDeviceGrantRepository struct{ db *bun.DB }
type SQLiteCatalogChangeRepository struct{ db *bun.DB }
type SQLiteCatalogMigrationRepository struct{ db *bun.DB }

type catalogUserStateRow struct {
	bun.BaseModel `bun:"table:library_user_states"`
	ID            string     `bun:"id,pk"`
	CatalogID     string     `bun:"catalog_id"`
	ItemID        string     `bun:"item_id"`
	UserID        string     `bun:"user_id"`
	Favorite      bool       `bun:"favorite"`
	Rating        int        `bun:"rating"`
	Progress      float64    `bun:"progress"`
	PositionMs    int64      `bun:"position_ms"`
	Locator       string     `bun:"locator"`
	Completed     bool       `bun:"completed"`
	Revision      int64      `bun:"revision"`
	LastOpenedAt  *time.Time `bun:"last_opened_at"`
	CreatedAt     time.Time  `bun:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at"`
}

type catalogDeviceGrantRow struct {
	bun.BaseModel  `bun:"table:library_device_grants"`
	ID             string     `bun:"id,pk"`
	CatalogID      string     `bun:"catalog_id"`
	DeviceID       string     `bun:"device_id"`
	DeviceName     string     `bun:"device_name"`
	CredentialHash string     `bun:"credential_hash"`
	PublicKeyHash  string     `bun:"public_key_hash"`
	ScopesJSON     string     `bun:"scopes_json"`
	Status         string     `bun:"status"`
	ExpiresAt      *time.Time `bun:"expires_at"`
	LastSeenAt     *time.Time `bun:"last_seen_at"`
	RevokedAt      *time.Time `bun:"revoked_at"`
	Revision       int64      `bun:"revision"`
	CreatedAt      time.Time  `bun:"created_at"`
	UpdatedAt      time.Time  `bun:"updated_at"`
}

type catalogChangeRow struct {
	bun.BaseModel `bun:"table:library_catalog_changes"`
	Sequence      int64     `bun:"sequence,pk,autoincrement"`
	CatalogID     string    `bun:"catalog_id"`
	EntityType    string    `bun:"entity_type"`
	EntityID      string    `bun:"entity_id"`
	Kind          string    `bun:"kind"`
	Revision      int64     `bun:"revision"`
	ActorID       string    `bun:"actor_id"`
	OccurredAt    time.Time `bun:"occurred_at"`
}

type catalogLegacyMappingRow struct {
	bun.BaseModel     `bun:"table:library_legacy_mappings"`
	MigrationID       string    `bun:"migration_id,pk"`
	CatalogID         string    `bun:"catalog_id"`
	SourceType        string    `bun:"source_type,pk"`
	SourceID          string    `bun:"source_id,pk"`
	TargetType        string    `bun:"target_type"`
	TargetID          string    `bun:"target_id"`
	SourceFingerprint string    `bun:"source_fingerprint"`
	MigratedAt        time.Time `bun:"migrated_at"`
}

type catalogMigrationCheckpointRow struct {
	bun.BaseModel `bun:"table:library_migration_checkpoints"`
	MigrationID   string     `bun:"migration_id,pk"`
	CatalogID     string     `bun:"catalog_id"`
	Phase         string     `bun:"phase,pk"`
	Status        string     `bun:"status"`
	Cursor        string     `bun:"cursor"`
	Processed     int64      `bun:"processed"`
	Failed        int64      `bun:"failed"`
	LastError     string     `bun:"last_error"`
	StartedAt     *time.Time `bun:"started_at"`
	FinishedAt    *time.Time `bun:"finished_at"`
	CreatedAt     time.Time  `bun:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at"`
}

func NewSQLiteUserStateRepository(db *bun.DB) *SQLiteUserStateRepository {
	return &SQLiteUserStateRepository{db: db}
}

func NewSQLiteDeviceGrantRepository(db *bun.DB) *SQLiteDeviceGrantRepository {
	return &SQLiteDeviceGrantRepository{db: db}
}

func NewSQLiteCatalogChangeRepository(db *bun.DB) *SQLiteCatalogChangeRepository {
	return &SQLiteCatalogChangeRepository{db: db}
}

func NewSQLiteCatalogMigrationRepository(db *bun.DB) *SQLiteCatalogMigrationRepository {
	return &SQLiteCatalogMigrationRepository{db: db}
}

func (repo *SQLiteUserStateRepository) Get(ctx context.Context, catalogID, itemID, userID string) (library.UserState, error) {
	row := new(catalogUserStateRow)
	err := repo.db.NewSelect().Model(row).
		Where("catalog_id = ?", strings.TrimSpace(catalogID)).
		Where("item_id = ?", strings.TrimSpace(itemID)).
		Where("user_id = ?", strings.TrimSpace(userID)).
		Scan(ctx)
	if err != nil {
		return library.UserState{}, err
	}
	return toDomainUserState(*row)
}

func (repo *SQLiteUserStateRepository) Save(ctx context.Context, item library.UserState) error {
	validated, err := library.NewUserState(library.UserStateParams{
		ID: item.ID, CatalogID: item.CatalogID, ItemID: item.ItemID, UserID: item.UserID,
		Favorite: item.Favorite, Rating: item.Rating, Progress: item.Progress, PositionMs: item.PositionMs,
		Locator: item.Locator, Completed: item.Completed, Revision: item.Revision,
		LastOpenedAt: item.LastOpenedAt, CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil {
		return err
	}
	row := catalogUserStateRow{
		ID: validated.ID, CatalogID: validated.CatalogID, ItemID: validated.ItemID, UserID: validated.UserID,
		Favorite: validated.Favorite, Rating: validated.Rating, Progress: validated.Progress,
		PositionMs: validated.PositionMs, Locator: validated.Locator, Completed: validated.Completed,
		Revision: validated.Revision, LastOpenedAt: validated.LastOpenedAt,
		CreatedAt: validated.CreatedAt, UpdatedAt: validated.UpdatedAt,
	}
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("catalog_id = EXCLUDED.catalog_id").
		Set("item_id = EXCLUDED.item_id").
		Set("user_id = EXCLUDED.user_id").
		Set("favorite = EXCLUDED.favorite").
		Set("rating = EXCLUDED.rating").
		Set("progress = EXCLUDED.progress").
		Set("position_ms = EXCLUDED.position_ms").
		Set("locator = EXCLUDED.locator").
		Set("completed = EXCLUDED.completed").
		Set("revision = EXCLUDED.revision").
		Set("last_opened_at = EXCLUDED.last_opened_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteUserStateRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*catalogUserStateRow)(nil)).
		Where("id = ?", strings.TrimSpace(id)).
		Exec(ctx)
	return err
}

func toDomainUserState(row catalogUserStateRow) (library.UserState, error) {
	return library.NewUserState(library.UserStateParams{
		ID: row.ID, CatalogID: row.CatalogID, ItemID: row.ItemID, UserID: row.UserID,
		Favorite: row.Favorite, Rating: row.Rating, Progress: row.Progress, PositionMs: row.PositionMs,
		Locator: row.Locator, Completed: row.Completed, Revision: row.Revision,
		LastOpenedAt: row.LastOpenedAt, CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

func (repo *SQLiteDeviceGrantRepository) ListByCatalogID(ctx context.Context, catalogID string) ([]library.DeviceGrant, error) {
	rows := make([]catalogDeviceGrantRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("catalog_id = ?", strings.TrimSpace(catalogID)).
		Order("updated_at DESC", "id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.DeviceGrant, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainDeviceGrant(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteDeviceGrantRepository) Get(ctx context.Context, id string) (library.DeviceGrant, error) {
	row := new(catalogDeviceGrantRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		return library.DeviceGrant{}, err
	}
	return toDomainDeviceGrant(*row)
}

func (repo *SQLiteDeviceGrantRepository) Save(ctx context.Context, item library.DeviceGrant) error {
	scopeValues := make([]string, len(item.Scopes))
	for index, scope := range item.Scopes {
		scopeValues[index] = string(scope)
	}
	validated, err := library.NewDeviceGrant(library.DeviceGrantParams{
		ID: item.ID, CatalogID: item.CatalogID, DeviceID: item.DeviceID, DeviceName: item.DeviceName,
		CredentialHash: item.CredentialHash, PublicKeyHash: item.PublicKeyHash, Scopes: scopeValues,
		Status: string(item.Status), ExpiresAt: item.ExpiresAt, LastSeenAt: item.LastSeenAt,
		RevokedAt: item.RevokedAt, Revision: item.Revision, CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil {
		return err
	}
	scopesJSON, err := json.Marshal(validated.Scopes)
	if err != nil {
		return err
	}
	row := catalogDeviceGrantRow{
		ID: validated.ID, CatalogID: validated.CatalogID, DeviceID: validated.DeviceID,
		DeviceName: validated.DeviceName, CredentialHash: validated.CredentialHash,
		PublicKeyHash: validated.PublicKeyHash, ScopesJSON: string(scopesJSON), Status: string(validated.Status),
		ExpiresAt: validated.ExpiresAt, LastSeenAt: validated.LastSeenAt, RevokedAt: validated.RevokedAt,
		Revision:  validated.Revision,
		CreatedAt: validated.CreatedAt, UpdatedAt: validated.UpdatedAt,
	}
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("catalog_id = EXCLUDED.catalog_id").
		Set("device_id = EXCLUDED.device_id").
		Set("device_name = EXCLUDED.device_name").
		Set("credential_hash = EXCLUDED.credential_hash").
		Set("public_key_hash = EXCLUDED.public_key_hash").
		Set("scopes_json = EXCLUDED.scopes_json").
		Set("status = EXCLUDED.status").
		Set("expires_at = EXCLUDED.expires_at").
		Set("last_seen_at = EXCLUDED.last_seen_at").
		Set("revoked_at = EXCLUDED.revoked_at").
		Set("revision = EXCLUDED.revision").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteDeviceGrantRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*catalogDeviceGrantRow)(nil)).
		Where("id = ?", strings.TrimSpace(id)).
		Exec(ctx)
	return err
}

func toDomainDeviceGrant(row catalogDeviceGrantRow) (library.DeviceGrant, error) {
	var scopes []string
	if err := json.Unmarshal([]byte(row.ScopesJSON), &scopes); err != nil {
		return library.DeviceGrant{}, fmt.Errorf("decode catalog device grant scopes: %w", err)
	}
	return library.NewDeviceGrant(library.DeviceGrantParams{
		ID: row.ID, CatalogID: row.CatalogID, DeviceID: row.DeviceID, DeviceName: row.DeviceName,
		CredentialHash: row.CredentialHash, PublicKeyHash: row.PublicKeyHash, Scopes: scopes,
		Status: row.Status, ExpiresAt: row.ExpiresAt, LastSeenAt: row.LastSeenAt,
		RevokedAt: row.RevokedAt, Revision: row.Revision, CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

func (repo *SQLiteDeviceGrantRepository) SaveDeviceGrantMutation(
	ctx context.Context,
	item library.DeviceGrant,
	expectedRevision int64,
	kind library.CatalogChangeKind,
	actorID string,
) error {
	scopeValues := make([]string, len(item.Scopes))
	for index, scope := range item.Scopes {
		scopeValues[index] = string(scope)
	}
	validated, err := library.NewDeviceGrant(library.DeviceGrantParams{
		ID: item.ID, CatalogID: item.CatalogID, DeviceID: item.DeviceID, DeviceName: item.DeviceName,
		CredentialHash: item.CredentialHash, PublicKeyHash: item.PublicKeyHash, Scopes: scopeValues,
		Status: string(item.Status), ExpiresAt: item.ExpiresAt, LastSeenAt: item.LastSeenAt,
		RevokedAt: item.RevokedAt, Revision: item.Revision,
		CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil || expectedRevision < 0 ||
		(expectedRevision == 0 && validated.Revision != 1) ||
		(expectedRevision > 0 && validated.Revision != expectedRevision+1) {
		return library.ErrInvalidDeviceGrant
	}
	if kind != library.CatalogChangeUpsert && kind != library.CatalogChangeDelete {
		return library.ErrInvalidCatalogChange
	}
	if kind == library.CatalogChangeDelete && validated.Status != library.DeviceGrantStatusRevoked ||
		kind == library.CatalogChangeUpsert && validated.Status != library.DeviceGrantStatusActive {
		return library.ErrInvalidDeviceGrant
	}
	scopesJSON, err := json.Marshal(validated.Scopes)
	if err != nil {
		return err
	}
	change, err := validateCatalogChangeForAppend(library.CatalogChange{
		CatalogID: validated.CatalogID, EntityType: library.CatalogEntityDeviceGrant,
		EntityID: validated.ID, Kind: kind, Revision: validated.Revision,
		ActorID: strings.TrimSpace(actorID), OccurredAt: validated.UpdatedAt,
	})
	if err != nil {
		return err
	}

	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if expectedRevision == 0 {
			_, err := tx.ExecContext(ctx, `
INSERT INTO library_device_grants (
  id, catalog_id, device_id, device_name, credential_hash, public_key_hash,
  scopes_json, status, expires_at, last_seen_at, revoked_at, revision, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, validated.ID, validated.CatalogID, validated.DeviceID, validated.DeviceName,
				validated.CredentialHash, validated.PublicKeyHash, string(scopesJSON), validated.Status,
				validated.ExpiresAt, validated.LastSeenAt, validated.RevokedAt, validated.Revision,
				validated.CreatedAt, validated.UpdatedAt)
			if err != nil {
				return err
			}
		} else {
			result, err := tx.ExecContext(ctx, `
UPDATE library_device_grants SET
  device_id = ?, device_name = ?, credential_hash = ?, public_key_hash = ?,
  scopes_json = ?, status = ?, expires_at = ?, revoked_at = ?,
  revision = ?, updated_at = ?
WHERE id = ? AND catalog_id = ? AND revision = ?
`, validated.DeviceID, validated.DeviceName, validated.CredentialHash, validated.PublicKeyHash,
				string(scopesJSON), validated.Status, validated.ExpiresAt,
				validated.RevokedAt, validated.Revision, validated.UpdatedAt,
				validated.ID, validated.CatalogID, expectedRevision)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected == 0 {
				return classifyRevisionFailure(ctx, tx, "library_device_grants", validated.ID, validated.CatalogID)
			}
		}
		_, err := appendCatalogChange(ctx, tx, change)
		return err
	})
}

func (repo *SQLiteDeviceGrantRepository) RecordDeviceGrantLastSeen(
	ctx context.Context,
	catalogID string,
	id string,
	seenAt time.Time,
) error {
	if strings.TrimSpace(catalogID) == "" || strings.TrimSpace(id) == "" || seenAt.IsZero() {
		return library.ErrInvalidDeviceGrant
	}
	seenAt = seenAt.UTC()
	cutoff := seenAt.Add(-time.Minute)
	result, err := repo.db.ExecContext(ctx, `
UPDATE library_device_grants
SET last_seen_at = ?
WHERE id = ? AND catalog_id = ? AND status = 'active'
  AND (last_seen_at IS NULL OR last_seen_at < ?)
`, seenAt, strings.TrimSpace(id), strings.TrimSpace(catalogID), cutoff)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	// Zero rows also means a recent successful request already refreshed the
	// timestamp. Treat that throttle case as success; authentication itself has
	// already checked active status and never relies on this telemetry write.
	_ = affected
	return nil
}

// Save appends a change. Sequence is intentionally never inserted: both zero
// and positive input values are treated as transport/read-model values, while
// SQLite AUTOINCREMENT remains the sole allocator of the durable cursor. A
// negative input is rejected as malformed.
func (repo *SQLiteCatalogChangeRepository) Save(ctx context.Context, item library.CatalogChange) error {
	change, err := validateCatalogChangeForAppend(item)
	if err != nil {
		return err
	}
	if change.Kind != library.CatalogChangeDelete {
		_, err = appendCatalogChange(ctx, repo.db, change)
		return err
	}

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	sequence, err := appendCatalogChange(ctx, tx, change)
	if err != nil {
		return err
	}
	if err := upsertCatalogTombstone(ctx, tx, library.Tombstone{
		Sequence: sequence, CatalogID: change.CatalogID, EntityType: change.EntityType,
		EntityID: change.EntityID, Revision: change.Revision, DeletedAt: change.OccurredAt,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (repo *SQLiteCatalogChangeRepository) ListAfter(ctx context.Context, catalogID string, sequence int64, limit int) ([]library.CatalogChange, error) {
	if sequence < 0 {
		return nil, library.ErrInvalidCatalogChange
	}
	if limit <= 0 {
		limit = defaultCatalogChangeLimit
	} else if limit > maximumCatalogChangeLimit {
		limit = maximumCatalogChangeLimit
	}
	rows := make([]catalogChangeRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("catalog_id = ?", strings.TrimSpace(catalogID)).
		Where("sequence > ?", sequence).
		Order("sequence ASC").
		Limit(limit).
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.CatalogChange, 0, len(rows))
	for _, row := range rows {
		item, err := library.NewCatalogChange(library.CatalogChangeParams{
			Sequence: row.Sequence, CatalogID: row.CatalogID, EntityType: row.EntityType,
			EntityID: row.EntityID, Kind: row.Kind, Revision: row.Revision,
			ActorID: row.ActorID, OccurredAt: row.OccurredAt,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// SaveTombstone appends its delete change and writes the tombstone in one
// transaction. As with Save, the supplied Sequence is ignored unless it is
// negative; SQLite allocates the change cursor and that cursor becomes the
// tombstone primary key.
func (repo *SQLiteCatalogChangeRepository) SaveTombstone(ctx context.Context, item library.Tombstone) error {
	if item.Sequence < 0 {
		return library.ErrInvalidTombstone
	}
	tombstone, err := library.NewTombstone(library.TombstoneParams{
		Sequence: 1, CatalogID: item.CatalogID, EntityType: string(item.EntityType),
		EntityID: item.EntityID, Revision: item.Revision, DeletedAt: item.DeletedAt,
		ExpiresAt: item.ExpiresAt,
	})
	if err != nil {
		return err
	}
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	sequence, err := appendCatalogChange(ctx, tx, library.CatalogChange{
		CatalogID: tombstone.CatalogID, EntityType: tombstone.EntityType,
		EntityID: tombstone.EntityID, Kind: library.CatalogChangeDelete,
		Revision: tombstone.Revision, OccurredAt: tombstone.DeletedAt,
	})
	if err != nil {
		return err
	}
	tombstone.Sequence = sequence
	if err := upsertCatalogTombstone(ctx, tx, tombstone); err != nil {
		return err
	}
	return tx.Commit()
}

func (repo *SQLiteCatalogChangeRepository) DeleteExpiredTombstones(ctx context.Context, before time.Time) (int, error) {
	if before.IsZero() {
		return 0, nil
	}
	result, err := repo.db.ExecContext(ctx, `
DELETE FROM library_catalog_tombstones
WHERE expires_at IS NOT NULL AND expires_at < ?
`, before.UTC())
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	return int(count), err
}

type catalogChangeExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func validateCatalogChangeForAppend(item library.CatalogChange) (library.CatalogChange, error) {
	if item.Sequence < 0 {
		return library.CatalogChange{}, library.ErrInvalidCatalogChange
	}
	return library.NewCatalogChange(library.CatalogChangeParams{
		Sequence: 1, CatalogID: item.CatalogID, EntityType: string(item.EntityType),
		EntityID: item.EntityID, Kind: string(item.Kind), Revision: item.Revision,
		ActorID: item.ActorID, OccurredAt: item.OccurredAt,
	})
}

func appendCatalogChange(ctx context.Context, executor catalogChangeExecutor, item library.CatalogChange) (int64, error) {
	result, err := executor.ExecContext(ctx, `
INSERT INTO library_catalog_changes (
  catalog_id, entity_type, entity_id, kind, revision, actor_id, occurred_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
`, item.CatalogID, item.EntityType, item.EntityID, item.Kind, item.Revision, item.ActorID, item.OccurredAt.UTC())
	if err != nil {
		return 0, err
	}
	sequence, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read appended catalog change sequence: %w", err)
	}
	if sequence <= 0 {
		return 0, errors.New("sqlite returned a non-positive catalog change sequence")
	}
	return sequence, nil
}

func upsertCatalogTombstone(ctx context.Context, executor catalogChangeExecutor, item library.Tombstone) error {
	_, err := executor.ExecContext(ctx, `
INSERT INTO library_catalog_tombstones (
  sequence, catalog_id, entity_type, entity_id, revision, deleted_at, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(catalog_id, entity_type, entity_id) DO UPDATE SET
  sequence = EXCLUDED.sequence,
  revision = EXCLUDED.revision,
  deleted_at = EXCLUDED.deleted_at,
  expires_at = EXCLUDED.expires_at
`, item.Sequence, item.CatalogID, item.EntityType, item.EntityID, item.Revision, item.DeletedAt.UTC(), item.ExpiresAt)
	return err
}

func (repo *SQLiteCatalogMigrationRepository) GetMapping(ctx context.Context, migrationID string, sourceType library.LegacyEntityType, sourceID string) (library.LegacyMapping, error) {
	row := new(catalogLegacyMappingRow)
	if err := repo.db.NewSelect().Model(row).
		Where("migration_id = ?", strings.TrimSpace(migrationID)).
		Where("source_type = ?", sourceType).
		Where("source_id = ?", strings.TrimSpace(sourceID)).
		Scan(ctx); err != nil {
		return library.LegacyMapping{}, err
	}
	return toDomainLegacyMapping(*row)
}

func (repo *SQLiteCatalogMigrationRepository) SaveMapping(ctx context.Context, item library.LegacyMapping) error {
	validated, err := library.NewLegacyMapping(library.LegacyMappingParams{
		MigrationID: item.MigrationID, CatalogID: item.CatalogID, SourceType: string(item.SourceType),
		SourceID: item.SourceID, TargetType: string(item.TargetType), TargetID: item.TargetID,
		SourceFingerprint: item.SourceFingerprint, MigratedAt: item.MigratedAt,
	})
	if err != nil {
		return err
	}
	row := catalogLegacyMappingRow{
		MigrationID: validated.MigrationID, CatalogID: validated.CatalogID,
		SourceType: string(validated.SourceType), SourceID: validated.SourceID,
		TargetType: string(validated.TargetType), TargetID: validated.TargetID,
		SourceFingerprint: validated.SourceFingerprint, MigratedAt: validated.MigratedAt,
	}
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(migration_id, source_type, source_id) DO UPDATE").
		Set("catalog_id = EXCLUDED.catalog_id").
		Set("target_type = EXCLUDED.target_type").
		Set("target_id = EXCLUDED.target_id").
		Set("source_fingerprint = EXCLUDED.source_fingerprint").
		Set("migrated_at = EXCLUDED.migrated_at").
		Exec(ctx)
	return err
}

func toDomainLegacyMapping(row catalogLegacyMappingRow) (library.LegacyMapping, error) {
	return library.NewLegacyMapping(library.LegacyMappingParams{
		MigrationID: row.MigrationID, CatalogID: row.CatalogID, SourceType: row.SourceType,
		SourceID: row.SourceID, TargetType: row.TargetType, TargetID: row.TargetID,
		SourceFingerprint: row.SourceFingerprint, MigratedAt: row.MigratedAt,
	})
}

func (repo *SQLiteCatalogMigrationRepository) GetCheckpoint(ctx context.Context, migrationID string, phase library.MigrationPhase) (library.MigrationCheckpoint, error) {
	row := new(catalogMigrationCheckpointRow)
	if err := repo.db.NewSelect().Model(row).
		Where("migration_id = ?", strings.TrimSpace(migrationID)).
		Where("phase = ?", phase).
		Scan(ctx); err != nil {
		return library.MigrationCheckpoint{}, err
	}
	return toDomainMigrationCheckpoint(*row)
}

func (repo *SQLiteCatalogMigrationRepository) SaveCheckpoint(ctx context.Context, item library.MigrationCheckpoint) error {
	validated, err := library.NewMigrationCheckpoint(library.MigrationCheckpointParams{
		MigrationID: item.MigrationID, CatalogID: item.CatalogID, Phase: string(item.Phase),
		Status: string(item.Status), Cursor: item.Cursor, Processed: item.Processed, Failed: item.Failed,
		LastError: item.LastError, StartedAt: item.StartedAt, FinishedAt: item.FinishedAt,
		CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil {
		return err
	}
	row := catalogMigrationCheckpointRow{
		MigrationID: validated.MigrationID, CatalogID: validated.CatalogID,
		Phase: string(validated.Phase), Status: string(validated.Status), Cursor: validated.Cursor,
		Processed: validated.Processed, Failed: validated.Failed, LastError: validated.LastError,
		StartedAt: validated.StartedAt, FinishedAt: validated.FinishedAt,
		CreatedAt: validated.CreatedAt, UpdatedAt: validated.UpdatedAt,
	}
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(migration_id, phase) DO UPDATE").
		Set("catalog_id = EXCLUDED.catalog_id").
		Set("status = EXCLUDED.status").
		Set("cursor = EXCLUDED.cursor").
		Set("processed = EXCLUDED.processed").
		Set("failed = EXCLUDED.failed").
		Set("last_error = EXCLUDED.last_error").
		Set("started_at = EXCLUDED.started_at").
		Set("finished_at = EXCLUDED.finished_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func toDomainMigrationCheckpoint(row catalogMigrationCheckpointRow) (library.MigrationCheckpoint, error) {
	return library.NewMigrationCheckpoint(library.MigrationCheckpointParams{
		MigrationID: row.MigrationID, CatalogID: row.CatalogID, Phase: row.Phase,
		Status: row.Status, Cursor: row.Cursor, Processed: row.Processed, Failed: row.Failed,
		LastError: row.LastError, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt,
		CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

var (
	_ library.UserStateRepository             = (*SQLiteUserStateRepository)(nil)
	_ library.DeviceGrantRepository           = (*SQLiteDeviceGrantRepository)(nil)
	_ library.DeviceGrantManagementRepository = (*SQLiteDeviceGrantRepository)(nil)
	_ library.CatalogChangeRepository         = (*SQLiteCatalogChangeRepository)(nil)
	_ library.CatalogMigrationRepository      = (*SQLiteCatalogMigrationRepository)(nil)
)
