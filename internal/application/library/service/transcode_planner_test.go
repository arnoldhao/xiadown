package service

import (
	"testing"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestResolveTranscodeDisplayNameUsesSourceMetadataTitle(t *testing.T) {
	t.Parallel()

	preset, err := library.NewTranscodePreset(library.TranscodePresetParams{
		ID:         "preset-h265",
		Name:       "H.265 MP4 Original",
		OutputType: string(library.TranscodeOutputVideo),
		Container:  "mp4",
		VideoCodec: "h265",
		AudioCodec: "aac",
	})
	if err != nil {
		t.Fatalf("new preset: %v", err)
	}

	sourceFile, err := library.NewLibraryFile(library.LibraryFileParams{
		ID:        "file-1",
		LibraryID: "lib-1",
		Kind:      string(library.FileKindTranscode),
		Name:      "Episode 01 - H.264 MP4 Original",
		Metadata:  library.FileMetadata{Title: "Episode 01", Author: "Uploader", Extractor: "youtube"},
		Storage: library.FileStorage{
			Mode:      "local_path",
			LocalPath: "/tmp/Episode 01 - H.264 MP4 Original.mp4",
		},
		Origin: library.FileOrigin{
			Kind:        "transcode",
			OperationID: "op-1",
		},
		State: library.FileState{Status: "active"},
	})
	if err != nil {
		t.Fatalf("new file: %v", err)
	}

	got := resolveTranscodeDisplayName(dto.CreateTranscodeJobRequest{}, sourceFile, &preset)
	if got != "Episode 01 - H.265 MP4 Original" {
		t.Fatalf("expected metadata-backed display name, got %q", got)
	}
}

func TestResolveTranscodeOutputNameKeepsSourceFileBase(t *testing.T) {
	t.Parallel()

	preset, err := library.NewTranscodePreset(library.TranscodePresetParams{
		ID:         "preset-h265",
		Name:       "H.265 MP4 Original",
		OutputType: string(library.TranscodeOutputVideo),
		Container:  "mp4",
		VideoCodec: "h265",
		AudioCodec: "aac",
	})
	if err != nil {
		t.Fatalf("new preset: %v", err)
	}

	got := resolveTranscodeOutputName("/tmp/Episode 01 - H.264 MP4 Original.mp4", &preset)
	if got != "Episode 01 - H.264 MP4 Original - H.265 MP4 Original" {
		t.Fatalf("expected disk file name to extend source base name, got %q", got)
	}
}
