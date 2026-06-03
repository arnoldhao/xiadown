package listenplayback

import (
	"context"
	"strings"
)

func NormalizeObservedPlaybackAudioQuality(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	switch trimmed {
	case "AUDIO_QUALITY_LOW":
		return "AUDIO_QUALITY_LOW"
	case "AUDIO_QUALITY_MEDIUM":
		return "AUDIO_QUALITY_MEDIUM"
	case "AUDIO_QUALITY_HIGH":
		return "AUDIO_QUALITY_HIGH"
	default:
		return ""
	}
}

func (service *PlayerService) UpdatePlaybackAudioQuality(ctx context.Context, observed string) Snapshot {
	if service == nil {
		return Snapshot{}
	}
	normalized := NormalizeObservedPlaybackAudioQuality(observed)
	service.mu.Lock()
	if service.observedPlaybackAudioQuality == normalized {
		snapshot := service.snapshotLocked(ctx)
		service.mu.Unlock()
		return snapshot
	}
	service.observedPlaybackAudioQuality = normalized
	service.mu.Unlock()
	return service.PublishSnapshot(ctx)
}
