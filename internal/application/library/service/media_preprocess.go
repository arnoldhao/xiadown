package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"xiadown/internal/domain/dependencies"
	"xiadown/internal/domain/library"
)

type mediaProbe struct {
	Format           string
	Title            string
	Artist           string
	Album            string
	AlbumArtist      string
	Genre            string
	TrackNumber      int
	DiscNumber       int
	Year             int
	Codec            string
	VideoCodec       string
	AudioCodec       string
	HasVideo         bool
	HasAudio         bool
	AttachedPicCount int
	SubtitleStreams  []mediaProbeSubtitleStream
	StreamInfo       bool
	DurationMs       int64
	Width            int
	Height           int
	FrameRate        float64
	BitrateKbps      int
	VideoBitrateKbps int
	AudioBitrateKbps int
	Channels         int
	SizeBytes        int64
	// ContentIdentitySignature is populated only by the Listen Local index.
	// General media probes deliberately avoid its additional bounded packet
	// sampling pass.
	ContentIdentitySignature string
	DPI                      int
}

type mediaProbeSubtitleStream struct {
	Index    int
	Codec    string
	Language string
}

type ffprobePayload struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	Index        int                `json:"index"`
	CodecType    string             `json:"codec_type"`
	CodecName    string             `json:"codec_name"`
	Width        int                `json:"width"`
	Height       int                `json:"height"`
	Channels     int                `json:"channels"`
	AvgFrameRate string             `json:"avg_frame_rate"`
	RFrameRate   string             `json:"r_frame_rate"`
	BitRate      string             `json:"bit_rate"`
	Disposition  ffprobeDisposition `json:"disposition"`
	Tags         map[string]string  `json:"tags"`
}

type ffprobeDisposition struct {
	AttachedPic int `json:"attached_pic"`
}

type ffprobeFormat struct {
	FormatName string            `json:"format_name"`
	Duration   string            `json:"duration"`
	Size       string            `json:"size"`
	BitRate    string            `json:"bit_rate"`
	Tags       map[string]string `json:"tags"`
}

func (probe mediaProbe) toMediaInfo() library.MediaInfo {
	result := library.MediaInfo{
		Format:     probe.Format,
		Codec:      probe.Codec,
		VideoCodec: probe.VideoCodec,
		AudioCodec: probe.AudioCodec,
	}
	if probe.DurationMs > 0 {
		value := probe.DurationMs
		result.DurationMs = &value
	}
	if probe.Width > 0 {
		value := probe.Width
		result.Width = &value
	}
	if probe.Height > 0 {
		value := probe.Height
		result.Height = &value
	}
	if probe.FrameRate > 0 {
		value := probe.FrameRate
		result.FrameRate = &value
	}
	if probe.BitrateKbps > 0 {
		value := probe.BitrateKbps
		result.BitrateKbps = &value
	}
	if probe.VideoBitrateKbps > 0 {
		value := probe.VideoBitrateKbps
		result.VideoBitrateKbps = &value
	}
	if probe.AudioBitrateKbps > 0 {
		value := probe.AudioBitrateKbps
		result.AudioBitrateKbps = &value
	}
	if probe.Channels > 0 {
		value := probe.Channels
		result.Channels = &value
	}
	if probe.SizeBytes > 0 {
		value := probe.SizeBytes
		result.SizeBytes = &value
	}
	if probe.DPI > 0 {
		value := probe.DPI
		result.DPI = &value
	}
	return result
}

func libraryBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads", "xiadown"), nil
}

func (service *LibraryService) resolveDownloadDirectory(ctx context.Context) (string, error) {
	if service != nil && service.settings != nil {
		settings, err := service.settings.GetSettings(ctx)
		if err == nil {
			if trimmed := strings.TrimSpace(settings.DownloadDirectory); trimmed != "" {
				return trimmed, nil
			}
		}
	}
	baseDir, err := libraryBaseDir()
	if err != nil {
		return "", err
	}
	return baseDir, nil
}

