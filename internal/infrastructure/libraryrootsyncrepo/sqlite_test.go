package libraryrootsyncrepo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	domain "xiadown/internal/domain/libraryrootsync"
	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteRepositoryPersistsProgressAndMissingEntries(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "root-sync.sqlite"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()
	now := time.Now().UTC()
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_catalogs (
  id, name, status, is_default, created_at, updated_at
) VALUES ('catalog', 'Library', 'active', TRUE, ?, ?)
`, now, now); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_storage_roots (
  id, catalog_id, name, path, volume_id, mode, is_default, status,
  last_error, created_at, updated_at
) VALUES (
  'root', 'catalog', 'Reference', '/reference', 'volume',
  'referenced', FALSE, 'online', '', ?, ?
)
`, now, now); err != nil {
		t.Fatalf("seed root: %v", err)
	}

	repository := NewSQLiteRepository(database.Bun)
	started := now.Add(time.Second)
	state, err := domain.NewState(domain.State{
		RootID: "root", Status: domain.StatusScanning, Generation: 2,
		FullScan: true, DiscoveredCount: 2, ProcessedCount: 1,
		StartedAt: &started, CreatedAt: now, UpdatedAt: started,
	})
	if err != nil {
		t.Fatalf("new state: %v", err)
	}
	if err := repository.SaveState(ctx, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := repository.AdvanceWatcherCursor(ctx, "root", 42); err != nil {
		t.Fatalf("advance watcher cursor: %v", err)
	}
	state.WatcherCursor = 7
	if err := repository.SaveState(ctx, state); err != nil {
		t.Fatalf("save stale watcher cursor: %v", err)
	}
	got, err := repository.GetState(ctx, "root")
	if err != nil || got.Generation != 2 || got.ProcessedCount != 1 ||
		got.WatcherCursor != 42 {
		t.Fatalf("get state: %#v err=%v", got, err)
	}

	for _, entry := range []domain.Entry{
		{
			RootID: "root", RelativePath: "old.mp4", FileID: "file-old",
			Status: domain.EntryActive, LastSeenGeneration: 1,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			RootID: "root", RelativePath: "current.mp4", FileID: "file-current",
			Status: domain.EntryActive, LastSeenGeneration: 2,
			CreatedAt: now, UpdatedAt: now,
		},
	} {
		if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_libraries (
  id, name, created_by_json, created_at, updated_at
) VALUES (?, ?, '{"source":"root_sync"}', ?, ?)
`, entry.FileID+"-library", entry.FileID, now, now); err != nil {
			t.Fatalf("seed library: %v", err)
		}
		if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO library_files (
  id, library_id, kind, name, display_name, metadata_json,
  storage_mode, storage_local_path, origin_kind, origin_import_batch_id,
  origin_import_path, origin_imported_at, origin_keep_source_file,
  state_json, created_at, updated_at
) VALUES (
  ?, ?, 'video', ?, ?, '{}', 'local_path', ?, 'import', 'batch',
  ?, ?, TRUE, '{"status":"active"}', ?, ?
)
`, entry.FileID, entry.FileID+"-library", entry.RelativePath, entry.RelativePath,
			"/reference/"+entry.RelativePath, "/reference/"+entry.RelativePath,
			now, now, now); err != nil {
			t.Fatalf("seed file: %v", err)
		}
		if err := repository.UpsertEntry(ctx, entry); err != nil {
			t.Fatalf("upsert entry: %v", err)
		}
	}
	sameSize, err := repository.ListActiveEntriesBySize(ctx, "root", 0)
	if err != nil || len(sameSize) != 2 {
		t.Fatalf("list active entries by size: entries=%+v err=%v", sameSize, err)
	}
	missing, err := repository.MarkUnseenEntriesMissing(ctx, "root", 2)
	if err != nil || missing != 1 {
		t.Fatalf("mark missing: count=%d err=%v", missing, err)
	}
	old, err := repository.GetEntry(ctx, "root", "old.mp4")
	if err != nil || old.Status != domain.EntryMissing {
		t.Fatalf("old entry: %#v err=%v", old, err)
	}
}
