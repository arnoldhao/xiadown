package libraryrootsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWalkScanTargetsExcludesTransientLibraryArtifacts(t *testing.T) {
	root := t.TempDir()
	durable := filepath.Join(root, "track.mp3")
	transient := []string{
		"track.999d1c0d-f524-4dee-b2a2-703ef73ca8c9.tmp.mp3",
		"video.999d1c0d-f524-4dee-b2a2-703ef73ca8c9.tmp",
		"download.webm.part",
		"download.webm.ytdl",
		"download.webm.aria2",
	}
	if err := os.WriteFile(durable, []byte("durable"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range transient {
		if err := os.WriteFile(filepath.Join(root, name), []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var discovered []string
	err := walkScanTargets(
		context.Background(),
		root,
		nil,
		func(item discoveredFile) error {
			discovered = append(discovered, item.relative)
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(discovered) != 1 || discovered[0] != filepath.Base(durable) {
		t.Fatalf("discovered = %#v, want only durable file", discovered)
	}
}

func TestWalkScanTargetsIgnoresMissingTransientEvent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(
		root,
		"track.999d1c0d-f524-4dee-b2a2-703ef73ca8c9.tmp.mp3",
	)
	called := false
	err := walkScanTargets(
		context.Background(),
		root,
		[]string{path},
		func(discoveredFile) error {
			called = true
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("missing transient watcher target was treated as a Library asset")
	}
}

func TestTransientLibraryArtifactDoesNotHideOrdinaryTemporaryName(t *testing.T) {
	for _, path := range []string{
		"mix.tmp.mp3",
		"notes.tmp",
		"track.not-a-uuid.tmp.flac",
		"multipart.mp3",
	} {
		if isTransientLibraryArtifact(path) {
			t.Fatalf("ordinary path %q was classified as an internal artifact", path)
		}
	}
}