func (service *LibraryService) resolveInputPath(ctx context.Context, rawPath string, source string, allowTemp bool) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", fmt.Errorf("path is required")
	}
	if service != nil && service.isAgentSource(source) {
		return service.resolveAgentPath(ctx, trimmed, allowTemp, true)
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func (service *LibraryService) probeLocalMedia(ctx context.Context, path string) mediaProbe {
	fallback := probeLocalMedia(path)
	if service == nil {
		return fallback
	}
	if isSubtitleFormat(fallback.Format) {
		return fallback
	}
	probe, err := service.ffprobeLocalMedia(ctx, path)
	if err != nil {
		return fallback
	}
	return mergeMediaProbe(fallback, probe)
}

func (service *LibraryService) probeRequiredMedia(ctx context.Context, path string) (mediaProbe, error) {
	fallback := probeLocalMedia(path)
	if isSubtitleFormat(fallback.Format) {
		return fallback, nil
	}
	if service == nil || service.tools == nil {
		return mediaProbe{}, fmt.Errorf("ffmpeg is not installed")
	}
	probe, err := service.ffprobeLocalMedia(ctx, path)
	if err != nil {
		return mediaProbe{}, err
	}
	return mergeMediaProbe(fallback, probe), nil
}

func probeLocalMedia(path string) mediaProbe {
	resolved := strings.TrimSpace(path)
	probe := mediaProbe{Format: normalizeTranscodeFormat(filepath.Ext(resolved))}
	if info, err := os.Stat(resolved); err == nil {
		probe.SizeBytes = info.Size()
	}
	switch probe.Format {
	case "mp3", "m4a", "wav", "flac", "aac", "opus", "ogg":
		probe.AudioCodec = probe.Format
		probe.Codec = probe.Format
		probe.HasAudio = true
	case "srt", "vtt", "ass", "ssa", "ttml", "xml":
		probe.Codec = probe.Format
	case "jpg", "jpeg", "png", "webp", "gif", "avif", "bmp", "ico", "svg",
		"m3u8", "mpd", "f4m", "ism",
		"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx",
		"woff", "woff2", "ttf", "otf", "eot",
		"json",
		"zip", "rar", "7z", "dmg", "pkg", "exe":
		// Non-media files still need format and size metadata, but codec fields
		// would be misleading in completed/file detail views.
	case "avi", "flv", "m2ts", "m4v", "mkv", "mov", "mp4", "mpeg", "mpg", "mts", "ogv", "ts", "webm", "wmv":
		probe.VideoCodec = probe.Format
		probe.AudioCodec = "aac"
		probe.Codec = probe.Format
		probe.HasVideo = true
		probe.HasAudio = true
	}
	return probe
}

func (service *LibraryService) ffprobeLocalMedia(ctx context.Context, path string) (mediaProbe, error) {
	execPath, err := resolveFFprobeExecPath(ctx, service.tools)
	if err != nil {
		return mediaProbe{}, err
	}
	probeCtx, cancel := withLocalMediaProbeTimeout(ctx)
	defer cancel()
	args := []string{
		"-v", "error",
		"-print_format", "json",
		"-show_entries", "stream=index,codec_type,codec_name,width,height,channels,avg_frame_rate,r_frame_rate,bit_rate,disposition,tags:format=format_name,duration,size,bit_rate:format_tags",
		"-show_streams",
		"-show_format",
	}
	args = appendLocalMediaFFprobeInput(args, path)
	command := exec.CommandContext(probeCtx, execPath, args...)
	configureLocalMediaToolCommand(command)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return mediaProbe{}, fmt.Errorf("ffprobe failed: %s", detail)
	}
	return parseFFprobeMediaProbe(output, path)
}

