package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type transcodeTestOperationRepo struct {
	saved []library.LibraryOperation
}

func (repo *transcodeTestOperationRepo) List(_ context.Context) ([]library.LibraryOperation, error) {
	return nil, nil
}

func (repo *transcodeTestOperationRepo) ListByLibraryID(_ context.Context, _ string) ([]library.LibraryOperation, error) {
	return nil, nil
}

func (repo *transcodeTestOperationRepo) Get(_ context.Context, id string) (library.LibraryOperation, error) {
	for index := len(repo.saved) - 1; index >= 0; index-- {
		if repo.saved[index].ID == id {
			return repo.saved[index], nil
		}
	}
	return library.LibraryOperation{}, library.ErrOperationNotFound
}

func (repo *transcodeTestOperationRepo) Save(_ context.Context, item library.LibraryOperation) error {
	repo.saved = append(repo.saved, item)
	return nil
}

func (repo *transcodeTestOperationRepo) Delete(_ context.Context, _ string) error {
	return nil
}

func TestBuildFFmpegTranscodeArgsVideoUsesPrimaryStreamsAndFaststart(t *testing.T) {
	plan := transcodePlan{
		request: dto.CreateTranscodeJobRequest{
			Format:     "mp4",
			VideoCodec: "copy",
			AudioCodec: "copy",
			Preset:     "medium",
			CRF:        23,
			Scale:      "1080p",
		},
		outputType: library.TranscodeOutputVideo,
	}

	args, err := buildFFmpegTranscodeArgs(plan, "/tmp/input.mp4", "/tmp/output.mp4")
	if err != nil {
		t.Fatalf("buildFFmpegTranscodeArgs returned error: %v", err)
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-vf scale=w=1920:h=1080:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=1920:1080:(ow-iw)/2:(oh-ih)/2") {
		t.Fatalf("expected scale filter in args, got %q", joined)
	}
	if !strings.Contains(joined, "-map 0:v:0? -map 0:a:0?") {
		t.Fatalf("expected explicit primary stream mapping, got %q", joined)
	}
	if !strings.Contains(joined, "-c:v libx264") {
		t.Fatalf("expected filtered copy job to fall back to libx264, got %q", joined)
	}
	if !strings.Contains(joined, "-c:a copy") {
		t.Fatalf("expected audio copy to remain unchanged, got %q", joined)
	}
	if !strings.Contains(joined, "-movflags +faststart") {
		t.Fatalf("expected mp4 output to use faststart, got %q", joined)
	}
}

func TestBuildFFmpegTranscodeArgsAudioOutputDisablesVideo(t *testing.T) {
	plan := transcodePlan{
		request: dto.CreateTranscodeJobRequest{
			Format: "mp3",
		},
		outputType: library.TranscodeOutputAudio,
	}

	args, err := buildFFmpegTranscodeArgs(plan, "/tmp/input.mp4", "/tmp/output.mp3")
	if err != nil {
		t.Fatalf("buildFFmpegTranscodeArgs returned error: %v", err)
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-map 0:a:0? -vn") {
		t.Fatalf("expected audio-only transcode to disable video, got %q", joined)
	}
	if !strings.Contains(joined, "-c:a libmp3lame") {
		t.Fatalf("expected default mp3 audio codec, got %q", joined)
	}
}

func TestFFmpegProgressReporterDoesNotResurrectCanceledOperation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 10, 10, 0, 0, time.UTC)
	persistedOperation := library.LibraryOperation{
		ID:         "op-1",
		LibraryID:  "lib-1",
		Kind:       "transcode",
		Status:     library.OperationStatusCanceled,
		OutputJSON: "{}",
		CreatedAt:  now,
		FinishedAt: &now,
	}
	operationRepo := &transcodeTestOperationRepo{saved: []library.LibraryOperation{persistedOperation}}
	staleOperation := persistedOperation
	staleOperation.Status = library.OperationStatusRunning
	staleOperation.FinishedAt = nil
	service := &LibraryService{
		operations: operationRepo,
		nowFunc: func() time.Time {
			return now
		},
	}
	reporter := newFFmpegProgressReporter(service, &staleOperation, 1000)
	reporter.currentMs = 500

	reporter.persistLocked(false)

	if len(operationRepo.saved) != 1 {
		t.Fatalf("expected no progress save after cancellation, got %d saves", len(operationRepo.saved)-1)
	}
	if operationRepo.saved[0].Status != library.OperationStatusCanceled {
		t.Fatalf("expected canceled operation to stay canceled, got %q", operationRepo.saved[0].Status)
	}
}

