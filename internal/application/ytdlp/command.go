package ytdlp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/dependencies"
	ydlpinfr "xiadown/internal/infrastructure/ytdlp"
)

var quickManualSubtitleLanguages = []string{"all", "-live_chat"}

func BuildArgs(request dto.CreateYTDLPJobRequest, outputTemplate string, printFilePath string, cookiesPath string, explicitToolArgs []string, proxyURL string, headers map[string]string, concurrentFragments int, streamStrategy StreamDownloadStrategy) []string {
	args := []string{
		"--no-playlist",
		"--newline",
		"--progress",
		"--progress-template",
		ProgressTemplate,
		"--print",
		"%()j",
		"--no-simulate",
		"--continue",
		"-o",
		outputTemplate,
	}
	if strings.TrimSpace(printFilePath) != "" {
		args = append(args, "--print-to-file", "after_move:filepath", printFilePath)
	} else {
		args = append(args, "--print", "after_move:filepath")
	}
	if len(explicitToolArgs) > 0 {
		args = append(args, explicitToolArgs...)
	}
	for _, extractorArg := range streamStrategy.ExtractorArgs {
		if trimmed := strings.TrimSpace(extractorArg); trimmed != "" {
			args = append(args, "--extractor-args", trimmed)
		}
	}
	if strings.TrimSpace(streamStrategy.Downloader) != "" {
		args = append(args, "--downloader", strings.TrimSpace(streamStrategy.Downloader))
	}
	for _, downloaderArg := range streamStrategy.DownloaderArgs {
		if trimmed := strings.TrimSpace(downloaderArg); trimmed != "" {
			args = append(args, "--downloader-args", trimmed)
		}
	}
	if shouldLimitPlaylistToFirstItem(request) {
		args = append(args, "--playlist-items", "1")
	}
	if strings.TrimSpace(proxyURL) != "" {
		args = append(args, "--proxy", proxyURL)
	}
	if streamStrategy.DisableConcurrentFragments {
		concurrentFragments = 1
	}
	if concurrentFragments > 1 {
		args = append(args, "--concurrent-fragments", fmt.Sprintf("%d", concurrentFragments))
	}
	args = append(args, buildHeaderArgs(headers)...)
	formatArg := ""
	quality := strings.ToLower(strings.TrimSpace(request.Quality))
	if quality == "audio" {
		formatArg = "ba/b"
	}
	formatID := strings.TrimSpace(request.FormatID)
	audioFormatID := strings.TrimSpace(request.AudioFormatID)
	if formatID != "" {
		formatArg = formatID
		if audioFormatID != "" {
			formatArg = formatID + "+" + audioFormatID
		}
	}
	if formatArg != "" {
		args = append(args, "-f", formatArg)
	} else if quality == "bitrate" {
		args = append(args, "-S", "res,br")
	}
	args = append(args, request.URL)
	if strings.TrimSpace(cookiesPath) != "" {
		args = append([]string{"--cookies", strings.TrimSpace(cookiesPath)}, args...)
	}
	return args
}

func BuildSubtitleArgs(request dto.CreateYTDLPJobRequest, outputTemplate string, subtitleTemplate string, cookiesPath string, explicitToolArgs []string, proxyURL string, headers map[string]string) []string {
	resolvedOutputTemplate := outputTemplate
	if strings.TrimSpace(subtitleTemplate) != "" {
		resolvedOutputTemplate = subtitleTemplate
	}
	args := []string{
		"--no-playlist",
		"--newline",
		"--progress",
		"--progress-template",
		ProgressTemplate,
		"--print",
		"%()j",
		"--no-simulate",
		"--skip-download",
		"-o",
		resolvedOutputTemplate,
	}
	if strings.TrimSpace(subtitleTemplate) != "" {
		args = append(args, "-o", "subtitle:"+subtitleTemplate)
	}
	if len(explicitToolArgs) > 0 {
		args = append(args, explicitToolArgs...)
	}
	if shouldLimitPlaylistToFirstItem(request) {
		args = append(args, "--playlist-items", "1")
	}
	if strings.TrimSpace(proxyURL) != "" {
		args = append(args, "--proxy", proxyURL)
	}
	args = append(args, buildHeaderArgs(headers)...)
	quickSubtitlePreset := request.SubtitleAll && strings.EqualFold(strings.TrimSpace(request.Mode), "quick")
	if request.SubtitleAll {
		args = append(args, "--write-subs")
		if request.SubtitleAuto && !quickSubtitlePreset {
			args = append(args, "--write-auto-subs")
		}
		if quickSubtitlePreset {
			args = append(args, "--sub-langs", strings.Join(quickManualSubtitleLanguages, ","))
		} else {
			args = append(args, "--all-subs")
		}
	} else if len(request.SubtitleLangs) > 0 {
		if request.SubtitleAuto {
			args = append(args, "--write-auto-subs")
		} else {
			args = append(args, "--write-subs")
		}
		args = append(args, "--sub-langs", strings.Join(request.SubtitleLangs, ","))
	}
	subtitleFormat := strings.TrimSpace(request.SubtitleFormat)
	if subtitleFormat == "" && quickSubtitlePreset {
		subtitleFormat = "vtt/best"
	}
	if subtitleFormat != "" {
		args = append(args, "--sub-format", subtitleFormat)
	}
	args = append(args, request.URL)
	if strings.TrimSpace(cookiesPath) != "" {
		args = append([]string{"--cookies", strings.TrimSpace(cookiesPath)}, args...)
	}
	return args
}

