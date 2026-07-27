package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/libraryrepo"
)

func TestProfessionalImportOnlyProbesMediaKinds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind string
		want bool
	}{
		{kind: "", want: true},
		{kind: string(library.FileKindVideo), want: true},
		{kind: string(library.FileKindAudio), want: true},
		{kind: string(library.FileKindTranscode), want: true},
		{kind: string(library.FileKindDocument), want: false},
		{kind: string(library.FileKindFont), want: false},
		{kind: string(library.FileKindArchive), want: false},
		{kind: string(library.FileKindOther), want: false},
	}
	for _, test := range tests {
		if got := professionalImportNeedsMediaProbe(test.kind); got != test.want {
			t.Errorf("professionalImportNeedsMediaProbe(%q) = %v, want %v", test.kind, got, test.want)
		}
	}
}

func TestProfessionalImportUsesFileHistoryAndEventRegistrationIdempotently(t *testing.T) {
	ctx := context.Background()
	database := openLibraryServiceTestDatabase(t, "professional-import.db")
	source := filepath.Join(t.TempDir(), "source.pdf")
	storage := filepath.Join(t.TempDir(), "managed.pdf")
	if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storage, []byte("managed"), 0o600); err != nil {
		t.Fatal(err)
	}
	libraries := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	files := libraryrepo.NewSQLiteFileRepository(database.Bun)
	histories := libraryrepo.NewSQLiteHistoryRepository(database.Bun)
	events := libraryrepo.NewSQLiteFileEventRepository(database.Bun)
	toolDirectory := t.TempDir()
	writeFFprobeTestFixture(t, toolDirectory, `{
		"streams": [{"codec_type": "video", "codec_name": "h264", "width": 1920, "height": 1080}],
		"format": {"format_name": "mov,mp4,m4a,3gp,3g2,mj2", "size": "7"}
	}`)
	service := &LibraryService{
		libraries: libraries, files: files, histories: histories, fileEvents: events,
		tools: &mediaProbeToolResolverStub{ready: true, toolDir: toolDirectory},
	}
	if _, err := service.EnsureProfessionalImportLibrary(ctx, "import-library", "Imported Library"); err != nil {
		t.Fatal(err)
	}
	request := ProfessionalImportRequest{
		BatchID: "batch-1", CandidateID: "candidate-1", LibraryID: "import-library",
		SourcePath: source, StoragePath: storage, DisplayName: "Source Book", Kind: "document",
		FileID: "file-1", HistoryID: "history-1", FileEventID: "event-1",
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := service.RegisterProfessionalImport(ctx, request); err != nil {
			t.Fatalf("register attempt %d: %v", attempt+1, err)
		}
	}
	file, err := files.Get(ctx, "file-1")
	if err != nil {
		t.Fatal(err)
	}
	if file.Storage.LocalPath != storage || file.Origin.Import == nil || file.Origin.Import.ImportPath != source || file.Origin.Import.BatchID != request.BatchID {
		t.Fatalf("storage/origin boundary was not preserved: %+v", file)
	}
	if file.Media.VideoCodec != "" || file.Media.Width != nil || file.Media.Height != nil {
		t.Fatalf("document import unexpectedly invoked the media probe: %+v", file.Media)
	}
	history, err := histories.ListByLibraryID(ctx, request.LibraryID)
	if err != nil || len(history) != 1 || history[0].ID != request.HistoryID || history[0].Refs.ImportBatchID != request.BatchID {
		t.Fatalf("unexpected import history: %+v, err=%v", history, err)
	}
	fileEvents, err := events.ListByLibraryID(ctx, request.LibraryID)
	if err != nil || len(fileEvents) != 1 || fileEvents[0].ID != request.FileEventID || fileEvents[0].FileID != request.FileID {
		t.Fatalf("unexpected file events: %+v, err=%v", fileEvents, err)
	}
}

