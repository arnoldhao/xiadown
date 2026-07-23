package libraryrepo

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteListenLocalMusicWriteRepositoryStateLyricsAndReceipts(t *testing.T) {
	ctx := context.Background()
	database, trackID, contentRevision := newListenLocalMusicWriteFixture(t, ctx)
	defer database.Close()
	repo := NewSQLiteListenLocalMusicWriteRepository(database.Bun)
	pending := listenLocalMusicTestMutation("manage", "createPlaylist", uuid.NewString(), 0, `{"name":"Pending"}`)
	pending.DependsOnMutationID = uuid.NewString()
	if _, err := repo.ApplyMutation(ctx, pending); !errors.Is(err, library.ErrListenLocalMusicDependencyPending) {
		t.Fatalf("missing mutation dependency err=%v", err)
	}

	favorite := listenLocalMusicTestMutation("state", "setFavorite", trackID, 0, `{"favorite":true}`)
	result, err := repo.ApplyMutation(ctx, favorite)
	if err != nil || result.Revision != 1 || result.Replayed {
		t.Fatalf("set favorite result=%#v err=%v", result, err)
	}
	replayed, err := repo.ApplyMutation(ctx, favorite)
	if err != nil || !replayed.Replayed || replayed.Revision != result.Revision {
		t.Fatalf("favorite replay=%#v err=%v", replayed, err)
	}
	changedHash := favorite
	changedHash.RequestHash = "sha256:" + strings.Repeat("b", 64)
	if _, err := repo.ApplyMutation(ctx, changedHash); !errors.Is(err, library.ErrListenLocalMusicIdempotencyConflict) {
		t.Fatalf("same mutation different hash err=%v", err)
	}
	stale := listenLocalMusicTestMutation("state", "setFavorite", trackID, 0, `{"favorite":false}`)
	if _, err := repo.ApplyMutation(ctx, stale); !errors.Is(err, library.ErrListenLocalMusicRevisionConflict) {
		t.Fatalf("stale favorite err=%v", err)
	}

	sessionID := uuid.NewString()
	progress := listenLocalMusicTestMutation("state", "setProgress", trackID, 0, mustMusicJSON(t, map[string]any{
		"positionMs": int64(9_000), "playSessionId": sessionID, "contentIdentityRevision": contentRevision,
	}))
	if result, err := repo.ApplyMutation(ctx, progress); err != nil || result.Revision != 1 {
		t.Fatalf("set progress result=%#v err=%v", result, err)
	}

	clientDocumentID := uuid.NewString()
	selectPayload := map[string]any{
		"clientDocumentId": clientDocumentID, "providerId": "lrclib", "providerTrackId": "candidate-7",
		"timingKind": "synced", "language": "zh", "contentHash": strings.Repeat("c", 64),
		"availability": "refetchRequired", "licensePolicy": "refetchRequired", "offsetMs": int64(-120),
	}
	selection := listenLocalMusicTestMutation("state", "selectProviderLyric", trackID, 0, mustMusicJSON(t, selectPayload))
	selected, err := repo.ApplyMutation(ctx, selection)
	if err != nil || selected.Revision != 1 {
		t.Fatalf("select lyric result=%#v err=%v", selected, err)
	}
	var selectedPayload struct {
		Document   library.ListenLocalMusicLyricDocument  `json:"document"`
		Selection  library.ListenLocalMusicLyricSelection `json:"selection"`
		IDMappings map[string]string                      `json:"idMappings"`
	}
	if err := json.Unmarshal(selected.Result, &selectedPayload); err != nil ||
		selectedPayload.Document.ID != clientDocumentID || selectedPayload.Selection.DocumentID != clientDocumentID ||
		selectedPayload.IDMappings[clientDocumentID] != clientDocumentID {
		t.Fatalf("canonical lyric payload=%s err=%v", selected.Result, err)
	}
	secondClientID := uuid.NewString()
	selectPayload["clientDocumentId"] = secondClientID
	secondSelection := listenLocalMusicTestMutation("state", "selectProviderLyric", trackID, 1, mustMusicJSON(t, selectPayload))
	remapped, err := repo.ApplyMutation(ctx, secondSelection)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(remapped.Result, &selectedPayload); err != nil || selectedPayload.IDMappings[secondClientID] != clientDocumentID {
		t.Fatalf("deduplicated lyric result=%s err=%v", remapped.Result, err)
	}

	readRepo := NewSQLiteListenLocalMusicReadRepository(database.Bun, "catalog-test")
	position, err := readRepo.GetSyncPosition(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := readRepo.ListSnapshot(ctx, library.ListenLocalMusicSnapshotQuery{
		Epoch: position.Epoch, HighWater: position.HighWater, Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, entity := range snapshot.Entities {
		seen[entity.EntityType] = true
	}
	for _, entityType := range []string{"track_state", "lyric_document", "lyric_selection"} {
		if !seen[entityType] {
			t.Fatalf("snapshot omitted %s: %#v", entityType, seen)
		}
	}
}

func TestSQLiteListenLocalMusicMembershipConflictUsesPublicJSONShape(t *testing.T) {
	ctx := context.Background()
	database, trackID, _ := newListenLocalMusicWriteFixture(t, ctx)
	defer database.Close()
	repo := NewSQLiteListenLocalMusicWriteRepository(database.Bun)

	created, err := repo.ApplyMutation(ctx, listenLocalMusicTestMutation(
		"manage", "setMembership", trackID, 0, `{"state":"excluded","reason":"user"}`,
	))
	if err != nil || created.Revision != 1 {
		t.Fatalf("create membership=%#v err=%v", created, err)
	}

	_, err = repo.ApplyMutation(ctx, listenLocalMusicTestMutation(
		"manage", "setMembership", trackID, 0, `{"state":"included"}`,
	))
	var conflict *library.ListenLocalMusicRevisionConflictError
	if !errors.As(err, &conflict) || conflict.CurrentRevision != 1 {
		t.Fatalf("membership conflict=%#v err=%v", conflict, err)
	}
	var current map[string]any
	if err := json.Unmarshal(conflict.Current, &current); err != nil {
		t.Fatal(err)
	}
	if current["fileId"] != trackID || current["state"] != "excluded" || current["reason"] != "user" {
		t.Fatalf("public membership conflict payload=%s", conflict.Current)
	}
	if _, leaked := current["FileID"]; leaked {
		t.Fatalf("domain field names leaked in conflict payload=%s", conflict.Current)
	}
}

func TestSQLiteListenLocalMusicWriteRepositoryPlaylistAndPlaySessionSemantics(t *testing.T) {
	ctx := context.Background()
	database, trackID, contentRevision := newListenLocalMusicWriteFixture(t, ctx)
	defer database.Close()
	repo := NewSQLiteListenLocalMusicWriteRepository(database.Bun)

	playlistID := uuid.NewString()
	created, err := repo.ApplyMutation(ctx, listenLocalMusicTestMutation(
		"manage", "createPlaylist", playlistID, 0, `{"name":"Offline Mix"}`,
	))
	if err != nil || created.Revision != 1 {
		t.Fatalf("create playlist=%#v err=%v", created, err)
	}
	itemA, itemB := uuid.NewString(), uuid.NewString()
	addedA, err := repo.ApplyMutation(ctx, listenLocalMusicTestMutation(
		"manage", "addPlaylistItem", playlistID, 1,
		mustMusicJSON(t, map[string]any{"clientItemId": itemA, "trackId": trackID}),
	))
	if err != nil || addedA.Revision != 2 {
		t.Fatalf("add A=%#v err=%v", addedA, err)
	}
	addedB, err := repo.ApplyMutation(ctx, listenLocalMusicTestMutation(
		"manage", "addPlaylistItem", playlistID, 2,
		mustMusicJSON(t, map[string]any{"clientItemId": itemB, "trackId": trackID}),
	))
	if err != nil || addedB.Revision != 3 {
		t.Fatalf("duplicate Track add B=%#v err=%v", addedB, err)
	}
	reordered, err := repo.ApplyMutation(ctx, listenLocalMusicTestMutation(
		"manage", "reorderPlaylist", playlistID, 3,
		mustMusicJSON(t, map[string]any{"itemIds": []string{itemB, itemA}}),
	))
	if err != nil || reordered.Revision != 4 {
		t.Fatalf("reorder=%#v err=%v", reordered, err)
	}
	_, err = repo.ApplyMutation(ctx, listenLocalMusicTestMutation(
		"manage", "reorderPlaylist", playlistID, 3,
		mustMusicJSON(t, map[string]any{"itemIds": []string{itemA, itemB}}),
	))
	var reorderConflict *library.ListenLocalMusicRevisionConflictError
	if !errors.As(err, &reorderConflict) || reorderConflict.CurrentRevision != 4 {
		t.Fatalf("stale reorder conflict=%#v err=%v", reorderConflict, err)
	}
	items, err := NewSQLiteListenLocalPlaylistRepository(database.Bun).ListItems(ctx, playlistID)
	if err != nil || len(items) != 2 || items[0].ID != itemB || items[1].ID != itemA ||
		items[0].FileID != trackID || items[1].FileID != trackID {
		t.Fatalf("duplicate/reordered items=%#v err=%v", items, err)
	}
	removed, err := repo.ApplyMutation(ctx, listenLocalMusicTestMutation(
		"manage", "removePlaylistItem", itemA, 4, mustMusicJSON(t, map[string]any{"playlistId": playlistID}),
	))
	if err != nil || removed.Revision != 5 {
		t.Fatalf("remove=%#v err=%v", removed, err)
	}
	var itemTombstones int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM listen_local_music_tombstones
WHERE entity_type = 'playlist_item' AND entity_id = ?
`, itemA).Scan(&itemTombstones); err != nil || itemTombstones != 1 {
		t.Fatalf("item tombstone count=%d err=%v", itemTombstones, err)
	}
	deleted, err := repo.ApplyMutation(ctx, listenLocalMusicTestMutation(
		"manage", "deletePlaylist", playlistID, 5, `{}`,
	))
	if err != nil || deleted.Revision != 6 {
		t.Fatalf("delete playlist=%#v err=%v", deleted, err)
	}
	var playlistTombstones, remainingItems int
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM listen_local_music_tombstones
WHERE entity_type = 'playlist' AND entity_id = ? AND revision = 6
`, playlistID).Scan(&playlistTombstones); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `
SELECT COUNT(*) FROM listen_local_playlist_items WHERE playlist_id = ?
`, playlistID).Scan(&remainingItems); err != nil {
		t.Fatal(err)
	}
	if playlistTombstones != 1 || remainingItems != 0 {
		t.Fatalf("playlist delete tombstones=%d remainingItems=%d", playlistTombstones, remainingItems)
	}

	playSessionID := uuid.NewString()
	first := library.ListenLocalMusicPlayEvent{
		SubjectID: library.ListenLocalMusicSubjectID, ActorDeviceID: "iphone-test",
		EventID: uuid.NewString(), RequestHash: "sha256:" + strings.Repeat("d", 64),
		PlaySessionID: playSessionID, Sequence: 1, TrackID: trackID,
		ContentIdentityRevision: contentRevision, CumulativeListenedDurationMs: 20_000, PositionMs: 20_000,
	}
	firstResult, err := repo.ApplyPlayEvent(ctx, first)
	if err != nil || !firstResult.Accepted || firstResult.Sequence != 1 || firstResult.TrackState == nil {
		t.Fatalf("first play checkpoint=%#v err=%v", firstResult, err)
	}
	terminal := first
	terminal.EventID = uuid.NewString()
	terminal.RequestHash = "sha256:" + strings.Repeat("e", 64)
	terminal.Sequence = 3
	terminal.CumulativeListenedDurationMs = 80_000
	terminal.PositionMs = 80_000
	terminal.Terminal = true
	terminal.Completed = true
	terminal.EndReason = "completed"
	terminalResult, err := repo.ApplyPlayEvent(ctx, terminal)
	if err != nil || !terminalResult.Accepted || terminalResult.TrackState == nil || terminalResult.TrackState.PlayCount != 1 {
		t.Fatalf("terminal checkpoint=%#v err=%v", terminalResult, err)
	}
	retry, err := repo.ApplyPlayEvent(ctx, terminal)
	if err != nil || !retry.Replayed || retry.TrackState == nil || retry.TrackState.PlayCount != 1 {
		t.Fatalf("terminal replay=%#v err=%v", retry, err)
	}
	older := first
	older.EventID = uuid.NewString()
	older.RequestHash = "sha256:" + strings.Repeat("f", 64)
	older.Sequence = 2
	older.CumulativeListenedDurationMs = 10_000
	older.PositionMs = 10_000
	olderResult, err := repo.ApplyPlayEvent(ctx, older)
	if err != nil || olderResult.Accepted || olderResult.Sequence != 3 || !olderResult.Terminal {
		t.Fatalf("old checkpoint ack=%#v err=%v", olderResult, err)
	}
	sequenceMismatch := first
	sequenceMismatch.EventID = uuid.NewString()
	sequenceMismatch.RequestHash = "sha256:" + strings.Repeat("0", 64)
	if _, err := repo.ApplyPlayEvent(ctx, sequenceMismatch); !errors.Is(err, library.ErrListenLocalMusicIdempotencyConflict) {
		t.Fatalf("same sequence different payload err=%v", err)
	}
}

func listenLocalMusicTestMutation(
	family string,
	mutationType string,
	entityID string,
	expectedRevision int64,
	payload string,
) library.ListenLocalMusicMutation {
	return library.ListenLocalMusicMutation{
		SubjectID: library.ListenLocalMusicSubjectID, ActorDeviceID: "iphone-test", Family: family,
		MutationID: uuid.NewString(), RequestHash: "sha256:" + strings.Repeat("a", 64),
		Type: mutationType, EntityID: entityID, ExpectedRevision: expectedRevision,
		Payload: json.RawMessage(payload), OccurredAt: time.Now().UTC(),
	}
}

func mustMusicJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func newListenLocalMusicWriteFixture(
	t *testing.T,
	ctx context.Context,
) (*persistence.Database, string, int64) {
	t.Helper()
	database, err := openLibraryRepoTestDatabase(t, ctx, "music-write.db")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 21, 16, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: uuid.NewString(), Name: "Music", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := NewSQLiteLibraryRepository(database.Bun).Save(ctx, libraryItem); err != nil {
		database.Close()
		t.Fatal(err)
	}
	trackID := uuid.NewString()
	path := filepath.Join(t.TempDir(), "track.mp3")
	file, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: trackID, LibraryID: libraryItem.ID, Kind: string(library.FileKindAudio), Name: "track.mp3",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: path},
		Origin: library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{
			ImportPath: path, ImportedAt: now,
		}},
		State: library.FileState{Status: "active"}, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := NewSQLiteFileRepository(database.Bun).Save(ctx, file); err != nil {
		database.Close()
		t.Fatal(err)
	}
	duration := int64(180_000)
	track, err := library.NewListenLocalTrack(library.ListenLocalTrackParams{
		FileID: trackID, LibraryID: libraryItem.ID, LocalPath: path,
		Title: "Track", Author: "Artist", Album: "Album", DurationMs: &duration,
		Availability: library.ListenLocalTrackAvailable, LastCheckedAt: &now,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := NewSQLiteListenLocalTrackRepository(database.Bun).Save(ctx, track); err != nil {
		database.Close()
		t.Fatal(err)
	}
	stored, err := NewSQLiteListenLocalTrackRepository(database.Bun).Get(ctx, trackID)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	return database, trackID, stored.ContentIdentityRevision
}
