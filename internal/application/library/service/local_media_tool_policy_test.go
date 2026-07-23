package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestLocalMediaInputPolicyAppliedToEveryFFmpegInput(t *testing.T) {
	t.Parallel()
	subtitlePath := filepath.Join(t.TempDir(), "subtitle.srt")
	if err := os.WriteFile(subtitlePath, []byte("1\n00:00:00,000 --> 00:00:01,000\ntext\n"), 0o600); err != nil {
		t.Fatalf("write subtitle: %v", err)
	}

	metadataArgs := buildListenLocalMetadataFFmpegArgs(
		"/tmp/input.mp3",
		"/tmp/output.mp3",
		dto.UpdateListenLocalTrackMetadataRequest{Title: "Song"},
	)
	assertEveryLocalFFmpegInputHasPolicy(t, metadataArgs)

	plan := transcodePlan{
		request: dto.CreateTranscodeJobRequest{
			Format:        "mkv",
			VideoCodec:    "h264",
			AudioCodec:    "aac",
			SubtitlePaths: []string{subtitlePath},
		},
		outputType: library.TranscodeOutputVideo,
	}
	transcodeArgs, err := buildFFmpegTranscodeArgs(plan, "/tmp/input.m3u8", "/tmp/output.mkv")
	if err != nil {
		t.Fatalf("build transcode args: %v", err)
	}
	assertEveryLocalFFmpegInputHasPolicy(t, transcodeArgs)
	if inputs := countArgument(transcodeArgs, "-i"); inputs != 2 {
		t.Fatalf("expected source and subtitle inputs, got %d: %v", inputs, transcodeArgs)
	}
}

func TestLocalMediaFFprobeInputPolicy(t *testing.T) {
	t.Parallel()

	base := []string{"-v", "error", "-show_format"}
	args := appendLocalMediaFFprobeInput(base, " /tmp/input.m3u8 ")
	wantPolicy := localMediaInputPolicyArgs()
	if len(args) != len(base)+len(wantPolicy)+1 {
		t.Fatalf("unexpected ffprobe args: %v", args)
	}
	if !slices.Equal(args[len(base):len(base)+len(wantPolicy)], wantPolicy) {
		t.Fatalf("ffprobe policy = %v, want %v", args[len(base):len(base)+len(wantPolicy)], wantPolicy)
	}
	if args[len(args)-1] != "/tmp/input.m3u8" {
		t.Fatalf("ffprobe input path was not normalized: %q", args[len(args)-1])
	}
}

func TestLocalMediaToolEnvironmentRemovesProxyRoutes(t *testing.T) {
	t.Parallel()

	environment := localMediaToolEnvironment([]string{
		"PATH=/usr/bin",
		"HTTP_PROXY=http://127.0.0.1:8080",
		"https_proxy=http://127.0.0.1:8081",
		"FTP_PROXY=http://127.0.0.1:8082",
		"ALL_PROXY=socks5://127.0.0.1:1080",
		"NO_PROXY=*",
		"HOME=/tmp/home",
	})
	joined := strings.ToLower(strings.Join(environment, "\n"))
	for _, key := range []string{"http_proxy=", "https_proxy=", "ftp_proxy=", "all_proxy=", "no_proxy="} {
		if strings.Contains(joined, key) {
			t.Fatalf("proxy variable %q survived: %v", key, environment)
		}
	}
	for _, expected := range []string{"PATH=/usr/bin", "HOME=/tmp/home"} {
		if !slices.Contains(environment, expected) {
			t.Fatalf("safe environment entry %q was removed: %v", expected, environment)
		}
	}
}

func TestLocalFFprobeDoesNotFetchNetworkReferenceFromManifest(t *testing.T) {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not available")
	}

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("not-media"))
	}))
	defer server.Close()

	manifestPath := filepath.Join(t.TempDir(), "remote.m3u8")
	manifest := "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4,\n" + server.URL + "/segment.ts\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write local manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := probeListenLocalMetadataManifest(ctx, ffprobePath, manifestPath); err == nil {
		t.Fatal("expected a local manifest with a network segment to be rejected")
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("ffprobe bypassed the local-only policy and made %d network request(s)", got)
	}
}

func assertEveryLocalFFmpegInputHasPolicy(t *testing.T, args []string) {
	t.Helper()
	want := localMediaInputPolicyArgs()
	inputs := 0
	for index, arg := range args {
		if arg != "-i" {
			continue
		}
		inputs++
		if index < len(want) || !slices.Equal(args[index-len(want):index], want) {
			t.Fatalf("input at index %d is not immediately guarded by %v: %v", index, want, args)
		}
	}
	if inputs == 0 {
		t.Fatalf("expected at least one FFmpeg input: %v", args)
	}
}

func countArgument(args []string, value string) int {
	count := 0
	for _, arg := range args {
		if arg == value {
			count++
		}
	}
	return count
}
