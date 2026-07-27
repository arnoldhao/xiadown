package libraryrootsync

import (
	"strings"
	"testing"
	"time"
)

func TestStateRejectsImpossibleProgress(t *testing.T) {
	_, err := NewState(State{
		RootID: "root", Status: StatusScanning,
		DiscoveredCount: 1, ProcessedCount: 2,
		CreatedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected impossible progress to fail")
	}
}

func TestEntryFingerprintMatchesIndexedFile(t *testing.T) {
	entry, err := NewEntry(Entry{
		RootID: "root", RelativePath: `folder\movie.mp4`,
		SizeBytes: 12, ModifiedUnixNano: 34, FileID: "file",
		Status: EntryActive, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("new entry: %v", err)
	}
	if entry.RelativePath != "folder/movie.mp4" ||
		!entry.FingerprintMatches(12, 34) ||
		entry.FingerprintMatches(13, 34) {
		t.Fatalf("unexpected fingerprint behavior: %#v", entry)
	}
	duplicate, err := NewEntry(Entry{
		RootID: "root", RelativePath: "copy.mp4",
		SizeBytes: 12, ModifiedUnixNano: 34,
		ContentHash: strings.Repeat("a", 64),
		Status:      EntryDuplicate, CreatedAt: time.Now(),
	})
	if err != nil || !duplicate.FingerprintMatches(12, 34) {
		t.Fatalf("duplicate fingerprint mismatch: %#v err=%v", duplicate, err)
	}
}

func TestEntryRejectsRelativePathsThatEscapeTheStorageRoot(t *testing.T) {
	for _, relativePath := range []string{
		"../outside.mp4",
		"folder/../../outside.mp4",
		"/absolute/outside.mp4",
		`..\outside.mp4`,
	} {
		t.Run(relativePath, func(t *testing.T) {
			_, err := NewEntry(Entry{
				RootID: "root", RelativePath: relativePath,
				SizeBytes: 1, ModifiedUnixNano: 1, FileID: "file",
				Status: EntryActive, CreatedAt: time.Now(),
			})
			if err == nil {
				t.Fatalf("relative path %q unexpectedly accepted", relativePath)
			}
		})
	}
}

func TestEntryCleansSafeRelativePathSegments(t *testing.T) {
	entry, err := NewEntry(Entry{
		RootID: "root", RelativePath: `folder\.\nested\clip.mp4`,
		SizeBytes: 1, ModifiedUnixNano: 1, FileID: "file",
		Status: EntryActive, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("new entry: %v", err)
	}
	if entry.RelativePath != "folder/nested/clip.mp4" {
		t.Fatalf("relative path = %q", entry.RelativePath)
	}
}
