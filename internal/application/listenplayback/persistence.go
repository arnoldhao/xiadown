package listenplayback

import "context"

func (service *PlayerService) RestorePlaybackSession(ctx context.Context) (bool, error) {
	defer service.PublishSnapshot(ctx)
	if service.store == nil {
		return false, nil
	}
	session, ok, err := service.store.LoadPlaybackSession(ctx)
	if err != nil || !ok {
		return ok, err
	}
	currentIndex := resolvedPersistedQueueIndex(session.CurrentIndex, session.CurrentVideoID, session.Queue)
	service.applyRestoredPlaybackSession(
		session.Queue,
		session.QueueKind,
		session.QueueTitle,
		currentIndex,
		session.Progress,
		session.Duration,
	)
	service.mu.Lock()
	service.shuffleEnabled = session.ShuffleEnabled
	service.repeatMode = session.RepeatMode
	service.volume = clampVolume(session.Volume)
	service.muted = session.Muted
	volumeBeforeMute := clampVolume(session.VolumeBeforeMute)
	if volumeBeforeMute > 0 {
		service.volumeBeforeMute = volumeBeforeMute
	} else if service.volume > 0 {
		service.volumeBeforeMute = service.volume
	}
	service.mu.Unlock()
	service.requestCurrentQueueMetadataEnrichment()
	return true, nil
}

func (service *PlayerService) ClearSavedPlaybackSession(ctx context.Context) error {
	if service.store == nil {
		return nil
	}
	return service.store.ClearPlaybackSession(ctx)
}

func (service *PlayerService) PersistPlaybackSession(ctx context.Context) {
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) saveCurrentSession(ctx context.Context) {
	if service.store == nil {
		return
	}
	session, ok := service.currentSession()
	if !ok {
		_ = service.store.ClearPlaybackSession(ctx)
		return
	}
	_ = service.store.SavePlaybackSession(ctx, session)
}

func (service *PlayerService) currentSession() (RestoredPlaybackSession, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.queue) == 0 {
		return RestoredPlaybackSession{}, false
	}
	index := safeQueueIndex(service.currentIndex, len(service.queue))
	currentVideoID := service.pendingPlayVideoID
	if currentVideoID == "" && service.hasCurrentTrack {
		currentVideoID = service.currentTrack.VideoID
	}
	if currentVideoID == "" {
		currentVideoID = service.queue[index].VideoID
	}
	duration := service.duration
	if duration <= 0 {
		duration = service.queue[index].DurationSeconds
	}
	progress := clampSeconds(service.progress)
	if duration > 0 && progress > duration {
		progress = duration
	}
	return RestoredPlaybackSession{
		Queue:            cloneTracks(service.queue),
		QueueKind:        service.queueKind,
		QueueTitle:       service.queueTitle,
		CurrentIndex:     index,
		CurrentVideoID:   currentVideoID,
		Progress:         progress,
		Duration:         duration,
		ShuffleEnabled:   service.shuffleEnabled,
		RepeatMode:       service.repeatMode,
		Volume:           service.volume,
		VolumeBeforeMute: service.volumeBeforeMute,
		Muted:            service.muted,
	}, true
}

func resolvedPersistedQueueIndex(savedIndex int, currentVideoID string, queue []Track) int {
	if savedIndex >= 0 && savedIndex < len(queue) {
		return savedIndex
	}
	currentVideoID = normalizedVideoID(currentVideoID)
	if currentVideoID != "" {
		for index, track := range queue {
			if normalizedVideoID(track.VideoID) == currentVideoID {
				return index
			}
		}
	}
	return safeQueueIndex(savedIndex, len(queue))
}
