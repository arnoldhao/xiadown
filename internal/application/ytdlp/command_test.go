package ytdlp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/dependencies"
)

type toolResolverStub struct {
	paths map[dependencies.DependencyName]string
}

func (stub toolResolverStub) ResolveExecPath(_ context.Context, name dependencies.DependencyName) (string, error) {
	if execPath, ok := stub.paths[name]; ok {
		return execPath, nil
	}
	return "", fmt.Errorf("%s not found", name)
}

func TestBuildExplicitToolArgsUsesConfiguredFFmpegAndBunPaths(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	ffmpegPath := filepath.Join(tempDir, "ffmpeg", "ffmpeg")
	bunPath := filepath.Join(tempDir, "bun", "bun")
	if err := os.MkdirAll(filepath.Dir(ffmpegPath), 0o755); err != nil {
		t.Fatalf("mkdir ffmpeg dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(bunPath), 0o755); err != nil {
		t.Fatalf("mkdir bun dir: %v", err)
	}
	if err := os.WriteFile(ffmpegPath, []byte(""), 0o755); err != nil {
		t.Fatalf("write ffmpeg file: %v", err)
	}
	if err := os.WriteFile(bunPath, []byte(""), 0o755); err != nil {
		t.Fatalf("write bun file: %v", err)
	}

	args := BuildExplicitToolArgs(context.Background(), toolResolverStub{
		paths: map[dependencies.DependencyName]string{
			dependencies.DependencyFFmpeg: ffmpegPath,
			dependencies.DependencyBun:    bunPath,
		},
	})

	if len(args) != 5 {
		t.Fatalf("expected 5 explicit tool args, got %d: %v", len(args), args)
	}
	if args[0] != "--ffmpeg-location" || args[1] != filepath.Dir(ffmpegPath) {
		t.Fatalf("unexpected ffmpeg args: %v", args[:2])
	}
	if args[2] != "--no-js-runtimes" || args[3] != "--js-runtimes" || args[4] != "bun:"+bunPath {
		t.Fatalf("unexpected js runtime args: %v", args[2:])
	}
}

func TestBuildCommandUsesExplicitToolArgsWithoutMutatingPATH(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")

	tempDir := t.TempDir()
	ffmpegPath := filepath.Join(tempDir, "ffmpeg", "ffmpeg")
	bunPath := filepath.Join(tempDir, "bun", "bun")
	if err := os.MkdirAll(filepath.Dir(ffmpegPath), 0o755); err != nil {
		t.Fatalf("mkdir ffmpeg dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(bunPath), 0o755); err != nil {
		t.Fatalf("mkdir bun dir: %v", err)
	}
	if err := os.WriteFile(ffmpegPath, []byte(""), 0o755); err != nil {
		t.Fatalf("write ffmpeg file: %v", err)
	}
	if err := os.WriteFile(bunPath, []byte(""), 0o755); err != nil {
		t.Fatalf("write bun file: %v", err)
	}

	command, err := BuildCommand(context.Background(), CommandOptions{
		ExecPath: filepath.Join(tempDir, "yt-dlp"),
		Tools: toolResolverStub{
			paths: map[dependencies.DependencyName]string{
				dependencies.DependencyFFmpeg: ffmpegPath,
				dependencies.DependencyBun:    bunPath,
			},
		},
		Request: dto.CreateYTDLPJobRequest{
			URL:            "https://example.com/watch?v=1",
			WriteThumbnail: true,
			SubtitleLangs:  []string{"en"},
		},
		OutputTemplate: filepath.Join(tempDir, "downloads", "%(title)s.%(ext)s"),
	})
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	defer command.Cancel()
	if command.Cleanup != nil {
		defer command.Cleanup()
	}

	argsJoined := strings.Join(command.Args, "\n")
	if !strings.Contains(argsJoined, "--ffmpeg-location\n"+filepath.Dir(ffmpegPath)) {
		t.Fatalf("expected explicit ffmpeg args, got %v", command.Args)
	}
	if !strings.Contains(argsJoined, "--no-js-runtimes") || !strings.Contains(argsJoined, "--js-runtimes\nbun:"+bunPath) {
		t.Fatalf("expected explicit bun runtime args, got %v", command.Args)
	}
	if strings.Contains(argsJoined, "--write-thumbnail") {
		t.Fatalf("expected primary command to omit thumbnail args, got %v", command.Args)
	}
	if strings.Contains(argsJoined, "--write-subs") || strings.Contains(argsJoined, "--write-auto-subs") {
		t.Fatalf("expected primary command to omit subtitle args, got %v", command.Args)
	}
	if !strings.Contains(argsJoined, "--continue") {
		t.Fatalf("expected primary command to enable partial download resume, got %v", command.Args)
	}

	pathEntry := ""
	for _, entry := range command.Cmd.Env {
		if strings.HasPrefix(entry, "PATH=") {
			pathEntry = entry
			break
		}
	}
	if pathEntry != "PATH=/usr/bin" {
		t.Fatalf("expected PATH to remain unchanged, got %q", pathEntry)
	}
}

func TestBuildCommandAddsSafeCapturedHeaders(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	command, err := BuildCommand(context.Background(), CommandOptions{
		ExecPath: filepath.Join(tempDir, "yt-dlp"),
		Request: dto.CreateYTDLPJobRequest{
			URL: "https://media.example/replay/index.m3u8",
		},
		OutputTemplate: filepath.Join(tempDir, "downloads", "%(title)s.%(ext)s"),
		Headers: map[string]string{
			"Referer":        "https://page.example/watch",
			"User-Agent":     "TestAgent",
			"Cookie":         "sid=1",
			"Range":          "bytes=0-1",
			"Content-Length": "2",
			"Sec-Fetch-Site": "same-origin",
			"Sec-CH-UA":      `"Chromium";v="129"`,
		},
	})
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	defer command.Cancel()
	if command.Cleanup != nil {
		defer command.Cleanup()
	}

	argsJoined := strings.Join(command.Args, "\n")
	for _, expected := range []string{
		"--add-header\nReferer: https://page.example/watch",
		"--add-header\nUser-Agent: TestAgent",
		"--add-header\nCookie: sid=1",
	} {
		if !strings.Contains(argsJoined, expected) {
			t.Fatalf("expected captured header args to contain %q, got %v", expected, command.Args)
		}
	}
	for _, forbidden := range []string{"Range:", "Content-Length:", "Sec-Fetch-Site:", "Sec-CH-UA:"} {
		if strings.Contains(argsJoined, forbidden) {
			t.Fatalf("expected unsafe header %q to be omitted, got %v", forbidden, command.Args)
		}
	}
}

func TestBuildCommandAddsConcurrentFragmentsOnlyWhenConfigured(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	defaultCommand, err := BuildCommand(context.Background(), CommandOptions{
		ExecPath: filepath.Join(tempDir, "yt-dlp"),
		Request: dto.CreateYTDLPJobRequest{
			URL: "https://media.example/replay/index.m3u8",
		},
		OutputTemplate: filepath.Join(tempDir, "downloads", "%(title)s.%(ext)s"),
	})
	if err != nil {
		t.Fatalf("build default command: %v", err)
	}
	defer defaultCommand.Cancel()
	if defaultCommand.Cleanup != nil {
		defer defaultCommand.Cleanup()
	}
	if strings.Contains(strings.Join(defaultCommand.Args, "\n"), "--concurrent-fragments") {
		t.Fatalf("expected default command to omit concurrent fragments, got %v", defaultCommand.Args)
	}

	configuredCommand, err := BuildCommand(context.Background(), CommandOptions{
		ExecPath: filepath.Join(tempDir, "yt-dlp"),
		Request: dto.CreateYTDLPJobRequest{
			URL: "https://media.example/replay/index.m3u8",
		},
		OutputTemplate:      filepath.Join(tempDir, "downloads", "%(title)s.%(ext)s"),
		ConcurrentFragments: 8,
	})
	if err != nil {
		t.Fatalf("build configured command: %v", err)
	}
	defer configuredCommand.Cancel()
	if configuredCommand.Cleanup != nil {
		defer configuredCommand.Cleanup()
	}
	if !strings.Contains(strings.Join(configuredCommand.Args, "\n"), "--concurrent-fragments\n8") {
		t.Fatalf("expected configured command to include concurrent fragments, got %v", configuredCommand.Args)
	}
}

func TestBuildCommandAddsFormatSortForBitrateQuality(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	command, err := BuildCommand(context.Background(), CommandOptions{
		ExecPath: filepath.Join(tempDir, "yt-dlp"),
		Request: dto.CreateYTDLPJobRequest{
			URL:     "https://example.com/watch?v=1",
			Quality: "bitrate",
		},
		OutputTemplate: filepath.Join(tempDir, "downloads", "%(title)s.%(ext)s"),
	})
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	defer command.Cancel()
	if command.Cleanup != nil {
		defer command.Cleanup()
	}

	argsJoined := strings.Join(command.Args, "\n")
	if !strings.Contains(argsJoined, "-S\nres,br") {
		t.Fatalf("expected bitrate quality to include format sort, got %v", command.Args)
	}
	hasFormatArg := false
	for _, arg := range command.Args {
		if arg == "-f" {
			hasFormatArg = true
			break
		}
	}
	if hasFormatArg {
		t.Fatalf("expected bitrate quality to keep yt-dlp default format selector, got %v", command.Args)
	}
}

func TestBuildCommandLimitsQuickPlaylistToFirstItem(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	command, err := BuildCommand(context.Background(), CommandOptions{
		ExecPath: filepath.Join(tempDir, "yt-dlp"),
		Request: dto.CreateYTDLPJobRequest{
			URL:  "https://www.youtube.com/playlist?list=PL123",
			Mode: "quick",
		},
		OutputTemplate: filepath.Join(tempDir, "downloads", "%(title)s.%(ext)s"),
	})
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	defer command.Cancel()
	if command.Cleanup != nil {
		defer command.Cleanup()
	}

	if !strings.Contains(strings.Join(command.Args, "\n"), "--playlist-items\n1") {
		t.Fatalf("expected quick command to limit playlist to first item, got %v", command.Args)
	}
}

func TestYTDLPFileLimitTargetRaisesWithinHardLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current uint64
		maximum uint64
		minimum uint64
		want    uint64
	}{
		{name: "already high enough", current: 8192, maximum: 8192, minimum: 4096, want: 8192},
		{name: "raise to minimum", current: 256, maximum: 8192, minimum: 4096, want: 4096},
		{name: "raise to hard limit", current: 256, maximum: 1024, minimum: 4096, want: 1024},
		{name: "hard limit not higher", current: 1024, maximum: 512, minimum: 4096, want: 1024},
		{name: "disabled", current: 256, maximum: 8192, minimum: 0, want: 256},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ytdlpFileLimitTarget(test.current, test.maximum, test.minimum)
			if got != test.want {
				t.Fatalf("expected target %d, got %d", test.want, got)
			}
		})
	}
}

