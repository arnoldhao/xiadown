package libraryrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

type SQLiteCatalogRepository struct{ db *bun.DB }
type SQLiteCatalogItemRepository struct{ db *bun.DB }
type SQLiteItemAssetRepository struct{ db *bun.DB }
type SQLiteStorageRootRepository struct{ db *bun.DB }
type SQLiteCatalogCollectionRepository struct{ db *bun.DB }
type SQLiteCatalogTagRepository struct{ db *bun.DB }

type catalogRow struct {
	bun.BaseModel `bun:"table:library_catalogs"`
	ID            string    `bun:"id,pk"`
	Name          string    `bun:"name"`
	Description   string    `bun:"description"`
	Status        string    `bun:"status"`
	IsDefault     bool      `bun:"is_default"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

type catalogItemRow struct {
	bun.BaseModel `bun:"table:library_catalog_items"`
	ID            string     `bun:"id,pk"`
	CatalogID     string     `bun:"catalog_id"`
	Category      string     `bun:"category"`
	Status        string     `bun:"status"`
	Title         string     `bun:"title"`
	SortTitle     string     `bun:"sort_title"`
	Description   string     `bun:"description"`
	Revision      int64      `bun:"revision"`
	TrashedAt     *time.Time `bun:"trashed_at"`
	CreatedAt     time.Time  `bun:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at"`
}

type itemAssetRow struct {
	bun.BaseModel `bun:"table:library_item_assets"`
	ID            string    `bun:"id,pk"`
	ItemID        string    `bun:"item_id"`
	FileID        string    `bun:"file_id"`
	Role          string    `bun:"role"`
	Label         string    `bun:"label"`
	Position      int       `bun:"position"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

type catalogItemPresentationAssetRow struct {
	ItemID          string         `bun:"item_id"`
	AssetID         string         `bun:"asset_id"`
	FileID          string         `bun:"file_id"`
	Role            string         `bun:"role"`
	Position        int            `bun:"position"`
	FileKind        string         `bun:"file_kind"`
	LocalPath       sql.NullString `bun:"local_path"`
	FileStateJSON   string         `bun:"file_state_json"`
	FileMediaJSON   sql.NullString `bun:"file_media_json"`
	FileUpdatedAt   time.Time      `bun:"file_updated_at"`
	StorageRootID   sql.NullString `bun:"storage_root_id"`
	StorageRootMode sql.NullString `bun:"storage_root_mode"`
	RootStatus      sql.NullString `bun:"root_status"`
	SyncEntryStatus string         `bun:"sync_entry_status"`
	SyncStateStatus sql.NullString `bun:"sync_state_status"`
	PreviewArtwork  bool           `bun:"preview_artwork"`
}

type storageRootRow struct {
	bun.BaseModel `bun:"table:library_storage_roots"`
	ID            string     `bun:"id,pk"`
	CatalogID     string     `bun:"catalog_id"`
	Name          string     `bun:"name"`
	Emoji         string     `bun:"emoji"`
	Path          string     `bun:"path"`
	VolumeID      string     `bun:"volume_id"`
	Mode          string     `bun:"mode"`
	IsDefault     bool       `bun:"is_default"`
	Status        string     `bun:"status"`
	LastCheckedAt *time.Time `bun:"last_checked_at"`
	LastError     string     `bun:"last_error"`
	CreatedAt     time.Time  `bun:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at"`
}

