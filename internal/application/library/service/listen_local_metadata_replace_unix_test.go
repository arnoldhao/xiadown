//go:build !windows

package service

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"xiadown/internal/application/library/dto"
)

func TestReplaceListenLocalMetadataFileKeepsExistingPlaybackHandleValid(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "track.mp3")
	source := filepath.Join(directory, "replacement.mp3")
	if err := os.WriteFile(destination, []byte("old audio bytes"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}
	if err := os.WriteFile(source, []byte("new audio bytes"), 0o600); err != nil {
		t.Fatalf("write replacement: %v", err)
	}
	playbackHandle, err := os.Open(destination)
	if err != nil {
		t.Fatalf("open playback handle: %v", err)
	}
	defer playbackHandle.Close()

	if err := replaceListenLocalMetadataFile(source, destination); err != nil {
		t.Fatalf("replace while original is open: %v", err)
	}
	playingBytes, err := io.ReadAll(playbackHandle)
	if err != nil {
		t.Fatalf("read existing playback handle: %v", err)
	}
	if string(playingBytes) != "old audio bytes" {
		t.Fatalf("existing playback handle changed: %q", playingBytes)
	}
	currentBytes, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read replaced path: %v", err)
	}
	if string(currentBytes) != "new audio bytes" {
		t.Fatalf("path was not atomically replaced: %q", currentBytes)
	}
}

func TestWriteListenLocalMetadataCommandFailureCleansTemporaryFile(t *testing.T) {
	realFFmpeg := listenLocalMetadataTestFFmpegPath()
	if realFFmpeg == "" {
		t.Skip("ffmpeg is unavailable")
	}
	realFFprobe := filepath.Join(filepath.Dir(realFFmpeg), ffprobeExecutableName())
	if _, err := os.Stat(realFFprobe); err != nil {
		t.Skip("matching ffprobe is unavailable")
	}
	directory := t.TempDir()
	path := createListenLocalMetadataFixture(t, realFFmpeg, directory, ".mp3", "libmp3lame", false, false, "custom_tag")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	toolDirectory := filepath.Join(directory, "tools")
	if err := os.Mkdir(toolDirectory, 0o700); err != nil {
		t.Fatalf("create tool directory: %v", err)
	}
	failingFFmpeg := filepath.Join(toolDirectory, ffmpegExecutableName())
	if err := os.WriteFile(failingFFmpeg, []byte("#!/bin/sh\nexit 7\n"), 0o700); err != nil {
		t.Fatalf("write failing ffmpeg: %v", err)
	}
	if err := os.Symlink(realFFprobe, filepath.Join(toolDirectory, ffprobeExecutableName())); err != nil {
		t.Fatalf("link ffprobe: %v", err)
	}
	service := &LibraryService{tools: &mediaProbeToolResolverStub{ready: true, toolDir: toolDirectory}}
	err = service.writeListenLocalMetadataWithFFmpeg(context.Background(), path, dto.UpdateListenLocalTrackMetadataRequest{Title: "New"})
	if err == nil {
		t.Fatal("expected ffmpeg failure")
	}
	temporary, _ := filepath.Glob(filepath.Join(directory, ".xiadown-metadata-*"))
	if len(temporary) != 0 {
		t.Fatalf("temporary files leaked after command failure: %#v", temporary)
	}
	current, readErr := os.ReadFile(path)
	if readErr != nil || string(current) != string(original) {
		t.Fatalf("original changed after command failure: equal=%t err=%v", string(current) == string(original), readErr)
	}
}
