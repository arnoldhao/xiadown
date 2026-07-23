package libraryrepo

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

const maxRetainedListenLocalMusicMutationReceipts = 100_000

var listenLocalMusicRequestHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type SQLiteListenLocalMusicWriteRepository struct {
	db *bun.DB
}

func NewSQLiteListenLocalMusicWriteRepository(db *bun.DB) *SQLiteListenLocalMusicWriteRepository {
	return &SQLiteListenLocalMusicWriteRepository{db: db}
}

type listenLocalMusicMutationReceiptRow struct {
	bun.BaseModel   `bun:"table:listen_local_music_mutation_receipts"`
	ReceiptSequence int64     `bun:"receipt_sequence,pk,autoincrement"`
	SubjectID       string    `bun:"subject_id"`
	MutationID      string    `bun:"mutation_id"`
	Family          string    `bun:"family"`
	RequestHash     string    `bun:"request_hash"`
	MutationType    string    `bun:"mutation_type"`
	EntityID        string    `bun:"entity_id"`
	ResultJSON      string    `bun:"result_json"`
	CreatedAt       time.Time `bun:"created_at"`
}

type listenLocalMusicTrackStateRow struct {
	bun.BaseModel           `bun:"table:listen_local_music_track_states"`
	SubjectID               string    `bun:"subject_id,pk"`
	TrackID                 string    `bun:"track_id,pk"`
	Revision                int64     `bun:"revision"`
	Favorite                bool      `bun:"favorite"`
	FavoriteRevision        int64     `bun:"favorite_revision"`
	PositionMs              int64     `bun:"position_ms"`
	PlaySessionID           string    `bun:"play_session_id"`
	ContentIdentityRevision int64     `bun:"content_identity_revision"`
	ProgressRevision        int64     `bun:"progress_revision"`
	CumulativeListenedMs    int64     `bun:"cumulative_listened_ms"`
	PlayCount               int64     `bun:"play_count"`
	SkipCount               int64     `bun:"skip_count"`
	UpdatedAt               time.Time `bun:"updated_at"`
}

type listenLocalMusicLyricDocumentRow struct {
	bun.BaseModel   `bun:"table:listen_local_music_lyric_documents"`
	ID              string    `bun:"id,pk"`
	TrackID         string    `bun:"track_id"`
	Revision        int64     `bun:"revision"`
	SourceKind      string    `bun:"source_kind"`
	ProviderID      string    `bun:"provider_id"`
	ProviderTrackID string    `bun:"provider_track_id"`
	TimingKind      string    `bun:"timing_kind"`
	Language        string    `bun:"language"`
	ContentHash     string    `bun:"content_hash"`
	Availability    string    `bun:"availability"`
	LicensePolicy   string    `bun:"license_policy"`
	CreatedAt       time.Time `bun:"created_at"`
	UpdatedAt       time.Time `bun:"updated_at"`
}