func shouldLimitPlaylistToFirstItem(request dto.CreateYTDLPJobRequest) bool {
	return strings.EqualFold(strings.TrimSpace(request.Mode), "quick")
}

func BuildCommand(ctx context.Context, options CommandOptions) (Command, error) {
	return buildCommand(ctx, options, false)
}

func BuildSubtitleCommand(ctx context.Context, options CommandOptions) (Command, error) {
	return buildCommand(ctx, options, true)
}

func buildCommand(ctx context.Context, options CommandOptions, subtitleOnly bool) (Command, error) {
	execPath := strings.TrimSpace(options.ExecPath)
	if execPath == "" {
		if options.Tools == nil {
			return Command{}, fmt.Errorf("yt-dlp exec path not resolved")
		}
		resolved, err := options.Tools.ResolveExecPath(ctx, dependencies.DependencyYTDLP)
		if err != nil {
			return Command{}, err
		}
		execPath = strings.TrimSpace(resolved)
	}
	if execPath == "" {
		return Command{}, fmt.Errorf("yt-dlp exec path not resolved")
	}

	printFilePath := ""
	cleanup := func() {}
	if printFile, err := os.CreateTemp("", "xiadown-ytdlp-output-*.txt"); err == nil {
		printFilePath = printFile.Name()
		_ = printFile.Close()
		cleanup = func() { _ = os.Remove(printFilePath) }
	}

	explicitToolArgs := BuildExplicitToolArgs(ctx, options.Tools)
	var args []string
	if subtitleOnly {
		args = BuildSubtitleArgs(
			options.Request,
			options.OutputTemplate,
			options.SubtitleTemplate,
			options.CookiesPath,
			explicitToolArgs,
			options.ProxyURL,
			options.Headers,
		)
		printFilePath = ""
		cleanup()
		cleanup = func() {}
	} else {
		args = BuildArgs(
			options.Request,
			options.OutputTemplate,
			printFilePath,
			options.CookiesPath,
			explicitToolArgs,
			options.ProxyURL,
			options.Headers,
			options.ConcurrentFragments,
			options.StreamStrategy,
		)
	}

	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Hour
	}
	if !subtitleOnly && options.ConcurrentFragments > 1 {
		ensureYTDLPFileLimit(minYTDLPFileLimit)
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	command := exec.CommandContext(runCtx, execPath, args...)
	command.Env = os.Environ()
	command.WaitDelay = 2 * time.Second
	ConfigureProcessGroup(command)
	sanitizedArgs := ydlpinfr.SanitizeArgs(args)

	return Command{
		Cmd:           command,
		Args:          args,
		SanitizedArgs: sanitizedArgs,
		PrintFilePath: printFilePath,
		Ctx:           runCtx,
		Cancel:        cancel,
		Cleanup:       cleanup,
	}, nil
}

func buildHeaderArgs(headers map[string]string) []string {
	if len(headers) == 0 {
		return nil
	}
	args := make([]string, 0, len(headers)*2)
	for key, value := range headers {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" || ytdlpHeaderForbidden(trimmedKey) {
			continue
		}
		args = append(args, "--add-header", trimmedKey+": "+trimmedValue)
	}
	return args
}

func ytdlpHeaderForbidden(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case ":authority", ":method", ":path", ":scheme",
		"accept-encoding", "connection", "content-length", "host",
		"if-modified-since", "if-none-match", "keep-alive",
		"proxy-connection", "range",
		"sec-ch-ua", "sec-ch-ua-arch", "sec-ch-ua-bitness",
		"sec-ch-ua-full-version", "sec-ch-ua-full-version-list",
		"sec-ch-ua-mobile", "sec-ch-ua-model", "sec-ch-ua-platform",
		"sec-ch-ua-platform-version", "sec-fetch-dest", "sec-fetch-mode",
		"sec-fetch-site", "sec-fetch-user", "transfer-encoding",
		"x-forwarded-for", "x-real-ip":
		return true
	default:
		return false
	}
}
