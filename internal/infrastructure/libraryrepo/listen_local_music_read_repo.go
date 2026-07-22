package libraryrepo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

const (
	maxRetainedListenLocalMusicChanges     = 50_000
	maxListenLocalMusicResourceHashEntries = 65_536
)

type listenLocalMusicResourceHashCall struct {
	done     chan struct{}
	checksum string
	err      error
}

type SQLiteListenLocalMusicReadRepository struct {
	db                 *bun.DB
	catalogID          string
	resourceHashMu     sync.Mutex
	resourceHashCache  map[string]string
	resourceHashOrder  []string
	resourceHashNext   int
	resourceHashCalls  map[string]*listenLocalMusicResourceHashCall
	resourceSHA256Func func(context.Context, string) (string, os.FileInfo, error)
}

func NewSQLiteListenLocalMusicReadRepository(db *bun.DB, catalogID string) *SQLiteListenLocalMusicReadRepository {
	return &SQLiteListenLocalMusicReadRepository{
		db: db, catalogID: strings.TrimSpace(catalogID),
		resourceHashCache:  make(map[string]string),
		resourceHashCalls:  make(map[string]*listenLocalMusicResourceHashCall),
		resourceSHA256Func: listenLocalMusicResourceSHA256,
	}
}

type listenLocalMusicSyncStateRow struct {
	Epoch         string `bun:"epoch"`
	HighWater     int64  `bun:"high_water"`
	MinimumCursor int64  `bun:"minimum_cursor"`
}

type listenLocalMusicSnapshotKeyRow struct {
	EntityType string `bun:"entity_type"`
	EntityID   string `bun:"entity_id"`
	Revision   int64  `bun:"revision"`
	SortRank   int    `bun:"sort_rank"`
}

type listenLocalMusicCatalogResourceRow struct {
	CatalogItemID          string         `bun:"catalog_item_id"`
	ResourceID             string         `bun:"resource_id"`
	FileID                 string         `bun:"file_id"`
	AssetRole              string         `bun:"asset_role"`
	AssetPosition          int            `bun:"asset_position"`
	FileName               string         `bun:"file_name"`
	LocalPath              sql.NullString `bun:"local_path"`
	StateJSON              string         `bun:"state_json"`
	RepresentationKind     string         `bun:"representation_kind"`
	RepresentationPurpose  string         `bun:"representation_purpose"`
	MediaType              string         `bun:"media_type"`
	Container              string         `bun:"container"`
	Codec                  string         `bun:"codec"`
	ChecksumAlgorithm      string         `bun:"checksum_algorithm"`
	Checksum               string         `bun:"checksum"`
	SizeBytes              sql.NullInt64  `bun:"size_bytes"`
	Availability           string         `bun:"availability"`
	RepresentationRevision int64          `bun:"representation_revision"`
}

func (repo *SQLiteListenLocalMusicReadRepository) GetSyncPosition(ctx context.Context) (library.ListenLocalMusicSyncPosition, error) {
	var result library.ListenLocalMusicSyncPosition
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := pruneListenLocalMusicChangesTx(ctx, tx, maxRetainedListenLocalMusicChanges); err != nil {
			return err
		}
		position, err := listenLocalMusicSyncPositionTx(ctx, tx)
		if err == nil {
			result = position
		}
		return err
	})
	return result, err
}

func (repo *SQLiteListenLocalMusicReadRepository) ListSnapshot(
	ctx context.Context,
	query library.ListenLocalMusicSnapshotQuery,
) (library.ListenLocalMusicSnapshotPage, error) {
	query.Epoch = strings.TrimSpace(query.Epoch)
	query.AfterType = strings.TrimSpace(query.AfterType)
	query.AfterEntity = strings.TrimSpace(query.AfterEntity)
	if query.Limit < 1 || query.Limit > 500 {
		return library.ListenLocalMusicSnapshotPage{}, library.ErrInvalidListenLocalMusicMembership
	}
	afterRank, ok := listenLocalMusicEntityRank(query.AfterType)
	if !ok || (query.AfterType == "") != (query.AfterEntity == "") {
		return library.ListenLocalMusicSnapshotPage{}, library.ErrInvalidListenLocalMusicMembership
	}

	var result library.ListenLocalMusicSnapshotPage
	err := repo.db.RunInTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		position, err := listenLocalMusicSyncPositionTx(ctx, tx)
		if err != nil {
			return err
		}
		if err := validateListenLocalMusicSnapshotPosition(query, position); err != nil {
			return err
		}
		keys := make([]listenLocalMusicSnapshotKeyRow, 0, query.Limit+1)
		if err := tx.NewRaw(`
SELECT entity_type, entity_id, revision, sort_rank
FROM (
  SELECT 'track' AS entity_type, file_id AS entity_id, revision, 1 AS sort_rank
  FROM listen_local_tracks
  WHERE NOT EXISTS (
    SELECT 1 FROM listen_local_music_memberships AS membership
    WHERE membership.file_id = listen_local_tracks.file_id
      AND membership.state = 'excluded'
  )
  UNION ALL
  SELECT 'playlist', id, revision, 2
  FROM listen_local_playlists
  UNION ALL
  SELECT 'playlist_item', id, revision, 3
  FROM listen_local_playlist_items
  WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'track_state', track_id, revision, 4
  FROM listen_local_music_track_states
  WHERE subject_id = 'music-owner'
  UNION ALL
  SELECT 'lyric_document', id, revision, 5
  FROM listen_local_music_lyric_documents
  UNION ALL
  SELECT 'lyric_selection', track_id, revision, 6
  FROM listen_local_music_lyric_selections
  WHERE subject_id = 'music-owner'
  UNION ALL
  SELECT 'membership', file_id, revision, 7
  FROM listen_local_music_memberships
) AS entity
WHERE sort_rank > ? OR (sort_rank = ? AND entity_id > ?)
ORDER BY sort_rank ASC, entity_id ASC
LIMIT ?
`, afterRank, afterRank, query.AfterEntity, query.Limit+1).Scan(ctx, &keys); err != nil {
			return fmt.Errorf("list Listen Local Music snapshot keys: %w", err)
		}
		result = library.ListenLocalMusicSnapshotPage{
			Entities: make([]library.ListenLocalMusicCanonicalEntity, 0, min(query.Limit, len(keys))),
			Position: library.ListenLocalMusicSyncPosition{
				Epoch: position.Epoch, HighWater: query.HighWater, MinimumCursor: position.MinimumCursor,
			},
			HasMore: len(keys) > query.Limit,
		}
		if len(keys) > query.Limit {
			keys = keys[:query.Limit]
		}
		for _, key := range keys {
			entity, err := repo.loadListenLocalMusicEntity(ctx, tx, key.EntityType, key.EntityID)
			if err != nil {
				return err
			}
			result.Entities = append(result.Entities, entity)
		}
		if result.HasMore && len(keys) > 0 {
			last := keys[len(keys)-1]
			result.NextType, result.NextEntity = last.EntityType, last.EntityID
		}
		return nil
	})
	return result, err
}

