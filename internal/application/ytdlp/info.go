package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"xiadown/internal/domain/dependencies"
)

func FetchInfo(ctx context.Context, options InfoOptions) (map[string]any, error) {
	targetURL := strings.TrimSpace(options.URL)
	if targetURL == "" {
		return nil, fmt.Errorf("yt-dlp url is required")
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
	args := []string{"--no-playlist", "--skip-download", "--dump-json"}
	if explicitToolArgs := BuildExplicitToolArgs(ctx, options.Tools); len(explicitToolArgs) > 0 {
		args = append(args, explicitToolArgs...)
	}
	if strings.TrimSpace(options.ProxyURL) != "" {
		args = append(args, "--proxy", strings.TrimSpace(options.ProxyURL))
	}
	if strings.TrimSpace(options.CookiesPath) != "" {
		args = append([]string{"--cookies", strings.TrimSpace(options.CookiesPath)}, args...)
	}
	args = append(args, targetURL)

	command := exec.CommandContext(ctx, execPath, args...)
	ConfigureProcessGroup(command)
	output, err := command.CombinedOutput()
	if err != nil {
		detail := truncateOutput(output)
		if detail != "" {
			return nil, fmt.Errorf("yt-dlp failed: %s", detail)
		}
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}
	raw := strings.TrimSpace(string(output))
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