func TestProfessionalImportRegistersEmbeddedArtworkForCatalogAndLocalMusic(
	t *testing.T,
) {
	ctx := context.Background()
	database := openLibraryServiceTestDatabase(t, "professional-artwork.db")
	storage := filepath.Join(t.TempDir(), "song.mp3")
	if err := os.WriteFile(storage, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	toolDirectory := t.TempDir()
	writeFFprobeTestFixture(t, toolDirectory, `{
		"streams": [
			{"index": 0, "codec_type": "audio", "codec_name": "mp3", "channels": 2},
			{"index": 1, "codec_type": "video", "codec_name": "mjpeg",
			 "disposition": {"attached_pic": 1}}
		],
		"format": {
			"format_name": "mp3",
			"duration": "120",
			"size": "5",
			"tags": {"title": "Tagged Song", "artist": "Tagged Artist"}
		}
	}`)

	libraries := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	files := libraryrepo.NewSQLiteFileRepository(database.Bun)
	localTracks := libraryrepo.NewSQLiteListenLocalTrackRepository(database.Bun)
	service := &LibraryService{
		libraries:                libraries,
		files:                    files,
		localTracks:              localTracks,
		histories:                libraryrepo.NewSQLiteHistoryRepository(database.Bun),
		fileEvents:               libraryrepo.NewSQLiteFileEventRepository(database.Bun),
		tools:                    &mediaProbeToolResolverStub{ready: true, toolDir: toolDirectory},
		embeddedArtworkDirectory: t.TempDir(),
		nowFunc: func() time.Time {
			return time.Date(2026, 7, 27, 8, 0, 0, 0, time.UTC)
		},
	}
	service.embeddedArtworkExtractor = func(
		_ context.Context,
		_ string,
		outputPath string,
	) error {
		return os.WriteFile(outputPath, []byte("jpeg-cover"), 0o600)
	}
	if _, err := service.EnsureProfessionalImportLibrary(
		ctx,
		"audio-library",
		"Audio",
	); err != nil {
		t.Fatal(err)
	}
	request := ProfessionalImportRequest{
		BatchID: "batch-audio", LibraryID: "audio-library",
		SourcePath: storage, StoragePath: storage,
		DisplayName: "Filename Song", Kind: string(library.FileKindAudio),
		FileID: "audio-file", HistoryID: "audio-history",
		FileEventID: "audio-event",
	}
	if _, err := service.RegisterProfessionalImport(ctx, request); err != nil {
		t.Fatal(err)
	}

	items, err := files.ListByLibraryID(ctx, request.LibraryID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("registered files = %#v, want audio plus artwork", items)
	}
	var source, cover library.LibraryFile
	for _, item := range items {
		if item.ID == request.FileID {
			source = item
		}
		if item.Kind == library.FileKindThumbnail {
			cover = item
		}
	}
	if source.Metadata.Title != "Tagged Song" ||
		source.Metadata.Author != "Tagged Artist" {
		t.Fatalf("embedded tags were not retained: %#v", source.Metadata)
	}
	if cover.ID == "" || cover.Lineage.RootFileID != source.ID ||
		!pathExists(cover.Storage.LocalPath) {
		t.Fatalf("embedded artwork was not registered: %#v", cover)
	}

	track, err := localTracks.Get(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if track.CoverLocalPath != cover.Storage.LocalPath ||
		track.Title != "Tagged Song" ||
		track.Author != "Tagged Artist" {
		t.Fatalf("local music projection missed artwork/tags: %#v", track)
	}

	projected, err := projectLegacyCatalogBundle(
		"catalog",
		items,
		time.Date(2026, 7, 27, 8, 1, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(projected) != 1 || len(projected[0].Assets) != 2 {
		t.Fatalf("catalog projection = %#v", projected)
	}
	artworkAssets := 0
	for _, asset := range projected[0].Assets {
		if asset.Role == library.ItemAssetRoleArtwork &&
			asset.FileID == cover.ID {
			artworkAssets++
		}
	}
	if artworkAssets != 1 {
		t.Fatalf("catalog artwork assets=%d, projection=%#v", artworkAssets, projected)
	}
}

func TestEmbeddedArtworkBackfillRepairsExistingProfessionalImport(t *testing.T) {
	ctx := context.Background()
	database := openLibraryServiceTestDatabase(t, "embedded-artwork-backfill.db")
	storage := filepath.Join(t.TempDir(), "existing.mp3")
	if err := os.WriteFile(storage, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	toolDirectory := t.TempDir()
	writeFFprobeTestFixture(t, toolDirectory, `{
		"streams": [
			{"index": 0, "codec_type": "audio", "codec_name": "mp3"},
			{"index": 1, "codec_type": "video", "codec_name": "png",
			 "disposition": {"attached_pic": 1}}
		],
		"format": {"format_name": "mp3", "duration": "60", "size": "5"}
	}`)

	libraries := libraryrepo.NewSQLiteLibraryRepository(database.Bun)
	files := libraryrepo.NewSQLiteFileRepository(database.Bun)
	localTracks := libraryrepo.NewSQLiteListenLocalTrackRepository(database.Bun)
	now := time.Date(2026, 7, 27, 8, 30, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "existing-library", Name: "Existing",
		CreatedBy: library.CreateMeta{Source: "professional_import"},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := libraries.Save(ctx, libraryItem); err != nil {
		t.Fatal(err)
	}
	source, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "existing-audio", LibraryID: libraryItem.ID,
		Kind: string(library.FileKindAudio), Name: "Existing Song",
		DisplayName: "Existing Song",
		Storage: library.FileStorage{
			Mode: "local_path", LocalPath: storage,
		},
		Origin: library.FileOrigin{
			Kind: "import",
			Import: &library.ImportOrigin{
				BatchID: "existing-batch", ImportPath: storage,
				ImportedAt: now, KeepSourceFile: true,
			},
		},
		Media:     &library.MediaInfo{Format: "mp3", AudioCodec: "mp3"},
		State:     library.FileState{Status: "active"},
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Save(ctx, source); err != nil {
		t.Fatal(err)
	}

	extractions := 0
	service := &LibraryService{
		libraries:                libraries,
		files:                    files,
		localTracks:              localTracks,
		tools:                    &mediaProbeToolResolverStub{ready: true, toolDir: toolDirectory},
		embeddedArtworkDirectory: t.TempDir(),
		nowFunc: func() time.Time {
			return now.Add(time.Minute)
		},
	}
	service.embeddedArtworkExtractor = func(
		_ context.Context,
		_ string,
		outputPath string,
	) error {
		extractions++
		return os.WriteFile(outputPath, []byte("backfilled-cover"), 0o600)
	}

	if err := service.BackfillEmbeddedArtwork(ctx); err != nil {
		t.Fatal(err)
	}
	track, err := localTracks.Get(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(track.CoverLocalPath) == "" {
		t.Fatalf("backfill did not update Local Music: %#v", track)
	}
	coverPath := track.CoverLocalPath
	readyMarker := service.embeddedArtworkReadyMarker(source)
	if !pathExists(readyMarker) {
		t.Fatalf("backfill did not commit reconciliation marker %q", readyMarker)
	}

	if err := os.Remove(readyMarker); err != nil {
		t.Fatal(err)
	}
	track.CoverLocalPath = ""
	track.UpdatedAt = now.Add(2 * time.Minute)
	if err := localTracks.Save(ctx, track); err != nil {
		t.Fatal(err)
	}
	if err := service.BackfillEmbeddedArtwork(ctx); err != nil {
		t.Fatal(err)
	}
	repaired, err := localTracks.Get(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if repaired.CoverLocalPath != coverPath {
		t.Fatalf("registered artwork was not reconciled: %#v", repaired)
	}
	if err := service.BackfillEmbeddedArtwork(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := files.ListByLibraryID(ctx, libraryItem.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || extractions != 1 {
		t.Fatalf("files=%#v extractions=%d, want one durable backfill", items, extractions)
	}
}