func (repo *SQLiteListenLocalMusicReadRepository) ListChanges(
	ctx context.Context,
	query library.ListenLocalMusicChangeQuery,
) (library.ListenLocalMusicChangePage, error) {
	query.Epoch = strings.TrimSpace(query.Epoch)
	if query.After < 0 || query.Limit < 1 || query.Limit > 500 {
		return library.ListenLocalMusicChangePage{}, library.ErrInvalidListenLocalMusicMembership
	}
	var result library.ListenLocalMusicChangePage
	err := repo.db.RunInTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		position, err := listenLocalMusicSyncPositionTx(ctx, tx)
		if err != nil {
			return err
		}
		if query.Epoch != position.Epoch || query.After < position.MinimumCursor || query.After > position.HighWater {
			return &library.ListenLocalMusicSyncResetError{Position: position}
		}
		rows, err := listLatestListenLocalMusicChangeRows(
			ctx, tx, query.After, position.HighWater, query.Limit+1,
		)
		if err != nil {
			return fmt.Errorf("list Listen Local Music changes: %w", err)
		}
		// The foundation journal intentionally stores invalidations rather than
		// public payloads. SQL coalesces each public entity to its last invalidation
		// in this window, then this page hydrates one canonical current entity or
		// tombstone. This guarantees wrapper and payload revisions agree even for
		// upsert→delete and delete→resurrect histories. Membership is part of the canonical
		// Music index projection: paired clients need excluded rows to remove a
		// Track immediately without waiting for a full snapshot reset.
		result = library.ListenLocalMusicChangePage{
			Changes:  make([]library.ListenLocalMusicChange, 0, min(query.Limit, len(rows))),
			Position: position, Cursor: position.HighWater, HasMore: len(rows) > query.Limit,
		}
		if len(rows) > query.Limit {
			rows = rows[:query.Limit]
			result.Cursor = rows[len(rows)-1].Sequence
		}
		for _, row := range rows {
			entity, err := repo.loadListenLocalMusicEntityOrTombstone(ctx, tx, row.EntityType, row.EntityID)
			if err != nil {
				return err
			}
			change := library.ListenLocalMusicChange{
				Sequence: row.Sequence, EntityType: entity.EntityType, EntityID: entity.EntityID,
				Operation: "upsert", Revision: entity.Revision, OccurredAt: row.OccurredAt.UTC(), Entity: &entity,
			}
			if entity.DeletedAt != nil {
				change.Operation = "delete"
				change.OccurredAt = entity.DeletedAt.UTC()
				change.Entity = nil
			}
			result.Changes = append(result.Changes, change)
		}
		return nil
	})
	return result, err
}

const latestListenLocalMusicChangeRowsSQL = `
SELECT
  change_row.sequence,
  change_row.entity_type,
  change_row.entity_id,
  change_row.operation,
  change_row.revision,
  change_row.occurred_at,
  change_row.payload_json
FROM listen_local_music_changes AS change_row NOT INDEXED
WHERE change_row.sequence > ?
  AND change_row.sequence <= ?
  AND change_row.entity_type IN (
    'track', 'playlist', 'playlist_item', 'membership',
    'track_state', 'lyric_document', 'lyric_selection'
  )
  AND NOT EXISTS (
    SELECT 1
    FROM listen_local_music_changes AS newer
    WHERE newer.entity_type = change_row.entity_type
      AND newer.entity_id = change_row.entity_id
      AND newer.sequence > change_row.sequence
      AND newer.sequence <= ?
  )
ORDER BY change_row.sequence ASC
LIMIT ?
`

