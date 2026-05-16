package youtubemusic

import "strings"

const (
	MusicVideoTypeOMV = "MUSIC_VIDEO_TYPE_OMV"
	MusicVideoTypeATV = "MUSIC_VIDEO_TYPE_ATV"
)

func MusicVideoTypeHasVideoContent(value string) bool {
	return strings.TrimSpace(value) == MusicVideoTypeOMV
}

func MusicVideoTypeVideoAvailability(value string) (bool, bool) {
	switch strings.TrimSpace(value) {
	case MusicVideoTypeOMV:
		return true, true
	case MusicVideoTypeATV:
		return false, true
	default:
		return false, false
	}
}

func ThumbnailSuggestsVideoContent(videoID string, thumbnailURL string) bool {
	normalizedVideoID := strings.TrimSpace(videoID)
	normalizedThumbnailURL := strings.ToLower(strings.TrimSpace(thumbnailURL))
	if normalizedVideoID == "" || normalizedThumbnailURL == "" {
		return false
	}
	return (strings.Contains(normalizedThumbnailURL, "i.ytimg.com/vi/"+strings.ToLower(normalizedVideoID)+"/") ||
		strings.Contains(normalizedThumbnailURL, "img.youtube.com/vi/"+strings.ToLower(normalizedVideoID)+"/") ||
		strings.Contains(normalizedThumbnailURL, "i.ytimg.com/vi_webp/"+strings.ToLower(normalizedVideoID)+"/") ||
		strings.Contains(normalizedThumbnailURL, "img.youtube.com/vi_webp/"+strings.ToLower(normalizedVideoID)+"/"))
}

func TrackVideoAvailability(musicVideoType string, videoID string, thumbnailURL string) (bool, bool) {
	if hasVideo, known := MusicVideoTypeVideoAvailability(musicVideoType); known {
		return hasVideo, true
	}
	if ThumbnailSuggestsVideoContent(videoID, thumbnailURL) {
		return true, true
	}
	if strings.TrimSpace(thumbnailURL) != "" {
		return false, true
	}
	return false, false
}
