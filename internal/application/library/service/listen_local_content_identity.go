package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

const listenLocalContentIdentityPacketCount = 12

type listenLocalContentIdentityPacketPayload struct {
	Packets []struct {
		DataHash string `json:"data_hash"`
	} `json:"packets"`
}

// listenLocalContentIdentitySignature fingerprints a bounded set of encoded
// audio packets near the beginning, middle, and end of a Track. Packet
// payloads exclude container tags and attached artwork, so metadata-only
// rewrites retain timeline identity. Sampling is seek-based and capped at 36
// packets; a library refresh never hashes an entire collection serially.
func (service *LibraryService) listenLocalContentIdentitySignature(
	ctx context.Context,
	path string,
	probe mediaProbe,
) string {
	probeCtx, cancel := context.WithTimeout(ctx, listenLocalProbeTimeout)
	defer cancel()
	hashes, err := service.probeListenLocalContentIdentityPackets(probeCtx, path, probe.DurationMs)
	if err == nil && len(hashes) > 0 {
		return buildListenLocalContentIdentitySignature("packet-v1", hashes...)
	}

	// Size/mtime cannot distinguish an audio replacement from a tag or artwork
	// rewrite, so it is not a safe fallback for content identity. Preserve the
	// last known packet baseline and retry on a later refresh instead.
	return ""
}

func stabilizeListenLocalContentIdentitySignature(
	existing string,
	candidate string,
) string {
	existing = strings.TrimSpace(existing)
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return existing
	}
	return candidate
}

func (service *LibraryService) probeListenLocalContentIdentityPackets(
	ctx context.Context,
	path string,
	durationMs int64,
) ([]string, error) {
	if service == nil {
		return nil, errors.New("Listen Local Music service is unavailable")
	}
	execPath, err := resolveFFprobeExecPath(ctx, service.tools)
	if err != nil {
		return nil, err
	}
	args := []string{
		"-v", "error",
		"-print_format", "json",
		"-select_streams", "a:0",
		"-show_packets",
		"-show_data_hash", "sha256",
		"-show_entries", "packet=data_hash",
		"-read_intervals", listenLocalContentIdentityReadIntervals(durationMs),
	}
	args = appendLocalMediaFFprobeInput(args, path)
	command := exec.CommandContext(ctx, execPath, args...)
	configureLocalMediaToolCommand(command)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe audio packet identity failed: %w", err)
	}
	payload := listenLocalContentIdentityPacketPayload{}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("decode ffprobe audio packet identity: %w", err)
	}
	hashes := make([]string, 0, len(payload.Packets))
	for _, packet := range payload.Packets {
		value := strings.ToLower(strings.TrimSpace(packet.DataHash))
		value = strings.TrimPrefix(value, "sha256:")
		if len(value) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(value); err != nil {
			continue
		}
		hashes = append(hashes, value)
	}
	if len(hashes) == 0 {
		return nil, errors.New("ffprobe returned no audio packet identity")
	}
	return hashes, nil
}

func listenLocalContentIdentityReadIntervals(durationMs int64) string {
	durationSeconds := max(float64(durationMs)/1000, 0)
	starts := []float64{0}
	if durationSeconds > 2 {
		starts = append(starts, durationSeconds/2, max(durationSeconds-1, 0))
	}
	slices.Sort(starts)
	intervals := make([]string, 0, len(starts))
	last := -1.0
	for _, start := range starts {
		start = float64(int64(start*1000)) / 1000
		if start == last {
			continue
		}
		last = start
		intervals = append(intervals, fmt.Sprintf(
			"%.3f%%+#%d",
			start,
			listenLocalContentIdentityPacketCount,
		))
	}
	return strings.Join(intervals, ",")
}

func buildListenLocalContentIdentitySignature(kind string, identities ...string) string {
	values := []string{
		"xiadown-music-content-identity-v1",
		strings.TrimSpace(kind),
	}
	values = append(values, identities...)
	hash := sha256.New()
	for _, value := range values {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	prefix := "mci1x:"
	switch strings.TrimSpace(kind) {
	case "packet-v1":
		prefix = "mci1p:"
	}
	return prefix + hex.EncodeToString(hash.Sum(nil))
}
