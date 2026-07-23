package libraryrepo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestSQLiteListenLocalMusicChangesKeysetCoalescesAcrossPages(t *testing.T) {
	ctx := context.Background()
	database, err := openLibraryRepoTestDatabase(t, ctx, "music-change-keyset.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO listen_local_music_memberships (
  file_id, state, reason, revision, created_at, updated_at
) VALUES
  ('entity-a', 'included', '', 3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('entity-b', 'included', '', 6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('entity-c', 'included', '', 4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('entity-d', 'included', '', 5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO listen_local_music_changes (
  entity_type, entity_id, operation, revision, occurred_at
) VALUES
  ('membership', 'entity-a', 'upsert', 1, CURRENT_TIMESTAMP),
  ('membership', 'entity-b', 'upsert', 2, CURRENT_TIMESTAMP),
  ('membership', 'entity-a', 'upsert', 3, CURRENT_TIMESTAMP),
  ('membership', 'entity-c', 'upsert', 4, CURRENT_TIMESTAMP),
  ('membership', 'entity-d', 'upsert', 5, CURRENT_TIMESTAMP),
  ('membership', 'entity-b', 'upsert', 6, CURRENT_TIMESTAMP);
`); err != nil {
		t.Fatalf("seed coalesced journal: %v", err)
	}

	repo := NewSQLiteListenLocalMusicReadRepository(database.Bun, "")
	position, err := repo.GetSyncPosition(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var gotIDs []string
	var gotSequences []int64
	seen := make(map[string]bool)
	after := int64(0)
	pageCount := 0
	for {
		pageCount++
		page, pageErr := repo.ListChanges(ctx, library.ListenLocalMusicChangeQuery{
			Epoch: position.Epoch, After: after, Limit: 2,
		})
		if pageErr != nil {
			t.Fatal(pageErr)
		}
		for _, change := range page.Changes {
			if seen[change.EntityID] {
				t.Fatalf("entity %q repeated across pages: %#v", change.EntityID, gotIDs)
			}
			seen[change.EntityID] = true
			gotIDs = append(gotIDs, change.EntityID)
			gotSequences = append(gotSequences, change.Sequence)
		}
		if !page.HasMore {
			if page.Cursor != position.HighWater {
				t.Fatalf("terminal cursor=%d want high-water=%d", page.Cursor, position.HighWater)
			}
			break
		}
		if len(page.Changes) != 2 || page.Cursor <= after || page.Cursor != page.Changes[len(page.Changes)-1].Sequence {
			t.Fatalf("invalid keyset page=%#v after=%d", page, after)
		}
		after = page.Cursor
	}
	if pageCount != 2 || !reflect.DeepEqual(gotIDs, []string{"entity-a", "entity-c", "entity-d", "entity-b"}) ||
		!reflect.DeepEqual(gotSequences, []int64{3, 4, 5, 6}) {
		t.Fatalf("coalesced pages=%d ids=%#v sequences=%#v", pageCount, gotIDs, gotSequences)
	}
}

func TestSQLiteListenLocalMusicChangesKeysetBoundsLargeJournalAndUsesIndexes(t *testing.T) {
	ctx := context.Background()
	database, err := openLibraryRepoTestDatabase(t, ctx, "music-change-keyset-large.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	// A Cartesian digit source avoids a test-side 50,000-statement loop while
	// preserving one distinct public entity per synthetic invalidation.
	if _, err := database.SQL.ExecContext(ctx, `
WITH digits(value) AS (
  VALUES (0), (1), (2), (3), (4), (5), (6), (7), (8), (9)
), numbers(value) AS (
  SELECT d0.value + 10*d1.value + 100*d2.value + 1000*d3.value + 10000*d4.value
  FROM digits AS d0
  CROSS JOIN digits AS d1
  CROSS JOIN digits AS d2
  CROSS JOIN digits AS d3
  CROSS JOIN digits AS d4
)
INSERT INTO listen_local_music_changes (
  entity_type, entity_id, operation, revision, occurred_at
)
SELECT 'membership', printf('bulk-%05d', value), 'upsert', 1, CURRENT_TIMESTAMP
FROM numbers
WHERE value < 50000;
`); err != nil {
		t.Fatalf("seed 50k journal: %v", err)
	}

	const pageLimit = 200
	rows, err := listLatestListenLocalMusicChangeRows(ctx, database.Bun, 0, 50_000, pageLimit+1)
	if err != nil {
		t.Fatalf("list bounded keyset page: %v", err)
	}
	if len(rows) != pageLimit+1 || rows[0].Sequence != 1 || rows[len(rows)-1].Sequence != pageLimit+1 {
		t.Fatalf("bounded page count=%d first=%d last=%d", len(rows), rows[0].Sequence, rows[len(rows)-1].Sequence)
	}

	planRows, err := database.SQL.QueryContext(
		ctx,
		"EXPLAIN QUERY PLAN "+latestListenLocalMusicChangeRowsSQL,
		0, 50_000, 50_000, pageLimit+1,
	)
	if err != nil {
		t.Fatalf("explain Music change keyset: %v", err)
	}
	defer planRows.Close()
	var plan strings.Builder
	outerUsesSequenceKeyset := false
	newerUsesEntityIndex := false
	for planRows.Next() {
		var id, parent, unused int
		var detail string
		if err := planRows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatalf("scan Music change query plan: %v", err)
		}
		plan.WriteString(detail)
		plan.WriteByte('\n')
		lowerDetail := strings.ToLower(detail)
		if strings.Contains(lowerDetail, "change_row") && strings.Contains(lowerDetail, "integer primary key") {
			outerUsesSequenceKeyset = true
		}
		if strings.Contains(lowerDetail, "newer") && strings.Contains(lowerDetail, "listen_local_music_changes_entity_idx") {
			newerUsesEntityIndex = true
		}
	}
	if err := planRows.Err(); err != nil {
		t.Fatalf("read Music change query plan: %v", err)
	}
	detail := plan.String()
	if !outerUsesSequenceKeyset || !newerUsesEntityIndex || strings.Contains(strings.ToUpper(detail), "USE TEMP B-TREE") {
		t.Fatalf("Music change query is not an indexed sequence keyset:\n%s", detail)
	}
}

func TestSQLiteListenLocalMusicReadRepositoryUsesSafeCanonicalPathFallbackWithoutCatalogMapping(t *testing.T) {
	ctx := context.Background()
	database, err := openLibraryRepoTestDatabase(t, ctx, "music-legacy-resource.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Date(2026, 7, 22, 18, 30, 0, 0, time.UTC)
	directory := t.TempDir()
	firstPath := filepath.Join(directory, "legacy-one.mp3")
	secondPath := filepath.Join(directory, "legacy-two.mp3")
	coverPath := filepath.Join(directory, "legacy-cover.jpg")
	firstBytes := []byte("legacy-track-one-v1")
	for path, payload := range map[string][]byte{
		firstPath:  firstBytes,
		secondPath: []byte("legacy-track-two"),
		coverPath:  []byte("legacy-cover-bytes"),
	} {
		if err := os.WriteFile(path, payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "library-legacy-music", Name: "Legacy Music", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSQLiteLibraryRepository(database.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatal(err)
	}
	fileRepo := NewSQLiteFileRepository(database.Bun)
	trackRepo := NewSQLiteListenLocalTrackRepository(database.Bun)
	for _, item := range []struct {
		id, path, title, cover string
	}{
		{"legacy-track-1", firstPath, "Legacy One", coverPath},
		{"legacy-track-2", secondPath, "Legacy Two", ""},
	} {
		info, statErr := os.Stat(item.path)
		if statErr != nil {
			t.Fatal(statErr)
		}
		file, buildErr := library.NewLibraryFile(library.LibraryFileParams{
			ID: item.id, LibraryID: libraryItem.ID, Kind: string(library.FileKindAudio), Name: filepath.Base(item.path),
			Storage: library.FileStorage{Mode: "local_path", LocalPath: item.path},
			Origin: library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{
				ImportPath: item.path, ImportedAt: now,
			}},
			State: library.FileState{Status: "active"}, CreatedAt: &now, UpdatedAt: &now,
		})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := fileRepo.Save(ctx, file); err != nil {
			t.Fatal(err)
		}
		size := info.Size()
		track, buildErr := library.NewListenLocalTrack(library.ListenLocalTrackParams{
			FileID: item.id, LibraryID: libraryItem.ID, LocalPath: item.path, CoverLocalPath: item.cover,
			Title: item.title, Format: "mp3", AudioCodec: "mp3", SizeBytes: &size,
			ModTimeUnix: info.ModTime().Unix(), Availability: library.ListenLocalTrackAvailable,
			LastCheckedAt: &now, CreatedAt: &now, UpdatedAt: &now,
		})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := trackRepo.Save(ctx, track); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: "catalog-without-legacy-assets", Name: "Library", Status: string(library.CatalogStatusActive),
		IsDefault: true, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSQLiteCatalogRepository(database.Bun).Save(ctx, catalog); err != nil {
		t.Fatal(err)
	}

	repo := NewSQLiteListenLocalMusicReadRepository(database.Bun, catalog.ID)
	projection, err := repo.GetTrackProjection(ctx, "legacy-track-1")
	if err != nil || projection.CatalogItemID != "" || len(projection.PlaybackResources) != 1 ||
		projection.ArtworkResource == nil {
		t.Fatalf("legacy projection=%#v err=%v", projection, err)
	}
	playback, artwork := projection.PlaybackResources[0], *projection.ArtworkResource
	wantPlaybackDigest := sha256.Sum256(firstBytes)
	wantCoverDigest := sha256.Sum256([]byte("legacy-cover-bytes"))
	firstInfo, _ := os.Stat(firstPath)
	coverInfo, _ := os.Stat(coverPath)
	if playback.Kind != library.ListenLocalMusicResourceOriginal || playback.Availability != "available" ||
		playback.Checksum != "sha256:"+hex.EncodeToString(wantPlaybackDigest[:]) ||
		playback.ByteLength == nil || *playback.ByteLength != firstInfo.Size() ||
		playback.ModTimeUnixNano != firstInfo.ModTime().UnixNano() || playback.LocalPath != firstPath ||
		!strings.HasPrefix(playback.ID, "mr1_") || strings.Contains(playback.ID, "legacy-track") {
		t.Fatalf("legacy playback=%#v", playback)
	}
	if artwork.Kind != library.ListenLocalMusicResourceArtwork || artwork.Availability != "available" ||
		artwork.Checksum != "sha256:"+hex.EncodeToString(wantCoverDigest[:]) ||
		artwork.ByteLength == nil || *artwork.ByteLength != coverInfo.Size() ||
		artwork.ModTimeUnixNano != coverInfo.ModTime().UnixNano() || artwork.LocalPath != coverPath ||
		!strings.HasPrefix(artwork.ID, "mr1_") {
		t.Fatalf("legacy artwork=%#v", artwork)
	}
	if resolved, err := repo.ResolveTrackResource(ctx, "legacy-track-1", playback.ID); err != nil ||
		resolved.LocalPath != firstPath || resolved.Checksum != playback.Checksum {
		t.Fatalf("resolve fallback=%#v err=%v", resolved, err)
	}
	if resolved, err := repo.ResolveTrackResource(ctx, "legacy-track-1", artwork.ID); err != nil ||
		resolved.LocalPath != coverPath || resolved.Checksum != artwork.Checksum {
		t.Fatalf("resolve fallback artwork=%#v err=%v", resolved, err)
	}
	if _, err := repo.ResolveTrackResource(ctx, "legacy-track-2", playback.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-Track fallback resolved: %v", err)
	}

	oldID := playback.ID
	replacement := []byte("legacy-track-one-v2-with-a-different-size")
	if err := os.WriteFile(firstPath, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	rotated, err := repo.GetTrackProjection(ctx, "legacy-track-1")
	if err != nil || len(rotated.PlaybackResources) != 1 || rotated.PlaybackResources[0].ID == oldID {
		t.Fatalf("rotated legacy projection=%#v oldID=%q err=%v", rotated, oldID, err)
	}
	wantRotatedDigest := sha256.Sum256(replacement)
	if rotated.PlaybackResources[0].Checksum != "sha256:"+hex.EncodeToString(wantRotatedDigest[:]) {
		t.Fatalf("rotated checksum=%q", rotated.PlaybackResources[0].Checksum)
	}
	if _, err := repo.ResolveTrackResource(ctx, "legacy-track-1", oldID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("old fallback resource ID resolved after byte rotation: %v", err)
	}
	if _, err := repo.ResolveTrackResource(ctx, "legacy-track-1", rotated.PlaybackResources[0].ID); err != nil {
		t.Fatalf("rotated fallback resource did not resolve: %v", err)
	}
}

func TestSQLiteListenLocalMusicReadRepositoryStableSnapshotChangesAndVersionedResources(t *testing.T) {
	ctx := context.Background()
	database, err := openLibraryRepoTestDatabase(t, ctx, "music-read.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Date(2026, 7, 21, 13, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	originalPath := filepath.Join(directory, "track-1.mp3")
	secondPath := filepath.Join(directory, "track-2.flac")
	compatiblePath := filepath.Join(directory, "track-1-compatible.m4a")
	artworkPath := filepath.Join(directory, "cover.jpg")
	privateFallbackCover := filepath.Join(directory, "unmanaged-private-cover.jpg")
	for path, payload := range map[string][]byte{
		originalPath:         []byte("track-one-audio-bytes"),
		secondPath:           []byte("track-two-audio-bytes"),
		compatiblePath:       []byte("track-one-compatible-aac-bytes"),
		artworkPath:          []byte("managed-artwork-bytes"),
		privateFallbackCover: []byte("must-never-be-selected"),
	} {
		if err := os.WriteFile(path, payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID: "library-music", Name: "Music", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSQLiteLibraryRepository(database.Bun).Save(ctx, libraryItem); err != nil {
		t.Fatal(err)
	}
	fileRepo := NewSQLiteFileRepository(database.Bun)
	for _, item := range []struct {
		id, name, kind, path string
	}{
		{"track-1", "track-1.mp3", string(library.FileKindAudio), originalPath},
		{"track-2", "track-2.flac", string(library.FileKindAudio), secondPath},
		{"artwork-1", "cover.jpg", string(library.FileKindThumbnail), artworkPath},
	} {
		file, buildErr := library.NewLibraryFile(library.LibraryFileParams{
			ID: item.id, LibraryID: libraryItem.ID, Kind: item.kind, Name: item.name,
			Storage: library.FileStorage{Mode: "local_path", LocalPath: item.path},
			Origin: library.FileOrigin{Kind: "import", Import: &library.ImportOrigin{
				ImportPath: item.path, ImportedAt: now,
			}},
			State: library.FileState{Status: "active"}, CreatedAt: &now, UpdatedAt: &now,
		})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := fileRepo.Save(ctx, file); err != nil {
			t.Fatal(err)
		}
	}
	compatibleFile, err := library.NewLibraryFile(library.LibraryFileParams{
		ID: "track-1-compatible", LibraryID: libraryItem.ID, Kind: string(library.FileKindTranscode),
		Name: "track-1-compatible.m4a", DisplayName: "Track One Compatible",
		Storage: library.FileStorage{Mode: "local_path", LocalPath: compatiblePath},
		Origin:  library.FileOrigin{Kind: "transcode", OperationID: "operation-compatible"},
		Lineage: library.FileLineage{RootFileID: "track-1"},
		State:   library.FileState{Status: "active"}, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fileRepo.Save(ctx, compatibleFile); err != nil {
		t.Fatal(err)
	}

	duration := int64(180_000)
	trackRepo := NewSQLiteListenLocalTrackRepository(database.Bun)
	for _, item := range []struct {
		id, path, title, format, codec, cover string
	}{
		{"track-1", originalPath, "Track One", "mp3", "mp3", privateFallbackCover},
		{"track-2", secondPath, "Track Two", "flac", "flac", ""},
		{"track-1-compatible", compatiblePath, "Track One Duplicate", "m4a", "aac", ""},
	} {
		info, statErr := os.Stat(item.path)
		if statErr != nil {
			t.Fatal(statErr)
		}
		size := info.Size()
		track, buildErr := library.NewListenLocalTrack(library.ListenLocalTrackParams{
			FileID: item.id, LibraryID: libraryItem.ID, LocalPath: item.path, CoverLocalPath: item.cover,
			Title: item.title, Author: "Artist", Album: "Album", Format: item.format, AudioCodec: item.codec,
			DurationMs: &duration, SizeBytes: &size, ModTimeUnix: info.ModTime().Unix(),
			Availability: library.ListenLocalTrackAvailable, LastCheckedAt: &now, CreatedAt: &now, UpdatedAt: &now,
		})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := trackRepo.Save(ctx, track); err != nil {
			t.Fatal(err)
		}
	}
	membershipRepo := NewSQLiteListenLocalMusicMembershipRepository(database.Bun)
	compatibleMembership, err := library.NewListenLocalMusicMembership(library.ListenLocalMusicMembershipParams{
		FileID: compatibleFile.ID, State: string(library.ListenLocalMusicMembershipExcluded), Reason: "policy",
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := membershipRepo.Save(ctx, compatibleMembership); err != nil {
		t.Fatal(err)
	}

	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: "catalog-1", Name: "Library", Status: string(library.CatalogStatusActive), IsDefault: true,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSQLiteCatalogRepository(database.Bun).Save(ctx, catalog); err != nil {
		t.Fatal(err)
	}
	itemRepo := NewSQLiteCatalogItemRepository(database.Bun)
	assetRepo := NewSQLiteItemAssetRepository(database.Bun)
	representationRepo := NewSQLiteRepresentationRepository(database.Bun)
	for _, item := range []struct {
		trackID, itemID, title, originalAssetID string
	}{
		{"track-1", "catalog-track-1", "Track One", "asset-track-1"},
		{"track-2", "catalog-track-2", "Track Two", "asset-track-2"},
	} {
		catalogItem, buildErr := library.NewItem(library.ItemParams{
			ID: item.itemID, CatalogID: catalog.ID, Category: string(library.ItemCategoryAudio),
			Status: string(library.ItemStatusActive), Title: item.title, SortTitle: item.title,
			CreatedAt: &now, UpdatedAt: &now,
		})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := itemRepo.Save(ctx, catalogItem); err != nil {
			t.Fatal(err)
		}
		asset, buildErr := library.NewItemAsset(library.ItemAssetParams{
			ID: item.originalAssetID, ItemID: item.itemID, FileID: item.trackID,
			Role: string(library.ItemAssetRoleOriginal), Position: 0, CreatedAt: &now, UpdatedAt: &now,
		})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := assetRepo.Save(ctx, asset); err != nil {
			t.Fatal(err)
		}
		path := originalPath
		mediaType, container, codec := "audio/mpeg", "mp3", "mp3"
		if item.trackID == "track-2" {
			path, mediaType, container, codec = secondPath, "audio/flac", "flac", "flac"
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatal(statErr)
		}
		size := info.Size()
		payload, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		persistedDigest := sha256.Sum256(payload)
		representation, buildErr := library.NewRepresentation(library.RepresentationParams{
			ID: item.originalAssetID, CatalogID: catalog.ID, ItemID: item.itemID,
			AssetID: item.originalAssetID, Kind: string(library.RepresentationKindOriginal),
			Purpose: string(library.RepresentationPurposePrimary), MediaType: mediaType, Container: container, Codec: codec,
			ChecksumAlgorithm: string(library.RepresentationChecksumSHA256), Checksum: hex.EncodeToString(persistedDigest[:]),
			SizeBytes: &size, Availability: string(library.RepresentationAvailabilityAvailable), Revision: 2,
			CreatedAt: &now, UpdatedAt: &now,
		})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if err := representationRepo.SaveRepresentation(ctx, representation); err != nil {
			t.Fatal(err)
		}
	}
	compatibleAsset, err := library.NewItemAsset(library.ItemAssetParams{
		ID: "asset-track-1-compatible", ItemID: "catalog-track-1", FileID: compatibleFile.ID,
		Role: string(library.ItemAssetRoleRepresentation), Position: 0, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := assetRepo.Save(ctx, compatibleAsset); err != nil {
		t.Fatal(err)
	}
	compatibleInfo, err := os.Stat(compatiblePath)
	if err != nil {
		t.Fatal(err)
	}
	compatibleSize := compatibleInfo.Size()
	compatibleBytes, err := os.ReadFile(compatiblePath)
	if err != nil {
		t.Fatal(err)
	}
	compatibleDigest := sha256.Sum256(compatibleBytes)
	compatibleRepresentation, err := library.NewRepresentation(library.RepresentationParams{
		ID: compatibleAsset.ID, CatalogID: catalog.ID, ItemID: "catalog-track-1", AssetID: compatibleAsset.ID,
		Kind: string(library.RepresentationKindOptimized), Purpose: string(library.RepresentationPurposePlayback),
		MediaType: "audio/mp4", Container: "m4a", Codec: "aac",
		ChecksumAlgorithm: string(library.RepresentationChecksumSHA256), Checksum: hex.EncodeToString(compatibleDigest[:]),
		SizeBytes: &compatibleSize, Availability: string(library.RepresentationAvailabilityAvailable), Revision: 4,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := representationRepo.SaveRepresentation(ctx, compatibleRepresentation); err != nil {
		t.Fatal(err)
	}
	artworkAsset, err := library.NewItemAsset(library.ItemAssetParams{
		ID: "asset-artwork-1", ItemID: "catalog-track-1", FileID: "artwork-1",
		Role: string(library.ItemAssetRoleArtwork), Position: 0, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := assetRepo.Save(ctx, artworkAsset); err != nil {
		t.Fatal(err)
	}
	artworkInfo, _ := os.Stat(artworkPath)
	artworkSize := artworkInfo.Size()
	artworkRepresentation, err := library.NewRepresentation(library.RepresentationParams{
		ID: artworkAsset.ID, CatalogID: catalog.ID, ItemID: "catalog-track-1", AssetID: artworkAsset.ID,
		Kind: string(library.RepresentationKindArtwork), Purpose: string(library.RepresentationPurposeArtwork),
		MediaType: "image/jpeg", Container: "jpeg", SizeBytes: &artworkSize,
		Availability: string(library.RepresentationAvailabilityAvailable), Revision: 3,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := representationRepo.SaveRepresentation(ctx, artworkRepresentation); err != nil {
		t.Fatal(err)
	}

	playlistRepo := NewSQLiteListenLocalPlaylistRepository(database.Bun)
	playlist, err := library.NewListenLocalPlaylist(library.ListenLocalPlaylistParams{
		ID: "playlist-1", Name: "Favorites", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := playlistRepo.Save(ctx, playlist); err != nil {
		t.Fatal(err)
	}
	first, _ := library.NewListenLocalPlaylistItem(playlist.ID, "track-1", 0, now)
	second, _ := library.NewListenLocalPlaylistItem(playlist.ID, "track-1", 1, now.Add(time.Second))
	playlist.UpdatedAt = now.Add(time.Minute)
	if err := playlistRepo.ReplaceItems(ctx, playlist, []library.ListenLocalPlaylistItem{first, second}); err != nil {
		t.Fatal(err)
	}

	readRepo := NewSQLiteListenLocalMusicReadRepository(database.Bun, catalog.ID)
	defaultHasher := readRepo.resourceSHA256Func
	resourceHashCalls := 0
	readRepo.resourceSHA256Func = func(ctx context.Context, path string) (string, os.FileInfo, error) {
		resourceHashCalls++
		return defaultHasher(ctx, path)
	}
	position, err := readRepo.GetSyncPosition(ctx)
	if err != nil || position.Epoch == "" || position.HighWater == 0 || position.MinimumCursor != 0 {
		t.Fatalf("position=%#v err=%v", position, err)
	}

	var entityOrder []string
	afterType, afterID := "", ""
	for {
		page, pageErr := readRepo.ListSnapshot(ctx, library.ListenLocalMusicSnapshotQuery{
			Epoch: position.Epoch, HighWater: position.HighWater, AfterType: afterType, AfterEntity: afterID, Limit: 1,
		})
		if pageErr != nil {
			t.Fatal(pageErr)
		}
		for _, entity := range page.Entities {
			entityOrder = append(entityOrder, entity.EntityType+":"+entity.EntityID)
		}
		if !page.HasMore {
			break
		}
		afterType, afterID = page.NextType, page.NextEntity
	}
	wantOrder := []string{
		"track:track-1", "track:track-2", "playlist:playlist-1",
		"playlist_item:" + first.ID, "playlist_item:" + second.ID,
		"membership:track-1-compatible",
	}
	// Playlist Item UUID order can differ from playlist position; the protocol
	// intentionally keys the stable snapshot by identity, not mutable order.
	if len(entityOrder) != len(wantOrder) || !reflect.DeepEqual(entityOrder[:3], wantOrder[:3]) {
		t.Fatalf("snapshot order=%#v", entityOrder)
	}
	wantItemIDs := []string{first.ID, second.ID}
	if entityOrder[3] > entityOrder[4] {
		entityOrder[3], entityOrder[4] = entityOrder[4], entityOrder[3]
	}
	if wantItemIDs[0] > wantItemIDs[1] {
		wantItemIDs[0], wantItemIDs[1] = wantItemIDs[1], wantItemIDs[0]
	}
	if entityOrder[3] != "playlist_item:"+wantItemIDs[0] || entityOrder[4] != "playlist_item:"+wantItemIDs[1] {
		t.Fatalf("snapshot playlist items=%#v want=%#v", entityOrder[3:], wantItemIDs)
	}
	if entityOrder[5] != "membership:track-1-compatible" {
		t.Fatalf("compatible output leaked as Track in snapshot order=%#v", entityOrder)
	}

	projection, err := readRepo.GetTrackProjection(ctx, "track-1")
	if err != nil || projection.CatalogItemID != "catalog-track-1" || len(projection.PlaybackResources) != 2 ||
		projection.ArtworkResource == nil {
		t.Fatalf("Track projection=%#v err=%v", projection, err)
	}
	var playback, compatiblePlayback library.ListenLocalMusicResource
	for _, resource := range projection.PlaybackResources {
		switch resource.Kind {
		case library.ListenLocalMusicResourceOriginal:
			playback = resource
		case library.ListenLocalMusicResourcePlaybackRepresentation:
			compatiblePlayback = resource
		}
	}
	originalDigest := sha256.Sum256([]byte("track-one-audio-bytes"))
	wantChecksum := "sha256:" + hex.EncodeToString(originalDigest[:])
	if !strings.HasPrefix(playback.ID, "mr1_") || playback.Checksum != wantChecksum ||
		playback.LocalPath != originalPath || projection.ArtworkResource.LocalPath != artworkPath ||
		projection.ArtworkResource.LocalPath == privateFallbackCover {
		t.Fatalf("resource projection playback=%#v artwork=%#v", playback, projection.ArtworkResource)
	}
	if compatiblePlayback.Kind != library.ListenLocalMusicResourcePlaybackRepresentation ||
		compatiblePlayback.FileID != compatibleFile.ID || compatiblePlayback.LocalPath != compatiblePath ||
		compatiblePlayback.MediaType != "audio/mp4" || compatiblePlayback.Container != "m4a" ||
		compatiblePlayback.Codec != "aac" || compatiblePlayback.Availability != "available" {
		t.Fatalf("compatible output was not attached to original Track: %#v", compatiblePlayback)
	}
	if resourceHashCalls != 1 {
		t.Fatalf("resource fallback hash calls=%d, want one cached artwork hash and no persisted playback hashes", resourceHashCalls)
	}
	if _, err := readRepo.GetTrackProjection(ctx, "track-1"); err != nil || resourceHashCalls != 1 {
		t.Fatalf("repeated projection did not reuse bounded fallback cache: calls=%d err=%v", resourceHashCalls, err)
	}
	if _, err := database.SQL.ExecContext(ctx,
		`UPDATE library_representations SET availability = 'missing' WHERE id = 'asset-track-2'`,
	); err != nil {
		t.Fatal(err)
	}
	managedMissing, err := readRepo.GetTrackProjection(ctx, "track-2")
	if err != nil || managedMissing.CatalogItemID != "catalog-track-2" ||
		len(managedMissing.PlaybackResources) != 1 ||
		managedMissing.PlaybackResources[0].Availability != "missing" {
		t.Fatalf("managed missing resource was bypassed by fallback: projection=%#v err=%v", managedMissing, err)
	}
	if _, err := database.SQL.ExecContext(ctx,
		`UPDATE library_representations SET availability = 'missing' WHERE id = 'asset-artwork-1'`,
	); err != nil {
		t.Fatal(err)
	}
	managedArtworkMissing, err := readRepo.GetTrackProjection(ctx, "track-1")
	if err != nil || managedArtworkMissing.ArtworkResource == nil ||
		managedArtworkMissing.ArtworkResource.Availability != "missing" ||
		managedArtworkMissing.ArtworkResource.LocalPath != artworkPath ||
		managedArtworkMissing.ArtworkResource.LocalPath == privateFallbackCover {
		t.Fatalf("managed missing artwork was bypassed by fallback: projection=%#v err=%v", managedArtworkMissing, err)
	}
	if _, err := readRepo.ResolveTrackResource(ctx, "track-1", playback.ID); err != nil {
		t.Fatalf("resolve current resource: %v", err)
	}
	if _, err := readRepo.ResolveTrackResource(ctx, "track-2", playback.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-Track resource error=%v", err)
	}
	originalInfo, err := os.Stat(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	replacement := []byte(strings.Repeat("z", int(*playback.ByteLength)))
	if err := os.WriteFile(originalPath, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(originalPath, originalInfo.ModTime(), originalInfo.ModTime()); err != nil {
		t.Fatal(err)
	}
	if _, err := readRepo.ResolveTrackResource(ctx, "track-1", playback.ID); err != nil {
		t.Fatalf("persisted descriptor should remain resolvable until checksum refresh: %v", err)
	}
	updatedProjection, err := readRepo.GetTrackProjection(ctx, "track-1")
	var updatedOriginal library.ListenLocalMusicResource
	for _, resource := range updatedProjection.PlaybackResources {
		if resource.Kind == library.ListenLocalMusicResourceOriginal {
			updatedOriginal = resource
		}
	}
	if err != nil || updatedOriginal.ID != playback.ID {
		t.Fatalf("persisted descriptor changed before checksum refresh: old=%q projection=%#v err=%v", playback.ID, updatedProjection, err)
	}
	replacementDigest := sha256.Sum256(replacement)
	if _, err := database.SQL.ExecContext(ctx,
		`UPDATE library_representations SET checksum = ? WHERE id = ?`,
		hex.EncodeToString(replacementDigest[:]), "asset-track-1",
	); err != nil {
		t.Fatal(err)
	}
	updatedProjection, err = readRepo.GetTrackProjection(ctx, "track-1")
	updatedOriginal = library.ListenLocalMusicResource{}
	for _, resource := range updatedProjection.PlaybackResources {
		if resource.Kind == library.ListenLocalMusicResourceOriginal {
			updatedOriginal = resource
		}
	}
	if err != nil || updatedOriginal.ID == playback.ID {
		t.Fatalf("resource ID did not rotate after persisted checksum refresh: old=%q projection=%#v err=%v", playback.ID, updatedProjection, err)
	}
	if _, err := readRepo.ResolveTrackResource(ctx, "track-1", playback.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("old resource resolved after checksum refresh: %v", err)
	}

	membership, _ := library.NewListenLocalMusicMembership(library.ListenLocalMusicMembershipParams{
		FileID: "track-1", State: string(library.ListenLocalMusicMembershipIncluded), CreatedAt: &now, UpdatedAt: &now,
	})
	if err := membershipRepo.Save(ctx, membership); err != nil {
		t.Fatal(err)
	}
	afterMembership, err := readRepo.GetSyncPosition(ctx)
	if err != nil || afterMembership.HighWater <= position.HighWater {
		t.Fatalf("membership position=%#v err=%v", afterMembership, err)
	}
	membershipChanges, err := readRepo.ListChanges(ctx, library.ListenLocalMusicChangeQuery{
		Epoch: position.Epoch, After: position.HighWater, Limit: 100,
	})
	if err != nil || len(membershipChanges.Changes) != 1 ||
		membershipChanges.Cursor != afterMembership.HighWater ||
		membershipChanges.Changes[0].EntityType != library.ListenLocalMusicEntityMembership ||
		membershipChanges.Changes[0].Entity == nil || membershipChanges.Changes[0].Entity.Membership == nil ||
		membershipChanges.Changes[0].Entity.Membership.FileID != "track-1" {
		t.Fatalf("membership change page=%#v err=%v", membershipChanges, err)
	}
	freshSnapshot, err := readRepo.ListSnapshot(ctx, library.ListenLocalMusicSnapshotQuery{
		Epoch: position.Epoch, HighWater: afterMembership.HighWater, Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	var snapshotMembership *library.ListenLocalMusicMembership
	var freshPlaylistItemIDs []string
	for index := range freshSnapshot.Entities {
		if freshSnapshot.Entities[index].EntityType == library.ListenLocalMusicEntityMembership &&
			freshSnapshot.Entities[index].EntityID == "track-1" {
			snapshotMembership = freshSnapshot.Entities[index].Membership
		}
		if freshSnapshot.Entities[index].EntityType == library.ListenLocalMusicEntityPlaylistItem &&
			freshSnapshot.Entities[index].PlaylistItem != nil {
			freshPlaylistItemIDs = append(freshPlaylistItemIDs, freshSnapshot.Entities[index].PlaylistItem.ID)
		}
	}
	if snapshotMembership == nil || snapshotMembership.FileID != "track-1" ||
		snapshotMembership.State != library.ListenLocalMusicMembershipIncluded {
		t.Fatalf("snapshot membership=%#v entities=%#v", snapshotMembership, freshSnapshot.Entities)
	}
	sort.Strings(freshPlaylistItemIDs)
	sort.Strings(wantItemIDs)
	if !reflect.DeepEqual(freshPlaylistItemIDs, wantItemIDs) {
		t.Fatalf("later snapshot changed PlaylistItem identities: first=%#v later=%#v", wantItemIDs, freshPlaylistItemIDs)
	}
	if _, err := readRepo.ListSnapshot(ctx, library.ListenLocalMusicSnapshotQuery{
		Epoch: position.Epoch, HighWater: position.HighWater, Limit: 10,
	}); !errors.Is(err, library.ErrListenLocalMusicSyncResetRequired) {
		t.Fatalf("drifted snapshot error=%v", err)
	}

	track, err := trackRepo.Get(ctx, "track-1")
	if err != nil {
		t.Fatal(err)
	}
	track.Title, track.UpdatedAt = "Edit One", now.Add(2*time.Minute)
	if err := trackRepo.Save(ctx, track); err != nil {
		t.Fatal(err)
	}
	track, _ = trackRepo.Get(ctx, "track-1")
	track.Title, track.UpdatedAt = "Edit Two", now.Add(3*time.Minute)
	if err := trackRepo.Save(ctx, track); err != nil {
		t.Fatal(err)
	}
	coalesced, err := readRepo.ListChanges(ctx, library.ListenLocalMusicChangeQuery{
		Epoch: position.Epoch, After: afterMembership.HighWater, Limit: 100,
	})
	if err != nil || len(coalesced.Changes) != 1 || coalesced.Changes[0].Entity == nil ||
		coalesced.Changes[0].Entity.Track == nil || coalesced.Changes[0].Entity.Track.Track.Title != "Edit Two" {
		t.Fatalf("coalesced page=%#v err=%v", coalesced, err)
	}

	beforeDelete := coalesced.Position.HighWater
	if err := trackRepo.Delete(ctx, "track-1"); err != nil {
		t.Fatal(err)
	}
	deleted, err := readRepo.ListChanges(ctx, library.ListenLocalMusicChangeQuery{
		Epoch: position.Epoch, After: beforeDelete, Limit: 100,
	})
	if err != nil || len(deleted.Changes) == 0 {
		t.Fatalf("delete page=%#v err=%v", deleted, err)
	}
	var trackDelete *library.ListenLocalMusicChange
	for index := range deleted.Changes {
		if deleted.Changes[index].EntityType == library.ListenLocalMusicEntityTrack {
			trackDelete = &deleted.Changes[index]
		}
	}
	if trackDelete == nil || trackDelete.Operation != "delete" || trackDelete.Entity != nil {
		t.Fatalf("Track delete=%#v page=%#v", trackDelete, deleted)
	}

	current, err := readRepo.GetSyncPosition(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx,
		`UPDATE listen_local_music_sync_state SET minimum_cursor = ? WHERE id = 1`, current.HighWater); err != nil {
		t.Fatal(err)
	}
	if _, err := readRepo.ListChanges(ctx, library.ListenLocalMusicChangeQuery{
		Epoch: current.Epoch, After: 0, Limit: 100,
	}); !errors.Is(err, library.ErrListenLocalMusicSyncResetRequired) {
		t.Fatalf("retention reset error=%v", err)
	}
}

func TestListenLocalMusicFallbackResourceHashCacheUsesStrongIdentityAndSingleflight(t *testing.T) {
	t.Run("same-size rewrite with restored mtime rotates identity", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cover.bin")
		if err := os.WriteFile(path, []byte("alpha"), 0o600); err != nil {
			t.Fatal(err)
		}
		before, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		repo := NewSQLiteListenLocalMusicReadRepository(nil, "")
		var calls atomic.Int32
		repo.resourceSHA256Func = func(ctx context.Context, path string) (string, os.FileInfo, error) {
			calls.Add(1)
			return listenLocalMusicResourceSHA256(ctx, path)
		}
		first, err := repo.cachedListenLocalMusicResourceSHA256(context.Background(), path, before)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.cachedListenLocalMusicResourceSHA256(context.Background(), path, before); err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 1 {
			t.Fatalf("unchanged fallback hash calls=%d, want one", calls.Load())
		}

		if err := os.WriteFile(path, []byte("bravo"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
			t.Fatal(err)
		}
		after, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if listenLocalMusicResourceStatIdentity(before) == listenLocalMusicResourceStatIdentity(after) {
			t.Skip("platform file metadata cannot distinguish a same-size rewrite with restored mtime")
		}
		second, err := repo.cachedListenLocalMusicResourceSHA256(context.Background(), path, after)
		if err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 2 || first == second {
			t.Fatalf("rewrite hash calls=%d first=%q second=%q", calls.Load(), first, second)
		}
	})

	t.Run("concurrent snapshot hydration hashes once", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cover.bin")
		if err := os.WriteFile(path, []byte("shared-fallback"), 0o600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		repo := NewSQLiteListenLocalMusicReadRepository(nil, "")
		entered := make(chan struct{})
		release := make(chan struct{})
		var calls atomic.Int32
		repo.resourceSHA256Func = func(ctx context.Context, path string) (string, os.FileInfo, error) {
			if calls.Add(1) == 1 {
				close(entered)
			}
			<-release
			return listenLocalMusicResourceSHA256(ctx, path)
		}
		const workers = 10
		start := make(chan struct{})
		errorsByWorker := make(chan error, workers)
		var wait sync.WaitGroup
		for range workers {
			wait.Add(1)
			go func() {
				defer wait.Done()
				<-start
				_, callErr := repo.cachedListenLocalMusicResourceSHA256(context.Background(), path, info)
				errorsByWorker <- callErr
			}()
		}
		close(start)
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("fallback hash leader did not start")
		}
		close(release)
		wait.Wait()
		close(errorsByWorker)
		for callErr := range errorsByWorker {
			if callErr != nil {
				t.Fatal(callErr)
			}
		}
		if calls.Load() != 1 {
			t.Fatalf("concurrent fallback hash calls=%d, want one", calls.Load())
		}
	})
}
