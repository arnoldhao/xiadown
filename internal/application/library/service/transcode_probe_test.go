package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestBuildTranscodeInputProbeResponseFiltersUpscalePresets(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	service := &LibraryService{nowFunc: func() time.Time { return now }}
	probe := mediaProbe{
		Format:     "mp4",
		HasVideo:   true,
		HasAudio:   true,
		VideoCodec: "h264",
		AudioCodec: "aac",
		Width:      1920,
		Height:     1080,
	}

	response := service.buildTranscodeInputProbeResponse(context.Background(), probe, defaultTranscodePresets(now))

	if response.MediaType != "video" {
		t.Fatalf("expected video media type, got %q", response.MediaType)
	}
	if response.RecommendedPresetID != "builtin-video-h264-mp4-original" {
		t.Fatalf("expected original H.264 MP4 recommendation, got %q", response.RecommendedPresetID)
	}
	if !stringSliceContains(response.CompatiblePresetIDs, "builtin-video-h264-mp4-1080p") {
		t.Fatalf("expected 1080p preset to remain compatible")
	}
	if stringSliceContains(response.CompatiblePresetIDs, "builtin-video-h264-mp4-2160p") {
		t.Fatalf("expected 2160p preset to be filtered for 1080p input")
	}
}

func TestBuildTranscodeInputProbeResponseRecommendsAudioDefault(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	service := &LibraryService{nowFunc: func() time.Time { return now }}
	probe := mediaProbe{
		Format:     "m4a",
		HasAudio:   true,
		AudioCodec: "aac",
		Channels:   2,
	}

	response := service.buildTranscodeInputProbeResponse(context.Background(), probe, defaultTranscodePresets(now))

	if response.MediaType != "audio" {
		t.Fatalf("expected audio media type, got %q", response.MediaType)
	}
	if response.RecommendedPresetID != "builtin-audio-mp3-320k" {
		t.Fatalf("expected MP3 320k recommendation, got %q", response.RecommendedPresetID)
	}
	if stringSliceContains(response.CompatiblePresetIDs, "builtin-video-h264-mp4-original") {
		t.Fatalf("expected video preset to be incompatible for audio input")
	}
	if !stringSliceContains(response.CompatiblePresetIDs, "builtin-audio-mp3-320k") {
		t.Fatalf("expected MP3 320k preset to be compatible")
	}
}

func TestResolveSourceFileForTranscodeImportsManualAudioIntoNewLibrary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "track.m4a")
	if err := os.WriteFile(inputPath, []byte("fake-audio"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	toolDir := filepath.Join(tempDir, "ffmpeg")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("mkdir tool dir: %v", err)
	}
	ffprobePath := filepath.Join(toolDir, ffprobeExecutableName())
	ffprobeScript := `#!/bin/sh
cat <<'JSON'
{"streams":[{"index":0,"codec_type":"audio","codec_name":"aac","channels":2,"bit_rate":"192000"}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"12.5","size":"1234","bit_rate":"192000"}}
JSON
`
	if err := os.WriteFile(ffprobePath, []byte(ffprobeScript), 0o755); err != nil {
		t.Fatalf("write ffprobe: %v", err)
	}

	libraries := &deleteRuleLibraryRepo{}
	files := &deleteRuleFileRepo{}
	service := &LibraryService{
		libraries:  libraries,
		files:      files,
		histories:  &deleteRuleHistoryRepo{},
		fileEvents: &deleteRuleFileEventRepo{},
		tools:      &mediaProbeToolResolverStub{ready: true, toolDir: toolDir},
		nowFunc: func() time.Time {
			return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
		},
	}

	sourceFile, err := service.resolveSourceFileForTranscode(ctx, dto.CreateTranscodeJobRequest{
		InputPath: inputPath,
		Source:    "xiadown.transcode.dialog",
	})
	if err != nil {
		t.Fatalf("resolve source file: %v", err)
	}

	if sourceFile.LibraryID == "" {
		t.Fatal("expected manual transcode source to be attached to a new library")
	}
	if sourceFile.Kind != library.FileKindAudio {
		t.Fatalf("expected imported source kind audio, got %q", sourceFile.Kind)
	}
	if sourceFile.Storage.LocalPath != inputPath {
		t.Fatalf("expected stored source path %q, got %q", inputPath, sourceFile.Storage.LocalPath)
	}
	if sourceFile.Origin.Kind != "import" || sourceFile.Origin.Import == nil {
		t.Fatalf("expected imported source origin, got %#v", sourceFile.Origin)
	}
	if sourceFile.Media == nil || sourceFile.Media.AudioCodec != "aac" {
		t.Fatalf("expected probed audio media info, got %#v", sourceFile.Media)
	}
	if len(libraries.items) != 1 {
		t.Fatalf("expected one created library, got %d", len(libraries.items))
	}
	if len(files.savedItems) != 1 {
		t.Fatalf("expected one imported file save, got %d", len(files.savedItems))
	}
}

func TestTranscodeSourceIdentityTracksParentAndRoot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	sourceFile, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:        "source-transcode",
		LibraryID: "library-1",
		Kind:      string(library.FileKindTranscode),
		Name:      "source.mp4",
		Storage:   library.FileStorage{Mode: "local_path", LocalPath: "/tmp/source.mp4"},
		Origin:    library.FileOrigin{Kind: "transcode", OperationID: "parent-op"},
		Lineage:   library.FileLineage{RootFileID: "root-file"},
		State:     library.FileState{Status: "active"},
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new source file: %v", err)
	}

	service := &LibraryService{}
	enriched := service.enrichTranscodeRequestForSource(context.Background(), dto.CreateTranscodeJobRequest{}, sourceFile)

	if enriched.FileID != sourceFile.ID {
		t.Fatalf("expected source file id in request, got %q", enriched.FileID)
	}
	if enriched.RootFileID != "root-file" {
		t.Fatalf("expected root file id to be preserved, got %q", enriched.RootFileID)
	}
	if parent := resolveTranscodeParentOperationID(sourceFile); parent != "parent-op" {
		t.Fatalf("expected parent operation id, got %q", parent)
	}
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
