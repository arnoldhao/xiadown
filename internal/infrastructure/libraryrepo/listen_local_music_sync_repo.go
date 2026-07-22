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

const (
	listenLocalEntityTrack        = "track"
	listenLocalEntityPlaylist     = "playlist"
	listenLocalEntityPlaylistItem = "playlist_item"
	listenLocalEntityMembership   = "membership"
)

type listenLocalMusicTombstoneRow struct {
	bun.BaseModel           `bun:"table:listen_local_music_tombstones"`
	EntityType              string         `bun:"entity_type,pk"`
	EntityID                string         `bun:"entity_id,pk"`
	Revision                int64          `bun:"revision"`
	ContentIdentityRevision sql.NullInt64  `bun:"content_identity_revision"`
	MetadataRevision        sql.NullInt64  `bun:"metadata_revision"`
	ResourceRevision        sql.NullInt64  `bun:"resource_revision"`
	DeletedAt               time.Time      `bun:"deleted_at"`
	PayloadJSON             sql.NullString `bun:"payload_json"`
}

type listenLocalMusicChangeRow struct {
	bun.BaseModel `bun:"table:listen_local_music_changes"`
	Sequence      int64          `bun:"sequence,pk,autoincrement"`
	EntityType    string         `bun:"entity_type"`
	EntityID      string         `bun:"entity_id"`
	Operation     string         `bun:"operation"`
	Revision      int64          `bun:"revision"`
	OccurredAt    time.Time      `bun:"occurred_at"`
	PayloadJSON   sql.NullString `bun:"payload_json"`
}

type listenLocalMusicMembershipRow struct {
	bun.BaseModel `bun:"table:listen_local_music_memberships"`
	FileID        string    `bun:"file_id,pk"`
	State         string    `bun:"state"`
	Reason        string    `bun:"reason"`
	Revision      int64     `bun:"revision"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

type SQLiteListenLocalMusicMembershipRepository struct{ db *bun.DB }

func NewSQLiteListenLocalMusicMembershipRepository(db *bun.DB) *SQLiteListenLocalMusicMembershipRepository {
	return &SQLiteListenLocalMusicMembershipRepository{db: db}
}

func (repo *SQLiteListenLocalMusicMembershipRepository) Get(ctx context.Context, fileID string) (library.ListenLocalMusicMembership, error) {
	row := new(listenLocalMusicMembershipRow)
	if err := repo.db.NewSelect().Model(row).Where("file_id = ?", strings.TrimSpace(fileID)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.ListenLocalMusicMembership{}, library.ErrListenLocalMusicMembershipNotFound
		}
		return library.ListenLocalMusicMembership{}, err
	}
	return toDomainListenLocalMusicMembership(*row)
}

func (repo *SQLiteListenLocalMusicMembershipRepository) Save(ctx context.Context, item library.ListenLocalMusicMembership) error {
	validated, err := library.NewListenLocalMusicMembership(library.ListenLocalMusicMembershipParams{
		FileID: item.FileID, State: string(item.State), Reason: item.Reason, Revision: item.Revision,
		CreatedAt: &item.CreatedAt, UpdatedAt: &item.UpdatedAt,
	})
	if err != nil {
		return err
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		existing := new(listenLocalMusicMembershipRow)
		findErr := tx.NewSelect().Model(existing).Where("file_id = ?", validated.FileID).Scan(ctx)
		isNew := errors.Is(findErr, sql.ErrNoRows)
		if findErr != nil && !isNew {
			return findErr
		}
		row := listenLocalMusicMembershipRow{
			FileID: validated.FileID, State: string(validated.State), Reason: validated.Reason,
			Revision: validated.Revision, CreatedAt: validated.CreatedAt, UpdatedAt: validated.UpdatedAt,
		}
		changed := isNew
		if isNew {
			tombstoneRevision, _, err := listenLocalTombstoneRevision(ctx, tx, listenLocalEntityMembership, row.FileID)
			if err != nil {
				return err
			}
			row.Revision = max(row.Revision, tombstoneRevision+1, 1)
		} else {
			row.CreatedAt = existing.CreatedAt
			row.Revision = existing.Revision
			changed = existing.State != row.State || existing.Reason != row.Reason
			if changed {
				row.Revision++
			} else {
				row.UpdatedAt = existing.UpdatedAt
			}
		}
		if _, err := tx.NewInsert().Model(&row).
			On("CONFLICT(file_id) DO UPDATE").
			Set("state = EXCLUDED.state").
			Set("reason = EXCLUDED.reason").
			Set("revision = EXCLUDED.revision").
			Set("updated_at = EXCLUDED.updated_at").
			Exec(ctx); err != nil {
			return err
		}
		if !changed {
			return nil
		}
		if err := clearListenLocalTombstone(ctx, tx, listenLocalEntityMembership, row.FileID); err != nil {
			return err
		}
		return appendListenLocalChange(ctx, tx, listenLocalEntityMembership, row.FileID, "upsert", row.Revision, row.UpdatedAt)
	})
}

func toDomainListenLocalMusicMembership(row listenLocalMusicMembershipRow) (library.ListenLocalMusicMembership, error) {
	return library.NewListenLocalMusicMembership(library.ListenLocalMusicMembershipParams{
		FileID: row.FileID, State: row.State, Reason: row.Reason, Revision: row.Revision,
		CreatedAt: &row.CreatedAt, UpdatedAt: &row.UpdatedAt,
	})
}

func listenLocalTombstoneRevision(ctx context.Context, tx bun.Tx, entityType, entityID string) (int64, int64, error) {
	row := new(listenLocalMusicTombstoneRow)
	if err := tx.NewSelect().Model(row).
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", entityID).
		Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	contentIdentityRevision := int64(0)
	if row.ContentIdentityRevision.Valid {
		contentIdentityRevision = row.ContentIdentityRevision.Int64
	}
	return row.Revision, contentIdentityRevision, nil
}

func listenLocalTrackTombstoneRevisions(ctx context.Context, tx bun.Tx, entityID string) (int64, int64, int64, int64, error) {
	row := new(listenLocalMusicTombstoneRow)
	if err := tx.NewSelect().Model(row).
		Where("entity_type = ?", listenLocalEntityTrack).
		Where("entity_id = ?", strings.TrimSpace(entityID)).
		Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, 0, 0, nil
		}
		return 0, 0, 0, 0, err
	}
	return row.Revision, nullInt64Value(row.ContentIdentityRevision), nullInt64Value(row.MetadataRevision), nullInt64Value(row.ResourceRevision), nil
}

func nullInt64Value(value sql.NullInt64) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}

func clearListenLocalTombstone(ctx context.Context, tx bun.Tx, entityType, entityID string) error {
	_, err := tx.NewDelete().Model((*listenLocalMusicTombstoneRow)(nil)).
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", entityID).
		Exec(ctx)
	return err
}

func appendListenLocalChange(ctx context.Context, tx bun.Tx, entityType, entityID, operation string, revision int64, occurredAt time.Time) error {
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	row := listenLocalMusicChangeRow{
		EntityType: entityType, EntityID: entityID, Operation: operation,
		Revision: revision, OccurredAt: occurredAt.UTC(),
	}
	_, err := tx.NewInsert().Model(&row).Exec(ctx)
	return err
}