func parseFFprobeMediaProbe(output []byte, path string) (mediaProbe, error) {
	payload := ffprobePayload{}
	if err := json.Unmarshal(output, &payload); err != nil {
		return mediaProbe{}, err
	}
	result := mediaProbe{
		Format:     normalizeFFprobeFormat(payload.Format.FormatName, path),
		StreamInfo: true,
	}
	applyMediaProbeTags(&result, payload.Format.Tags)
	if result.SizeBytes == 0 {
		result.SizeBytes = parseFFprobeSize(payload.Format.Size)
	}
	if result.DurationMs == 0 {
		result.DurationMs = parseFFprobeDurationMillis(payload.Format.Duration)
	}
	if result.BitrateKbps == 0 {
		result.BitrateKbps = parseFFprobeBitrateKbps(payload.Format.BitRate)
	}
	for _, stream := range payload.Streams {
		switch strings.ToLower(strings.TrimSpace(stream.CodecType)) {
		case "video":
			if stream.Disposition.AttachedPic > 0 {
				result.AttachedPicCount++
				continue
			}
			result.HasVideo = true
			if result.VideoCodec == "" {
				result.VideoCodec = normalizeTranscodeFormat(stream.CodecName)
			}
			if result.Width == 0 && stream.Width > 0 {
				result.Width = stream.Width
			}
			if result.Height == 0 && stream.Height > 0 {
				result.Height = stream.Height
			}
			if result.FrameRate == 0 {
				result.FrameRate = parseFFprobeFrameRate(firstNonEmpty(stream.AvgFrameRate, stream.RFrameRate))
			}
			if result.BitrateKbps == 0 {
				result.BitrateKbps = parseFFprobeBitrateKbps(stream.BitRate)
			}
			if result.VideoBitrateKbps == 0 {
				result.VideoBitrateKbps = parseFFprobeBitrateKbps(stream.BitRate)
			}
		case "audio":
			result.HasAudio = true
			applyMediaProbeTags(&result, stream.Tags)
			if result.AudioCodec == "" {
				result.AudioCodec = normalizeTranscodeFormat(stream.CodecName)
			}
			if result.Channels == 0 && stream.Channels > 0 {
				result.Channels = stream.Channels
			}
			if result.BitrateKbps == 0 {
				result.BitrateKbps = parseFFprobeBitrateKbps(stream.BitRate)
			}
			if result.AudioBitrateKbps == 0 {
				result.AudioBitrateKbps = parseFFprobeBitrateKbps(stream.BitRate)
			}
		case "subtitle":
			result.SubtitleStreams = append(result.SubtitleStreams, mediaProbeSubtitleStream{
				Index:    stream.Index,
				Codec:    normalizeTranscodeFormat(stream.CodecName),
				Language: strings.TrimSpace(stream.Tags["language"]),
			})
		}
	}
	if result.Codec == "" {
		result.Codec = firstNonEmpty(result.VideoCodec, result.AudioCodec)
	}
	if result.SizeBytes == 0 {
		if info, err := os.Stat(strings.TrimSpace(path)); err == nil {
			result.SizeBytes = info.Size()
		}
	}
	return result, nil
}

func applyMediaProbeTags(probe *mediaProbe, tags map[string]string) {
	if probe == nil || len(tags) == 0 {
		return
	}
	normalized := make(map[string]string, len(tags))
	for key, value := range tags {
		key = strings.ToLower(strings.TrimSpace(key))
		key = strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_")
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			normalized[key] = value
		}
	}
	if probe.Title == "" {
		probe.Title = firstNonEmpty(normalized["title"])
	}
	if probe.Artist == "" {
		probe.Artist = firstNonEmpty(normalized["artist"], normalized["author"], normalized["composer"])
	}
	if probe.Album == "" {
		probe.Album = firstNonEmpty(normalized["album"])
	}
	if probe.AlbumArtist == "" {
		probe.AlbumArtist = firstNonEmpty(normalized["album_artist"], normalized["albumartist"])
	}
	if probe.Genre == "" {
		probe.Genre = firstNonEmpty(normalized["genre"])
	}
	if probe.TrackNumber == 0 {
		probe.TrackNumber = parseMediaTagOrdinal(firstNonEmpty(normalized["track"], normalized["tracknumber"], normalized["track_number"]))
	}
	if probe.DiscNumber == 0 {
		probe.DiscNumber = parseMediaTagOrdinal(firstNonEmpty(normalized["disc"], normalized["discnumber"], normalized["disc_number"], normalized["disk"]))
	}
	if probe.Year == 0 {
		probe.Year = parseMediaTagYear(firstNonEmpty(normalized["date"], normalized["year"], normalized["originaldate"]))
	}
}

