package ytdlp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"xiadown/internal/domain/dependencies"
)

func hermeticYTDLPEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment)+3)
	for _, item := range environment {
		key := item
		if index := strings.IndexByte(item, '='); index >= 0 {
			key = item[:index]
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "pythonhome", "pythonpath", "pythonstartup", "pythoninspect", "pythonuserbase", "pythonnousersite", "pythonsafepath", "ytdlp_no_plugins":
			continue
		default:
			result = append(result, item)
		}
	}
	// The command-line policy is authoritative. These environment guards also
	// protect Python-based builds before yt-dlp finishes parsing its arguments.
	return append(result,
		"PYTHONNOUSERSITE=1",
		"PYTHONSAFEPATH=1",
		"YTDLP_NO_PLUGINS=1",
	)
}

// HermeticEnvironment exposes the same startup environment to yt-dlp probes
// outside this package without duplicating the isolation policy.
func HermeticEnvironment(environment []string) []string {
	return hermeticYTDLPEnvironment(environment)
}

func BuildExplicitToolArgs(ctx context.Context, resolver ToolResolver) []string {
	args := make([]string, 0, 5)
	if ffmpegDir := resolveFFmpegDir(ctx, resolver); ffmpegDir != "" {
		args = append(args, "--ffmpeg-location", ffmpegDir)
	}
	if jsRuntime := resolveJSRuntimeArg(ctx, resolver); jsRuntime != "" {
		args = append(args, "--no-js-runtimes", "--js-runtimes", jsRuntime)
	}
	return args
}

func resolveFFmpegDir(ctx context.Context, resolver ToolResolver) string {
	if resolver != nil {
		if execPath, err := resolver.ResolveExecPath(ctx, dependencies.DependencyFFmpeg); err == nil {
			dir := filepath.Dir(execPath)
			if pathExists(dir) {
				return dir
			}
		}
	}
	ffmpegName := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegName = "ffmpeg.exe"
	}
	if found, err := exec.LookPath(ffmpegName); err == nil {
		return filepath.Dir(found)
	}
	if found := findCommonBinaryPath(ffmpegName); found != "" {
		return filepath.Dir(found)
	}
	return ""
}

func resolveJSRuntimeArg(ctx context.Context, resolver ToolResolver) string {
	execPath := resolveBunExecPath(ctx, resolver)
	if execPath == "" {
		return ""
	}
	return "bun:" + execPath
}

func resolveBunExecPath(ctx context.Context, resolver ToolResolver) string {
	if resolver != nil {
		if execPath, err := resolver.ResolveExecPath(ctx, dependencies.DependencyBun); err == nil {
			trimmed := strings.TrimSpace(execPath)
			if pathExists(trimmed) {
				return trimmed
			}
		}
	}
	bunName := executableName("bun")
	if runtime.GOOS == "windows" {
		bunName = "bun.exe"
	}
	if found, err := exec.LookPath(bunName); err == nil {
		return found
	}
	return findCommonBinaryPath(bunName)
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func findCommonBinaryPath(name string) string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	candidates := []string{
		filepath.Join("/opt/homebrew/bin", name),
		filepath.Join("/usr/local/bin", name),
	}
	for _, candidate := range candidates {
		if pathExists(candidate) {
			return candidate
		}
	}
	return ""
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