// listLatestListenLocalMusicChangeRows keeps journal paging bounded without
// changing the wire cursor: each returned sequence is the final invalidation
// for that entity in (after, highWater], ordered exactly as the former in-memory
// coalescing result. The correlated lookup is covered by
// listen_local_music_changes_entity_idx(entity_type, entity_id, sequence DESC).
func listLatestListenLocalMusicChangeRows(
	ctx context.Context,
	db bun.IDB,
	after int64,
	highWater int64,
	limit int,
) ([]listenLocalMusicChangeRow, error) {
	rows := make([]listenLocalMusicChangeRow, 0, limit)
	if err := db.NewRaw(
		latestListenLocalMusicChangeRowsSQL,
		after, highWater, highWater, limit,
	).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (repo *SQLiteListenLocalMusicReadRepository) GetTrackProjection(
	ctx context.Context,
	trackID string,
) (library.ListenLocalMusicTrackProjection, error) {
	var result library.ListenLocalMusicTrackProjection
	err := repo.db.RunInTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		projection, err := repo.loadListenLocalMusicTrackProjection(ctx, tx, strings.TrimSpace(trackID))
		if err == nil {
			result = projection
		}
		return err
	})
	return result, err
}

func (repo *SQLiteListenLocalMusicReadRepository) ListPlaylistProjections(
	ctx context.Context,
	query library.ListenLocalMusicPlaylistQuery,
) (library.ListenLocalMusicPlaylistPage, error) {
	query.Epoch = strings.TrimSpace(query.Epoch)
	query.AfterID = strings.TrimSpace(query.AfterID)
	if query.Limit < 1 || query.Limit > 200 {
		return library.ListenLocalMusicPlaylistPage{}, library.ErrInvalidListenLocalPlaylist
	}
	var result library.ListenLocalMusicPlaylistPage
	err := repo.db.RunInTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		position, err := listenLocalMusicSyncPositionTx(ctx, tx)
		if err != nil {
			return err
		}
		if query.Epoch != position.Epoch {
			return &library.ListenLocalMusicSyncResetError{Position: position}
		}
		rows := make([]listenLocalPlaylistRow, 0, query.Limit+1)
		if err := tx.NewSelect().Model(&rows).
			Where("id > ?", query.AfterID).
			Order("id ASC").
			Limit(query.Limit + 1).
			Scan(ctx); err != nil {
			return err
		}
		result = library.ListenLocalMusicPlaylistPage{
			Items:   make([]library.ListenLocalMusicPlaylistProjection, 0, min(query.Limit, len(rows))),
			HasMore: len(rows) > query.Limit,
		}
		if len(rows) > query.Limit {
			rows = rows[:query.Limit]
		}
		for _, row := range rows {
			playlist, err := toDomainListenLocalPlaylist(row)
			if err != nil {
				return err
			}
			items, err := loadListenLocalPlaylistItems(ctx, tx, row.ID)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, library.ListenLocalMusicPlaylistProjection{Playlist: playlist, Items: items})
		}
		if result.HasMore && len(rows) > 0 {
			result.NextID = rows[len(rows)-1].ID
		}
		return nil
	})
	return result, err
}

func (repo *SQLiteListenLocalMusicReadRepository) ResolveTrackResource(
	ctx context.Context,
	trackID string,
	resourceID string,
) (library.ListenLocalMusicResource, error) {
	projection, err := repo.GetTrackProjection(ctx, strings.TrimSpace(trackID))
	if err != nil {
		return library.ListenLocalMusicResource{}, err
	}
	resourceID = strings.TrimSpace(resourceID)
	for _, resource := range projection.PlaybackResources {
		if resource.ID == resourceID && resource.Availability == "available" && strings.TrimSpace(resource.LocalPath) != "" {
			return resource, nil
		}
	}
	if projection.ArtworkResource != nil && projection.ArtworkResource.ID == resourceID &&
		projection.ArtworkResource.Availability == "available" && strings.TrimSpace(projection.ArtworkResource.LocalPath) != "" {
		return *projection.ArtworkResource, nil
	}
	return library.ListenLocalMusicResource{}, sql.ErrNoRows
}

func listenLocalMusicSyncPositionTx(ctx context.Context, db bun.IDB) (library.ListenLocalMusicSyncPosition, error) {
	state := new(listenLocalMusicSyncStateRow)
	if err := db.NewSelect().Table("listen_local_music_sync_state").Column("epoch", "high_water", "minimum_cursor").Where("id = 1").Scan(ctx, state); err != nil {
		return library.ListenLocalMusicSyncPosition{}, err
	}
	minimumCursor := max(state.MinimumCursor, int64(0))
	if minimumCursor > state.HighWater {
		minimumCursor = state.HighWater
	}
	return library.ListenLocalMusicSyncPosition{
		Epoch: strings.TrimSpace(state.Epoch), HighWater: state.HighWater, MinimumCursor: minimumCursor,
	}, nil
}