func TestBuildCommandUsesStreamDownloadStrategy(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	command, err := BuildCommand(context.Background(), CommandOptions{
		ExecPath: filepath.Join(tempDir, "yt-dlp"),
		Request: dto.CreateYTDLPJobRequest{
			URL: "https://media.example/replay/index.m3u8",
		},
		OutputTemplate:      filepath.Join(tempDir, "downloads", "%(title)s.%(ext)s"),
		ConcurrentFragments: 8,
		StreamStrategy: StreamDownloadStrategy{
			Downloader:    StreamDownloaderNativeM3U8,
			ExtractorArgs: []string{"generic:hls_key=00112233445566778899aabbccddeeff"},
		},
	})
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	defer command.Cancel()
	if command.Cleanup != nil {
		defer command.Cleanup()
	}
	argsJoined := strings.Join(command.Args, "\n")
	for _, expected := range []string{
		"--downloader\nm3u8:native",
		"--extractor-args\ngeneric:hls_key=00112233445566778899aabbccddeeff",
		"--concurrent-fragments\n8",
	} {
		if !strings.Contains(argsJoined, expected) {
			t.Fatalf("expected command args to contain %q, got %v", expected, command.Args)
		}
	}
	sanitizedArgsJoined := strings.Join(command.SanitizedArgs, "\n")
	if strings.Contains(sanitizedArgsJoined, "00112233445566778899aabbccddeeff") ||
		!strings.Contains(sanitizedArgsJoined, "generic:hls_key=****") {
		t.Fatalf("expected hls key extractor arg to be sanitized, got %v", command.SanitizedArgs)
	}
}

