package libraryrepo

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

var listenLocalMusicContentHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type listenLocalMusicSetFavoritePayload struct {
	Favorite *bool `json:"favorite"`
}

type listenLocalMusicSetProgressPayload struct {
	PositionMs              *int64 `json:"positionMs"`
	PlaySessionID           string `json:"playSessionId"`
	ContentIdentityRevision int64  `json:"contentIdentityRevision"`
}

type listenLocalMusicSelectProviderLyricPayload struct {
	ClientDocumentID string `json:"clientDocumentId"`
	ProviderID       string `json:"providerId"`
	ProviderTrackID  string `json:"providerTrackId"`
	TimingKind       string `json:"timingKind"`
	Language         string `json:"language,omitempty"`
	ContentHash      string `json:"contentHash,omitempty"`
	Availability     string `json:"availability"`
	LicensePolicy    string `json:"licensePolicy"`
	OffsetMs         *int64 `json:"offsetMs"`
}

func (repo *SQLiteListenLocalMusicWriteRepository) applyStateMutationTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	switch mutation.Type {
	case "setFavorite":
		return repo.setFavoriteTx(ctx, tx, mutation)
	case "setProgress":
		return repo.setProgressTx(ctx, tx, mutation)
	case "selectProviderLyric":
		return repo.selectProviderLyricTx(ctx, tx, mutation)
	default:
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
}

