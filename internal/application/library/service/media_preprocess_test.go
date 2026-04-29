package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"xiadown/internal/domain/dependencies"
)

func TestParseFFprobeMediaProbe(t *testing.T) {
	t.Parallel()

	output := []byte(`{
		"streams": [
			{
				"codec_type": "video",
				"codec_name": "h264",
				"width": 1920,
				"height": 1080,
				"avg_frame_rate": "60000/1001",
				"bit_rate": "400000"
			},
			{
				"codec_type": "audio",
				"codec_name": "aac",
				"channels": 2,
				"bit_rate": "256000"
			}
		],
		"format": {
			"format_name": "mov,mp4,m4a,3gp,3g2,mj2",
			"duration": "12.345",
			"size": "1048576",
			"bit_rate": "512000"
		}
	}`)

	got, err := parseFFprobeMediaProbe(output, "/tmp/example.mp4")
	if err != nil {
		t.Fatalf("parse ffprobe: %v", err)
	}
	if got.Format != "mp4" {
		t.Fatalf("expected format mp4, got %q", got.Format)
	}
	if got.VideoCodec != "h264" {
		t.Fatalf("expected video codec h264, got %q", got.VideoCodec)
	}
	if got.AudioCodec != "aac" {
		t.Fatalf("expected audio codec aac, got %q", got.AudioCodec)
	}
	if got.Codec != "h264" {
		t.Fatalf("expected primary codec h264, got %q", got.Codec)
	}
	if got.Width != 1920 || got.Height != 1080 {
		t.Fatalf("expected 1920x1080, got %dx%d", got.Width, got.Height)
	}
	if got.Channels != 2 {
		t.Fatalf("expected 2 channels, got %d", got.Channels)
	}
	if got.BitrateKbps != 512 {
		t.Fatalf("expected 512kbps, got %d", got.BitrateKbps)
	}
	if got.VideoBitrateKbps != 400 {
		t.Fatalf("expected 400kbps video bitrate, got %d", got.VideoBitrateKbps)
	}
	if got.AudioBitrateKbps != 256 {
		t.Fatalf("expected 256kbps audio bitrate, got %d", got.AudioBitrateKbps)
	}
	if got.DurationMs != 12345 {
		t.Fatalf("expected 12345ms, got %d", got.DurationMs)
	}
	if got.SizeBytes != 1048576 {
		t.Fatalf("expected size 1048576, got %d", got.SizeBytes)
	}
	if got.FrameRate < 59.93 || got.FrameRate > 59.95 {
		t.Fatalf("expected frame rate near 59.94, got %.4f", got.FrameRate)
	}
}

func TestResolveFFprobeExecPathUsesDependenciesOnly(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	ffprobePath := filepath.Join(tempDir, ffprobeExecutableName())
	if err := os.WriteFile(ffprobePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write ffprobe stub: %v", err)
	}

	resolver := &mediaProbeToolResolverStub{
		ready:   true,
		toolDir: tempDir,
	}
	got, err := resolveFFprobeExecPath(context.Background(), resolver)
	if err != nil {
		t.Fatalf("resolve ffprobe exec path: %v", err)
	}
	if got != ffprobePath {
		t.Fatalf("expected %q, got %q", ffprobePath, got)
	}
}

func TestResolveFFprobeExecPathRequiresDependencyReadiness(t *testing.T) {
	t.Parallel()

	resolver := &mediaProbeToolResolverStub{
		ready:  false,
		reason: "not_installed",
	}
	_, err := resolveFFprobeExecPath(context.Background(), resolver)
	if err == nil || err.Error() != "ffmpeg is not installed" {
		t.Fatalf("expected ffmpeg is not installed error, got %v", err)
	}
}

type mediaProbeToolResolverStub struct {
	ready    bool
	reason   string
	toolDir  string
	execPath string
	err      error
}

func (stub *mediaProbeToolResolverStub) ResolveExecPath(context.Context, dependencies.DependencyName) (string, error) {
	if stub.err != nil {
		return "", stub.err
	}
	return stub.execPath, nil
}

func (stub *mediaProbeToolResolverStub) ResolveDependencyDirectory(context.Context, dependencies.DependencyName) (string, error) {
	if stub.err != nil {
		return "", stub.err
	}
	return stub.toolDir, nil
}

func (stub *mediaProbeToolResolverStub) DependencyReadiness(context.Context, dependencies.DependencyName) (bool, string, error) {
	if stub.err != nil {
		return false, "", stub.err
	}
	return stub.ready, stub.reason, nil
}
