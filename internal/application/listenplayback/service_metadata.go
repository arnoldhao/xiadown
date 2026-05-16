package listenplayback

import (
	"context"
	"strings"
	"time"
)

const trackMetadataEnrichmentTimeout = 25 * time.Second

func (service *PlayerService) MergeTrackMetadata(ctx context.Context, track Track) {
	defer service.PublishSnapshot(ctx)
	track.VideoID = normalizedVideoID(track.VideoID)
	if track.VideoID == "" {
		return
	}
	track.ID = stringsTrim(track.ID)
	track.Title = stringsTrim(track.Title)
	track.Artist = stringsTrim(track.Artist)
	track.ArtistBrowseID = stringsTrim(track.ArtistBrowseID)
	track.DurationLabel = stringsTrim(track.DurationLabel)
	track.ThumbnailURL = stringsTrim(track.ThumbnailURL)
	track.MusicVideoType = stringsTrim(track.MusicVideoType)
	track.LikeStatus = stringsTrim(track.LikeStatus)

	service.mu.Lock()
	if service.hasCurrentTrack && service.currentTrack.VideoID == track.VideoID {
		service.currentTrack = mergeTrackMetadata(service.currentTrack, track)
	}
	for index := range service.queue {
		if service.queue[index].VideoID == track.VideoID {
			service.queue[index] = mergeTrackMetadata(service.queue[index], track)
		}
	}
	service.mu.Unlock()
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) UpdateVideoAvailability(ctx context.Context, available bool, known bool) {
	if !known {
		return
	}
	service.mu.Lock()
	changed := service.applyVideoAvailabilityLocked(available)
	service.mu.Unlock()
	if !changed {
		return
	}
	service.PublishSnapshot(ctx)
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) requestTrackMetadataEnrichment(track Track) {
	if service == nil || service.library == nil {
		return
	}
	track = normalizeTrack(track)
	if !trackNeedsMetadataEnrichment(track) {
		return
	}
	videoID := track.VideoID
	service.mu.Lock()
	if service.metadataEnrichmentInFlight == nil {
		service.metadataEnrichmentInFlight = make(map[string]struct{})
	}
	if service.metadataEnrichmentAttempted == nil {
		service.metadataEnrichmentAttempted = make(map[string]struct{})
	}
	if _, exists := service.metadataEnrichmentInFlight[videoID]; exists {
		service.mu.Unlock()
		return
	}
	if _, exists := service.metadataEnrichmentAttempted[videoID]; exists {
		service.mu.Unlock()
		return
	}
	service.metadataEnrichmentInFlight[videoID] = struct{}{}
	service.metadataEnrichmentAttempted[videoID] = struct{}{}
	service.mu.Unlock()

	go func() {
		defer func() {
			service.mu.Lock()
			delete(service.metadataEnrichmentInFlight, videoID)
			service.mu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), trackMetadataEnrichmentTimeout)
		defer cancel()
		metadata, err := service.library.TrackMetadata(ctx, videoID)
		if err != nil {
			return
		}
		metadata.VideoID = normalizedVideoID(metadata.VideoID)
		if metadata.VideoID == "" {
			metadata.VideoID = videoID
		}
		if metadata.VideoID != videoID || !trackMetadataHasUsefulFields(metadata) {
			return
		}
		service.MergeTrackMetadata(context.Background(), metadata)
	}()
}

func trackNeedsMetadataEnrichment(track Track) bool {
	track = normalizeTrack(track)
	if track.VideoID == "" {
		return false
	}
	return isPlaceholderTrackTitle(track.Title, track.VideoID) ||
		isMissingTrackArtist(track.Artist) ||
		(track.DurationLabel == "" && track.DurationSeconds <= 0) ||
		track.ThumbnailURL == "" ||
		track.MusicVideoType == ""
}

func trackMetadataHasUsefulFields(track Track) bool {
	return (strings.TrimSpace(track.Title) != "" && !isPlaceholderTrackTitle(track.Title, track.VideoID)) ||
		!isMissingTrackArtist(track.Artist) ||
		strings.TrimSpace(track.ArtistBrowseID) != "" ||
		strings.TrimSpace(track.DurationLabel) != "" ||
		track.DurationSeconds > 0 ||
		strings.TrimSpace(track.ThumbnailURL) != "" ||
		strings.TrimSpace(track.MusicVideoType) != "" ||
		strings.TrimSpace(track.LikeStatus) != ""
}

func isPlaceholderTrackTitle(title string, videoID string) bool {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" || trimmed == strings.TrimSpace(videoID) {
		return true
	}
	switch strings.ToLower(trimmed) {
	case "unknown", "loading...", "youtube music":
		return true
	default:
		return false
	}
}

func isMissingTrackArtist(artist string) bool {
	trimmed := strings.TrimSpace(artist)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "unknown", "unknown artist", "youtube", "youtube music":
		return true
	}
	return false
}

func (service *PlayerService) applyVideoAvailabilityLocked(available bool) bool {
	if !service.hasCurrentTrack {
		return false
	}
	changed := !service.currentTrack.VideoAvailabilityKnown || service.currentTrack.HasVideo != available
	service.currentTrack.VideoAvailabilityKnown = true
	service.currentTrack.HasVideo = available
	if current, ok := service.currentQueueTrackLocked(); ok && current.VideoID == service.currentTrack.VideoID {
		index := safeQueueIndex(service.currentIndex, len(service.queue))
		if index >= 0 && index < len(service.queue) {
			service.queue[index].VideoAvailabilityKnown = true
			service.queue[index].HasVideo = available
		}
	}
	return changed
}

func mergeTrackMetadata(existing Track, incoming Track) Track {
	existing = normalizeTrack(existing)
	if incoming.ID != "" {
		existing.ID = incoming.ID
	}
	if incoming.Title != "" && incoming.Title != incoming.VideoID {
		existing.Title = incoming.Title
	}
	if incoming.Artist != "" {
		existing.Artist = incoming.Artist
	}
	if incoming.ArtistBrowseID != "" {
		existing.ArtistBrowseID = incoming.ArtistBrowseID
	}
	if incoming.DurationLabel != "" {
		existing.DurationLabel = incoming.DurationLabel
	}
	if incoming.DurationSeconds > 0 {
		existing.DurationSeconds = incoming.DurationSeconds
	}
	if incoming.ThumbnailURL != "" {
		existing.ThumbnailURL = incoming.ThumbnailURL
	}
	if incoming.MusicVideoType != "" {
		existing.MusicVideoType = incoming.MusicVideoType
	}
	if incoming.LikeStatus != "" {
		existing.LikeStatus = incoming.LikeStatus
	}
	if incoming.VideoAvailabilityKnown {
		existing.VideoAvailabilityKnown = true
		existing.HasVideo = incoming.HasVideo
	}
	return normalizeTrack(existing)
}