func TestBuildSubtitleCommandUsesSubtitleArgsWithoutMutatingPATH(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")

	tempDir := t.TempDir()
	ffmpegPath := filepath.Join(tempDir, "ffmpeg", "ffmpeg")
	bunPath := filepath.Join(tempDir, "bun", "bun")
	if err := os.MkdirAll(filepath.Dir(ffmpegPath), 0o755); err != nil {
		t.Fatalf("mkdir ffmpeg dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(bunPath), 0o755); err != nil {
		t.Fatalf("mkdir bun dir: %v", err)
	}
	if err := os.WriteFile(ffmpegPath, []byte(""), 0o755); err != nil {
		t.Fatalf("write ffmpeg file: %v", err)
	}
	if err := os.WriteFile(bunPath, []byte(""), 0o755); err != nil {
		t.Fatalf("write bun file: %v", err)
	}

	command, err := BuildSubtitleCommand(context.Background(), CommandOptions{
		ExecPath: filepath.Join(tempDir, "yt-dlp"),
		Tools: toolResolverStub{
			paths: map[dependencies.DependencyName]string{
				dependencies.DependencyFFmpeg: ffmpegPath,
				dependencies.DependencyBun:    bunPath,
			},
		},
		Request: dto.CreateYTDLPJobRequest{
			URL:            "https://example.com/watch?v=1",
			SubtitleLangs:  []string{"en", "ja"},
			SubtitleAuto:   true,
			SubtitleFormat: "vtt",
		},
		OutputTemplate:   filepath.Join(tempDir, "downloads", "%(title)s.%(ext)s"),
		SubtitleTemplate: filepath.Join(tempDir, "downloads", "subtitles", "%(title)s.%(ext)s"),
	})
	if err != nil {
		t.Fatalf("build subtitle command: %v", err)
	}
	defer command.Cancel()

	argsJoined := strings.Join(command.Args, "\n")
	for _, expected := range []string{
		"--skip-download",
		"--write-auto-subs",
		"--sub-langs\nen,ja",
		"--sub-format\nvtt",
		"-o\n" + filepath.Join(tempDir, "downloads", "subtitles", "%(title)s.%(ext)s"),
		"-o\nsubtitle:" + filepath.Join(tempDir, "downloads", "subtitles", "%(title)s.%(ext)s"),
		"--ffmpeg-location\n" + filepath.Dir(ffmpegPath),
		"--no-js-runtimes",
		"--js-runtimes\nbun:" + bunPath,
	} {
		if !strings.Contains(argsJoined, expected) {
			t.Fatalf("expected subtitle command args to contain %q, got %v", expected, command.Args)
		}
	}

	pathEntry := ""
	for _, entry := range command.Cmd.Env {
		if strings.HasPrefix(entry, "PATH=") {
			pathEntry = entry
			break
		}
	}
	if pathEntry != "PATH=/usr/bin" {
		t.Fatalf("expected PATH to remain unchanged, got %q", pathEntry)
	}
	if command.PrintFilePath != "" {
		t.Fatalf("expected subtitle command not to allocate print file, got %q", command.PrintFilePath)
	}
}

func TestBuildSubtitleCommandLimitsQuickSubtitlePresetToManualSubtitles(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")

	tempDir := t.TempDir()
	command, err := BuildSubtitleCommand(context.Background(), CommandOptions{
		ExecPath: filepath.Join(tempDir, "yt-dlp"),
		Request: dto.CreateYTDLPJobRequest{
			URL:            "https://www.youtube.com/watch?v=1",
			Mode:           "quick",
			SubtitleAll:    true,
			SubtitleAuto:   true,
			WriteThumbnail: true,
		},
		OutputTemplate:   filepath.Join(tempDir, "downloads", "%(title)s.%(ext)s"),
		SubtitleTemplate: filepath.Join(tempDir, "downloads", "subtitles", "%(title)s.%(ext)s"),
	})
	if err != nil {
		t.Fatalf("build subtitle command: %v", err)
	}
	defer command.Cancel()

	argsJoined := strings.Join(command.Args, "\n")
	for _, expected := range []string{
		"--skip-download",
		"--playlist-items\n1",
		"--write-subs",
		"--sub-langs\nall,-live_chat",
		"--sub-format\nvtt/best",
	} {
		if !strings.Contains(argsJoined, expected) {
			t.Fatalf("expected quick subtitle command args to contain %q, got %v", expected, command.Args)
		}
	}
	if strings.Contains(argsJoined, "--all-subs") {
		t.Fatalf("expected quick subtitle command to avoid --all-subs, got %v", command.Args)
	}
	if strings.Contains(argsJoined, "--write-auto-subs") {
		t.Fatalf("expected quick subtitle command to avoid auto subtitles, got %v", command.Args)
	}
}

func TestBuildInfoArgsUsesFlatPlaylistDumpSingleJSON(t *testing.T) {
	t.Parallel()

	args := BuildInfoArgs(InfoOptions{
		URL:          " https://www.youtube.com/playlist?list=PL123 ",
		FlatPlaylist: true,
		ProxyURL:     "http://127.0.0.1:7890",
		CookiesPath:  "/tmp/cookies.txt",
	}, []string{"--ffmpeg-location", "/opt/ffmpeg"})
	argsJoined := strings.Join(args, "\n")
	for _, expected := range []string{
		"--cookies\n/tmp/cookies.txt",
		"--skip-download",
		"--flat-playlist",
		"--dump-single-json",
		"--ffmpeg-location\n/opt/ffmpeg",
		"--proxy\nhttp://127.0.0.1:7890",
		"https://www.youtube.com/playlist?list=PL123",
	} {
		if !strings.Contains(argsJoined, expected) {
			t.Fatalf("expected info args to contain %q, got %v", expected, args)
		}
	}
	if strings.Contains(argsJoined, "--no-playlist") || strings.Contains(argsJoined, "--dump-json") {
		t.Fatalf("expected flat playlist info args to avoid single-video flags, got %v", args)
	}
}
