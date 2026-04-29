package service

import (
	"context"
	"path/filepath"
	"testing"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestResolveInitialLibraryName(t *testing.T) {
	t.Parallel()

	if got := resolveInitialLibraryName("lib-123", "https://example.com/watch?v=1", true); got != "lib-123" {
		t.Fatalf("expected generated id as initial name, got %q", got)
	}

	if got := resolveInitialLibraryName("lib-123", "Imported File", false); got != "Imported File" {
		t.Fatalf("expected fallback name for non-download library, got %q", got)
	}
}

func TestResolveLibraryNameFromFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "strip extension", in: "Episode 01.mp4", want: "Episode 01"},
		{name: "keep plain name", in: "Episode 01", want: "Episode 01"},
		{name: "path base", in: "/tmp/output/clip.final.mkv", want: "clip.final"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveLibraryNameFromFile(tc.in); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestToLibraryFileDTOBackfillsFormatFromPath(t *testing.T) {
	t.Parallel()

	item, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:        "file-1",
		LibraryID: "lib-1",
		Kind:      string(library.FileKindVideo),
		Name:      "clip.mp4",
		Storage: library.FileStorage{
			Mode:      "local_path",
			LocalPath: "/tmp/clip.mp4",
		},
		Origin: library.FileOrigin{
			Kind: "import",
			Import: &library.ImportOrigin{
				BatchID:    "batch-1",
				ImportPath: "/tmp/clip.mp4",
			},
		},
		State: library.FileState{
			Status: "ready",
		},
	})
	if err != nil {
		t.Fatalf("new file: %v", err)
	}

	got := toLibraryFileDTO(item)
	if got.Media == nil {
		t.Fatalf("expected media dto with fallback format")
	}
	if got.Media.Format != "mp4" {
		t.Fatalf("expected fallback format mp4, got %q", got.Media.Format)
	}
	if got.DisplayLabel != "clip" {
		t.Fatalf("expected fallback display label clip, got %q", got.DisplayLabel)
	}
}

func TestToLibraryFileDTOIncludesDisplayMetadataAndFileName(t *testing.T) {
	t.Parallel()

	item, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:        "file-2",
		LibraryID: "lib-1",
		Kind:      string(library.FileKindTranscode),
		Name:      "Episode 01 - H.264 MP4 Original",
		Metadata: library.FileMetadata{
			Title:     "Episode 01",
			Author:    "Uploader",
			Extractor: "youtube",
		},
		Storage: library.FileStorage{
			Mode:      "local_path",
			LocalPath: "/tmp/Episode 01 - H.264 MP4 Original.mp4",
		},
		Origin: library.FileOrigin{
			Kind:        "transcode",
			OperationID: "op-1",
		},
		State: library.FileState{
			Status: "active",
		},
	})
	if err != nil {
		t.Fatalf("new file: %v", err)
	}

	got := toLibraryFileDTO(item)
	if got.DisplayName != "Episode 01 - H.264 MP4 Original" {
		t.Fatalf("expected display name to mirror logical file title, got %q", got.DisplayName)
	}
	if got.FileName != "Episode 01 - H.264 MP4 Original.mp4" {
		t.Fatalf("expected file name from local path, got %q", got.FileName)
	}
	if got.Metadata.Title != "Episode 01" || got.Metadata.Author != "Uploader" || got.Metadata.Extractor != "youtube" {
		t.Fatalf("expected metadata to be mapped, got %#v", got.Metadata)
	}
}

func TestDeriveManagedOutputPathUsesSourceDirectory(t *testing.T) {
	t.Parallel()

	service := &LibraryService{}
	got, err := service.deriveManagedOutputPath(
		context.Background(),
		"Episode 01 - H.264 MP4 Original",
		"mp4",
		"/tmp/xiadown/input/Episode 01.mkv",
	)
	if err != nil {
		t.Fatalf("deriveManagedOutputPath returned error: %v", err)
	}
	want := filepath.Join("/tmp/xiadown/input", "Episode 01 - H.264 MP4 Original.mp4")
	if got != want {
		t.Fatalf("expected output path %q, got %q", want, got)
	}
}

