package libraryrepo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteExternalProcessRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "library-processes.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID:        "lib-1",
		Name:      "Library",
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new library: %v", err)
	}
	if err := NewSQLiteLibraryRepository(db.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatalf("save library: %v", err)
	}
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID:          "op-1",
		LibraryID:   libraryItem.ID,
		Kind:        "transcode",
		Status:      string(library.OperationStatusRunning),
		DisplayName: "Transcode",
		InputJSON:   "{}",
		OutputJSON:  "{}",
		CreatedAt:   &now,
	})
	if err != nil {
		t.Fatalf("new operation: %v", err)
	}
	if err := NewSQLiteOperationRepository(db.Bun).Save(ctx, operation); err != nil {
		t.Fatalf("save operation: %v", err)
	}

	repo := NewSQLiteExternalProcessRepository(db.Bun)
	record := library.ExternalProcess{
		ID:             "process-1",
		OperationID:    operation.ID,
		Kind:           operation.Kind,
		Tool:           "ffmpeg",
		PID:            1234,
		ProcessGroupID: 1234,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repo.Save(ctx, record); err != nil {
		t.Fatalf("save external process: %v", err)
	}

	items, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list external processes: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one external process, got %#v", items)
	}
	if items[0].OperationID != operation.ID || items[0].Tool != "ffmpeg" || items[0].PID != 1234 || items[0].ProcessGroupID != 1234 {
		t.Fatalf("unexpected external process: %#v", items[0])
	}

	if err := repo.Delete(ctx, record.ID); err != nil {
		t.Fatalf("delete external process: %v", err)
	}
	items, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected external process to be deleted, got %#v", items)
	}
}