func TestBuildFFmpegTranscodeArgsExpandedAudioContainersUseExpectedCodec(t *testing.T) {
	testCases := []struct {
		name          string
		format        string
		audioCodec    string
		expectedCodec string
	}{
		{name: "m4a aac", format: "m4a", audioCodec: "aac", expectedCodec: "-c:a aac"},
		{name: "ogg opus", format: "ogg", audioCodec: "opus", expectedCodec: "-c:a libopus"},
		{name: "flac lossless", format: "flac", audioCodec: "flac", expectedCodec: "-c:a flac"},
		{name: "wav pcm", format: "wav", audioCodec: "pcm", expectedCodec: "-c:a pcm_s16le"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			plan := transcodePlan{
				request: dto.CreateTranscodeJobRequest{
					Format:     testCase.format,
					AudioCodec: testCase.audioCodec,
				},
				outputType: library.TranscodeOutputAudio,
			}

			args, err := buildFFmpegTranscodeArgs(plan, "/tmp/input.mp4", "/tmp/output."+testCase.format)
			if err != nil {
				t.Fatalf("buildFFmpegTranscodeArgs returned error: %v", err)
			}

			joined := strings.Join(args, " ")
			if !strings.Contains(joined, testCase.expectedCodec) {
				t.Fatalf("expected %q in args, got %q", testCase.expectedCodec, joined)
			}
		})
	}
}

func TestBuildTranscodeOperationOutputIncludesCoreFields(t *testing.T) {
	request := dto.CreateTranscodeJobRequest{
		DeleteSourceFileAfterTranscode: true,
	}

	outputJSON := buildTranscodeOperationOutput(request, "completed", "/tmp/output.mp4")

	payload := map[string]any{}
	if err := json.Unmarshal([]byte(outputJSON), &payload); err != nil {
		t.Fatalf("buildTranscodeOperationOutput returned invalid json: %v", err)
	}
	if got := payload["status"]; got != "completed" {
		t.Fatalf("unexpected status: %#v", got)
	}
	if got := payload["outputPath"]; got != "/tmp/output.mp4" {
		t.Fatalf("unexpected outputPath: %#v", got)
	}
	if got := payload["deleteSourceFileAfterTranscode"]; got != true {
		t.Fatalf("unexpected deleteSourceFileAfterTranscode: %#v", got)
	}
}

func TestEnsureManagedOutputParentDirCreatesDirectory(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "libraries", "lib-1", "video.mkv")

	if err := ensureManagedOutputParentDir(outputPath); err != nil {
		t.Fatalf("ensureManagedOutputParentDir returned error: %v", err)
	}

	info, err := os.Stat(filepath.Dir(outputPath))
	if err != nil {
		t.Fatalf("expected parent directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected parent path to be a directory")
	}
}

func TestWithFFmpegProgressArgsInsertsFlagsBeforeOutputPath(t *testing.T) {
	args := []string{"-y", "-i", "/tmp/input.mp4", "-c:v", "libx264", "/tmp/output.mp4"}

	got := withFFmpegProgressArgs(args)
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "-nostdin -progress pipe:1 -nostats /tmp/output.mp4") {
		t.Fatalf("expected progress flags before output path, got %q", joined)
	}
	if strings.Count(joined, "-progress") != 1 {
		t.Fatalf("expected exactly one progress flag, got %q", joined)
	}
}

func TestRunFFmpegCommandWithProgressDrainsOutputBeforeWait(t *testing.T) {
	t.Setenv("XIADOWN_FFMPEG_HELPER", "1")

	service := &LibraryService{}
	output, err := service.runFFmpegCommandWithProgress(
		context.Background(),
		nil,
		os.Args[0],
		[]string{"-test.run=TestFFmpegCommandHelper", "--", "output.m4a"},
		"",
		1000,
	)
	if err != nil {
		t.Fatalf("expected helper command to succeed, got %v; output: %s", err, output)
	}
	if !strings.Contains(output, "ffmpeg diagnostic line 9999") {
		t.Fatalf("expected stderr output to be fully drained, got tail: %q", output)
	}
}

func TestFFmpegCommandHelper(t *testing.T) {
	if os.Getenv("XIADOWN_FFMPEG_HELPER") != "1" {
		return
	}
	for index := 0; index < 10000; index++ {
		fmt.Fprintf(os.Stderr, "ffmpeg diagnostic line %d\n", index)
	}
	fmt.Fprintln(os.Stdout, "out_time=00:00:01.000000")
	fmt.Fprintln(os.Stdout, "speed=1.0x")
	fmt.Fprintln(os.Stdout, "progress=end")
	os.Exit(0)
}