func pruneListenLocalMusicChangesTx(ctx context.Context, tx bun.Tx, maxChanges int) error {
	if maxChanges < 1 {
		return errors.New("invalid Listen Local Music change retention policy")
	}
	var cutoff int64
	err := tx.NewRaw(`
SELECT sequence
FROM listen_local_music_changes
ORDER BY sequence DESC
LIMIT 1 OFFSET ?
`, maxChanges).Scan(ctx, &cutoff)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("select Listen Local Music retention boundary: %w", err)
	}
	if cutoff < 1 {
		return nil
	}
	if _, err := tx.NewUpdate().Table("listen_local_music_sync_state").
		Set("minimum_cursor = MAX(minimum_cursor, ?)", cutoff).
		Where("id = 1").Exec(ctx); err != nil {
		return fmt.Errorf("advance Listen Local Music minimum cursor: %w", err)
	}
	if _, err := tx.NewDelete().Model((*listenLocalMusicChangeRow)(nil)).Where("sequence <= ?", cutoff).Exec(ctx); err != nil {
		return fmt.Errorf("prune Listen Local Music changes: %w", err)
	}
	return nil
}

func validateListenLocalMusicSnapshotPosition(
	query library.ListenLocalMusicSnapshotQuery,
	position library.ListenLocalMusicSyncPosition,
) error {
	// The journal currently stores invalidations and canonical rows, not full
	// historical payloads. Requiring the negotiated high-water to remain current
	// gives every page a provably stable database snapshot; a concurrent write
	// forces a fresh bootstrap instead of returning a mixed-generation page.
	if query.Epoch != position.Epoch || query.HighWater != position.HighWater || query.HighWater < position.MinimumCursor {
		return &library.ListenLocalMusicSyncResetError{Position: position}
	}
	return nil
}

func listenLocalMusicEntityRank(entityType string) (int, bool) {
	switch strings.TrimSpace(entityType) {
	case "":
		return 0, true
	case library.ListenLocalMusicEntityTrack:
		return 1, true
	case library.ListenLocalMusicEntityPlaylist:
		return 2, true
	case library.ListenLocalMusicEntityPlaylistItem:
		return 3, true
	case library.ListenLocalMusicEntityTrackState:
		return 4, true
	case library.ListenLocalMusicEntityLyricDocument:
		return 5, true
	case library.ListenLocalMusicEntityLyricSelection:
		return 6, true
	case library.ListenLocalMusicEntityMembership:
		return 7, true
	default:
		return 0, false
	}
}

func (repo *SQLiteListenLocalMusicReadRepository) loadListenLocalMusicEntity(
	ctx context.Context,
	db bun.IDB,
	entityType string,
	entityID string,
) (library.ListenLocalMusicCanonicalEntity, error) {
	entityType, entityID = strings.TrimSpace(entityType), strings.TrimSpace(entityID)
	switch entityType {
	case library.ListenLocalMusicEntityTrack:
		projection, err := repo.loadListenLocalMusicTrackProjection(ctx, db, entityID)
		if err != nil {
			return library.ListenLocalMusicCanonicalEntity{}, err
		}
		return library.ListenLocalMusicCanonicalEntity{
			EntityType: entityType, EntityID: entityID, Revision: projection.Track.Revision, Track: &projection,
		}, nil
	case library.ListenLocalMusicEntityPlaylist:
		row := new(listenLocalPlaylistRow)
		if err := db.NewSelect().Model(row).Where("id = ?", entityID).Scan(ctx); err != nil {
			return library.ListenLocalMusicCanonicalEntity{}, err
		}
		playlist, err := toDomainListenLocalPlaylist(*row)
		if err != nil {
			return library.ListenLocalMusicCanonicalEntity{}, err
		}
		return library.ListenLocalMusicCanonicalEntity{
			EntityType: entityType, EntityID: entityID, Revision: playlist.Revision, Playlist: &playlist,
		}, nil
	case library.ListenLocalMusicEntityPlaylistItem:
		row := new(listenLocalPlaylistItemRow)
		if err := db.NewSelect().Model(row).Where("id = ?", entityID).Where("deleted_at IS NULL").Scan(ctx); err != nil {
			return library.ListenLocalMusicCanonicalEntity{}, err
		}
		item, err := toDomainListenLocalPlaylistItem(*row)
		if err != nil {
			return library.ListenLocalMusicCanonicalEntity{}, err
		}
		return library.ListenLocalMusicCanonicalEntity{
			EntityType: entityType, EntityID: entityID, Revision: item.Revision, PlaylistItem: &item,
		}, nil
	case library.ListenLocalMusicEntityMembership:
		row := new(listenLocalMusicMembershipRow)
		if err := db.NewSelect().Model(row).Where("file_id = ?", entityID).Scan(ctx); err != nil {
			return library.ListenLocalMusicCanonicalEntity{}, err
		}
		membership, err := toDomainListenLocalMusicMembership(*row)
		if err != nil {
			return library.ListenLocalMusicCanonicalEntity{}, err
		}
		return library.ListenLocalMusicCanonicalEntity{
			EntityType: entityType, EntityID: entityID, Revision: membership.Revision, Membership: &membership,
		}, nil
	case library.ListenLocalMusicEntityTrackState:
		row := new(listenLocalMusicTrackStateRow)
		if err := db.NewSelect().Model(row).
			Where("subject_id = ?", library.ListenLocalMusicSubjectID).
			Where("track_id = ?", entityID).
			Scan(ctx); err != nil {
			return library.ListenLocalMusicCanonicalEntity{}, err
		}
		state := listenLocalMusicTrackStateFromRow(*row)
		return library.ListenLocalMusicCanonicalEntity{
			EntityType: entityType, EntityID: entityID, Revision: state.Revision, TrackState: &state,
		}, nil
	case library.ListenLocalMusicEntityLyricDocument:
		row := new(listenLocalMusicLyricDocumentRow)
		if err := db.NewSelect().Model(row).Where("id = ?", entityID).Scan(ctx); err != nil {
			return library.ListenLocalMusicCanonicalEntity{}, err
		}
		document := listenLocalMusicLyricDocumentFromRow(*row)
		return library.ListenLocalMusicCanonicalEntity{
			EntityType: entityType, EntityID: entityID, Revision: document.Revision, LyricDocument: &document,
		}, nil
	case library.ListenLocalMusicEntityLyricSelection:
		row := new(listenLocalMusicLyricSelectionRow)
		if err := db.NewSelect().Model(row).
			Where("subject_id = ?", library.ListenLocalMusicSubjectID).
			Where("track_id = ?", entityID).
			Scan(ctx); err != nil {
			return library.ListenLocalMusicCanonicalEntity{}, err
		}
		selection := listenLocalMusicLyricSelectionFromRow(*row)
		return library.ListenLocalMusicCanonicalEntity{
			EntityType: entityType, EntityID: entityID, Revision: selection.Revision, LyricSelection: &selection,
		}, nil
	default:
		return library.ListenLocalMusicCanonicalEntity{}, sql.ErrNoRows
	}
}

