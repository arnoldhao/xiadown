package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"xiadown/internal/infrastructure/libraryrepo"
)

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
	service := &LibraryService{
		libraries: libraries, files: files, histories: histories, fileEvents: events,
		nowFunc: nil,
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
	history, err := histories.ListByLibraryID(ctx, request.LibraryID)
	if err != nil || len(history) != 1 || history[0].ID != request.HistoryID || history[0].Refs.ImportBatchID != request.BatchID {
		t.Fatalf("unexpected import history: %+v, err=%v", history, err)
	}
	fileEvents, err := events.ListByLibraryID(ctx, request.LibraryID)
	if err != nil || len(fileEvents) != 1 || fileEvents[0].ID != request.FileEventID || fileEvents[0].FileID != request.FileID {
		t.Fatalf("unexpected file events: %+v, err=%v", fileEvents, err)
	}
}
