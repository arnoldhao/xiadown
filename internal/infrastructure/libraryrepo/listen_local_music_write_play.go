package libraryrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

const maxRetainedListenLocalMusicPlayEventReceipts = 100_000

type listenLocalMusicPlaySessionRow struct {
	bun.BaseModel           `bun:"table:listen_local_music_play_sessions"`
	SubjectID               string    `bun:"subject_id,pk"`
	PlaySessionID           string    `bun:"play_session_id,pk"`
	TrackID                 string    `bun:"track_id"`
	ContentIdentityRevision int64     `bun:"content_identity_revision"`
	MaxSequence             int64     `bun:"max_sequence"`
	CumulativeListenedMs    int64     `bun:"cumulative_listened_ms"`
	PositionMs              int64     `bun:"position_ms"`
	Terminal                bool      `bun:"terminal"`
	Completed               bool      `bun:"completed"`
	EndReason               string    `bun:"end_reason"`
	UpdatedAt               time.Time `bun:"updated_at"`
}

type listenLocalMusicPlayCheckpointRow struct {
	bun.BaseModel        `bun:"table:listen_local_music_play_event_checkpoints"`
	SubjectID            string       `bun:"subject_id,pk"`
	PlaySessionID        string       `bun:"play_session_id,pk"`
	Sequence             int64        `bun:"sequence,pk"`
	EventID              string       `bun:"event_id"`
	RequestHash          string       `bun:"request_hash"`
	CumulativeListenedMs int64        `bun:"cumulative_listened_ms"`
	PositionMs           int64        `bun:"position_ms"`
	Terminal             bool         `bun:"terminal"`
	Completed            bool         `bun:"completed"`
	EndReason            string       `bun:"end_reason"`
	DeviceOccurredAt     sql.NullTime `bun:"device_occurred_at"`
	ReceivedAt           time.Time    `bun:"received_at"`
}

type listenLocalMusicPlayReceiptRow struct {
	bun.BaseModel   `bun:"table:listen_local_music_play_event_receipts"`
	ReceiptSequence int64     `bun:"receipt_sequence,pk,autoincrement"`
	SubjectID       string    `bun:"subject_id"`
	EventID         string    `bun:"event_id"`
	RequestHash     string    `bun:"request_hash"`
	ResultJSON      string    `bun:"result_json"`
	CreatedAt       time.Time `bun:"created_at"`
}

