package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type countingMusicOperationRepository struct {
	*deleteRuleOperationRepo
	listCalls int
}

func (repo *countingMusicOperationRepository) List(ctx context.Context) ([]library.LibraryOperation, error) {
	repo.listCalls++
	return repo.deleteRuleOperationRepo.List(ctx)
}

func TestMusicIOSRepresentationStatusUsesDurableOpaqueOperationLifecycle(t *testing.T) {
	now := time.Now().UTC()
	operations := []library.LibraryOperation{
		testMusicIOSRepresentationOperation(t, "track-1", "6d0ce530-1fc3-4da4-9328-a59a139336e2", library.OperationStatusFailed, now),
		testMusicIOSRepresentationOperation(t, "track-1", "c50a667a-e38f-4371-803e-bc69d190c1f3", library.OperationStatusRunning, now.Add(time.Second)),
	}
	status := musicIOSRepresentationStatusForTrack(operations, "track-1")
	if status.Status != library.ListenLocalMusicCompatibleRepresentationGenerating || status.ErrorCode != "" {
		t.Fatalf("active status=%#v", status)
	}
	status = musicIOSRepresentationStatusForTrack(operations[:1], "track-1")
	if status.Status != library.ListenLocalMusicCompatibleRepresentationFailed || status.ErrorCode != "generation_failed" {
		t.Fatalf("failed status=%#v", status)
	}
	status = musicIOSRepresentationStatusForTrack(nil, "track-1")
	if status.Status != library.ListenLocalMusicCompatibleRepresentationUnsupported {
		t.Fatalf("empty status=%#v", status)
	}
	// The private operation error is intentionally not part of any projected
	// representation status.
	operations[0].ErrorMessage = "/Users/private/music/input.ogg: decoder exploded"
	status = musicIOSRepresentationStatusForTrack(operations[:1], "track-1")
	if status.ErrorCode != "generation_failed" {
		t.Fatalf("private error leaked through status=%#v", status)
	}
}

func TestMusicIOSRepresentationStatusesReadOperationJournalOncePerTrackBatch(t *testing.T) {
	now := time.Now().UTC()
	first := testMusicIOSRepresentationOperation(
		t, "track-1", "6d0ce530-1fc3-4da4-9328-a59a139336e2", library.OperationStatusRunning, now,
	)
	second := testMusicIOSRepresentationOperation(
		t, "track-2", "c50a667a-e38f-4371-803e-bc69d190c1f3", library.OperationStatusFailed, now,
	)
	operations := &countingMusicOperationRepository{deleteRuleOperationRepo: &deleteRuleOperationRepo{
		items: map[string]library.LibraryOperation{first.ID: first, second.ID: second},
	}}
	service := &LibraryService{
		localTracks: &listenLocalRefreshTrackRepository{items: map[string]library.ListenLocalTrack{
			"track-1": {FileID: "track-1"}, "track-2": {FileID: "track-2"},
		}},
		operations: operations,
	}
	statuses, err := service.GetIOSCompatibleRepresentationStatuses(
		context.Background(), []string{"track-1", "track-2"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if operations.listCalls != 1 {
		t.Fatalf("operation journal List calls=%d, want one for the whole Track page", operations.listCalls)
	}
	if statuses["track-1"].Status != library.ListenLocalMusicCompatibleRepresentationGenerating ||
		statuses["track-2"].Status != library.ListenLocalMusicCompatibleRepresentationFailed {
		t.Fatalf("batch statuses=%#v", statuses)
	}
}

func TestMusicIOSRepresentationRequestReplaysRequestUUIDAndRejectsCrossTrackReuse(t *testing.T) {
	ctx := context.Background()
	requestID := "6d0ce530-1fc3-4da4-9328-a59a139336e2"
	operation := testMusicIOSRepresentationOperation(
		t, "track-1", requestID, library.OperationStatusQueued, time.Now().UTC(),
	)
	tracks := &listenLocalRefreshTrackRepository{items: map[string]library.ListenLocalTrack{
		"track-1": {FileID: "track-1", Availability: library.ListenLocalTrackAvailable},
		"track-2": {FileID: "track-2", Availability: library.ListenLocalTrackAvailable},
	}}
	files := &deleteRuleFileRepo{items: map[string]library.LibraryFile{
		"track-1": {ID: "track-1", Storage: library.FileStorage{LocalPath: "/private/track-1.ogg"}},
		"track-2": {ID: "track-2", Storage: library.FileStorage{LocalPath: "/private/track-2.ogg"}},
	}}
	operations := &deleteRuleOperationRepo{items: map[string]library.LibraryOperation{operation.ID: operation}}
	memberships := &listenLocalMembershipRepository{}
	service := &LibraryService{
		localTracks: tracks, localMusicMemberships: memberships, files: files, operations: operations,
	}

	status, err := service.RequestIOSCompatibleRepresentation(ctx, "track-1", requestID)
	if err != nil || status.Status != library.ListenLocalMusicCompatibleRepresentationGenerating {
		t.Fatalf("idempotent replay status=%#v err=%v", status, err)
	}
	if len(operations.items) != 1 {
		t.Fatalf("idempotent replay enqueued another operation: %d", len(operations.items))
	}
	status, err = service.RequestIOSCompatibleRepresentation(
		ctx, "track-1", "7edec2d3-cd72-48d4-868b-1448bd220f62",
	)
	if err != nil || status.Status != library.ListenLocalMusicCompatibleRepresentationGenerating ||
		len(operations.items) != 1 {
		t.Fatalf("active Track retry enqueued a second operation: status=%#v count=%d err=%v", status, len(operations.items), err)
	}
	_, err = service.RequestIOSCompatibleRepresentation(ctx, "track-2", requestID)
	if !errors.Is(err, library.ErrListenLocalMusicCompatibleRepresentationConflict) {
		t.Fatalf("cross-Track request ID error=%v", err)
	}
}

func testMusicIOSRepresentationOperation(
	t *testing.T,
	trackID string,
	requestID string,
	status library.OperationStatus,
	createdAt time.Time,
) library.LibraryOperation {
	t.Helper()
	input, err := json.Marshal(dto.CreateTranscodeJobRequest{
		FileID: trackID, PresetID: musicIOSRepresentationPresetID,
		Source: musicIOSRepresentationSource, RunID: requestID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return library.LibraryOperation{
		ID: requestID, LibraryID: "library-1", Kind: "transcode", Status: status,
		DisplayName: "Compatible Audio", InputJSON: string(input), CreatedAt: createdAt,
	}
}