func (repo *SQLiteListenLocalMusicReadRepository) loadListenLocalMusicEntityOrTombstone(
	ctx context.Context,
	db bun.IDB,
	entityType string,
	entityID string,
) (library.ListenLocalMusicCanonicalEntity, error) {
	entity, err := repo.loadListenLocalMusicEntity(ctx, db, entityType, entityID)
	if err == nil {
		return entity, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return library.ListenLocalMusicCanonicalEntity{}, err
	}
	tombstone := new(listenLocalMusicTombstoneRow)
	if err := db.NewSelect().Model(tombstone).
		Where("entity_type = ?", strings.TrimSpace(entityType)).
		Where("entity_id = ?", strings.TrimSpace(entityID)).
		Scan(ctx); err != nil {
		return library.ListenLocalMusicCanonicalEntity{}, err
	}
	deletedAt := tombstone.DeletedAt.UTC()
	return library.ListenLocalMusicCanonicalEntity{
		EntityType: tombstone.EntityType, EntityID: tombstone.EntityID,
		Revision: tombstone.Revision, DeletedAt: &deletedAt,
	}, nil
}

func (repo *SQLiteListenLocalMusicReadRepository) loadListenLocalMusicTrackProjection(
	ctx context.Context,
	db bun.IDB,
	trackID string,
) (library.ListenLocalMusicTrackProjection, error) {
	row := new(listenLocalTrackRow)
	if err := db.NewSelect().Model(row).Where("file_id = ?", strings.TrimSpace(trackID)).Scan(ctx); err != nil {
		return library.ListenLocalMusicTrackProjection{}, err
	}
	var excluded int
	if err := db.NewSelect().Table("listen_local_music_memberships").ColumnExpr("COUNT(*)").
		Where("file_id = ?", strings.TrimSpace(trackID)).Where("state = 'excluded'").Scan(ctx, &excluded); err != nil {
		return library.ListenLocalMusicTrackProjection{}, err
	}
	if excluded > 0 {
		return library.ListenLocalMusicTrackProjection{}, sql.ErrNoRows
	}
	track, err := toDomainListenLocalTrack(*row)
	if err != nil {
		return library.ListenLocalMusicTrackProjection{}, err
	}
	projection := library.ListenLocalMusicTrackProjection{Track: track, PlaybackResources: []library.ListenLocalMusicResource{}}
	resources := make([]listenLocalMusicCatalogResourceRow, 0)
	if repo.catalogID != "" {
		if err := db.NewRaw(`
WITH track_item AS (
  SELECT item.id
  FROM library_item_assets AS original
  JOIN library_catalog_items AS item ON item.id = original.item_id
  WHERE original.file_id = ? AND original.role = 'original' AND item.catalog_id = ?
	AND item.status <> 'trashed'
  ORDER BY item.id ASC
  LIMIT 1
)
SELECT item.id AS catalog_item_id,
       COALESCE(NULLIF(representation.id, ''), asset.id) AS resource_id,
       asset.file_id, asset.role AS asset_role, asset.position AS asset_position,
       file.name AS file_name, file.storage_local_path AS local_path, file.state_json,
       COALESCE(representation.kind, '') AS representation_kind,
       COALESCE(representation.purpose, '') AS representation_purpose,
       COALESCE(representation.media_type, '') AS media_type,
       COALESCE(representation.container, '') AS container,
       COALESCE(representation.codec, '') AS codec,
	   COALESCE(representation.checksum_algorithm, '') AS checksum_algorithm,
       COALESCE(representation.checksum, '') AS checksum,
       representation.size_bytes,
       COALESCE(representation.availability, 'available') AS availability,
       COALESCE(representation.revision, 1) AS representation_revision
FROM track_item
JOIN library_catalog_items AS item ON item.id = track_item.id
JOIN library_item_assets AS asset ON asset.item_id = item.id
JOIN library_files AS file ON file.id = asset.file_id
LEFT JOIN library_representations AS representation ON representation.asset_id = asset.id
WHERE asset.role IN ('original', 'representation', 'artwork')
ORDER BY CASE asset.role WHEN 'representation' THEN 0 WHEN 'original' THEN 1 ELSE 2 END,
         asset.position ASC, resource_id ASC
`, track.FileID, repo.catalogID).Scan(ctx, &resources); err != nil {
			return library.ListenLocalMusicTrackProjection{}, err
		}
	}

	var artworkCandidates []library.ListenLocalMusicResource
	for _, resourceRow := range resources {
		if projection.CatalogItemID == "" {
			projection.CatalogItemID = strings.TrimSpace(resourceRow.CatalogItemID)
		}
		resource := repo.listenLocalMusicResourceFromCatalog(ctx, track, resourceRow)
		if resource.Kind == library.ListenLocalMusicResourceArtwork {
			artworkCandidates = append(artworkCandidates, resource)
			continue
		}
		projection.PlaybackResources = append(projection.PlaybackResources, resource)
	}
	// Legacy Listen Local tracks can predate the Catalog item/asset graph. Only
	// bridge that exact absence: a managed candidate, including a missing or
	// policy-unavailable one, must remain authoritative and must never be
	// bypassed by the canonical Track paths.
	if len(projection.PlaybackResources) == 0 {
		if resource, ok := repo.listenLocalMusicResourceFromCanonicalTrackPath(
			ctx,
			track,
			track.LocalPath,
			library.ListenLocalMusicResourceOriginal,
			"canonical-track-original",
		); ok {
			projection.PlaybackResources = append(projection.PlaybackResources, resource)
		}
	}
	if len(artworkCandidates) == 0 {
		if resource, ok := repo.listenLocalMusicResourceFromCanonicalTrackPath(
			ctx,
			track,
			track.CoverLocalPath,
			library.ListenLocalMusicResourceArtwork,
			"canonical-track-artwork",
		); ok {
			artworkCandidates = append(artworkCandidates, resource)
		}
	}
	sort.SliceStable(projection.PlaybackResources, func(left, right int) bool {
		l, r := projection.PlaybackResources[left], projection.PlaybackResources[right]
		if (l.Availability == "available") != (r.Availability == "available") {
			return l.Availability == "available"
		}
		if l.Kind != r.Kind {
			return l.Kind == library.ListenLocalMusicResourcePlaybackRepresentation
		}
		return l.ID < r.ID
	})
	if len(artworkCandidates) > 0 {
		sort.SliceStable(artworkCandidates, func(left, right int) bool {
			if (artworkCandidates[left].Availability == "available") != (artworkCandidates[right].Availability == "available") {
				return artworkCandidates[left].Availability == "available"
			}
			return artworkCandidates[left].ID < artworkCandidates[right].ID
		})
		value := artworkCandidates[0]
		projection.ArtworkResource = &value
	}
	return projection, nil
}