func parseMediaTagOrdinal(value string) int {
	value = strings.TrimSpace(value)
	if separator := strings.IndexAny(value, "/-"); separator >= 0 {
		value = value[:separator]
	}
	result, _ := strconv.Atoi(strings.TrimSpace(value))
	if result < 0 {
		return 0
	}
	return result
}

func parseMediaTagYear(value string) int {
	value = strings.TrimSpace(value)
	if len(value) >= 4 {
		value = value[:4]
	}
	result, _ := strconv.Atoi(value)
	if result < 1000 || result > 9999 {
		return 0
	}
	return result
}

func mergeMediaProbe(base mediaProbe, override mediaProbe) mediaProbe {
	result := base
	if strings.TrimSpace(override.Format) != "" {
		result.Format = override.Format
	}
	if strings.TrimSpace(override.Title) != "" {
		result.Title = override.Title
	}
	if strings.TrimSpace(override.Artist) != "" {
		result.Artist = override.Artist
	}
	if strings.TrimSpace(override.Album) != "" {
		result.Album = override.Album
	}
	if strings.TrimSpace(override.AlbumArtist) != "" {
		result.AlbumArtist = override.AlbumArtist
	}
	if strings.TrimSpace(override.Genre) != "" {
		result.Genre = override.Genre
	}
	if override.TrackNumber > 0 {
		result.TrackNumber = override.TrackNumber
	}
	if override.DiscNumber > 0 {
		result.DiscNumber = override.DiscNumber
	}
	if override.Year > 0 {
		result.Year = override.Year
	}
	if strings.TrimSpace(override.Codec) != "" {
		result.Codec = override.Codec
	}
	if strings.TrimSpace(override.VideoCodec) != "" {
		result.VideoCodec = override.VideoCodec
	}
	if strings.TrimSpace(override.AudioCodec) != "" {
		result.AudioCodec = override.AudioCodec
	}
	if override.StreamInfo {
		result.StreamInfo = true
		result.HasAudio = override.HasAudio
		result.HasVideo = override.HasVideo
		result.AttachedPicCount = override.AttachedPicCount
		result.SubtitleStreams = append([]mediaProbeSubtitleStream(nil), override.SubtitleStreams...)
		if !override.HasAudio {
			result.AudioCodec = ""
			result.AudioBitrateKbps = 0
		}
		if !override.HasVideo {
			result.VideoCodec = ""
			result.Width = 0
			result.Height = 0
			result.FrameRate = 0
			result.VideoBitrateKbps = 0
		}
	}
	if override.DurationMs > 0 {
		result.DurationMs = override.DurationMs
	}
	if override.Width > 0 {
		result.Width = override.Width
	}
	if override.Height > 0 {
		result.Height = override.Height
	}
	if override.FrameRate > 0 {
		result.FrameRate = override.FrameRate
	}
	if override.BitrateKbps > 0 {
		result.BitrateKbps = override.BitrateKbps
	}
	if override.VideoBitrateKbps > 0 {
		result.VideoBitrateKbps = override.VideoBitrateKbps
	}
	if override.AudioBitrateKbps > 0 {
		result.AudioBitrateKbps = override.AudioBitrateKbps
	}
	if override.Channels > 0 {
		result.Channels = override.Channels
	}
	if override.SizeBytes > 0 {
		result.SizeBytes = override.SizeBytes
	}
	if override.DPI > 0 {
		result.DPI = override.DPI
	}
	if strings.TrimSpace(result.Codec) == "" {
		result.Codec = firstNonEmpty(result.VideoCodec, result.AudioCodec)
	}
	return result
}

