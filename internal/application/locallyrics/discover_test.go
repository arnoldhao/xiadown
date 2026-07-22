package locallyrics

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscoverSupportsBothNamingSchemesAndRanksCapability(t *testing.T) {
	directory := t.TempDir()
	mediaPath := filepath.Join(directory, "track.mp3")
	mustWriteTestFile(t, mediaPath, "audio")

	// TTML has an exact line timeline, while the full-media-name LRC has exact
	// word timing. Capability, not extension order, must win.
	mustWriteTestFile(t, filepath.Join(directory, "track.ttml"), `<tt><body><div><p begin="1s" end="3s">Line timed only</p></div></body></tt>`)
	mustWriteTestFile(t, filepath.Join(directory, "track.mp3.lrc"), `[00:01.00]<00:01.00>Word <00:02.00>timed<00:03.00>`)
	mustWriteTestFile(t, filepath.Join(directory, "track.mp3.t.vtt"), "WEBVTT\n\n00:00:01.100 --> 00:00:03.000\n逐字翻译")

	candidates, err := DiscoverSidecars(context.Background(), mediaPath, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected both basename schemes, got %#v", candidates)
	}
	if filepath.Base(candidates[0].Path) != "track.mp3.lrc" || candidates[0].Result.TimingQuality != TimingQualityWord {
		t.Fatalf("expected word-timed full-name candidate first, got %#v", candidates[0])
	}
	if candidates[0].Result.Lines[0].Translation != "逐字翻译" || filepath.Base(candidates[0].TranslationPath) != "track.mp3.t.vtt" {
		t.Fatalf("expected paired translation sidecar: %#v", candidates[0])
	}
	if candidates[0].Result.SourcePath == "" || candidates[0].Result.Attribution.Kind != SourceSidecar {
		t.Fatalf("expected source path and attribution: %#v", candidates[0].Result)
	}
}

func TestLoadBestSidecarReturnsNoLyricsWithoutCandidate(t *testing.T) {
	directory := t.TempDir()
	mediaPath := filepath.Join(directory, "track.flac")
	mustWriteTestFile(t, mediaPath, "audio")
	if _, err := LoadBestSidecar(context.Background(), mediaPath, Options{}); !errors.Is(err, ErrNoLyrics) {
		t.Fatalf("expected ErrNoLyrics, got %v", err)
	}
}

func TestSidecarDiscoveryRejectsEscapingSymlinkAndOversizedFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires privileges on some Windows hosts")
	}
	directory := t.TempDir()
	outsideDirectory := t.TempDir()
	mediaPath := filepath.Join(directory, "track.mp3")
	mustWriteTestFile(t, mediaPath, "audio")
	outsideLyric := filepath.Join(outsideDirectory, "outside.lrc")
	mustWriteTestFile(t, outsideLyric, "[00:01.00]outside")
	if err := os.Symlink(outsideLyric, filepath.Join(directory, "track.lrc")); err != nil {
		t.Fatal(err)
	}

	if _, err := DiscoverSidecars(context.Background(), mediaPath, Options{}); !errors.Is(err, ErrUnsafeFile) {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
	if _, _, err := secureReadFile(directory, outsideLyric, Options{}); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("expected root escape rejection, got %v", err)
	}

	os.Remove(filepath.Join(directory, "track.lrc"))
	mustWriteTestFile(t, filepath.Join(directory, "track.lrc"), strings.Repeat("x", 65))
	if _, err := DiscoverSidecars(context.Background(), mediaPath, Options{MaxBytes: 64}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected oversized sidecar rejection, got %v", err)
	}
}

func TestCandidateNamesUsePortableWindowsPathSemantics(t *testing.T) {
	specs := candidateFileNames(`C:\Music\Album\Track.FLAC`)
	mainNames := make(map[string]bool)
	translationNames := make(map[string]bool)
	for _, spec := range specs {
		if strings.ContainsAny(spec.mainName, `/\`) {
			t.Fatalf("candidate leaked a directory component: %q", spec.mainName)
		}
		mainNames[spec.mainName] = true
		for _, name := range spec.translationNames {
			if strings.ContainsAny(name, `/\`) {
				t.Fatalf("translation candidate leaked a directory component: %q", name)
			}
			translationNames[name] = true
		}
	}
	for _, expected := range []string{"Track.lrc", "Track.vtt", "Track.ttml", "Track.FLAC.lrc", "Track.FLAC.vtt", "Track.FLAC.ttml"} {
		if !mainNames[expected] {
			t.Fatalf("missing Windows candidate %q in %#v", expected, mainNames)
		}
	}
	for _, expected := range []string{"Track.t.lrc", "Track.t.vtt", "Track.FLAC.t.lrc", "Track.FLAC.t.vtt"} {
		if !translationNames[expected] {
			t.Fatalf("missing Windows translation candidate %q", expected)
		}
	}
}

func mustWriteTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
