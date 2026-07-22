package libraryrepo

import (
	"context"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

type SQLiteRepresentationRepository struct{ db *bun.DB }
type SQLiteMetadataEntryRepository struct{ db *bun.DB }

type catalogRepresentationRow struct {
	bun.BaseModel     `bun:"table:library_representations"`
	ID                string    `bun:"id,pk"`
	CatalogID         string    `bun:"catalog_id"`
	ItemID            string    `bun:"item_id"`
	AssetID           string    `bun:"asset_id"`
	Kind              string    `bun:"kind"`
	Purpose           string    `bun:"purpose"`
	MediaType         string    `bun:"media_type"`
	Container         string    `bun:"container"`
	Codec             string    `bun:"codec"`
	Width             *int      `bun:"width"`
	Height            *int      `bun:"height"`
	DurationMs        *int64    `bun:"duration_ms"`
	BitrateBps        *int64    `bun:"bitrate_bps"`
	Language          string    `bun:"language"`
	ChecksumAlgorithm string    `bun:"checksum_algorithm"`
	Checksum          string    `bun:"checksum"`
	SizeBytes         *int64    `bun:"size_bytes"`
	Availability      string    `bun:"availability"`
	Revision          int64     `bun:"revision"`
	CreatedAt         time.Time `bun:"created_at"`
	UpdatedAt         time.Time `bun:"updated_at"`
}

type catalogMetadataEntryRow struct {
	bun.BaseModel    `bun:"table:library_metadata_entries"`
	ID               string    `bun:"id,pk"`
	CatalogID        string    `bun:"catalog_id"`
	ItemID           string    `bun:"item_id"`
	RepresentationID *string   `bun:"representation_id"`
	Namespace        string    `bun:"namespace"`
	Key              string    `bun:"key"`
	ValueType        string    `bun:"value_type"`
	ValueJSON        string    `bun:"value_json"`
	Language         string    `bun:"language"`
	Position         int       `bun:"position"`
	Source           string    `bun:"source"`
	Provenance       string    `bun:"provenance"`
	Confidence       *float64  `bun:"confidence"`
	Locked           bool      `bun:"locked"`
	Revision         int64     `bun:"revision"`
	CreatedAt        time.Time `bun:"created_at"`
	UpdatedAt        time.Time `bun:"updated_at"`
}

func NewSQLiteRepresentationRepository(db *bun.DB) *SQLiteRepresentationRepository {
	return &SQLiteRepresentationRepository{db: db}
}

func NewSQLiteMetadataEntryRepository(db *bun.DB) *SQLiteMetadataEntryRepository {
	return &SQLiteMetadataEntryRepository{db: db}
}

func (repo *SQLiteRepresentationRepository) ListRepresentationsByItemID(ctx context.Context, itemID string) ([]library.Representation, error) {
	rows := make([]catalogRepresentationRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where("item_id = ?", strings.TrimSpace(itemID)).
		OrderExpr("CASE kind WHEN 'original' THEN 0 WHEN 'optimized' THEN 1 WHEN 'preview' THEN 2 WHEN 'thumbnail' THEN 3 ELSE 4 END").
		OrderExpr("purpose ASC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.Representation, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainRepresentation(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteRepresentationRepository) GetRepresentation(ctx context.Context, id string) (library.Representation, error) {
	row := new(catalogRepresentationRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		return library.Representation{}, err
	}
	return toDomainRepresentation(*row)
}

func (repo *SQLiteRepresentationRepository) SaveRepresentation(ctx context.Context, item library.Representation) error {
	validated, err := validateRepresentation(item)
	if err != nil {
		return err
	}
	row := toRepresentationRow(validated)
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("catalog_id = EXCLUDED.catalog_id").
		Set("item_id = EXCLUDED.item_id").
		Set("asset_id = EXCLUDED.asset_id").
		Set("kind = EXCLUDED.kind").
		Set("purpose = EXCLUDED.purpose").
		Set("media_type = EXCLUDED.media_type").
		Set("container = EXCLUDED.container").
		Set("codec = EXCLUDED.codec").
		Set("width = EXCLUDED.width").
		Set("height = EXCLUDED.height").
		Set("duration_ms = EXCLUDED.duration_ms").
		Set("bitrate_bps = EXCLUDED.bitrate_bps").
		Set("language = EXCLUDED.language").
		Set("checksum_algorithm = EXCLUDED.checksum_algorithm").
		Set("checksum = EXCLUDED.checksum").
		Set("size_bytes = EXCLUDED.size_bytes").
		Set("availability = EXCLUDED.availability").
		Set("revision = EXCLUDED.revision").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteRepresentationRepository) DeleteRepresentation(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*catalogRepresentationRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func (repo *SQLiteMetadataEntryRepository) ListMetadataEntriesByItemID(ctx context.Context, itemID string) ([]library.MetadataEntry, error) {
	return repo.list(ctx, "item_id = ?", strings.TrimSpace(itemID))
}

func (repo *SQLiteMetadataEntryRepository) ListMetadataEntriesByRepresentationID(ctx context.Context, representationID string) ([]library.MetadataEntry, error) {
	return repo.list(ctx, "representation_id = ?", strings.TrimSpace(representationID))
}

func (repo *SQLiteMetadataEntryRepository) list(ctx context.Context, where string, arg any) ([]library.MetadataEntry, error) {
	rows := make([]catalogMetadataEntryRow, 0)
	if err := repo.db.NewSelect().Model(&rows).
		Where(where, arg).
		OrderExpr("namespace ASC").
		OrderExpr("key ASC").
		OrderExpr("language ASC").
		OrderExpr("position ASC").
		OrderExpr("id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]library.MetadataEntry, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainMetadataEntry(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteMetadataEntryRepository) GetMetadataEntry(ctx context.Context, id string) (library.MetadataEntry, error) {
	row := new(catalogMetadataEntryRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		return library.MetadataEntry{}, err
	}
	return toDomainMetadataEntry(*row)
}

func (repo *SQLiteMetadataEntryRepository) SaveMetadataEntry(ctx context.Context, item library.MetadataEntry) error {
	validated, err := validateMetadataEntry(item)
	if err != nil {
		return err
	}
	row := toMetadataEntryRow(validated)
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("catalog_id = EXCLUDED.catalog_id").
		Set("item_id = EXCLUDED.item_id").
		Set("representation_id = EXCLUDED.representation_id").
		Set("namespace = EXCLUDED.namespace").
		Set("key = EXCLUDED.key").
		Set("value_type = EXCLUDED.value_type").
		Set("value_json = EXCLUDED.value_json").
		Set("language = EXCLUDED.language").
		Set("position = EXCLUDED.position").
		Set("source = EXCLUDED.source").
		Set("provenance = EXCLUDED.provenance").
		Set("confidence = EXCLUDED.confidence").
		Set("locked = EXCLUDED.locked").
		Set("revision = EXCLUDED.revision").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteMetadataEntryRepository) DeleteMetadataEntry(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*catalogMetadataEntryRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func validateRepresentation(item library.Representation) (library.Representation, error) {
	return library.NewRepresentation(library.RepresentationParams{
		ID: item.ID, CatalogID: item.CatalogID, ItemID: item.ItemID, AssetID: item.AssetID,
		Kind: string(item.Kind), Purpose: string(item.Purpose), MediaType: item.MediaType,
		Container: item.Container, Codec: item.Codec, Width: item.Width, Height: item.Height,
		DurationMs: item.DurationMs, BitrateBps: item.BitrateBps, Language: item.Language,
		ChecksumAlgorithm: string(item.ChecksumAlgorithm), Checksum: item.Checksum,
		SizeBytes: item.SizeBytes, Availability: string(item.Availability), Revision: item.Revision,
		CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
}

func toRepresentationRow(item library.Representation) catalogRepresentationRow {
	return catalogRepresentationRow{
		ID: item.ID, CatalogID: item.CatalogID, ItemID: item.ItemID, AssetID: item.AssetID,
		Kind: string(item.Kind), Purpose: string(item.Purpose), MediaType: item.MediaType,
		Container: item.Container, Codec: item.Codec, Width: item.Width, Height: item.Height,
		DurationMs: item.DurationMs, BitrateBps: item.BitrateBps, Language: item.Language,
		ChecksumAlgorithm: string(item.ChecksumAlgorithm), Checksum: item.Checksum,
		SizeBytes: item.SizeBytes, Availability: string(item.Availability), Revision: item.Revision,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toDomainRepresentation(row catalogRepresentationRow) (library.Representation, error) {
	return library.NewRepresentation(library.RepresentationParams{
		ID: row.ID, CatalogID: row.CatalogID, ItemID: row.ItemID, AssetID: row.AssetID,
		Kind: row.Kind, Purpose: row.Purpose, MediaType: row.MediaType,
		Container: row.Container, Codec: row.Codec, Width: row.Width, Height: row.Height,
		DurationMs: row.DurationMs, BitrateBps: row.BitrateBps, Language: row.Language,
		ChecksumAlgorithm: row.ChecksumAlgorithm, Checksum: row.Checksum,
		SizeBytes: row.SizeBytes, Availability: row.Availability, Revision: row.Revision,
		CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

func validateMetadataEntry(item library.MetadataEntry) (library.MetadataEntry, error) {
	return library.NewMetadataEntry(library.MetadataEntryParams{
		ID: item.ID, CatalogID: item.CatalogID, ItemID: item.ItemID, RepresentationID: item.RepresentationID,
		Namespace: item.Namespace, Key: item.Key, ValueType: string(item.ValueType), ValueJSON: string(item.Value),
		Language: item.Language, Position: item.Position, Source: string(item.Source), Provenance: item.Provenance,
		Confidence: item.Confidence, Locked: item.Locked, Revision: item.Revision,
		CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
}

func toMetadataEntryRow(item library.MetadataEntry) catalogMetadataEntryRow {
	var representationID *string
	if item.RepresentationID != "" {
		value := item.RepresentationID
		representationID = &value
	}
	return catalogMetadataEntryRow{
		ID: item.ID, CatalogID: item.CatalogID, ItemID: item.ItemID, RepresentationID: representationID,
		Namespace: item.Namespace, Key: item.Key, ValueType: string(item.ValueType), ValueJSON: string(item.Value),
		Language: item.Language, Position: item.Position, Source: string(item.Source), Provenance: item.Provenance,
		Confidence: item.Confidence, Locked: item.Locked, Revision: item.Revision,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toDomainMetadataEntry(row catalogMetadataEntryRow) (library.MetadataEntry, error) {
	representationID := ""
	if row.RepresentationID != nil {
		representationID = *row.RepresentationID
	}
	return library.NewMetadataEntry(library.MetadataEntryParams{
		ID: row.ID, CatalogID: row.CatalogID, ItemID: row.ItemID, RepresentationID: representationID,
		Namespace: row.Namespace, Key: row.Key, ValueType: row.ValueType, ValueJSON: row.ValueJSON,
		Language: row.Language, Position: row.Position, Source: row.Source, Provenance: row.Provenance,
		Confidence: row.Confidence, Locked: row.Locked, Revision: row.Revision,
		CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

var (
	_ library.RepresentationRepository = (*SQLiteRepresentationRepository)(nil)
	_ library.MetadataEntryRepository  = (*SQLiteMetadataEntryRepository)(nil)
)