func (repo *SQLiteListenLocalMusicReadRepository) listenLocalMusicResourceFromCanonicalTrackPath(
	ctx context.Context,
	track library.ListenLocalTrack,
	path string,
	kind string,
	sourceID string,
) (library.ListenLocalMusicResource, bool) {
	path = strings.TrimSpace(path)
	if path == "" || (kind != library.ListenLocalMusicResourceOriginal && kind != library.ListenLocalMusicResourceArtwork) {
		return library.ListenLocalMusicResource{}, false
	}
	if kind == library.ListenLocalMusicResourceOriginal && track.Availability != library.ListenLocalTrackAvailable {
		return library.ListenLocalMusicResource{}, false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return library.ListenLocalMusicResource{}, false
	}
	checksum, err := repo.cachedListenLocalMusicResourceSHA256(ctx, path, info)
	if err != nil || !strings.HasPrefix(checksum, "sha256:") {
		return library.ListenLocalMusicResource{}, false
	}

	extension := strings.ToLower(filepath.Ext(path))
	mediaType := mime.TypeByExtension(extension)
	if separator := strings.IndexByte(mediaType, ';'); separator >= 0 {
		mediaType = strings.TrimSpace(mediaType[:separator])
	}
	container := strings.TrimPrefix(extension, ".")
	codec := ""
	if kind == library.ListenLocalMusicResourceOriginal {
		if format := strings.ToLower(strings.TrimSpace(track.Format)); format != "" {
			container = format
		}
		codec = strings.TrimSpace(track.AudioCodec)
	}
	size := info.Size()
	resource := library.ListenLocalMusicResource{
		FileID: track.FileID, Revision: max(track.ResourceRevision, int64(1)),
		Kind: kind, MediaType: mediaType, Container: container, Codec: codec,
		ByteLength: &size, Checksum: checksum, Availability: "available",
		LocalPath: path, ModTimeUnixNano: info.ModTime().UnixNano(),
	}
	resource.ID = listenLocalMusicPublicResourceID(
		track.FileID, sourceID, kind, resource.Revision,
		resource.ByteLength, resource.Checksum,
	)
	resource.ETag = listenLocalMusicResourceETag(track.FileID, resource)
	return resource, true
}

