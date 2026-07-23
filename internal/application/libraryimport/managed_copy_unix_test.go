//go:build darwin || linux

package libraryimport

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	importdomain "xiadown/internal/domain/libraryimport"
)

func TestManagedCopyRejectsSymlinksAtEveryManagedPathBoundary(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "movie.mp4")
	writeTestFile(t, source, "managed source bytes")
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := hashFileSHA256(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	candidate := importdomain.Candidate{
		ID: "symlink-boundary", SourcePath: source, DisplayName: "movie.mp4",
		Category: importdomain.CategoryVideo, SizeBytes: info.Size(), ContentHash: digest,
	}

	tests := []struct {
		name    string
		prepare func(t *testing.T, root, outside string)
	}{
		{
			name: "category ancestor",
			prepare: func(t *testing.T, root, outside string) {
				t.Helper()
				if err := os.Symlink(outside, filepath.Join(root, "video")); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			},
		},
		{
			name: "hash ancestor",
			prepare: func(t *testing.T, root, outside string) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(root, "video"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, "video", digest[:2])); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			},
		},
		{
			name: "stage leaf",
			prepare: func(t *testing.T, root, outside string) {
				t.Helper()
				directory := filepath.Join(root, "video", digest[:2], digest[2:4])
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(
					filepath.Join(outside, "sentinel"),
					filepath.Join(directory, ".xiadown-import-symlink-boundary.stage"),
				); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			},
		},
		{
			name: "destination leaf",
			prepare: func(t *testing.T, root, outside string) {
				t.Helper()
				directory := filepath.Join(root, "video", digest[:2], digest[2:4])
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(
					filepath.Join(outside, "sentinel"),
					filepath.Join(directory, digest+"-movie.mp4"),
				); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := canonicalManagedTestRoot(t)
			outside := canonicalManagedTestRoot(t)
			writeTestFile(t, filepath.Join(outside, "sentinel"), "must remain unchanged")
			test.prepare(t, root, outside)

			if _, err := copyIntoManagedRoot(ctx, root, candidate); err == nil {
				t.Fatal("managed copy unexpectedly followed a symlink")
			}
			body, err := os.ReadFile(filepath.Join(outside, "sentinel"))
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "must remain unchanged" {
				t.Fatalf("outside target was modified: %q", body)
			}
			entries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != "sentinel" {
				t.Fatalf("managed copy created content outside root: %v", entries)
			}
		})
	}
}