func (repo *SQLiteListenLocalMusicWriteRepository) setFavoriteTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	var payload listenLocalMusicSetFavoritePayload
	if err := decodeListenLocalMusicPayload(mutation.Payload, &payload); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if payload.Favorite == nil {
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	track, err := requireActiveListenLocalMusicTrackTx(ctx, tx, mutation.EntityID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	state, found, err := loadListenLocalMusicTrackStateTx(ctx, tx, mutation.SubjectID, mutation.EntityID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if mutation.ExpectedRevision != state.FavoriteRevision {
		return library.ListenLocalMusicMutationResult{}, listenLocalMusicCurrentRevisionConflict(
			state.FavoriteRevision, listenLocalMusicTrackStateFromRow(state),
		)
	}
	if found && state.Favorite == *payload.Favorite {
		return newListenLocalMusicMutationResult(mutation, state.FavoriteRevision, map[string]any{
			"trackState": listenLocalMusicTrackStateFromRow(state),
		})
	}
	now := mutation.OccurredAt
	state.SubjectID = mutation.SubjectID
	state.TrackID = mutation.EntityID
	state.Favorite = *payload.Favorite
	state.FavoriteRevision++
	state.Revision = max(state.Revision+1, int64(1))
	state.ContentIdentityRevision = max(state.ContentIdentityRevision, track.ContentIdentityRevision)
	state.UpdatedAt = now
	if err := saveListenLocalMusicTrackStateTx(ctx, tx, state); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	canonical := listenLocalMusicTrackStateFromRow(state)
	if err := appendListenLocalMusicPayloadChange(
		ctx, tx, library.ListenLocalMusicEntityTrackState, state.TrackID, state.Revision, canonical, now,
	); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	return newListenLocalMusicMutationResult(mutation, state.FavoriteRevision, map[string]any{"trackState": canonical})
}

func (repo *SQLiteListenLocalMusicWriteRepository) setProgressTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	var payload listenLocalMusicSetProgressPayload
	if err := decodeListenLocalMusicPayload(mutation.Payload, &payload); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	payload.PlaySessionID = strings.ToLower(strings.TrimSpace(payload.PlaySessionID))
	if payload.PositionMs == nil || *payload.PositionMs < 0 || !validClientMusicUUID(payload.PlaySessionID) || payload.ContentIdentityRevision < 1 {
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	track, err := requireActiveListenLocalMusicTrackTx(ctx, tx, mutation.EntityID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if payload.ContentIdentityRevision != track.ContentIdentityRevision {
		return library.ListenLocalMusicMutationResult{}, library.ErrListenLocalMusicContentChanged
	}
	state, found, err := loadListenLocalMusicTrackStateTx(ctx, tx, mutation.SubjectID, mutation.EntityID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if mutation.ExpectedRevision != state.ProgressRevision {
		return library.ListenLocalMusicMutationResult{}, listenLocalMusicCurrentRevisionConflict(
			state.ProgressRevision, listenLocalMusicTrackStateFromRow(state),
		)
	}
	if found && state.PositionMs == *payload.PositionMs && state.PlaySessionID == payload.PlaySessionID &&
		state.ContentIdentityRevision == payload.ContentIdentityRevision {
		return newListenLocalMusicMutationResult(mutation, state.ProgressRevision, map[string]any{
			"trackState": listenLocalMusicTrackStateFromRow(state),
		})
	}
	now := mutation.OccurredAt
	state.SubjectID = mutation.SubjectID
	state.TrackID = mutation.EntityID
	state.PositionMs = *payload.PositionMs
	state.PlaySessionID = payload.PlaySessionID
	state.ContentIdentityRevision = payload.ContentIdentityRevision
	state.ProgressRevision++
	state.Revision = max(state.Revision+1, int64(1))
	state.UpdatedAt = now
	if err := saveListenLocalMusicTrackStateTx(ctx, tx, state); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	canonical := listenLocalMusicTrackStateFromRow(state)
	if err := appendListenLocalMusicPayloadChange(
		ctx, tx, library.ListenLocalMusicEntityTrackState, state.TrackID, state.Revision, canonical, now,
	); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	return newListenLocalMusicMutationResult(mutation, state.ProgressRevision, map[string]any{"trackState": canonical})
}

func (repo *SQLiteListenLocalMusicWriteRepository) selectProviderLyricTx(
	ctx context.Context,
	tx bun.Tx,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	var payload listenLocalMusicSelectProviderLyricPayload
	if err := decodeListenLocalMusicPayload(mutation.Payload, &payload); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	payload.ClientDocumentID = strings.ToLower(strings.TrimSpace(payload.ClientDocumentID))
	payload.ProviderID = strings.TrimSpace(payload.ProviderID)
	payload.ProviderTrackID = strings.TrimSpace(payload.ProviderTrackID)
	payload.TimingKind = strings.TrimSpace(payload.TimingKind)
	payload.Language = strings.TrimSpace(payload.Language)
	payload.ContentHash = strings.ToLower(strings.TrimSpace(payload.ContentHash))
	payload.Availability = strings.TrimSpace(payload.Availability)
	payload.LicensePolicy = strings.TrimSpace(payload.LicensePolicy)
	if !validClientMusicUUID(payload.ClientDocumentID) || payload.ProviderID == "" || payload.ProviderTrackID == "" || payload.OffsetMs == nil ||
		len(payload.ProviderID) > 255 || len(payload.ProviderTrackID) > 1024 || len(payload.Language) > 64 ||
		(payload.TimingKind != "plain" && payload.TimingKind != "synced") ||
		(payload.ContentHash != "" && !listenLocalMusicContentHashPattern.MatchString(payload.ContentHash)) ||
		(payload.Availability != "content" && payload.Availability != "refetchRequired" && payload.Availability != "unavailable") ||
		(payload.LicensePolicy != "cacheAllowed" && payload.LicensePolicy != "refetchRequired") {
		return library.ListenLocalMusicMutationResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	if _, err := requireActiveListenLocalMusicTrackTx(ctx, tx, mutation.EntityID); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	selection, found, err := loadListenLocalMusicLyricSelectionTx(ctx, tx, mutation.SubjectID, mutation.EntityID)
	if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if mutation.ExpectedRevision != selection.Revision {
		return library.ListenLocalMusicMutationResult{}, listenLocalMusicCurrentRevisionConflict(
			selection.Revision, listenLocalMusicLyricSelectionFromRow(selection),
		)
	}

	now := mutation.OccurredAt
	document := new(listenLocalMusicLyricDocumentRow)
	err = tx.NewSelect().Model(document).
		Where("track_id = ?", mutation.EntityID).
		Where("source_kind = 'provider'").
		Where("provider_id = ?", payload.ProviderID).
		Where("provider_track_id = ?", payload.ProviderTrackID).
		Where("content_hash = ?", payload.ContentHash).
		Scan(ctx)
	createdDocument := false
	if errors.Is(err, sql.ErrNoRows) {
		documentID := payload.ClientDocumentID
		var collision int
		if err := tx.NewSelect().Table("listen_local_music_lyric_documents").ColumnExpr("COUNT(*)").
			Where("id = ?", documentID).Scan(ctx, &collision); err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
		if collision > 0 {
			documentID = uuid.NewString()
		}
		document = &listenLocalMusicLyricDocumentRow{
			ID: documentID, TrackID: mutation.EntityID, Revision: 1, SourceKind: "provider",
			ProviderID: payload.ProviderID, ProviderTrackID: payload.ProviderTrackID,
			TimingKind: payload.TimingKind, Language: payload.Language, ContentHash: payload.ContentHash,
			Availability: payload.Availability, LicensePolicy: payload.LicensePolicy,
			CreatedAt: now, UpdatedAt: now,
		}
		if _, err := tx.NewInsert().Model(document).Exec(ctx); err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
		if err := clearListenLocalTombstone(ctx, tx, library.ListenLocalMusicEntityLyricDocument, document.ID); err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
		createdDocument = true
	} else if err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	documentCanonical := listenLocalMusicLyricDocumentFromRow(*document)
	if createdDocument {
		if err := appendListenLocalMusicPayloadChange(
			ctx, tx, library.ListenLocalMusicEntityLyricDocument, document.ID, document.Revision, documentCanonical, now,
		); err != nil {
			return library.ListenLocalMusicMutationResult{}, err
		}
	}

	if found && selection.DocumentID == document.ID && selection.OffsetMs == *payload.OffsetMs {
		return newListenLocalMusicMutationResult(mutation, selection.Revision, map[string]any{
			"document":   documentCanonical,
			"selection":  listenLocalMusicLyricSelectionFromRow(selection),
			"idMappings": map[string]string{payload.ClientDocumentID: document.ID},
		})
	}
	selection.SubjectID = mutation.SubjectID
	selection.TrackID = mutation.EntityID
	selection.DocumentID = document.ID
	selection.OffsetMs = *payload.OffsetMs
	selection.Revision = max(selection.Revision+1, int64(1))
	selection.UpdatedAt = now
	if _, err := tx.NewInsert().Model(&selection).
		On("CONFLICT(subject_id, track_id) DO UPDATE").
		Set("document_id = EXCLUDED.document_id").
		Set("offset_ms = EXCLUDED.offset_ms").
		Set("revision = EXCLUDED.revision").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	if err := clearListenLocalTombstone(ctx, tx, library.ListenLocalMusicEntityLyricSelection, selection.TrackID); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	selectionCanonical := listenLocalMusicLyricSelectionFromRow(selection)
	if err := appendListenLocalMusicPayloadChange(
		ctx, tx, library.ListenLocalMusicEntityLyricSelection, selection.TrackID, selection.Revision, selectionCanonical, now,
	); err != nil {
		return library.ListenLocalMusicMutationResult{}, err
	}
	return newListenLocalMusicMutationResult(mutation, selection.Revision, map[string]any{
		"document": documentCanonical, "selection": selectionCanonical,
		"idMappings": map[string]string{payload.ClientDocumentID: document.ID},
	})
}

func requireActiveListenLocalMusicTrackTx(ctx context.Context, tx bun.Tx, trackID string) (listenLocalTrackRow, error) {
	row := new(listenLocalTrackRow)
	if err := tx.NewSelect().Model(row).Where("file_id = ?", strings.TrimSpace(trackID)).Scan(ctx); err != nil {
		return listenLocalTrackRow{}, err
	}
	var excluded int
	if err := tx.NewSelect().Table("listen_local_music_memberships").ColumnExpr("COUNT(*)").
		Where("file_id = ?", strings.TrimSpace(trackID)).Where("state = 'excluded'").Scan(ctx, &excluded); err != nil {
		return listenLocalTrackRow{}, err
	}
	if excluded > 0 {
		return listenLocalTrackRow{}, sql.ErrNoRows
	}
	return *row, nil
}

func loadListenLocalMusicTrackStateTx(
	ctx context.Context,
	tx bun.Tx,
	subjectID string,
	trackID string,
) (listenLocalMusicTrackStateRow, bool, error) {
	row := new(listenLocalMusicTrackStateRow)
	err := tx.NewSelect().Model(row).Where("subject_id = ?", subjectID).Where("track_id = ?", trackID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return listenLocalMusicTrackStateRow{SubjectID: subjectID, TrackID: trackID}, false, nil
	}
	return *row, err == nil, err
}

func saveListenLocalMusicTrackStateTx(ctx context.Context, tx bun.Tx, row listenLocalMusicTrackStateRow) error {
	_, err := tx.NewInsert().Model(&row).
		On("CONFLICT(subject_id, track_id) DO UPDATE").
		Set("revision = EXCLUDED.revision").
		Set("favorite = EXCLUDED.favorite").
		Set("favorite_revision = EXCLUDED.favorite_revision").
		Set("position_ms = EXCLUDED.position_ms").
		Set("play_session_id = EXCLUDED.play_session_id").
		Set("content_identity_revision = EXCLUDED.content_identity_revision").
		Set("progress_revision = EXCLUDED.progress_revision").
		Set("cumulative_listened_ms = EXCLUDED.cumulative_listened_ms").
		Set("play_count = EXCLUDED.play_count").
		Set("skip_count = EXCLUDED.skip_count").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	if err != nil {
		return err
	}
	return clearListenLocalTombstone(ctx, tx, library.ListenLocalMusicEntityTrackState, row.TrackID)
}

func loadListenLocalMusicLyricSelectionTx(
	ctx context.Context,
	tx bun.Tx,
	subjectID string,
	trackID string,
) (listenLocalMusicLyricSelectionRow, bool, error) {
	row := new(listenLocalMusicLyricSelectionRow)
	err := tx.NewSelect().Model(row).Where("subject_id = ?", subjectID).Where("track_id = ?", trackID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return listenLocalMusicLyricSelectionRow{SubjectID: subjectID, TrackID: trackID}, false, nil
	}
	return *row, err == nil, err
}