func TestBuildLibraryFileDisplayLabel(t *testing.T) {
	t.Parallel()

	t.Run("video", func(t *testing.T) {
		t.Parallel()
		width := 1920
		height := 1080
		frameRate := 59.94
		got := buildLibraryFileDisplayLabel(dto.LibraryFileDTO{
			Name: "Example.Final.Cut.1080p.H264.mp4",
			Kind: "video",
			Media: &dto.LibraryMediaInfoDTO{
				Format:     "mp4",
				VideoCodec: "h264",
				Width:      &width,
				Height:     &height,
				FrameRate:  &frameRate,
			},
		})
		if got != "Example Final Cut · 1080p · 59.94fps · H264" {
			t.Fatalf("expected video display label, got %q", got)
		}
	})

	t.Run("audio", func(t *testing.T) {
		t.Parallel()
		bitrate := 256
		got := buildLibraryFileDisplayLabel(dto.LibraryFileDTO{
			Name: "Example Interview.aac.m4a",
			Kind: "audio",
			Media: &dto.LibraryMediaInfoDTO{
				Format:      "m4a",
				AudioCodec:  "aac",
				BitrateKbps: &bitrate,
			},
		})
		if got != "Example Interview · AAC · 256kbps" {
			t.Fatalf("expected audio display label, got %q", got)
		}
	})

	t.Run("subtitle", func(t *testing.T) {
		t.Parallel()
		got := buildLibraryFileDisplayLabel(dto.LibraryFileDTO{
			Name: "Episode 01.zh-CN.srt",
			Kind: "subtitle",
			Media: &dto.LibraryMediaInfoDTO{
				Format:   "srt",
				Language: "zh-CN",
			},
		})
		if got != "Episode 01 · ZH-CN" {
			t.Fatalf("expected subtitle display label, got %q", got)
		}
	})

	t.Run("thumbnail", func(t *testing.T) {
		t.Parallel()
		width := 1920
		height := 1080
		got := buildLibraryFileDisplayLabel(dto.LibraryFileDTO{
			Name: "cover-shot.1920x1080.png",
			Kind: "thumbnail",
			Media: &dto.LibraryMediaInfoDTO{
				Format: "png",
				Width:  &width,
				Height: &height,
			},
		})
		if got != "cover shot · 1920x1080" {
			t.Fatalf("expected thumbnail display label, got %q", got)
		}
	})
}

func TestDetectSubtitleLanguage(t *testing.T) {
	t.Parallel()

	t.Run("matches configured alias from file name", func(t *testing.T) {
		t.Parallel()
		config := library.DefaultModuleConfig()
		item, err := library.NewLibraryFile(library.LibraryFileParams{
			ID:        "file-subtitle",
			LibraryID: "lib-1",
			Kind:      string(library.FileKindSubtitle),
			Name:      "Episode 01.zh-TW.srt",
			Storage: library.FileStorage{
				Mode:       "hybrid",
				LocalPath:  "/tmp/Episode 01.zh-TW.srt",
				DocumentID: "doc-1",
			},
			Origin: library.FileOrigin{
				Kind:        "download",
				OperationID: "op-1",
			},
			State: library.FileState{Status: "active"},
		})
		if err != nil {
			t.Fatalf("new file: %v", err)
		}
		if got := detectSubtitleLanguage(item, "這是一段字幕內容", config); got != "zh-TW" {
			t.Fatalf("expected zh-TW, got %q", got)
		}
	})

	t.Run("falls back to content heuristics", func(t *testing.T) {
		t.Parallel()
		config := library.DefaultModuleConfig()
		item, err := library.NewLibraryFile(library.LibraryFileParams{
			ID:        "file-subtitle-ja",
			LibraryID: "lib-1",
			Kind:      string(library.FileKindSubtitle),
			Name:      "Episode 01.srt",
			Storage: library.FileStorage{
				Mode:       "hybrid",
				LocalPath:  "/tmp/Episode 01.srt",
				DocumentID: "doc-1",
			},
			Origin: library.FileOrigin{
				Kind:   "import",
				Import: &library.ImportOrigin{BatchID: "batch-1", ImportPath: "/tmp/Episode 01.srt"},
			},
			State: library.FileState{Status: "active"},
		})
		if err != nil {
			t.Fatalf("new file: %v", err)
		}
		if got := detectSubtitleLanguage(item, "こんにちは。\n今日はいい天気ですね。", config); got != "ja" {
			t.Fatalf("expected ja, got %q", got)
		}
	})

	t.Run("returns other when outside configured enum", func(t *testing.T) {
		t.Parallel()
		config := library.DefaultModuleConfig()
		item, err := library.NewLibraryFile(library.LibraryFileParams{
			ID:        "file-subtitle-other",
			LibraryID: "lib-1",
			Kind:      string(library.FileKindSubtitle),
			Name:      "Episode 01.srt",
			Storage: library.FileStorage{
				Mode:       "hybrid",
				LocalPath:  "/tmp/Episode 01.srt",
				DocumentID: "doc-1",
			},
			Origin: library.FileOrigin{
				Kind:        "download",
				OperationID: "op-1",
			},
			State: library.FileState{Status: "active"},
		})
		if err != nil {
			t.Fatalf("new file: %v", err)
		}
		if got := detectSubtitleLanguage(item, "zxqy prlm nvtr 12345", config); got != "other" {
			t.Fatalf("expected other, got %q", got)
		}
	})
}
