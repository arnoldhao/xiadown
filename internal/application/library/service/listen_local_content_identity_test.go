package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xiadown/internal/application/library/dto"
)

func TestListenLocalContentIdentitySignatureSeparatesTimelineFromMetadata(t *testing.T) {
	t.Parallel()
	packets := []string{
		strings.Repeat("1", 64),
		strings.Repeat("2", 64),
		strings.Repeat("3", 64),
	}
	baseline := buildListenLocalContentIdentitySignature("packet-v1", packets...)
	if !strings.HasPrefix(baseline, "mci1p:") || len(baseline) != len("mci1p:")+64 {
		t.Fatalf("signature=%q", baseline)
	}

	// Descriptive/container probe fields are deliberately absent from this
	// input: only the encoded audio packet payloads define the private baseline.
	if got := buildListenLocalContentIdentitySignature("packet-v1", packets...); got != baseline {
		t.Fatalf("metadata-only edit changed content identity: before=%q after=%q", baseline, got)
	}
	if got := stabilizeListenLocalContentIdentitySignature(baseline, ""); got != baseline {
		t.Fatalf("transient identity failure discarded baseline: got=%q want=%q", got, baseline)
	}

	replacedPackets := append([]string(nil), packets...)
	replacedPackets[1] = strings.Repeat("a", 64)
	if got := buildListenLocalContentIdentitySignature("packet-v1", replacedPackets...); got == baseline {
		t.Fatal("same-duration audio replacement retained content identity")
	}
}

func TestListenLocalContentIdentityReadIntervalsAreBounded(t *testing.T) {
	t.Parallel()
	if got, want := listenLocalContentIdentityReadIntervals(180_000), "0.000%+#12,90.000%+#12,179.000%+#12"; got != want {
		t.Fatalf("intervals=%q want=%q", got, want)
	}
	if got, want := listenLocalContentIdentityReadIntervals(1_500), "0.000%+#12"; got != want {
		t.Fatalf("short intervals=%q want=%q", got, want)
	}
}

func TestListenLocalContentIdentityPacketSamplingIgnoresTagsAndDetectsReplacement(t *testing.T) {
	ffmpegPath := listenLocalMetadataTestFFmpegPath()
	if ffmpegPath == "" {
		t.Skip("ffmpeg is not available")
	}
	ffprobePath := filepath.Join(filepath.Dir(ffmpegPath), ffprobeExecutableName())
	if info, err := os.Stat(ffprobePath); err != nil || info.IsDir() {
		t.Skip("matching ffprobe is not available")
	}

	directory := t.TempDir()
	path := createListenLocalMetadataFixture(t, ffmpegPath, directory, ".mp3", "libmp3lame", false, false, "")
	// Use a long-enough fixture to exercise the beginning, middle, and end
	// read-intervals rather than proving only the first-packet window.
	runListenLocalFixtureFFmpeg(t, ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-c:a", "libmp3lame", "-ac", "2", path,
	)
	service := &LibraryService{tools: &mediaProbeToolResolverStub{
		ready: true, toolDir: filepath.Dir(ffprobePath), execPath: ffmpegPath,
	}}
	probe, err := service.ffprobeLocalMedia(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	baseline := service.listenLocalContentIdentitySignature(context.Background(), path, probe)

	if err := service.writeListenLocalMetadataWithFFmpeg(
		context.Background(),
		path,
		dto.UpdateListenLocalTrackMetadataRequest{Title: "New title", Author: "New artist"},
	); err != nil {
		t.Fatal(err)
	}
	probe, err = service.ffprobeLocalMedia(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if afterTags := service.listenLocalContentIdentitySignature(context.Background(), path, probe); afterTags != baseline {
		t.Fatalf("metadata rewrite changed sampled timeline identity: before=%q after=%q", baseline, afterTags)
	}

	runListenLocalFixtureFFmpeg(t, ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-f", "lavfi", "-i", "sine=frequency=880:duration=3",
		"-c:a", "libmp3lame", "-ac", "2", path,
	)
	probe, err = service.ffprobeLocalMedia(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if replacement := service.listenLocalContentIdentitySignature(context.Background(), path, probe); replacement == baseline {
		t.Fatal("same-duration replacement retained sampled timeline identity")
	}
}
