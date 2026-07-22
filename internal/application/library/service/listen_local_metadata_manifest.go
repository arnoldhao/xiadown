package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"reflect"
	"strconv"
	"strings"

	"xiadown/internal/domain/library"
)

type listenLocalMetadataManifest struct {
	Streams  []listenLocalMetadataManifestStream  `json:"streams"`
	Chapters []listenLocalMetadataManifestChapter `json:"chapters"`
	Format   listenLocalMetadataManifestFormat    `json:"format"`
}

type listenLocalMetadataManifestStream struct {
	CodecType   string            `json:"codec_type"`
	CodecName   string            `json:"codec_name"`
	Disposition map[string]int    `json:"disposition"`
	Tags        map[string]string `json:"tags"`
}

type listenLocalMetadataManifestChapter struct {
	StartTime string            `json:"start_time"`
	EndTime   string            `json:"end_time"`
	Tags      map[string]string `json:"tags"`
}

type listenLocalMetadataManifestFormat struct {
	FormatName string            `json:"format_name"`
	Duration   string            `json:"duration"`
	Tags       map[string]string `json:"tags"`
}

func probeListenLocalMetadataManifest(
	ctx context.Context,
	ffprobePath string,
	path string,
) (listenLocalMetadataManifest, error) {
	probeCtx, cancel := withLocalMediaProbeTimeout(ctx)
	defer cancel()
	args := []string{
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		"-show_chapters",
		"-show_format",
	}
	args = appendLocalMediaFFprobeInput(args, path)
	command := exec.CommandContext(probeCtx, ffprobePath, args...)
	configureLocalMediaToolCommand(command)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return listenLocalMetadataManifest{}, fmt.Errorf("inspect local track preservation data: %s", detail)
	}
	manifest := listenLocalMetadataManifest{}
	if err := json.Unmarshal(output, &manifest); err != nil {
		return listenLocalMetadataManifest{}, fmt.Errorf("inspect local track preservation data: %w", err)
	}
	return manifest, nil
}

func verifyListenLocalMetadataPreserved(
	before listenLocalMetadataManifest,
	after listenLocalMetadataManifest,
) error {
	if len(before.Streams) != len(after.Streams) {
		return listenLocalMetadataPreservationError("stream count changed")
	}
	for index := range before.Streams {
		left := before.Streams[index]
		right := after.Streams[index]
		if !strings.EqualFold(strings.TrimSpace(left.CodecType), strings.TrimSpace(right.CodecType)) ||
			!strings.EqualFold(strings.TrimSpace(left.CodecName), strings.TrimSpace(right.CodecName)) ||
			!reflect.DeepEqual(left.Disposition, right.Disposition) {
			return listenLocalMetadataPreservationError(fmt.Sprintf("stream %d changed", index))
		}
		if err := verifyListenLocalPreservedTags(left.Tags, right.Tags, true); err != nil {
			return listenLocalMetadataPreservationError(fmt.Sprintf("stream %d: %v", index, err))
		}
	}

	if len(before.Chapters) != len(after.Chapters) {
		return listenLocalMetadataPreservationError("chapter count changed")
	}
	for index := range before.Chapters {
		left := before.Chapters[index]
		right := after.Chapters[index]
		if !listenLocalMetadataTimesNear(left.StartTime, right.StartTime, 0.1) ||
			!listenLocalMetadataTimesNear(left.EndTime, right.EndTime, 0.1) {
			return listenLocalMetadataPreservationError(fmt.Sprintf("chapter %d timing changed", index))
		}
		if err := verifyListenLocalPreservedTags(left.Tags, right.Tags, false); err != nil {
			return listenLocalMetadataPreservationError(fmt.Sprintf("chapter %d: %v", index, err))
		}
	}

	if err := verifyListenLocalPreservedTags(before.Format.Tags, after.Format.Tags, true); err != nil {
		return listenLocalMetadataPreservationError(err.Error())
	}
	if !listenLocalMetadataDurationsNear(before.Format.Duration, after.Format.Duration, 1) {
		return listenLocalMetadataPreservationError(fmt.Sprintf("duration changed from %q to %q", before.Format.Duration, after.Format.Duration))
	}
	return nil
}

func listenLocalMetadataDurationsNear(left string, right string, toleranceSeconds float64) bool {
	leftSeconds, leftErr := strconv.ParseFloat(strings.TrimSpace(left), 64)
	rightSeconds, rightErr := strconv.ParseFloat(strings.TrimSpace(right), 64)
	if leftErr != nil || rightErr != nil {
		// Some Ogg-family files do not expose a format duration until a decoder
		// scans packets. Stream-copy preservation is already enforced above.
		return true
	}
	return math.Abs(leftSeconds-rightSeconds) <= toleranceSeconds
}

func listenLocalMetadataPreservationError(detail string) error {
	return fmt.Errorf("%w: remux would not preserve %s", library.ErrListenLocalMetadataUnsupported, detail)
}

func verifyListenLocalPreservedTags(before map[string]string, after map[string]string, ignoreEdited bool) error {
	left := normalizeListenLocalManifestTags(before)
	right := normalizeListenLocalManifestTags(after)
	for key, value := range left {
		if listenLocalManifestVolatileTag(key) || (ignoreEdited && listenLocalManifestEditedTag(key)) {
			continue
		}
		if rightValue, ok := right[key]; !ok || rightValue != value {
			return fmt.Errorf("tag %q changed or was removed", key)
		}
	}
	return nil
}

func normalizeListenLocalManifestTags(tags map[string]string) map[string]string {
	result := make(map[string]string, len(tags))
	for key, value := range tags {
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "" {
			result[key] = strings.TrimSpace(value)
		}
	}
	return result
}

func listenLocalManifestEditedTag(key string) bool {
	key = strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "-", "_"), " ", "_")
	switch key {
	case "title", "artist", "author", "album", "album_artist", "albumartist", "genre",
		"track", "tracknumber", "track_number", "disc", "discnumber", "disc_number", "disk",
		"date", "year", "originaldate":
		return true
	default:
		return false
	}
}

func listenLocalManifestVolatileTag(key string) bool {
	key = strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "-", "_"), " ", "_")
	switch key {
	case "encoder", "encoding_tool", "major_brand", "minor_version", "compatible_brands",
		"creation_time", "handler_name", "vendor_id", "duration":
		return true
	default:
		return false
	}
}

func listenLocalMetadataTimesNear(left string, right string, toleranceSeconds float64) bool {
	leftSeconds, leftErr := strconv.ParseFloat(strings.TrimSpace(left), 64)
	rightSeconds, rightErr := strconv.ParseFloat(strings.TrimSpace(right), 64)
	if leftErr != nil || rightErr != nil {
		return strings.TrimSpace(left) == strings.TrimSpace(right)
	}
	return math.Abs(leftSeconds-rightSeconds) <= toleranceSeconds
}