func (repo *SQLiteListenLocalMusicReadRepository) listenLocalMusicResourceFromCatalog(
	ctx context.Context,
	track library.ListenLocalTrack,
	row listenLocalMusicCatalogResourceRow,
) library.ListenLocalMusicResource {
	kind := library.ListenLocalMusicResourcePlaybackRepresentation
	if row.AssetRole == "original" {
		kind = library.ListenLocalMusicResourceOriginal
	} else if row.AssetRole == "artwork" || row.RepresentationKind == "artwork" ||
		row.RepresentationKind == "thumbnail" || row.RepresentationPurpose == "artwork" {
		kind = library.ListenLocalMusicResourceArtwork
	}
	revision := max(track.ResourceRevision, row.RepresentationRevision, int64(1))
	availability := strings.ToLower(strings.TrimSpace(row.Availability))
	if availability == "" {
		availability = "available"
	}
	var state library.FileState
	_ = json.Unmarshal([]byte(row.StateJSON), &state)
	if !row.LocalPath.Valid || strings.TrimSpace(row.LocalPath.String) == "" || state.Deleted || strings.TrimSpace(state.LastError) != "" {
		availability = "missing"
	}
	switch strings.ToLower(strings.TrimSpace(state.Status)) {
	case "deleted", "missing", "offline", "error", "unavailable":
		availability = "missing"
	}
	if kind == library.ListenLocalMusicResourceOriginal && track.Availability != library.ListenLocalTrackAvailable {
		availability = "missing"
	}
	byteLength := int64OrNil(row.SizeBytes)
	if kind == library.ListenLocalMusicResourceOriginal && byteLength == nil {
		byteLength = track.SizeBytes
	}
	mediaType := strings.TrimSpace(row.MediaType)
	if mediaType == "" {
		mediaType = mime.TypeByExtension(strings.ToLower(filepath.Ext(row.FileName)))
		if separator := strings.IndexByte(mediaType, ';'); separator >= 0 {
			mediaType = strings.TrimSpace(mediaType[:separator])
		}
	}
	container := strings.TrimSpace(row.Container)
	if container == "" {
		container = strings.TrimPrefix(strings.ToLower(filepath.Ext(row.FileName)), ".")
	}
	codec := strings.TrimSpace(row.Codec)
	if codec == "" && kind == library.ListenLocalMusicResourceOriginal {
		codec = track.AudioCodec
	}
	resource := library.ListenLocalMusicResource{
		FileID: strings.TrimSpace(row.FileID), Revision: revision,
		Kind: kind, MediaType: mediaType, Container: container, Codec: codec,
		ByteLength:   byteLength,
		Availability: availability, LocalPath: strings.TrimSpace(row.LocalPath.String),
	}
	if info, err := os.Stat(resource.LocalPath); err == nil && info.Mode().IsRegular() {
		size := info.Size()
		resource.ByteLength = &size
		resource.ModTimeUnixNano = info.ModTime().UnixNano()
		resource.Checksum = validPersistedListenLocalMusicSHA256(row.ChecksumAlgorithm, row.Checksum)
		if resource.Checksum == "" {
			resource.Checksum, err = repo.cachedListenLocalMusicResourceSHA256(ctx, resource.LocalPath, info)
			if err != nil {
				resource.Availability = "missing"
			}
		}
	} else {
		resource.Availability = "missing"
	}
	resource.ID = listenLocalMusicPublicResourceID(
		track.FileID, strings.TrimSpace(row.ResourceID), kind, revision,
		resource.ByteLength, resource.Checksum,
	)
	resource.ETag = listenLocalMusicResourceETag(track.FileID, resource)
	return resource
}

func validPersistedListenLocalMusicSHA256(algorithm, digest string) string {
	if strings.ToLower(strings.TrimSpace(algorithm)) != "sha256" {
		return ""
	}
	digest = strings.ToLower(strings.TrimSpace(digest))
	if len(digest) != 64 {
		return ""
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return ""
	}
	return "sha256:" + digest
}

