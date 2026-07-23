package ytdlp

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http/httpguts"
	"golang.org/x/net/idna"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/dependencies"
	ydlpinfr "xiadown/internal/infrastructure/ytdlp"
)

var quickManualSubtitleLanguages = []string{"all", "-live_chat"}

const (
	ytdlpAllowedFormatProtocolPattern = `^(https?|m3u8(_native)?|http_dash_segments)$`
	ytdlpAllowedFormatProtocolFilter  = `[protocol~='` + ytdlpAllowedFormatProtocolPattern + `']`
)

func hermeticYTDLPArgs() []string {
	return []string{
		"--ignore-config",
		"--no-config-locations",
		"--no-plugin-dirs",
		"--no-exec",
	}
}

// HermeticArgs applies XiaDown's process boundary to auxiliary yt-dlp
// invocations such as the dependency version probe.
func HermeticArgs(args ...string) []string {
	return append(hermeticYTDLPArgs(), args...)
}

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
	formatArg := ytdlpDefaultNetworkFormatSelector()
	quality := strings.ToLower(strings.TrimSpace(request.Quality))
	if quality == "audio" {
		formatArg = ytdlpNetworkFormatSelector("ba") + "/" + ytdlpNetworkFormatSelector("b")
	}
	formatID := strings.TrimSpace(request.FormatID)
	audioFormatID := strings.TrimSpace(request.AudioFormatID)
	if formatID != "" {
		formatArg = ytdlpExactNetworkFormatSelector(formatID)
		if audioFormatID != "" {
			formatArg += "+" + ytdlpExactNetworkFormatSelector(audioFormatID)
		}
	}
	args = append(args, "-f", formatArg)
	if quality == "bitrate" && formatID == "" {
		args = append(args, "-S", "res,br")
	}
	args = append(args, strings.TrimSpace(request.URL))
	if strings.TrimSpace(cookiesPath) != "" {
		args = append([]string{"--cookies", strings.TrimSpace(cookiesPath)}, args...)
	}
	return HermeticArgs(args...)
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
	args = append(args, strings.TrimSpace(request.URL))
	if strings.TrimSpace(cookiesPath) != "" {
		args = append([]string{"--cookies", strings.TrimSpace(cookiesPath)}, args...)
	}
	return HermeticArgs(args...)
}

func shouldLimitPlaylistToFirstItem(request dto.CreateYTDLPJobRequest) bool {
	return strings.EqualFold(strings.TrimSpace(request.Mode), "quick")
}

func ytdlpDefaultNetworkFormatSelector() string {
	return ytdlpNetworkFormatSelector("bv*") + "+" + ytdlpNetworkFormatSelector("ba") + "/" + ytdlpNetworkFormatSelector("b")
}

func ytdlpNetworkFormatSelector(selector string) string {
	return strings.TrimSpace(selector) + ytdlpAllowedFormatProtocolFilter
}

func ytdlpExactNetworkFormatSelector(formatID string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(strings.TrimSpace(formatID))
	return "all[format_id='" + escaped + "']" + ytdlpAllowedFormatProtocolFilter
}

func BuildCommand(ctx context.Context, options CommandOptions) (Command, error) {
	return buildCommand(ctx, options, false)
}

func BuildSubtitleCommand(ctx context.Context, options CommandOptions) (Command, error) {
	return buildCommand(ctx, options, true)
}

func buildCommand(ctx context.Context, options CommandOptions, subtitleOnly bool) (Command, error) {
	if err := ValidateNetworkURL(options.Request.URL); err != nil {
		return Command{}, fmt.Errorf("invalid yt-dlp URL: %w", err)
	}
	for _, formatID := range []string{options.Request.FormatID, options.Request.AudioFormatID} {
		if strings.ContainsAny(formatID, "\x00\r\n") {
			return Command{}, fmt.Errorf("invalid yt-dlp format ID")
		}
	}
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
	command.Env = hermeticYTDLPEnvironment(os.Environ())
	// An explicit XiaDown proxy is authoritative for the whole process tree,
	// not only for public-API restricted jobs. yt-dlp may spawn ffmpeg or other
	// helpers, and inherited system proxy/NO_PROXY variables would let those
	// children take a route different from the managed gateway.
	if strings.TrimSpace(options.ProxyURL) != "" {
		command.Env = restrictedProxyEnvironment(command.Env, strings.TrimSpace(options.ProxyURL))
	}
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

func restrictedProxyEnvironment(environment []string, proxyURL string) []string {
	result := make([]string, 0, len(environment)+8)
	for _, item := range environment {
		key := item
		if index := strings.IndexByte(item, '='); index >= 0 {
			key = item[:index]
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "http_proxy", "https_proxy", "all_proxy", "no_proxy":
			continue
		default:
			result = append(result, item)
		}
	}
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		result = append(result, key+"="+proxyURL)
	}
	return append(result, "NO_PROXY=", "no_proxy=")
}

func buildHeaderArgs(headers map[string]string) []string {
	if len(headers) == 0 {
		return nil
	}
	args := make([]string, 0, len(headers)*2)
	for key, value := range headers {
		normalizedKey, normalizedValue, ok := normalizeYTDLPCommandHeader(key, value)
		if !ok {
			continue
		}
		args = append(args, "--add-header", normalizedKey+": "+normalizedValue)
	}
	return args
}

func normalizeYTDLPCommandHeader(key string, value string) (string, string, bool) {
	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	trimmedValue := strings.TrimSpace(value)
	if normalizedKey == "" || trimmedValue == "" || !httpguts.ValidHeaderFieldValue(trimmedValue) {
		return "", "", false
	}
	switch normalizedKey {
	case "user-agent":
		return "User-Agent", trimmedValue, true
	case "accept":
		return "Accept", trimmedValue, true
	case "accept-language":
		return "Accept-Language", trimmedValue, true
	case "referer":
		origin, ok := normalizeYTDLPHeaderOrigin(trimmedValue)
		return "Referer", origin + "/", ok
	case "origin":
		origin, ok := normalizeYTDLPHeaderOrigin(trimmedValue)
		return "Origin", origin, ok
	default:
		return "", "", false
	}
}

func normalizeYTDLPHeaderOrigin(raw string) (string, bool) {
	if ValidateNetworkURL(raw) != nil {
		return "", false
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(parsed.Hostname())), ".")
	if parsedIP := net.ParseIP(strings.Trim(host, "[]")); parsedIP != nil {
		host = parsedIP.String()
	} else {
		host, err = idna.Lookup.ToASCII(host)
		if err != nil || host == "" {
			return "", false
		}
	}
	port := strings.TrimSpace(parsed.Port())
	if port != "" {
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return "", false
		}
		port = strconv.Itoa(parsedPort)
	}
	defaultPort := port == "" || (scheme == "https" && port == "443") || (scheme == "http" && port == "80")
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if !defaultPort {
		host = net.JoinHostPort(strings.Trim(host, "[]"), port)
	}
	return scheme + "://" + host, true
}
