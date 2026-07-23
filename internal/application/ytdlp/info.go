package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"xiadown/internal/domain/dependencies"
)

func FetchInfo(ctx context.Context, options InfoOptions) (map[string]any, error) {
	targetURL := strings.TrimSpace(options.URL)
	if targetURL == "" {
		return nil, fmt.Errorf("yt-dlp url is required")
	}
	if err := ValidateNetworkURL(targetURL); err != nil {
		return nil, fmt.Errorf("invalid yt-dlp URL: %w", err)
	}
	execPath := strings.TrimSpace(options.ExecPath)
	if execPath == "" {
		if options.Tools == nil {
			return nil, fmt.Errorf("yt-dlp exec path not resolved")
		}
		resolved, err := options.Tools.ResolveExecPath(ctx, dependencies.DependencyYTDLP)
		if err != nil {
			return nil, err
		}
		execPath = strings.TrimSpace(resolved)
	}
	if execPath == "" {
		return nil, fmt.Errorf("yt-dlp exec path not resolved")
	}
	args := BuildInfoArgs(options, BuildExplicitToolArgs(ctx, options.Tools))

	runCtx := ctx
	cancel := func() {}
	if options.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, options.Timeout)
	}
	defer cancel()

	command := exec.CommandContext(runCtx, execPath, args...)
	command.Env = hermeticYTDLPEnvironment(os.Environ())
	// FetchInfo is also allowed to spawn extractor helpers. Keep its complete
	// process tree on the same stable gateway as full downloads instead of
	// letting inherited HTTP_PROXY/NO_PROXY values select a second route.
	if proxyURL := strings.TrimSpace(options.ProxyURL); proxyURL != "" {
		command.Env = restrictedProxyEnvironment(command.Env, proxyURL)
	}
	command.WaitDelay = 2 * time.Second
	ConfigureProcessGroup(command)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		if runCtx.Err() != nil {
			return nil, runCtx.Err()
		}
		return nil, fmt.Errorf("start yt-dlp: %w", err)
	}
	stopProcessGroupKiller := StartProcessGroupKiller(
		runCtx,
		command,
		command.WaitDelay,
	)
	err := command.Wait()
	stopProcessGroupKiller()
	outputBytes := output.Bytes()
	if err != nil {
		if runCtx.Err() != nil {
			return nil, runCtx.Err()
		}
		detail := truncateOutput(outputBytes)
		if detail != "" {
			return nil, fmt.Errorf("yt-dlp failed: %s", detail)
		}
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}
	raw := strings.TrimSpace(output.String())
	if raw == "" {
		return nil, fmt.Errorf("yt-dlp info json not found")
	}
	if isYTDLPPlaceholderDetail(raw) {
		return nil, fmt.Errorf("yt-dlp info json not found: %s", raw)
	}
	lines := strings.Split(raw, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") {
			var info map[string]any
			if err := json.Unmarshal([]byte(line), &info); err == nil {
				return info, nil
			}
		}
		if idx := strings.Index(line, "{"); idx > 0 {
			var info map[string]any
			if err := json.Unmarshal([]byte(line[idx:]), &info); err == nil {
				return info, nil
			}
		}
	}
	var info map[string]any
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		detail := truncateOutput([]byte(raw))
		if detail == "" {
			return nil, fmt.Errorf("yt-dlp info json not found")
		}
		return nil, fmt.Errorf("yt-dlp info json parse failed: %s", detail)
	}
	return info, nil
}

func BuildInfoArgs(options InfoOptions, explicitToolArgs []string) []string {
	args := []string{"--skip-download"}
	if options.FlatPlaylist {
		args = append(args, "--flat-playlist", "--dump-single-json")
	} else {
		args = append(args, "--no-playlist", "--dump-json")
	}
	if len(explicitToolArgs) > 0 {
		args = append(args, explicitToolArgs...)
	}
	if strings.TrimSpace(options.ProxyURL) != "" {
		args = append(args, "--proxy", strings.TrimSpace(options.ProxyURL))
	}
	if strings.TrimSpace(options.CookiesPath) != "" {
		args = append([]string{"--cookies", strings.TrimSpace(options.CookiesPath)}, args...)
	}
	args = append(args, strings.TrimSpace(options.URL))
	return HermeticArgs(args...)
}

func truncateOutput(output []byte) string {
	const maxBytes = 2000
	if len(output) <= maxBytes {
		return strings.TrimSpace(string(output))
	}
	return strings.TrimSpace(string(output[:maxBytes])) + "..."
}

func isYTDLPPlaceholderDetail(detail string) bool {
	lower := strings.ToLower(strings.TrimSpace(detail))
	switch lower {
	case "na", "n/a", "null", "none":
		return true
	default:
		return false
	}
}