func (repo *SQLiteListenLocalMusicReadRepository) cachedListenLocalMusicResourceSHA256(
	ctx context.Context,
	path string,
	info os.FileInfo,
) (string, error) {
	key := fmt.Sprintf("%s\x00%s", strings.TrimSpace(path), listenLocalMusicResourceStatIdentity(info))
	repo.resourceHashMu.Lock()
	if repo.resourceHashCalls == nil {
		repo.resourceHashCalls = make(map[string]*listenLocalMusicResourceHashCall)
	}
	checksum, found := repo.resourceHashCache[key]
	if found {
		repo.resourceHashMu.Unlock()
		return checksum, nil
	}
	if call := repo.resourceHashCalls[key]; call != nil {
		repo.resourceHashMu.Unlock()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-call.done:
			return call.checksum, call.err
		}
	}
	call := &listenLocalMusicResourceHashCall{done: make(chan struct{})}
	repo.resourceHashCalls[key] = call
	repo.resourceHashMu.Unlock()
	hasher := repo.resourceSHA256Func
	if hasher == nil {
		hasher = listenLocalMusicResourceSHA256
	}
	checksum, hashedInfo, err := hasher(ctx, path)
	if err == nil && listenLocalMusicResourceStatIdentity(hashedInfo) != listenLocalMusicResourceStatIdentity(info) {
		err = errors.New("Music resource changed while hashing")
	}
	repo.resourceHashMu.Lock()
	if repo.resourceHashCache == nil {
		repo.resourceHashCache = make(map[string]string)
	}
	if err == nil {
		if len(repo.resourceHashCache) < maxListenLocalMusicResourceHashEntries {
			repo.resourceHashOrder = append(repo.resourceHashOrder, key)
		} else {
			if len(repo.resourceHashOrder) != maxListenLocalMusicResourceHashEntries {
				repo.resourceHashOrder = make([]string, 0, maxListenLocalMusicResourceHashEntries)
				for cachedKey := range repo.resourceHashCache {
					repo.resourceHashOrder = append(repo.resourceHashOrder, cachedKey)
				}
				sort.Strings(repo.resourceHashOrder)
				repo.resourceHashNext = 0
			}
			evicted := repo.resourceHashOrder[repo.resourceHashNext]
			delete(repo.resourceHashCache, evicted)
			repo.resourceHashOrder[repo.resourceHashNext] = key
			repo.resourceHashNext = (repo.resourceHashNext + 1) % maxListenLocalMusicResourceHashEntries
		}
		repo.resourceHashCache[key] = checksum
	}
	call.checksum, call.err = checksum, err
	delete(repo.resourceHashCalls, key)
	close(call.done)
	repo.resourceHashMu.Unlock()
	return checksum, err
}

func listenLocalMusicResourceStatIdentity(info os.FileInfo) string {
	if info == nil {
		return "missing"
	}
	parts := []string{fmt.Sprintf("%d:%d:%d", info.Size(), info.ModTime().UnixNano(), info.Mode())}
	value := reflect.ValueOf(info.Sys())
	if value.IsValid() && value.Kind() == reflect.Pointer && !value.IsNil() {
		value = value.Elem()
	}
	if value.IsValid() && value.Kind() == reflect.Struct {
		for _, name := range []string{
			"Dev", "Ino", "Ctim", "Ctimespec", "Birthtimespec", "CreationTime",
			"VolumeSerialNumber", "FileIndexHigh", "FileIndexLow",
		} {
			field := value.FieldByName(name)
			if field.IsValid() && field.CanInterface() {
				parts = append(parts, name+"="+fmt.Sprint(field.Interface()))
			}
		}
	}
	return strings.Join(parts, ":")
}

func listenLocalMusicPublicResourceID(
	trackID, sourceID, kind string,
	revision int64,
	byteLength *int64,
	checksum string,
) string {
	length := int64(-1)
	if byteLength != nil {
		length = *byteLength
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"music-resource-v2\x00%s\x00%s\x00%s\x00%d\x00%d\x00%s",
		trackID, sourceID, kind, revision, length, strings.ToLower(strings.TrimSpace(checksum)),
	)))
	return "mr1_" + base64.RawURLEncoding.EncodeToString(digest[:])
}

func listenLocalMusicResourceETag(trackID string, resource library.ListenLocalMusicResource) string {
	length := int64(-1)
	if resource.ByteLength != nil {
		length = *resource.ByteLength
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"%s\x00%s\x00%d\x00%d\x00%s",
		trackID, resource.ID, resource.Revision, length, strings.ToLower(strings.TrimSpace(resource.Checksum)),
	)))
	return `"` + hex.EncodeToString(digest[:16]) + `"`
}

func listenLocalMusicResourceSHA256(ctx context.Context, path string) (string, os.FileInfo, error) {
	file, err := os.Open(strings.TrimSpace(path))
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		if err == nil {
			err = errors.New("Music resource is not a regular file")
		}
		return "", nil, err
	}
	digest := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			if _, err := digest.Write(buffer[:read]); err != nil {
				return "", nil, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", nil, readErr
		}
	}
	afterInfo, err := file.Stat()
	if err != nil || !afterInfo.Mode().IsRegular() {
		return "", nil, errors.New("Music resource changed while hashing")
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), afterInfo, nil
}

func loadListenLocalPlaylistItems(ctx context.Context, db bun.IDB, playlistID string) ([]library.ListenLocalPlaylistItem, error) {
	rows := make([]listenLocalPlaylistItemRow, 0)
	if err := db.NewSelect().Model(&rows).
		Where("playlist_id = ?", strings.TrimSpace(playlistID)).
		Where("deleted_at IS NULL").
		Order("position ASC", "id ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	items := make([]library.ListenLocalPlaylistItem, 0, len(rows))
	for _, row := range rows {
		item, err := toDomainListenLocalPlaylistItem(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

var _ library.ListenLocalMusicReadRepository = (*SQLiteListenLocalMusicReadRepository)(nil)