func (repo *SQLiteListenLocalMusicWriteRepository) ApplyPlayEvent(
	ctx context.Context,
	event library.ListenLocalMusicPlayEvent,
) (library.ListenLocalMusicPlayEventResult, error) {
	event = normalizeListenLocalMusicPlayEvent(event)
	if !validListenLocalMusicPlayEvent(event) {
		return library.ListenLocalMusicPlayEventResult{}, library.ErrInvalidListenLocalMusicMutation
	}
	var result library.ListenLocalMusicPlayEventResult
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		replayed, found, err := listenLocalMusicPlayReceiptTx(ctx, tx, event)
		if err != nil {
			return err
		}
		if found {
			result = replayed
			return nil
		}
		track, err := requireActiveListenLocalMusicTrackTx(ctx, tx, event.TrackID)
		if err != nil {
			return err
		}
		if track.ContentIdentityRevision != event.ContentIdentityRevision {
			return library.ErrListenLocalMusicContentChanged
		}

		session, sessionFound, err := loadListenLocalMusicPlaySessionTx(ctx, tx, event)
		if err != nil {
			return err
		}
		if sessionFound && (session.TrackID != event.TrackID ||
			session.ContentIdentityRevision != event.ContentIdentityRevision) {
			return library.ErrListenLocalMusicContentChanged
		}
		state, stateFound, err := loadListenLocalMusicTrackStateTx(ctx, tx, event.SubjectID, event.TrackID)
		if err != nil {
			return err
		}
		var currentState *library.ListenLocalMusicTrackState
		if stateFound {
			value := listenLocalMusicTrackStateFromRow(state)
			currentState = &value
		}
		checkpointByEvent := new(listenLocalMusicPlayCheckpointRow)
		err = tx.NewSelect().Model(checkpointByEvent).
			Where("subject_id = ?", event.SubjectID).
			Where("event_id = ?", event.EventID).
			Scan(ctx)
		if err == nil {
			if checkpointByEvent.PlaySessionID != event.PlaySessionID ||
				!strings.EqualFold(checkpointByEvent.RequestHash, event.RequestHash) {
				return library.ErrListenLocalMusicIdempotencyConflict
			}
			result = listenLocalMusicPlayResult(event, session, false, true, currentState)
			return storeListenLocalMusicPlayReceiptTx(ctx, tx, event, result)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		checkpoint := new(listenLocalMusicPlayCheckpointRow)
		err = tx.NewSelect().Model(checkpoint).
			Where("subject_id = ?", event.SubjectID).
			Where("play_session_id = ?", event.PlaySessionID).
			Where("sequence = ?", event.Sequence).
			Scan(ctx)
		if err == nil {
			if !strings.EqualFold(checkpoint.RequestHash, event.RequestHash) {
				return library.ErrListenLocalMusicIdempotencyConflict
			}
			result = listenLocalMusicPlayResult(event, session, false, true, currentState)
			return storeListenLocalMusicPlayReceiptTx(ctx, tx, event, result)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		if sessionFound && event.Sequence == session.MaxSequence {
			return library.ErrListenLocalMusicIdempotencyConflict
		}
		if sessionFound && (event.Sequence < session.MaxSequence || session.Terminal) {
			result = listenLocalMusicPlayResult(event, session, false, false, currentState)
			return storeListenLocalMusicPlayReceiptTx(ctx, tx, event, result)
		}
		if sessionFound && event.CumulativeListenedDurationMs < session.CumulativeListenedMs {
			return library.ErrInvalidListenLocalMusicMutation
		}
		priorCumulative := int64(0)
		if sessionFound {
			priorCumulative = session.CumulativeListenedMs
		}
		delta := event.CumulativeListenedDurationMs - priorCumulative
		if delta > math.MaxInt64-state.CumulativeListenedMs {
			return library.ErrInvalidListenLocalMusicMutation
		}
		state.SubjectID = event.SubjectID
		state.TrackID = event.TrackID
		state.Revision = max(state.Revision+1, int64(1))
		state.PositionMs = event.PositionMs
		state.PlaySessionID = event.PlaySessionID
		state.ContentIdentityRevision = event.ContentIdentityRevision
		state.ProgressRevision++
		state.CumulativeListenedMs += delta
		if event.Terminal {
			if event.Completed {
				state.PlayCount++
			} else {
				state.SkipCount++
			}
		}
		state.UpdatedAt = event.ReceivedAt
		if err := saveListenLocalMusicTrackStateTx(ctx, tx, state); err != nil {
			return err
		}

		session.SubjectID = event.SubjectID
		session.PlaySessionID = event.PlaySessionID
		session.TrackID = event.TrackID
		session.ContentIdentityRevision = event.ContentIdentityRevision
		session.MaxSequence = event.Sequence
		session.CumulativeListenedMs = event.CumulativeListenedDurationMs
		session.PositionMs = event.PositionMs
		session.Terminal = event.Terminal
		session.Completed = event.Completed
		session.EndReason = event.EndReason
		session.UpdatedAt = event.ReceivedAt
		if err := saveListenLocalMusicPlaySessionTx(ctx, tx, session); err != nil {
			return err
		}
		checkpoint = &listenLocalMusicPlayCheckpointRow{
			SubjectID: event.SubjectID, PlaySessionID: event.PlaySessionID, Sequence: event.Sequence,
			EventID: event.EventID, RequestHash: event.RequestHash,
			CumulativeListenedMs: event.CumulativeListenedDurationMs, PositionMs: event.PositionMs,
			Terminal: event.Terminal, Completed: event.Completed, EndReason: event.EndReason,
			DeviceOccurredAt: nullTime(event.DeviceOccurredAt), ReceivedAt: event.ReceivedAt,
		}
		if _, err := tx.NewInsert().Model(checkpoint).Exec(ctx); err != nil {
			return err
		}
		canonicalState := listenLocalMusicTrackStateFromRow(state)
		if err := appendListenLocalMusicPayloadChange(
			ctx, tx, library.ListenLocalMusicEntityTrackState, event.TrackID,
			state.Revision, canonicalState, event.ReceivedAt,
		); err != nil {
			return err
		}
		result = listenLocalMusicPlayResult(event, session, true, false, &canonicalState)
		return storeListenLocalMusicPlayReceiptTx(ctx, tx, event, result)
	})
	return result, err
}

func normalizeListenLocalMusicPlayEvent(event library.ListenLocalMusicPlayEvent) library.ListenLocalMusicPlayEvent {
	event.SubjectID = strings.TrimSpace(event.SubjectID)
	event.ActorDeviceID = strings.TrimSpace(event.ActorDeviceID)
	event.EventID = strings.ToLower(strings.TrimSpace(event.EventID))
	event.RequestHash = strings.ToLower(strings.TrimSpace(event.RequestHash))
	event.PlaySessionID = strings.ToLower(strings.TrimSpace(event.PlaySessionID))
	event.TrackID = strings.TrimSpace(event.TrackID)
	event.EndReason = strings.TrimSpace(event.EndReason)
	if event.DeviceOccurredAt != nil {
		value := event.DeviceOccurredAt.UTC()
		event.DeviceOccurredAt = &value
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = time.Now().UTC()
	} else {
		event.ReceivedAt = event.ReceivedAt.UTC()
	}
	return event
}

