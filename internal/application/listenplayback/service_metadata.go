package listenplayback

import (
	"context"
	"regexp"
	"strings"
	"time"
)

const (
	trackMetadataEnrichmentTimeout      = 25 * time.Second
	queueMetadataEnrichmentRequestDelay = 100 * time.Millisecond
)

var trackArtistReleaseYearPattern = regexp.MustCompile(`^(?:19|20)\d{2}\s*年?$`)

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
	track.ArtistSource = stringsTrim(track.ArtistSource)
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

func (service *PlayerService) requestTrackMetadataEnrichment(track Track) {
	service.requestTracksMetadataEnrichment([]Track{track})
}

func (service *PlayerService) requestCurrentQueueMetadataEnrichment() {
	if service == nil {
		return
	}
	service.mu.Lock()
	tracks := cloneTracks(service.queue)
	service.mu.Unlock()
	service.requestTracksMetadataEnrichment(tracks)
}

func (service *PlayerService) requestTracksMetadataEnrichment(tracks []Track) {
	videoIDs := service.reserveTrackMetadataEnrichment(tracks)
	if len(videoIDs) == 0 {
		return
	}

	go service.enrichTrackMetadata(videoIDs)
}

func (service *PlayerService) reserveTrackMetadataEnrichment(tracks []Track) []string {
	if service == nil || service.library == nil {
		return nil
	}
	if len(tracks) == 0 {
		return nil
	}
	videoIDs := make([]string, 0, len(tracks))
	seen := make(map[string]struct{}, len(tracks))

	service.mu.Lock()
	defer service.mu.Unlock()
	if service.metadataEnrichmentInFlight == nil {
		service.metadataEnrichmentInFlight = make(map[string]struct{})
	}
	if service.metadataEnrichmentAttempted == nil {
		service.metadataEnrichmentAttempted = make(map[string]struct{})
	}
	for _, track := range tracks {
		track = normalizeTrack(track)
		if !trackNeedsMetadataEnrichment(track) {
			continue
		}
		videoID := track.VideoID
		if _, exists := seen[videoID]; exists {
			continue
		}
		seen[videoID] = struct{}{}
		if _, exists := service.metadataEnrichmentInFlight[videoID]; exists {
			continue
		}
		if _, exists := service.metadataEnrichmentAttempted[videoID]; exists {
			continue
		}
		service.metadataEnrichmentInFlight[videoID] = struct{}{}
		service.metadataEnrichmentAttempted[videoID] = struct{}{}
		videoIDs = append(videoIDs, videoID)
	}

	return videoIDs
}

func (service *PlayerService) enrichTrackMetadata(videoIDs []string) {
	for index, videoID := range videoIDs {
		service.enrichSingleTrackMetadata(videoID)
		if index < len(videoIDs)-1 {
			time.Sleep(queueMetadataEnrichmentRequestDelay)
		}
	}
}

func (service *PlayerService) enrichSingleTrackMetadata(videoID string) {
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
}

func trackNeedsMetadataEnrichment(track Track) bool {
	track = normalizeTrack(track)
	if track.VideoID == "" {
		return false
	}
	return isPlaceholderTrackTitle(track.Title, track.VideoID) ||
		!hasTrustedTrackArtist(track) ||
		(track.DurationLabel == "" && track.DurationSeconds <= 0) ||
		track.ThumbnailURL == "" ||
		track.MusicVideoType == ""
}

func trackMetadataHasUsefulFields(track Track) bool {
	return (strings.TrimSpace(track.Title) != "" && !isPlaceholderTrackTitle(track.Title, track.VideoID)) ||
		hasTrustedTrackArtist(track) ||
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
	return trackArtistReleaseYearPattern.MatchString(trimmed)
}

func hasTrustedTrackArtist(track Track) bool {
	track.Artist = stringsTrim(track.Artist)
	track.ArtistBrowseID = stringsTrim(track.ArtistBrowseID)
	track.ArtistSource = stringsTrim(track.ArtistSource)
	if isMissingTrackArtist(track.Artist) {
		return false
	}
	if track.ArtistBrowseID != "" {
		return true
	}
	return track.ArtistSource == TrackArtistSourceAPILinked ||
		track.ArtistSource == TrackArtistSourceAPILinkedMultiple ||
		track.ArtistSource == TrackArtistSourceAPIMetadata
}

func shouldAcceptIncomingTrackArtist(existing Track, incoming Track) bool {
	if isMissingTrackArtist(incoming.Artist) {
		return false
	}
	return hasTrustedTrackArtist(incoming)
}

func mergeTrackMetadata(existing Track, incoming Track) Track {
	existing = normalizeTrack(existing)
	incoming = normalizeTrack(incoming)
	if incoming.Title != "" && incoming.Title != incoming.VideoID {
		existing.Title = incoming.Title
	}
	acceptedArtist := false
	if incoming.Artist != "" && shouldAcceptIncomingTrackArtist(existing, incoming) {
		existing.Artist = incoming.Artist
		acceptedArtist = true
	}
	if incoming.ArtistBrowseID != "" {
		existing.ArtistBrowseID = incoming.ArtistBrowseID
	}
	if len(incoming.Artists) > 0 && (acceptedArtist || len(existing.Artists) == 0) {
		existing.Artists = incoming.Artists
	}
	if acceptedArtist && incoming.ArtistSource != "" {
		existing.ArtistSource = incoming.ArtistSource
	}
	if acceptedArtist && incoming.ArtistBrowseID != "" {
		existing.ArtistSource = TrackArtistSourceAPILinked
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
