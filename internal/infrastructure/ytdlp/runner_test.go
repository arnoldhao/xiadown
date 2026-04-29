package ytdlp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunRecordsSubtitlePathsFromStderr(t *testing.T) {
	t.Parallel()

	if os.Getenv("XIADOWN_YTDLP_RUNNER_HELPER") == "stderr-subtitle" {
		fmt.Fprintf(os.Stderr, "[info] Writing video subtitles to: %s\n", os.Getenv("XIADOWN_YTDLP_SUBTITLE_PATH"))
		return
	}

	subtitlePath := filepath.Join(t.TempDir(), "sample.en.vtt")
	command := exec.Command(os.Args[0], "-test.run=^TestRunRecordsSubtitlePathsFromStderr$")
	command.Env = append(os.Environ(),
		"XIADOWN_YTDLP_RUNNER_HELPER=stderr-subtitle",
		"XIADOWN_YTDLP_SUBTITLE_PATH="+subtitlePath,
	)

	result, err := Run(RunOptions{Command: command})
	if err != nil {
		t.Fatalf("run helper command: %v", err)
	}
	if len(result.SubtitleLogPaths) != 1 || result.SubtitleLogPaths[0] != subtitlePath {
		t.Fatalf("expected stderr subtitle path %q, got %#v", subtitlePath, result.SubtitleLogPaths)
	}
}

func TestRunPublishesOutputPathWhenRecorded(t *testing.T) {
	t.Parallel()

	if os.Getenv("XIADOWN_YTDLP_RUNNER_HELPER") == "download-destination" {
		fmt.Fprintf(os.Stderr, "[download] Destination: %s\n", os.Getenv("XIADOWN_YTDLP_OUTPUT_PATH"))
		return
	}

	outputPath := filepath.Join(t.TempDir(), "sample.f315.webm")
	command := exec.Command(os.Args[0], "-test.run=^TestRunPublishesOutputPathWhenRecorded$")
	command.Env = append(os.Environ(),
		"XIADOWN_YTDLP_RUNNER_HELPER=download-destination",
		"XIADOWN_YTDLP_OUTPUT_PATH="+outputPath,
	)
	publishedPaths := make([]string, 0, 1)

	result, err := Run(RunOptions{
		Command: command,
		OutputPath: func(path string) {
			publishedPaths = append(publishedPaths, path)
		},
	})
	if err != nil {
		t.Fatalf("run helper command: %v", err)
	}
	if len(result.OutputPaths) != 1 || result.OutputPaths[0] != outputPath {
		t.Fatalf("expected result output path %q, got %#v", outputPath, result.OutputPaths)
	}
	if len(publishedPaths) != 1 || publishedPaths[0] != outputPath {
		t.Fatalf("expected published output path %q, got %#v", outputPath, publishedPaths)
	}
}
