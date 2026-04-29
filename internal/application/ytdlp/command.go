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

func BuildArgs(request dto.CreateYTDLPJobRequest, outputTemplate string, printFilePath string, cookiesPath string, explicitToolArgs []string, proxyURL string) []string {
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
	if strings.TrimSpace(proxyURL) != "" {
		args = append(args, "--proxy", proxyURL)
	}
	formatArg := ""
	if strings.EqualFold(strings.TrimSpace(request.Quality), "audio") {
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
	}
	args = append(args, request.URL)
	if strings.TrimSpace(cookiesPath) != "" {
		args = append([]string{"--cookies", strings.TrimSpace(cookiesPath)}, args...)
	}
	return args
}

func BuildSubtitleArgs(request dto.CreateYTDLPJobRequest, outputTemplate string, subtitleTemplate string, cookiesPath string, explicitToolArgs []string, proxyURL string) []string {
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
	if strings.TrimSpace(proxyURL) != "" {
		args = append(args, "--proxy", proxyURL)
	}
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
		)
	}

	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Hour
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