func TestBuildFFmpegTranscodeArgsVP9UsesLibvpxBestPracticeFlags(t *testing.T) {
	plan := transcodePlan{
		request: dto.CreateTranscodeJobRequest{
			Format:      "webm",
			VideoCodec:  "vp9",
			AudioCodec:  "opus",
			QualityMode: "crf",
			CRF:         defaultVP9VideoCRF,
		},
		outputType: library.TranscodeOutputVideo,
	}

	args, err := buildFFmpegTranscodeArgs(plan, "/tmp/input.mp4", "/tmp/output.webm")
	if err != nil {
		t.Fatalf("buildFFmpegTranscodeArgs returned error: %v", err)
	}

	joined := strings.Join(args, " ")
	if strings.Contains(joined, "-preset ") {
		t.Fatalf("did not expect libvpx-vp9 to receive x264/x265 preset flags, got %q", joined)
	}
	if !strings.Contains(joined, "-deadline good -cpu-used 0 -row-mt 1") {
		t.Fatalf("expected libvpx-vp9 best-practice quality flags, got %q", joined)
	}
	if !strings.Contains(joined, "-b:v 0 -crf 20") {
		t.Fatalf("expected VP9 CRF mode to disable target bitrate, got %q", joined)
	}
}

func TestBuildFFmpegTranscodeArgsLosslessAudioSkipsBitrateFlags(t *testing.T) {
	plan := transcodePlan{
		request: dto.CreateTranscodeJobRequest{
			Format:           "flac",
			AudioCodec:       "flac",
			AudioBitrateKbps: 320,
		},
		outputType: library.TranscodeOutputAudio,
	}

	args, err := buildFFmpegTranscodeArgs(plan, "/tmp/input.wav", "/tmp/output.flac")
	if err != nil {
		t.Fatalf("buildFFmpegTranscodeArgs returned error: %v", err)
	}

	joined := strings.Join(args, " ")
	if strings.Contains(joined, "-b:a") {
		t.Fatalf("did not expect bitrate flags for lossless audio, got %q", joined)
	}
}

func TestFFmpegProgressReporterPublishesPercentFromStructuredOutput(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	operation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID:          "op-transcode",
		LibraryID:   "lib-1",
		Kind:        "transcode",
		Status:      string(library.OperationStatusRunning),
		DisplayName: "demo",
		InputJSON:   "{}",
		OutputJSON:  "{}",
		CreatedAt:   &now,
	})
	if err != nil {
		t.Fatalf("new operation: %v", err)
	}

	repo := &transcodeTestOperationRepo{}
	service := &LibraryService{
		operations: repo,
		nowFunc:    func() time.Time { return now },
	}
	reporter := newFFmpegProgressReporter(service, &operation, 10000)

	reporter.HandleLine("out_time=00:00:05.000000")
	reporter.HandleLine("speed=1.5x")
	reporter.HandleLine("progress=continue")

	if len(repo.saved) != 1 {
		t.Fatalf("expected one progress save, got %d", len(repo.saved))
	}
	progress := repo.saved[0].Progress
	if progress == nil || progress.Percent == nil || *progress.Percent != 50 {
		t.Fatalf("expected 50%% progress, got %#v", progress)
	}
	if progress.Current == nil || *progress.Current != 5000 {
		t.Fatalf("expected current progress 5000ms, got %#v", progress)
	}
	if progress.Total == nil || *progress.Total != 10000 {
		t.Fatalf("expected total progress 10000ms, got %#v", progress)
	}
	if progress.Speed != "1.5x" {
		t.Fatalf("expected speed 1.5x, got %#v", progress)
	}
	if progress.Stage != progressText("library.progress.transcoding") {
		t.Fatalf("expected stage key %q, got %#v", progressText("library.progress.transcoding"), progress)
	}
}

func TestParseFFmpegProgressMillisSupportsTimestampAndMicroseconds(t *testing.T) {
	if got, ok := parseFFmpegProgressMillis("out_time", "00:00:04.500000"); !ok || got != 4500 {
		t.Fatalf("expected timestamp progress 4500ms, got %d, %v", got, ok)
	}
	if got, ok := parseFFmpegProgressMillis("out_time_ms", "4500000"); !ok || got != 4500 {
		t.Fatalf("expected microsecond progress 4500ms, got %d, %v", got, ok)
	}
	if got, ok := parseFFmpegProgressMillis("out_time_us", "7200000"); !ok || got != 7200 {
		t.Fatalf("expected microsecond progress 7200ms, got %d, %v", got, ok)
	}
}

func TestFFmpegIsHardwareVideoCodec(t *testing.T) {
	if !ffmpegIsHardwareVideoCodec("h264_nvenc") {
		t.Fatalf("expected nvenc to be recognized as hardware codec")
	}
	if ffmpegIsHardwareVideoCodec("libx264") {
		t.Fatalf("did not expect libx264 to be recognized as hardware codec")
	}
}