func validListenLocalMusicPlayEvent(event library.ListenLocalMusicPlayEvent) bool {
	return event.SubjectID == library.ListenLocalMusicSubjectID && event.ActorDeviceID != "" &&
		validClientMusicUUID(event.EventID) && validClientMusicUUID(event.PlaySessionID) &&
		listenLocalMusicRequestHashPattern.MatchString(event.RequestHash) &&
		event.Sequence > 0 && event.TrackID != "" && len(event.TrackID) <= 255 &&
		event.ContentIdentityRevision > 0 && event.CumulativeListenedDurationMs >= 0 &&
		event.PositionMs >= 0 && (!event.Completed || event.Terminal) && len(event.EndReason) <= 120
}

func listenLocalMusicPlayReceiptTx(
	ctx context.Context,
	tx bun.Tx,
	event library.ListenLocalMusicPlayEvent,
) (library.ListenLocalMusicPlayEventResult, bool, error) {
	row := new(listenLocalMusicPlayReceiptRow)
	err := tx.NewSelect().Model(row).
		Where("subject_id = ?", event.SubjectID).
		Where("event_id = ?", event.EventID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return library.ListenLocalMusicPlayEventResult{}, false, nil
	}
	if err != nil {
		return library.ListenLocalMusicPlayEventResult{}, false, err
	}
	if !strings.EqualFold(row.RequestHash, event.RequestHash) {
		return library.ListenLocalMusicPlayEventResult{}, false, library.ErrListenLocalMusicIdempotencyConflict
	}
	var result library.ListenLocalMusicPlayEventResult
	if err := json.Unmarshal([]byte(row.ResultJSON), &result); err != nil || result.EventID == "" {
		if err == nil {
			err = errors.New("Music play-event receipt is incomplete")
		}
		return library.ListenLocalMusicPlayEventResult{}, false, err
	}
	result.Replayed = true
	return result, true, nil
}

func loadListenLocalMusicPlaySessionTx(
	ctx context.Context,
	tx bun.Tx,
	event library.ListenLocalMusicPlayEvent,
) (listenLocalMusicPlaySessionRow, bool, error) {
	row := new(listenLocalMusicPlaySessionRow)
	err := tx.NewSelect().Model(row).
		Where("subject_id = ?", event.SubjectID).
		Where("play_session_id = ?", event.PlaySessionID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return listenLocalMusicPlaySessionRow{
			SubjectID: event.SubjectID, PlaySessionID: event.PlaySessionID,
			TrackID: event.TrackID, ContentIdentityRevision: event.ContentIdentityRevision,
		}, false, nil
	}
	return *row, err == nil, err
}

func saveListenLocalMusicPlaySessionTx(ctx context.Context, tx bun.Tx, row listenLocalMusicPlaySessionRow) error {
	_, err := tx.NewInsert().Model(&row).
		On("CONFLICT(subject_id, play_session_id) DO UPDATE").
		Set("max_sequence = EXCLUDED.max_sequence").
		Set("cumulative_listened_ms = EXCLUDED.cumulative_listened_ms").
		Set("position_ms = EXCLUDED.position_ms").
		Set("terminal = EXCLUDED.terminal").
		Set("completed = EXCLUDED.completed").
		Set("end_reason = EXCLUDED.end_reason").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func listenLocalMusicPlayResult(
	event library.ListenLocalMusicPlayEvent,
	session listenLocalMusicPlaySessionRow,
	accepted bool,
	replayed bool,
	state *library.ListenLocalMusicTrackState,
) library.ListenLocalMusicPlayEventResult {
	stateRevision := int64(0)
	if state != nil {
		stateRevision = state.Revision
	}
	return library.ListenLocalMusicPlayEventResult{
		EventID: event.EventID, PlaySessionID: event.PlaySessionID,
		Sequence: session.MaxSequence, CumulativeListenedDurationMs: session.CumulativeListenedMs,
		PositionMs: session.PositionMs, Terminal: session.Terminal,
		Accepted: accepted, Replayed: replayed, TrackStateRevision: stateRevision, TrackState: state,
	}
}

func storeListenLocalMusicPlayReceiptTx(
	ctx context.Context,
	tx bun.Tx,
	event library.ListenLocalMusicPlayEvent,
	result library.ListenLocalMusicPlayEventResult,
) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	row := listenLocalMusicPlayReceiptRow{
		SubjectID: event.SubjectID, EventID: event.EventID, RequestHash: event.RequestHash,
		ResultJSON: string(encoded), CreatedAt: event.ReceivedAt,
	}
	if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
		return err
	}
	_, err = tx.NewRaw(`
DELETE FROM listen_local_music_play_event_receipts
WHERE subject_id = ? AND receipt_sequence IN (
  SELECT receipt_sequence
  FROM listen_local_music_play_event_receipts
  WHERE subject_id = ?
  ORDER BY receipt_sequence DESC
  LIMIT -1 OFFSET ?
)
`, event.SubjectID, event.SubjectID, maxRetainedListenLocalMusicPlayEventReceipts).Exec(ctx)
	return err
}