type catalogCollectionRow struct {
	bun.BaseModel `bun:"table:library_collections"`
	ID            string    `bun:"id,pk"`
	CatalogID     string    `bun:"catalog_id"`
	Name          string    `bun:"name"`
	Description   string    `bun:"description"`
	Kind          string    `bun:"kind"`
	SmartQuery    string    `bun:"smart_query"`
	Revision      int64     `bun:"revision"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

type catalogCollectionItemRow struct {
	bun.BaseModel `bun:"table:library_collection_items"`
	ID            string    `bun:"id,pk"`
	CollectionID  string    `bun:"collection_id"`
	ItemID        string    `bun:"item_id"`
	Position      int       `bun:"position"`
	AddedAt       time.Time `bun:"added_at"`
}

type catalogTagRow struct {
	bun.BaseModel  `bun:"table:library_tags"`
	ID             string    `bun:"id,pk"`
	CatalogID      string    `bun:"catalog_id"`
	Name           string    `bun:"name"`
	NormalizedName string    `bun:"normalized_name"`
	CreatedAt      time.Time `bun:"created_at"`
	UpdatedAt      time.Time `bun:"updated_at"`
}

type catalogItemTagRow struct {
	bun.BaseModel `bun:"table:library_item_tags"`
	ID            string    `bun:"id,pk"`
	ItemID        string    `bun:"item_id"`
	TagID         string    `bun:"tag_id"`
	AddedBy       string    `bun:"added_by"`
	AddedAt       time.Time `bun:"added_at"`
}

func NewSQLiteCatalogRepository(db *bun.DB) *SQLiteCatalogRepository {
	return &SQLiteCatalogRepository{db: db}
}

func NewSQLiteCatalogItemRepository(db *bun.DB) *SQLiteCatalogItemRepository {
	return &SQLiteCatalogItemRepository{db: db}
}

func NewSQLiteItemAssetRepository(db *bun.DB) *SQLiteItemAssetRepository {
	return &SQLiteItemAssetRepository{db: db}
}

func NewSQLiteStorageRootRepository(db *bun.DB) *SQLiteStorageRootRepository {
	return &SQLiteStorageRootRepository{db: db}
}

func NewSQLiteCatalogCollectionRepository(db *bun.DB) *SQLiteCatalogCollectionRepository {
	return &SQLiteCatalogCollectionRepository{db: db}
}

func NewSQLiteCatalogTagRepository(db *bun.DB) *SQLiteCatalogTagRepository {
	return &SQLiteCatalogTagRepository{db: db}
}

func (repo *SQLiteCatalogRepository) List(ctx context.Context) ([]library.Catalog, error) {
	rows := make([]catalogRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		OrderExpr("is_default DESC").
		OrderExpr("name COLLATE NOCASE ASC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.Catalog, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainCatalog(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteCatalogRepository) Get(ctx context.Context, id string) (library.Catalog, error) {
	row := new(catalogRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		return library.Catalog{}, err
	}
	return toDomainCatalog(*row)
}

func (repo *SQLiteCatalogRepository) Save(ctx context.Context, item library.Catalog) error {
	row := catalogRow{
		ID: item.ID, Name: item.Name, Description: item.Description, Status: string(item.Status),
		IsDefault: item.IsDefault, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("description = EXCLUDED.description").
		Set("status = EXCLUDED.status").
		Set("is_default = EXCLUDED.is_default").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteCatalogRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*catalogRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func (repo *SQLiteCatalogItemRepository) ListByCatalogID(ctx context.Context, catalogID string) ([]library.Item, error) {
	rows := make([]catalogItemRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("catalog_id = ?", strings.TrimSpace(catalogID)).
		OrderExpr("updated_at DESC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.Item, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainCatalogItem(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteCatalogItemRepository) ListStorageScopedByCatalogID(
	ctx context.Context,
	catalogID string,
) ([]library.Item, error) {
	rows := make([]catalogItemRow, 0)
	if err := repo.storageScopedCatalogItemsQuery(&rows, catalogID).
		OrderExpr("updated_at DESC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	return mapCatalogItemRows(rows)
}

func (repo *SQLiteCatalogItemRepository) ListCatalogItemsPage(
	ctx context.Context,
	catalogID string,
	page library.CatalogItemPageQuery,
) ([]library.Item, int, error) {
	if page.Limit <= 0 || page.Offset < 0 {
		return []library.Item{}, 0, nil
	}
	countQuery := repo.catalogItemPageQuery(
		(*catalogItemRow)(nil),
		catalogID,
		page,
	)
	total, err := countQuery.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	if page.Offset >= total {
		return []library.Item{}, total, nil
	}

	rows := make([]catalogItemRow, 0, page.Limit)
	query := repo.catalogItemPageQuery(&rows, catalogID, page)
	switch page.Sort {
	case "created_desc":
		query = query.OrderExpr("created_at DESC").OrderExpr("id ASC")
	case "created_asc":
		query = query.OrderExpr("created_at ASC").OrderExpr("id ASC")
	case "title_asc":
		query = query.OrderExpr("sort_title COLLATE NOCASE ASC").OrderExpr("id ASC")
	case "title_desc":
		query = query.OrderExpr("sort_title COLLATE NOCASE DESC").OrderExpr("id ASC")
	case "category_asc":
		query = query.OrderExpr("category ASC").
			OrderExpr("sort_title COLLATE NOCASE ASC").
			OrderExpr("id ASC")
	default:
		query = query.OrderExpr("updated_at DESC").OrderExpr("id ASC")
	}
	if err := query.Limit(page.Limit).Offset(page.Offset).Scan(ctx); err != nil {
		return nil, 0, err
	}
	items, err := mapCatalogItemRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (repo *SQLiteCatalogItemRepository) catalogItemPageQuery(
	model any,
	catalogID string,
	page library.CatalogItemPageQuery,
) *bun.SelectQuery {
	var query *bun.SelectQuery
	if page.StorageScoped {
		query = repo.storageScopedCatalogItemsQuery(model, catalogID)
	} else {
		query = repo.db.NewSelect().Model(model).
			Where("catalog_id = ?", strings.TrimSpace(catalogID))
	}
	if category := strings.TrimSpace(page.Category); category != "" {
		query = query.Where("category = ?", category)
	}
	if status := strings.TrimSpace(page.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if page.ExcludeTrashed {
		query = query.Where("status <> 'trashed'").Where("trashed_at IS NULL")
	}
	if search := strings.TrimSpace(page.Query); search != "" {
		pattern := catalogItemLikePattern(search)
		query = query.Where(
			"(LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(description) LIKE ? ESCAPE '\\')",
			pattern,
			pattern,
		)
	}
	return query
}

func catalogItemLikePattern(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	).Replace(value)
	return "%" + value + "%"
}

// ListSnapshotPageByCatalogID uses the immutable Item identity as a keyset.
// The public snapshot handler binds afterID to one epoch/high-water cursor and
// verifies that cursor again after the page has been materialized.
func (repo *SQLiteCatalogItemRepository) ListSnapshotPageByCatalogID(
	ctx context.Context,
	catalogID string,
	afterID string,
	limit int,
) ([]library.Item, error) {
	if limit <= 0 {
		return []library.Item{}, nil
	}
	rows := make([]catalogItemRow, 0, limit)
	query := repo.db.NewSelect().Model(&rows).
		Where("catalog_id = ?", strings.TrimSpace(catalogID)).
		Where("status <> 'trashed'").
		Where("trashed_at IS NULL")
	if afterID = strings.TrimSpace(afterID); afterID != "" {
		query = query.Where("id > ?", afterID)
	}
	if err := query.OrderExpr("id ASC").Limit(limit).Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.Item, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainCatalogItem(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteCatalogItemRepository) ListStorageScopedSnapshotPageByCatalogID(
	ctx context.Context,
	catalogID string,
	afterID string,
	limit int,
) ([]library.Item, error) {
	if limit <= 0 {
		return []library.Item{}, nil
	}
	rows := make([]catalogItemRow, 0, limit)
	query := repo.storageScopedCatalogItemsQuery(&rows, catalogID).
		Where("status <> 'trashed'").
		Where("trashed_at IS NULL")
	if afterID = strings.TrimSpace(afterID); afterID != "" {
		query = query.Where("id > ?", afterID)
	}
	if err := query.OrderExpr("id ASC").Limit(limit).Scan(ctx); err != nil {
		return nil, err
	}
	return mapCatalogItemRows(rows)
}

func (repo *SQLiteCatalogItemRepository) storageScopedCatalogItemsQuery(
	model any,
	catalogID string,
) *bun.SelectQuery {
	catalogID = strings.TrimSpace(catalogID)
	return repo.db.NewSelect().Model(model).
		ModelTableExpr("library_catalog_items AS catalog_item_row").
		Where("catalog_item_row.catalog_id = ?", catalogID).
		Where(`
EXISTS (
  SELECT 1
  FROM library_item_assets AS asset
  JOIN library_files AS file ON file.id = asset.file_id
  JOIN library_storage_roots AS root ON root.id = file.storage_root_id
  WHERE asset.item_id = catalog_item_row.id
    AND root.catalog_id = catalog_item_row.catalog_id
    AND asset.role IN ('original', 'representation')
)`)
}

func mapCatalogItemRows(rows []catalogItemRow) ([]library.Item, error) {
	result := make([]library.Item, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainCatalogItem(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteCatalogItemRepository) Get(ctx context.Context, id string) (library.Item, error) {
	row := new(catalogItemRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		return library.Item{}, err
	}
	return toDomainCatalogItem(*row)
}

func (repo *SQLiteCatalogItemRepository) Save(ctx context.Context, item library.Item) error {
	row := catalogItemRow{
		ID: item.ID, CatalogID: item.CatalogID, Category: string(item.Category), Status: string(item.Status),
		Title: item.Title, SortTitle: item.SortTitle, Description: item.Description, Revision: item.Revision,
		TrashedAt: item.TrashedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("catalog_id = EXCLUDED.catalog_id").
		Set("category = EXCLUDED.category").
		Set("status = EXCLUDED.status").
		Set("title = EXCLUDED.title").
		Set("sort_title = EXCLUDED.sort_title").
		Set("description = EXCLUDED.description").
		Set("revision = EXCLUDED.revision").
		Set("trashed_at = EXCLUDED.trashed_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteCatalogItemRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*catalogItemRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func (repo *SQLiteItemAssetRepository) ListByItemID(ctx context.Context, itemID string) ([]library.ItemAsset, error) {
	rows := make([]itemAssetRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("item_id = ?", strings.TrimSpace(itemID)).
		OrderExpr("position ASC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.ItemAsset, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainItemAsset(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteItemAssetRepository) ListCatalogItemPresentationAssets(
	ctx context.Context,
	itemIDs []string,
) ([]library.CatalogItemPresentationAsset, error) {
	normalized := make([]string, 0, len(itemIDs))
	seen := make(map[string]struct{}, len(itemIDs))
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		if _, duplicate := seen[itemID]; duplicate {
			continue
		}
		seen[itemID] = struct{}{}
		normalized = append(normalized, itemID)
	}
	if len(normalized) == 0 {
		return []library.CatalogItemPresentationAsset{}, nil
	}

	rows := make([]catalogItemPresentationAssetRow, 0, len(normalized))
	err := repo.db.NewSelect().
		TableExpr("library_item_assets AS asset").
		ColumnExpr("asset.item_id AS item_id").
		ColumnExpr("asset.id AS asset_id").
		ColumnExpr("asset.file_id AS file_id").
		ColumnExpr("asset.role AS role").
		ColumnExpr("asset.position AS position").
		ColumnExpr("file.kind AS file_kind").
		ColumnExpr("file.storage_local_path AS local_path").
		ColumnExpr("file.state_json AS file_state_json").
		ColumnExpr("file.media_json AS file_media_json").
		ColumnExpr("file.updated_at AS file_updated_at").
		ColumnExpr("root.id AS storage_root_id").
		ColumnExpr("root.mode AS storage_root_mode").
		ColumnExpr("root.status AS root_status").
		ColumnExpr(`
CASE
  WHEN MAX(CASE WHEN entry.status = 'active' THEN 1 ELSE 0 END) = 1 THEN 'active'
  WHEN MAX(CASE WHEN entry.status = 'failed' THEN 1 ELSE 0 END) = 1 THEN 'failed'
  WHEN MAX(CASE WHEN entry.status = 'missing' THEN 1 ELSE 0 END) = 1 THEN 'missing'
  ELSE ''
END AS sync_entry_status`).
		ColumnExpr("sync_state.status AS sync_state_status").
		ColumnExpr(`
MAX(CASE
  WHEN representation.availability = 'available'
   AND (
     representation.kind IN ('artwork', 'thumbnail')
     OR representation.purpose = 'artwork'
   )
  THEN 1 ELSE 0
END) AS preview_artwork`).
		Join("JOIN library_files AS file ON file.id = asset.file_id").
		Join("LEFT JOIN library_storage_roots AS root ON root.id = file.storage_root_id").
		Join(`
LEFT JOIN library_storage_root_sync_entries AS entry
  ON entry.file_id = file.id
 AND entry.root_id = file.storage_root_id`).
		Join(`
LEFT JOIN library_storage_root_sync_states AS sync_state
  ON sync_state.root_id = file.storage_root_id`).
		Join(`
LEFT JOIN library_representations AS representation
  ON representation.asset_id = asset.id
 AND representation.item_id = asset.item_id`).
		Where("asset.item_id IN (?)", bun.In(normalized)).
		GroupExpr("asset.id").
		OrderExpr(`
asset.item_id ASC,
CASE asset.role
  WHEN 'original' THEN 0
  WHEN 'representation' THEN 1
  WHEN 'artwork' THEN 2
  ELSE 3
END ASC,
asset.position ASC,
asset.id ASC`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	result := make([]library.CatalogItemPresentationAsset, 0, len(rows))
	for _, row := range rows {
		state := library.FileState{}
		if err := json.Unmarshal([]byte(row.FileStateJSON), &state); err != nil {
			return nil, err
		}
		var media *library.MediaInfo
		if row.FileMediaJSON.Valid && strings.TrimSpace(row.FileMediaJSON.String) != "" {
			media = new(library.MediaInfo)
			if err := json.Unmarshal([]byte(row.FileMediaJSON.String), media); err != nil {
				return nil, err
			}
		}
		result = append(result, library.CatalogItemPresentationAsset{
			ItemID: row.ItemID, AssetID: row.AssetID, FileID: row.FileID,
			Role: library.ItemAssetRole(row.Role), Position: row.Position,
			Kind: library.FileKind(row.FileKind), LocalPath: stringOrEmpty(row.LocalPath),
			FileState: state, Media: media, FileUpdatedAt: row.FileUpdatedAt,
			StorageRootID:   stringOrEmpty(row.StorageRootID),
			StorageRootMode: library.StorageRootMode(stringOrEmpty(row.StorageRootMode)),
			RootStatus:      library.StorageRootStatus(stringOrEmpty(row.RootStatus)),
			SyncEntryStatus: row.SyncEntryStatus,
			SyncStateStatus: stringOrEmpty(row.SyncStateStatus),
			PreviewArtwork:  row.PreviewArtwork,
		})
	}
	return result, nil
}

func (repo *SQLiteItemAssetRepository) Get(ctx context.Context, id string) (library.ItemAsset, error) {
	row := new(itemAssetRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		return library.ItemAsset{}, err
	}
	return toDomainItemAsset(*row)
}

func (repo *SQLiteItemAssetRepository) Save(ctx context.Context, item library.ItemAsset) error {
	row := itemAssetRow{
		ID: item.ID, ItemID: item.ItemID, FileID: item.FileID, Role: string(item.Role), Label: item.Label,
		Position: item.Position, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("item_id = EXCLUDED.item_id").
		Set("file_id = EXCLUDED.file_id").
		Set("role = EXCLUDED.role").
		Set("label = EXCLUDED.label").
		Set("position = EXCLUDED.position").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteItemAssetRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*itemAssetRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func (repo *SQLiteStorageRootRepository) ListByCatalogID(ctx context.Context, catalogID string) ([]library.StorageRoot, error) {
	rows := make([]storageRootRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("catalog_id = ?", strings.TrimSpace(catalogID)).
		OrderExpr("name COLLATE NOCASE ASC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.StorageRoot, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainStorageRoot(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteStorageRootRepository) Get(ctx context.Context, id string) (library.StorageRoot, error) {
	row := new(storageRootRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		return library.StorageRoot{}, err
	}
	return toDomainStorageRoot(*row)
}

func (repo *SQLiteStorageRootRepository) Save(ctx context.Context, item library.StorageRoot) error {
	row := storageRootRow{
		ID: item.ID, CatalogID: item.CatalogID, Name: item.Name, Emoji: item.Emoji, Path: item.Path, VolumeID: item.VolumeID,
		Mode: string(item.Mode), IsDefault: item.IsDefault, Status: string(item.Status), LastCheckedAt: item.LastCheckedAt,
		LastError: item.LastError, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	return saveStorageRootRow(ctx, repo.db, &row)
}

func saveStorageRootRow(ctx context.Context, db bun.IDB, row *storageRootRow) error {
	_, err := db.NewInsert().Model(row).
		On("CONFLICT(id) DO UPDATE").
		Set("catalog_id = EXCLUDED.catalog_id").
		Set("name = EXCLUDED.name").
		Set("emoji = EXCLUDED.emoji").
		Set("path = EXCLUDED.path").
		Set("volume_id = EXCLUDED.volume_id").
		Set("mode = EXCLUDED.mode").
		Set("is_default = EXCLUDED.is_default").
		Set("status = EXCLUDED.status").
		Set("last_checked_at = EXCLUDED.last_checked_at").
		Set("last_error = EXCLUDED.last_error").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

// SaveAsDefault switches the write target atomically so the partial unique
// index can never observe two defaults or an avoidable default-less window.
func (repo *SQLiteStorageRootRepository) SaveAsDefault(ctx context.Context, item library.StorageRoot) error {
	if !item.IsDefault || item.Mode != library.StorageRootModeManaged {
		return library.ErrInvalidStorageRoot
	}
	row := storageRootRow{
		ID: item.ID, CatalogID: item.CatalogID, Name: item.Name, Emoji: item.Emoji, Path: item.Path, VolumeID: item.VolumeID,
		Mode: string(item.Mode), IsDefault: true, Status: string(item.Status), LastCheckedAt: item.LastCheckedAt,
		LastError: item.LastError, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().
			Model((*storageRootRow)(nil)).
			Set("is_default = FALSE").
			Where("catalog_id = ?", item.CatalogID).
			Where("id <> ?", item.ID).
			Exec(ctx); err != nil {
			return err
		}
		return saveStorageRootRow(ctx, tx, &row)
	})
}

// SaveRelocatingFiles updates the root mount point and the absolute
// compatibility cache together. Root-relative ownership remains authoritative.
func (repo *SQLiteStorageRootRepository) SaveRelocatingFiles(
	ctx context.Context,
	item library.StorageRoot,
	filePaths map[string]string,
) error {
	row := storageRootRow{
		ID: item.ID, CatalogID: item.CatalogID, Name: item.Name, Emoji: item.Emoji, Path: item.Path, VolumeID: item.VolumeID,
		Mode: string(item.Mode), IsDefault: item.IsDefault, Status: string(item.Status), LastCheckedAt: item.LastCheckedAt,
		LastError: item.LastError, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if item.IsDefault {
			if _, err := tx.NewUpdate().
				Model((*storageRootRow)(nil)).
				Set("is_default = FALSE").
				Where("catalog_id = ?", item.CatalogID).
				Where("id <> ?", item.ID).
				Exec(ctx); err != nil {
				return err
			}
		}
		if err := saveStorageRootRow(ctx, tx, &row); err != nil {
			return err
		}
		for fileID, localPath := range filePaths {
			result, err := tx.NewUpdate().
				Model((*fileRow)(nil)).
				Set("storage_local_path = ?", nullString(localPath)).
				Where("id = ?", strings.TrimSpace(fileID)).
				Where("storage_root_id = ?", item.ID).
				Exec(ctx)
			if err != nil {
				return err
			}
			updated, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if updated == 0 {
				continue
			}
			if _, err := tx.NewUpdate().
				Model((*listenLocalTrackRow)(nil)).
				Set("local_path = ?", localPath).
				Where("file_id = ?", strings.TrimSpace(fileID)).
				Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repo *SQLiteStorageRootRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*storageRootRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func (repo *SQLiteCatalogCollectionRepository) ListByCatalogID(ctx context.Context, catalogID string) ([]library.Collection, error) {
	rows := make([]catalogCollectionRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("catalog_id = ?", strings.TrimSpace(catalogID)).
		OrderExpr("updated_at DESC").
		OrderExpr("name COLLATE NOCASE ASC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.Collection, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainCatalogCollection(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteCatalogCollectionRepository) ListByCatalogIDPage(
	ctx context.Context,
	catalogID string,
	limit int,
	offset int,
) ([]library.Collection, error) {
	rows := make([]catalogCollectionRow, 0, limit)
	if err := repo.db.NewSelect().Model(&rows).
		Where("catalog_id = ?", strings.TrimSpace(catalogID)).
		OrderExpr("updated_at DESC").
		OrderExpr("name COLLATE NOCASE ASC").
		OrderExpr("id ASC").
		Limit(limit).
		Offset(offset).
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.Collection, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainCatalogCollection(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteCatalogCollectionRepository) Get(ctx context.Context, id string) (library.Collection, error) {
	row := new(catalogCollectionRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		return library.Collection{}, err
	}
	return toDomainCatalogCollection(*row)
}

func (repo *SQLiteCatalogCollectionRepository) Save(ctx context.Context, item library.Collection) error {
	row := toCatalogCollectionRow(item)
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("catalog_id = EXCLUDED.catalog_id").
		Set("name = EXCLUDED.name").
		Set("description = EXCLUDED.description").
		Set("kind = EXCLUDED.kind").
		Set("smart_query = EXCLUDED.smart_query").
		Set("revision = EXCLUDED.revision").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteCatalogCollectionRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*catalogCollectionRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func (repo *SQLiteCatalogCollectionRepository) ListItems(ctx context.Context, collectionID string) ([]library.CollectionItem, error) {
	rows := make([]catalogCollectionItemRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("collection_id = ?", strings.TrimSpace(collectionID)).
		OrderExpr("position ASC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.CollectionItem, 0, len(rows))
	for _, row := range rows {
		item, err := library.NewCollectionItem(row.ID, row.CollectionID, row.ItemID, row.Position, row.AddedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteCatalogCollectionRepository) ListItemsPage(
	ctx context.Context,
	collectionID string,
	limit int,
	offset int,
) ([]library.CollectionItem, error) {
	rows := make([]catalogCollectionItemRow, 0, limit)
	if err := repo.db.NewSelect().Model(&rows).
		Where("collection_id = ?", strings.TrimSpace(collectionID)).
		OrderExpr("position ASC").
		OrderExpr("id ASC").
		Limit(limit).
		Offset(offset).
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.CollectionItem, 0, len(rows))
	for _, row := range rows {
		item, err := library.NewCollectionItem(row.ID, row.CollectionID, row.ItemID, row.Position, row.AddedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// ReplaceItems updates the collection metadata and ordered membership as one
// unit. Invalid replacement input is rejected before the transaction starts.
func (repo *SQLiteCatalogCollectionRepository) ReplaceItems(ctx context.Context, collection library.Collection, items []library.CollectionItem) error {
	collectionID := strings.TrimSpace(collection.ID)
	if collectionID == "" {
		return library.ErrInvalidCollection
	}
	rows := make([]catalogCollectionItemRow, 0, len(items))
	for position, item := range items {
		if item.CollectionID != collectionID || item.Position != position || strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.ItemID) == "" || item.AddedAt.IsZero() {
			return library.ErrInvalidCollectionItem
		}
		rows = append(rows, catalogCollectionItemRow{
			ID: item.ID, CollectionID: collectionID, ItemID: item.ItemID, Position: position, AddedAt: item.AddedAt,
		})
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		result, err := tx.NewUpdate().Model((*catalogCollectionRow)(nil)).
			Set("name = ?", collection.Name).
			Set("description = ?", collection.Description).
			Set("kind = ?", string(collection.Kind)).
			Set("smart_query = ?", collection.SmartQuery).
			Set("revision = ?", collection.Revision).
			Set("updated_at = ?", collection.UpdatedAt).
			Where("id = ?", collectionID).
			Exec(ctx)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return sql.ErrNoRows
		}
		if _, err := tx.NewDelete().Model((*catalogCollectionItemRow)(nil)).Where("collection_id = ?", collectionID).Exec(ctx); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		_, err = tx.NewInsert().Model(&rows).Exec(ctx)
		return err
	})
}

func (repo *SQLiteCatalogTagRepository) ListByCatalogID(ctx context.Context, catalogID string) ([]library.Tag, error) {
	rows := make([]catalogTagRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("catalog_id = ?", strings.TrimSpace(catalogID)).
		OrderExpr("normalized_name ASC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.Tag, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainCatalogTag(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteCatalogTagRepository) ListByCatalogIDPage(
	ctx context.Context,
	catalogID string,
	limit int,
	offset int,
) ([]library.Tag, error) {
	rows := make([]catalogTagRow, 0, limit)
	if err := repo.db.NewSelect().Model(&rows).
		Where("catalog_id = ?", strings.TrimSpace(catalogID)).
		OrderExpr("normalized_name ASC").
		OrderExpr("id ASC").
		Limit(limit).
		Offset(offset).
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.Tag, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainCatalogTag(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteCatalogTagRepository) Save(ctx context.Context, item library.Tag) error {
	row := catalogTagRow{
		ID: item.ID, CatalogID: item.CatalogID, Name: item.Name, NormalizedName: item.NormalizedName,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("catalog_id = EXCLUDED.catalog_id").
		Set("name = EXCLUDED.name").
		Set("normalized_name = EXCLUDED.normalized_name").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteCatalogTagRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*catalogTagRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func (repo *SQLiteCatalogTagRepository) ListByItemID(ctx context.Context, itemID string) ([]library.ItemTag, error) {
	rows := make([]catalogItemTagRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("item_id = ?", strings.TrimSpace(itemID)).
		OrderExpr("added_at ASC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.ItemTag, 0, len(rows))
	for _, row := range rows {
		item, err := library.NewItemTag(row.ID, row.ItemID, row.TagID, row.AddedBy, row.AddedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// ReplaceItemTags replaces one item's tag bindings atomically. A foreign-key
// or uniqueness failure rolls the deletion back and preserves the old set.
func (repo *SQLiteCatalogTagRepository) ReplaceItemTags(ctx context.Context, itemID string, tags []library.ItemTag) error {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return library.ErrInvalidItemTag
	}
	rows := make([]catalogItemTagRow, 0, len(tags))
	for _, item := range tags {
		if item.ItemID != itemID || strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.TagID) == "" || item.AddedAt.IsZero() {
			return library.ErrInvalidItemTag
		}
		rows = append(rows, catalogItemTagRow{
			ID: item.ID, ItemID: itemID, TagID: item.TagID, AddedBy: item.AddedBy, AddedAt: item.AddedAt,
		})
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		exists, err := tx.NewSelect().Model((*catalogItemRow)(nil)).Where("id = ?", itemID).Exists(ctx)
		if err != nil {
			return err
		}
		if !exists {
			return sql.ErrNoRows
		}
		if _, err := tx.NewDelete().Model((*catalogItemTagRow)(nil)).Where("item_id = ?", itemID).Exec(ctx); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		_, err = tx.NewInsert().Model(&rows).Exec(ctx)
		return err
	})
}

func toDomainCatalog(row catalogRow) (library.Catalog, error) {
	return library.NewCatalog(library.CatalogParams{
		ID: row.ID, Name: row.Name, Description: row.Description, Status: row.Status,
		IsDefault: row.IsDefault, CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

func toDomainCatalogItem(row catalogItemRow) (library.Item, error) {
	return library.NewItem(library.ItemParams{
		ID: row.ID, CatalogID: row.CatalogID, Category: row.Category, Status: row.Status,
		Title: row.Title, SortTitle: row.SortTitle, Description: row.Description, Revision: row.Revision,
		TrashedAt: row.TrashedAt, CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

func toDomainItemAsset(row itemAssetRow) (library.ItemAsset, error) {
	return library.NewItemAsset(library.ItemAssetParams{
		ID: row.ID, ItemID: row.ItemID, FileID: row.FileID, Role: row.Role, Label: row.Label,
		Position: row.Position, CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

func toDomainStorageRoot(row storageRootRow) (library.StorageRoot, error) {
	return library.NewStorageRoot(library.StorageRootParams{
		ID: row.ID, CatalogID: row.CatalogID, Name: row.Name, Emoji: row.Emoji, Path: row.Path, VolumeID: row.VolumeID,
		Mode: row.Mode, IsDefault: row.IsDefault, Status: row.Status, LastCheckedAt: row.LastCheckedAt, LastError: row.LastError,
		CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

func toCatalogCollectionRow(item library.Collection) catalogCollectionRow {
	return catalogCollectionRow{
		ID: item.ID, CatalogID: item.CatalogID, Name: item.Name, Description: item.Description,
		Kind: string(item.Kind), SmartQuery: item.SmartQuery, Revision: item.Revision,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toDomainCatalogCollection(row catalogCollectionRow) (library.Collection, error) {
	return library.NewCollection(library.CollectionParams{
		ID: row.ID, CatalogID: row.CatalogID, Name: row.Name, Description: row.Description,
		Kind: row.Kind, SmartQuery: row.SmartQuery, Revision: row.Revision,
		CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

func toDomainCatalogTag(row catalogTagRow) (library.Tag, error) {
	item, err := library.NewTag(library.TagParams{
		ID: row.ID, CatalogID: row.CatalogID, Name: row.Name, CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
	if err != nil {
		return library.Tag{}, err
	}
	// Detect corrupt or out-of-band rows instead of silently normalizing them.
	if item.NormalizedName != row.NormalizedName {
		return library.Tag{}, library.ErrInvalidTag
	}
	return item, nil
}

var (
	_ library.CatalogRepository           = (*SQLiteCatalogRepository)(nil)
	_ library.CatalogItemRepository       = (*SQLiteCatalogItemRepository)(nil)
	_ library.ItemAssetRepository         = (*SQLiteItemAssetRepository)(nil)
	_ library.StorageRootRepository       = (*SQLiteStorageRootRepository)(nil)
	_ library.CatalogCollectionRepository = (*SQLiteCatalogCollectionRepository)(nil)
	_ library.CatalogTagRepository        = (*SQLiteCatalogTagRepository)(nil)
)
