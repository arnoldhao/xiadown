package listenplayback

import "xiadown/internal/application/youtubemusic"

func trackVideoAvailabilityFromMetadata(track Track) (bool, bool) {
	return youtubemusic.TrackVideoAvailability(track.MusicVideoType, track.VideoID, track.ThumbnailURL)
}

func normalizeTrackVideoAvailability(track Track) Track {
	track.HasVideo = false
	track.VideoAvailabilityKnown = false
	if hasVideo, known := trackVideoAvailabilityFromMetadata(track); known {
		track.HasVideo = hasVideo
		track.VideoAvailabilityKnown = true
	}
	return track
}

func shouldUseObservedThumbnailForVideoAvailability(track Track, observedThumbnailURL string) bool {
	if stringsTrim(observedThumbnailURL) == "" {
		return false
	}
	if _, known := youtubemusic.MusicVideoTypeVideoAvailability(track.MusicVideoType); known {
		return false
	}
	if !youtubemusic.ThumbnailSuggestsVideoContent(track.VideoID, observedThumbnailURL) {
		return false
	}
	return !youtubemusic.ThumbnailSuggestsVideoContent(track.VideoID, track.ThumbnailURL)
}