type listenLocalMusicLyricSelectionRow struct {
	bun.BaseModel `bun:"table:listen_local_music_lyric_selections"`
	SubjectID     string    `bun:"subject_id,pk"`
	TrackID       string    `bun:"track_id,pk"`
	DocumentID    string    `bun:"document_id"`
	OffsetMs      int64     `bun:"offset_ms"`
	Revision      int64     `bun:"revision"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

func (repo *SQLiteListenLocalMusicWriteRepository) ApplyMutation(
	ctx context.Context,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	mutation = normalizeListenLocalMusicMutation(mutation)
	if !validListenLocalMusicMutationIdentity(mutation) {
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	var result library.ListenLocalMusicMutationResult
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		replayed, found, err := listenLocalMusicMutationReceiptTx(ctx, tx, mutation)
		if err != nil {
			return err
		}
		if found {
			result = replayed
			return nil
		}
		if mutation.DependsOnMutationID != "" {
			var dependencies int
			if err := tx.NewSelect().Table("listen_local_music_mutation_receipts").ColumnExpr("COUNT(*)").
				Where("subject_id = ?", mutation.SubjectID).
				Where("mutation_id = ?", mutation.DependsOnMutationID).
				Scan(ctx, &dependencies); err != nil {
				return err
			}
			if dependencies == 0 {
				return library.ErrListenLocalMusicDependencyPending
			}
		}

		reserved := &listenLocalMusicMutationReceiptRow{
			SubjectID: mutation.SubjectID, MutationID: mutation.MutationID, Family: mutation.Family,
			RequestHash: mutation.RequestHash, MutationType: mutation.Type, EntityID: mutation.EntityID,
			ResultJSON: `{}`, CreatedAt: mutation.OccurredAt,
		}
		insert, err := tx.NewInsert().Model(reserved).On("CONFLICT(subject_id, mutation_id) DO NOTHING").Exec(ctx)
		if err != nil {
			return err
		}
		inserted, _ := insert.RowsAffected()
		if inserted == 0 {
			replayed, found, err := listenLocalMusicMutationReceiptTx(ctx, tx, mutation)
			if err != nil {
				return err
			}
			if !found {
				return errors.New("Music mutation receipt reservation disappeared")
			}
			result = replayed
			return nil
		}

		switch mutation.Family {
		case library.ListenLocalMusicMutationFamilyState:
			result, err = repo.applyStateMutationTx(ctx, tx, mutation)
		case library.ListenLocalMusicMutationFamilyManage:
			result, err = repo.applyManageMutationTx(ctx, tx, mutation)
		default:
			err = library.ErrInvalidListenLocalMusicMutation
		}
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*listenLocalMusicMutationReceiptRow)(nil)).
			Set("result_json = ?", string(encoded)).
			Where("subject_id = ?", mutation.SubjectID).
			Where("mutation_id = ?", mutation.MutationID).
			Exec(ctx); err != nil {
			return err
		}
		return pruneListenLocalMusicMutationReceiptsTx(ctx, tx, mutation.SubjectID)
	})
	return result, err
}

func normalizeListenLocalMusicMutation(mutation library.ListenLocalMusicMutation) library.ListenLocalMusicMutation {
	mutation.SubjectID = strings.TrimSpace(mutation.SubjectID)
	mutation.ActorDeviceID = strings.TrimSpace(mutation.ActorDeviceID)
	mutation.Family = strings.TrimSpace(mutation.Family)
	mutation.MutationID = strings.ToLower(strings.TrimSpace(mutation.MutationID))
	mutation.RequestHash = strings.ToLower(strings.TrimSpace(mutation.RequestHash))
	mutation.Type = strings.TrimSpace(mutation.Type)
	mutation.EntityID = strings.TrimSpace(mutation.EntityID)
	mutation.DependsOnMutationID = strings.ToLower(strings.TrimSpace(mutation.DependsOnMutationID))
	if mutation.OccurredAt.IsZero() {
		mutation.OccurredAt = time.Now().UTC()
	} else {
		mutation.OccurredAt = mutation.OccurredAt.UTC()
	}
	return mutation
}

func validListenLocalMusicMutationIdentity(mutation library.ListenLocalMusicMutation) bool {
	if mutation.SubjectID != library.ListenLocalMusicSubjectID || mutation.ActorDeviceID == "" ||
		mutation.ExpectedRevision < 0 || mutation.EntityID == "" || len(mutation.EntityID) > 255 ||
		!listenLocalMusicRequestHashPattern.MatchString(mutation.RequestHash) || len(mutation.Payload) == 0 {
		return false
	}
	if _, err := uuid.Parse(mutation.MutationID); err != nil {
		return false
	}
	if mutation.DependsOnMutationID != "" {
		if mutation.DependsOnMutationID == mutation.MutationID {
			return false
		}
		if _, err := uuid.Parse(mutation.DependsOnMutationID); err != nil {
			return false
		}
	}
	return mutation.Family == library.ListenLocalMusicMutationFamilyState ||
		mutation.Family == library.ListenLocalMusicMutationFamilyManage
}

func listenLocalMusicMutationReceiptTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, bool, error) {
	row := new(listenLocalMusicMutationReceiptRow)
	err := tx.NewSelect().Model(row).
		Where("subject_id = ?", mutation.SubjectID).
		Where("mutation_id = ?", mutation.MutationID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return library.ListenLocalMusicMutationResult{}, false, nil
	}
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, false, err
	}
	if row.Family != mutation.Family || row.MutationType != mutation.Type || row.EntityID != mutation.EntityID ||
		!strings.EqualFold(row.RequestHash, mutation.RequestHash) {
		return library.ListenLocalMusicMutationResult{}, false, library.ErrListenLocalMusicIdempotencyConflict
	}
	var result library.ListenLocalMusicMutationResult
	if err := json.Unmarshal([]byte(row.ResultJSON), &result); err != nil || result.MutationID == "" {
		if err == nil {
			err = errors.New("Music mutation receipt is incomplete")
		}
		return library.ListenLocalMusicMutationResult{}, false, err
	}
	result.Replayed = true
	return result, true, nil
}

func decodeListenLocalMusicPayload(payload json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return library.ErrInvalidListenLocalMusicMutation
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return library.ErrInvalidListenLocalMusicMutation
	}
	return nil
}

func newListenLocalMusicMutationResult(
	mutation library.ListenLocalMusicMutation,
	revision int64,
	payload any,
) (library.ListenLocalMusicMutationResult, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	return library.ListenLocalMusicMutationResult{
		MutationID: mutation.MutationID, Type: mutation.Type, EntityID: mutation.EntityID,
		Revision: revision, Result: encoded,
	}, nil
}

func appendListenLocalMusicPayloadChange(
	ctx context.Context,
	tx bun.Tx,
	entityType, entityID string,
	revision int64,
	payload any,
	occurredAt time.Time,
) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	row := listenLocalMusicChangeRow{
		EntityType: entityType, EntityID: entityID, Operation: "upsert", Revision: revision,
		OccurredAt: occurredAt.UTC(), PayloadJSON: sql.NullString{String: string(encoded), Valid: true},
	}
	_, err = tx.NewInsert().Model(&row).Exec(ctx)
	return err
}

func pruneListenLocalMusicMutationReceiptsTx(ctx context.Context, tx bun.Tx, subjectID string) error {
	_, err := tx.NewRaw(`
DELETE FROM listen_local_music_mutation_receipts
WHERE subject_id = ? AND receipt_sequence IN (
  SELECT receipt_sequence
  FROM listen_local_music_mutation_receipts
  WHERE subject_id = ?
  ORDER BY receipt_sequence DESC
  LIMIT -1 OFFSET ?
)
`, subjectID, subjectID, maxRetainedListenLocalMusicMutationReceipts).Exec(ctx)
	return err
}

func listenLocalMusicTrackStateFromRow(row listenLocalMusicTrackStateRow) library.ListenLocalMusicTrackState {
	return library.ListenLocalMusicTrackState{
		SubjectID: row.SubjectID, TrackID: row.TrackID, Revision: row.Revision,
		Favorite: row.Favorite, FavoriteRevision: row.FavoriteRevision,
		PositionMs: row.PositionMs, PlaySessionID: row.PlaySessionID,
		ContentIdentityRevision: row.ContentIdentityRevision, ProgressRevision: row.ProgressRevision,
		CumulativeListenedMs: row.CumulativeListenedMs, PlayCount: row.PlayCount, SkipCount: row.SkipCount,
		UpdatedAt: row.UpdatedAt.UTC(),
	}
}

func listenLocalMusicLyricDocumentFromRow(row listenLocalMusicLyricDocumentRow) library.ListenLocalMusicLyricDocument {
	return library.ListenLocalMusicLyricDocument{
		ID: row.ID, TrackID: row.TrackID, Revision: row.Revision, SourceKind: row.SourceKind,
		ProviderID: row.ProviderID, ProviderTrackID: row.ProviderTrackID, TimingKind: row.TimingKind,
		Language: row.Language, ContentHash: row.ContentHash, Availability: row.Availability,
		LicensePolicy: row.LicensePolicy, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
}

func listenLocalMusicLyricSelectionFromRow(row listenLocalMusicLyricSelectionRow) library.ListenLocalMusicLyricSelection {
	return library.ListenLocalMusicLyricSelection{
		SubjectID: row.SubjectID, TrackID: row.TrackID, DocumentID: row.DocumentID,
		OffsetMs: row.OffsetMs, Revision: row.Revision, UpdatedAt: row.UpdatedAt.UTC(),
	}
}

func validClientMusicUUID(value string) bool {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Version() == 4
}

func listenLocalMusicCurrentRevisionConflict(revision int64, current any) error {
	encoded, _ := json.Marshal(current)
	return &library.ListenLocalMusicRevisionConflictError{CurrentRevision: revision, Current: encoded}
}

var _ library.ListenLocalMusicWriteRepository = (*SQLiteListenLocalMusicWriteRepository)(nil)