func normalizeFFprobeFormat(formatName string, path string) string {
	if extension := normalizeFileExtension(path); extension != "" {
		return extension
	}
	for _, candidate := range strings.Split(strings.TrimSpace(formatName), ",") {
		if trimmed := normalizeTranscodeFormat(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseFFprobeDurationMillis(value string) int64 {
	durationSeconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || durationSeconds <= 0 {
		return 0
	}
	return int64(durationSeconds * 1000)
}

func parseFFprobeBitrateKbps(value string) int {
	bitrate, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || bitrate <= 0 {
		return 0
	}
	return int((bitrate + 500) / 1000)
}

func parseFFprobeSize(value string) int64 {
	size, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || size <= 0 {
		return 0
	}
	return size
}

func parseFFprobeFrameRate(value string) float64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	if strings.Contains(trimmed, "/") {
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) != 2 {
			return 0
		}
		numerator, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil || numerator <= 0 {
			return 0
		}
		denominator, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil || denominator <= 0 {
			return 0
		}
		return numerator / denominator
	}
	frameRate, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || frameRate <= 0 {
		return 0
	}
	return frameRate
}

func normalizeTranscodeFormat(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	trimmed = strings.TrimPrefix(trimmed, ".")
	return trimmed
}

func normalizeFileExtension(path string) string {
	return normalizeTranscodeFormat(filepath.Ext(strings.TrimSpace(path)))
}

func isSubtitleFormat(format string) bool {
	switch normalizeSubtitleFormat(format) {
	case "srt", "vtt", "ass", "ssa", "itt", "fcpxml":
		return true
	default:
		return false
	}
}

func resolveFFprobeExecPath(ctx context.Context, resolver ToolResolver) (string, error) {
	if resolver == nil {
		return "", fmt.Errorf("ffmpeg is not installed")
	}
	ready, reason, err := resolver.DependencyReadiness(ctx, dependencies.DependencyFFmpeg)
	if err != nil {
		return "", err
	}
	if !ready {
		switch strings.TrimSpace(reason) {
		case "invalid":
			return "", fmt.Errorf("ffmpeg is invalid")
		case "ffprobe_not_found":
			return "", fmt.Errorf("ffprobe is not installed")
		case "missing_exec_path", "exec_not_found", "not_found", "not_installed", "":
			return "", fmt.Errorf("ffmpeg is not installed")
		default:
			return "", fmt.Errorf("ffmpeg is not ready: %s", reason)
		}
	}
	dir, err := resolver.ResolveDependencyDirectory(ctx, dependencies.DependencyFFmpeg)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(strings.TrimSpace(dir), ffprobeExecutableName())
	info, statErr := os.Stat(candidate)
	if statErr != nil || info.IsDir() {
		return "", fmt.Errorf("ffprobe is not installed")
	}
	return candidate, nil
}

func resolveFFmpegExecPath(ctx context.Context, resolver ToolResolver) (string, error) {
	if resolver == nil {
		return "", fmt.Errorf("ffmpeg is not installed")
	}
	ready, reason, err := resolver.DependencyReadiness(ctx, dependencies.DependencyFFmpeg)
	if err != nil {
		return "", err
	}
	if !ready {
		switch strings.TrimSpace(reason) {
		case "invalid":
			return "", fmt.Errorf("ffmpeg is invalid")
		case "ffprobe_not_found":
			return "", fmt.Errorf("ffprobe is not installed")
		case "missing_exec_path", "exec_not_found", "not_found", "not_installed", "":
			return "", fmt.Errorf("ffmpeg is not installed")
		default:
			return "", fmt.Errorf("ffmpeg is not ready: %s", reason)
		}
	}
	dir, err := resolver.ResolveDependencyDirectory(ctx, dependencies.DependencyFFmpeg)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(strings.TrimSpace(dir), ffmpegExecutableName())
	info, statErr := os.Stat(candidate)
	if statErr != nil || info.IsDir() {
		return "", fmt.Errorf("ffmpeg is not installed")
	}
	return candidate, nil
}

func ffmpegExecutableName() string {
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

func ffprobeExecutableName() string {
	if runtime.GOOS == "windows" {
		return "ffprobe.exe"
	}
	return "ffprobe"
}
