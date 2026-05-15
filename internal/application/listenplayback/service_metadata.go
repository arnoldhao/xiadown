package listenplayback

import "context"

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
