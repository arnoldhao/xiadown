package listenplayback

import (
	"context"
)

func (service *PlayerService) PlayQueue(ctx context.Context, tracks []Track, startingAt int, title string) error {
	defer service.PublishSnapshot(ctx)
	normalized := normalizeTracks(tracks)
	if len(normalized) == 0 {
		return nil
	}
	index := safeQueueIndex(startingAt, len(normalized))
	service.mu.Lock()
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	service.queueKind = QueueKindPlaylist
	service.queueTitle = stringsTrim(title)
	service.mixContinuationToken = ""
	if service.shuffleEnabled && len(normalized) > 1 {
		service.materializeShuffleQueueLocked(normalized, index, false, true)
	} else {
		service.queueOrderBeforeShuffle = nil
		service.queue = normalized
		service.currentIndex = index
	}
	track := service.queue[safeQueueIndex(service.currentIndex, len(service.queue))]
	action, err := service.preparePlayTrackLocked(track, VideoLoadStandard, PlayOptions{})
	service.mu.Unlock()
	if err != nil {
		return err
	}
	if err := service.executeActions(ctx, action); err != nil {
		return err
	}
	service.requestTracksMetadataEnrichment(normalized)
	service.saveCurrentSession(ctx)
	return nil
}

func (service *PlayerService) PlayWithRadio(ctx context.Context, track Track, title string) error {
	defer service.PublishSnapshot(ctx)
	track = normalizeTrack(track)
	if track.VideoID == "" {
		return nil
	}
	service.mu.Lock()
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	service.queue = []Track{track}
	service.queueOrderBeforeShuffle = nil
	service.queueKind = QueueKindRadio
	service.queueTitle = stringsTrim(title)
	service.currentIndex = 0
	service.mixContinuationToken = ""
	action, err := service.preparePlayTrackLocked(track, VideoLoadStandard, PlayOptions{})
	service.mu.Unlock()
	if err != nil {
		return err
	}
	if err := service.executeActions(ctx, action); err != nil {
		return err
	}
	service.requestTracksMetadataEnrichment([]Track{track})
	service.PublishSnapshot(ctx)
	if service.library != nil {
		radioTracks, err := service.library.Radio(ctx, track.VideoID, radioQueueLimit)
		if err == nil && len(radioTracks) > 0 {
			service.applyRadioQueue(ctx, track, radioTracks)
		}
	}
	service.saveCurrentSession(ctx)
	return nil
}

func (service *PlayerService) PlayRadioQueue(ctx context.Context, tracks []Track, startingAt int, title string) error {
	defer service.PublishSnapshot(ctx)
	normalized := normalizeTracks(tracks)
	if len(normalized) == 0 {
		return nil
	}
	index := safeQueueIndex(startingAt, len(normalized))
	service.mu.Lock()
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	service.queueKind = QueueKindRadio
	service.queueTitle = stringsTrim(title)
	service.mixContinuationToken = ""
	if service.shuffleEnabled && len(normalized) > 1 {
		service.materializeShuffleQueueLocked(normalized, index, false, true)
	} else {
		service.queueOrderBeforeShuffle = nil
		service.queue = normalized
		service.currentIndex = index
	}
	track := service.queue[safeQueueIndex(service.currentIndex, len(service.queue))]
	action, err := service.preparePlayTrackLocked(track, VideoLoadStandard, PlayOptions{})
	service.mu.Unlock()
	if err != nil {
		return err
	}
	if err := service.executeActions(ctx, action); err != nil {
		return err
	}
	service.requestTracksMetadataEnrichment(normalized)
	service.saveCurrentSession(ctx)
	return nil
}

func (service *PlayerService) PlayWithMix(ctx context.Context, playlistID string, startVideoID string, title string) error {
	defer service.PublishSnapshot(ctx)
	if service.library == nil {
		return nil
	}
	result, err := service.library.MixQueue(ctx, stringsTrim(playlistID), stringsTrim(startVideoID))
	if err != nil {
		return err
	}
	tracks := normalizeTracks(result.Tracks)
	if len(tracks) == 0 {
		return nil
	}
	shuffleTracks(tracks, service.random)
	service.mu.Lock()
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	service.queue = tracks
	service.queueOrderBeforeShuffle = nil
	service.queueKind = QueueKindMix
	service.queueTitle = stringsTrim(title)
	service.currentIndex = 0
	service.mixContinuationToken = stringsTrim(result.ContinuationToken)
	action, err := service.preparePlayTrackLocked(tracks[0], VideoLoadStandard, PlayOptions{})
	service.mu.Unlock()
	if err != nil {
		return err
	}
	if err := service.executeActions(ctx, action); err != nil {
		return err
	}
	service.requestTracksMetadataEnrichment(tracks)
	service.saveCurrentSession(ctx)
	return nil
}

func (service *PlayerService) PlayFromQueue(ctx context.Context, index int) error {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	if index < 0 || index >= len(service.queue) {
		service.mu.Unlock()
		return nil
	}
	service.clearForwardSkipNavigationStackLocked()
	service.currentIndex = index
	track := service.queue[index]
	action, err := service.preparePlayTrackLocked(track, VideoLoadStandard, PlayOptions{})
	service.mu.Unlock()
	if err != nil {
		return err
	}
	if err := service.executeActions(ctx, action); err != nil {
		return err
	}
	service.PublishSnapshot(ctx)
	if err := service.FetchMoreMixSongsIfNeeded(ctx); err != nil {
		return err
	}
	service.saveCurrentSession(ctx)
	return nil
}

func (service *PlayerService) Next(ctx context.Context) error {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	service.clearRestoredPlaybackSessionStateLocked()
	if len(service.queue) == 0 {
		hasPending := service.pendingPlayVideoID != ""
		service.mu.Unlock()
		if hasPending {
			return service.executeActions(ctx, transportAction{kind: "next"})
		}
		return nil
	}

	if service.currentIndex < len(service.queue)-1 {
		action, err := service.playQueueIndexLocked(service.currentIndex+1, true, VideoLoadStandard)
		service.mu.Unlock()
		if err != nil {
			return err
		}
		if err := service.executeActions(ctx, action); err != nil {
			return err
		}
		service.PublishSnapshot(ctx)
		if err := service.FetchMoreMixSongsIfNeeded(ctx); err != nil {
			return err
		}
		service.saveCurrentSession(ctx)
		return nil
	}

	if service.repeatMode == RepeatModeAll {
		action, err := service.playQueueIndexLocked(0, true, VideoLoadStandard)
		service.mu.Unlock()
		if err != nil {
			return err
		}
		if err := service.executeActions(ctx, action); err != nil {
			return err
		}
		service.saveCurrentSession(ctx)
		return nil
	}

	hasContinuation := service.mixContinuationToken != ""
	previousCount := len(service.queue)
	service.mu.Unlock()
	if !hasContinuation {
		return nil
	}
	if err := service.FetchMoreMixSongsIfNeeded(ctx); err != nil {
		return err
	}
	service.mu.Lock()
	if len(service.queue) <= previousCount || service.currentIndex >= len(service.queue)-1 {
		service.mu.Unlock()
		return nil
	}
	action, err := service.playQueueIndexLocked(service.currentIndex+1, true, VideoLoadStandard)
	service.mu.Unlock()
	if err != nil {
		return err
	}
	if err := service.executeActions(ctx, action); err != nil {
		return err
	}
	service.saveCurrentSession(ctx)
	return nil
}

func (service *PlayerService) Previous(ctx context.Context) error {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	service.clearRestoredPlaybackSessionStateLocked()
	if len(service.queue) == 0 {
		progress := service.progress
		hasPending := service.pendingPlayVideoID != ""
		service.mu.Unlock()
		if progress > 3 {
			return service.Seek(ctx, 0)
		}
		if hasPending {
			return service.executeActions(ctx, transportAction{kind: "previous"})
		}
		return nil
	}

	if service.progress > 3 {
		service.mu.Unlock()
		return service.Seek(ctx, 0)
	}

	priorIndex := -1
	for len(service.forwardSkipIndexStack) > 0 {
		last := service.forwardSkipIndexStack[len(service.forwardSkipIndexStack)-1]
		service.forwardSkipIndexStack = service.forwardSkipIndexStack[:len(service.forwardSkipIndexStack)-1]
		if last >= 0 && last < len(service.queue) {
			priorIndex = last
			break
		}
	}
	if priorIndex >= 0 {
		action, err := service.playQueueIndexLocked(priorIndex, false, VideoLoadStandard)
		service.mu.Unlock()
		if err != nil {
			return err
		}
		if err := service.executeActions(ctx, action); err != nil {
			return err
		}
		service.saveCurrentSession(ctx)
		return nil
	}

	if service.currentIndex > 0 {
		action, err := service.playQueueIndexLocked(service.currentIndex-1, false, VideoLoadStandard)
		service.mu.Unlock()
		if err != nil {
			return err
		}
		if err := service.executeActions(ctx, action); err != nil {
			return err
		}
		service.saveCurrentSession(ctx)
		return nil
	}
	service.mu.Unlock()
	return service.Seek(ctx, 0)
}

func (service *PlayerService) ClearQueueEntirely(ctx context.Context) {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	service.mixContinuationToken = ""
	service.queue = nil
	service.queueOrderBeforeShuffle = nil
	service.queueKind = QueueKindNone
	service.queueTitle = ""
	service.currentIndex = 0
	service.mu.Unlock()
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) ClearQueue(ctx context.Context) {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	service.mixContinuationToken = ""
	service.queueOrderBeforeShuffle = nil
	if service.hasCurrentTrack {
		service.queue = []Track{service.currentTrack}
		service.currentIndex = 0
	} else {
		service.queue = nil
		service.currentIndex = 0
		service.queueKind = QueueKindNone
		service.queueTitle = ""
	}
	service.mu.Unlock()
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) InsertNextInQueue(ctx context.Context, tracks []Track) {
	defer service.PublishSnapshot(ctx)
	tracks = normalizeTracks(tracks)
	if len(tracks) == 0 {
		return
	}
	service.mu.Lock()
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	insertIndex := service.currentIndex + 1
	if insertIndex > len(service.queue) {
		insertIndex = len(service.queue)
	}
	service.queue = append(service.queue[:insertIndex], append(tracks, service.queue[insertIndex:]...)...)
	service.mu.Unlock()
	service.requestTracksMetadataEnrichment(tracks)
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) AppendToQueue(ctx context.Context, tracks []Track) {
	defer service.PublishSnapshot(ctx)
	tracks = normalizeTracks(tracks)
	if len(tracks) == 0 {
		return
	}
	service.mu.Lock()
	service.recordQueueStateForUndoLocked()
	service.queue = append(service.queue, tracks...)
	service.mu.Unlock()
	service.requestTracksMetadataEnrichment(tracks)
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) RemoveFromQueue(ctx context.Context, videoIDs map[string]struct{}) {
	defer service.PublishSnapshot(ctx)
	if len(videoIDs) == 0 {
		return
	}
	service.mu.Lock()
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	nextQueue := service.queue[:0]
	for _, track := range service.queue {
		if _, remove := videoIDs[track.VideoID]; !remove {
			nextQueue = append(nextQueue, track)
		}
	}
	service.queue = append([]Track(nil), nextQueue...)
	service.realignCurrentIndexLocked()
	service.mu.Unlock()
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) ReorderQueue(ctx context.Context, videoIDs []string) {
	defer service.PublishSnapshot(ctx)
	if len(videoIDs) == 0 {
		return
	}
	service.mu.Lock()
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	lookup := make(map[string]Track, len(service.queue))
	for _, track := range service.queue {
		lookup[track.VideoID] = track
	}
	reordered := make([]Track, 0, len(videoIDs))
	seen := make(map[string]struct{}, len(videoIDs))
	for _, videoID := range videoIDs {
		videoID = normalizedVideoID(videoID)
		if _, exists := seen[videoID]; exists {
			continue
		}
		track, ok := lookup[videoID]
		if !ok {
			continue
		}
		reordered = append(reordered, track)
		seen[videoID] = struct{}{}
	}
	service.queue = reordered
	service.realignCurrentIndexLocked()
	service.mu.Unlock()
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) MoveQueueItems(ctx context.Context, source []int, destination int) {
	defer service.PublishSnapshot(ctx)
	if len(source) == 0 {
		return
	}
	service.mu.Lock()
	if len(service.queue) == 0 {
		service.mu.Unlock()
		return
	}
	sourceSet := make(map[int]struct{}, len(source))
	orderedSource := make([]int, 0, len(source))
	for _, index := range source {
		if index < 0 || index >= len(service.queue) {
			service.mu.Unlock()
			return
		}
		if index == service.currentIndex {
			service.mu.Unlock()
			return
		}
		if _, exists := sourceSet[index]; exists {
			continue
		}
		sourceSet[index] = struct{}{}
		orderedSource = append(orderedSource, index)
	}
	if len(orderedSource) == 0 {
		service.mu.Unlock()
		return
	}
	if destination < 0 || destination > len(service.queue) || destination == service.currentIndex {
		service.mu.Unlock()
		return
	}
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	current := Track{}
	hasCurrent := false
	if service.currentIndex >= 0 && service.currentIndex < len(service.queue) {
		current = service.queue[service.currentIndex]
		hasCurrent = true
	}
	moving := make([]Track, 0, len(orderedSource))
	remaining := make([]Track, 0, len(service.queue)-len(orderedSource))
	for index, track := range service.queue {
		if _, move := sourceSet[index]; move {
			moving = append(moving, track)
			continue
		}
		remaining = append(remaining, track)
	}
	insertAt := destination
	removedBeforeDestination := 0
	for index := range sourceSet {
		if index < destination {
			removedBeforeDestination++
		}
	}
	insertAt -= removedBeforeDestination
	if insertAt < 0 {
		insertAt = 0
	}
	if insertAt > len(remaining) {
		insertAt = len(remaining)
	}
	nextQueue := make([]Track, 0, len(service.queue))
	nextQueue = append(nextQueue, remaining[:insertAt]...)
	nextQueue = append(nextQueue, moving...)
	nextQueue = append(nextQueue, remaining[insertAt:]...)
	service.queue = nextQueue
	if hasCurrent {
		for index, track := range service.queue {
			if track.VideoID == current.VideoID {
				service.currentIndex = index
				break
			}
		}
	}
	service.mu.Unlock()
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) ShuffleQueue(ctx context.Context) {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	if len(service.queue) <= 1 {
		service.mu.Unlock()
		return
	}
	service.materializeShuffleQueueLocked(service.queue, service.currentIndex, true, false)
	service.mu.Unlock()
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) UndoQueue(ctx context.Context) {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	if len(service.queueUndo) == 0 {
		service.mu.Unlock()
		return
	}
	previous := service.queueUndo[len(service.queueUndo)-1]
	service.queueUndo = service.queueUndo[:len(service.queueUndo)-1]
	service.queueRedo = append(service.queueRedo, QueueSnapshot{
		Queue:        cloneTracks(service.queue),
		CurrentIndex: service.currentIndex,
	})
	service.queue = cloneTracks(previous.Queue)
	service.currentIndex = safeQueueIndex(previous.CurrentIndex, len(service.queue))
	service.clearForwardSkipNavigationStackLocked()
	service.realignCurrentTrackLocked()
	service.mu.Unlock()
	service.requestCurrentQueueMetadataEnrichment()
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) RedoQueue(ctx context.Context) {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	if len(service.queueRedo) == 0 {
		service.mu.Unlock()
		return
	}
	next := service.queueRedo[len(service.queueRedo)-1]
	service.queueRedo = service.queueRedo[:len(service.queueRedo)-1]
	service.queueUndo = append(service.queueUndo, QueueSnapshot{
		Queue:        cloneTracks(service.queue),
		CurrentIndex: service.currentIndex,
	})
	service.queue = cloneTracks(next.Queue)
	service.currentIndex = safeQueueIndex(next.CurrentIndex, len(service.queue))
	service.clearForwardSkipNavigationStackLocked()
	service.realignCurrentTrackLocked()
	service.mu.Unlock()
	service.requestCurrentQueueMetadataEnrichment()
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) FetchMoreMixSongsIfNeeded(ctx context.Context) error {
	defer service.PublishSnapshot(ctx)
	service.mu.Lock()
	token := service.mixContinuationToken
	remaining := len(service.queue) - service.currentIndex - 1
	if token == "" || service.fetchingMoreMixSongs || service.library == nil || remaining > mixFetchThresholdRemaining {
		service.mu.Unlock()
		return nil
	}
	service.fetchingMoreMixSongs = true
	service.mu.Unlock()

	result, err := service.library.MixQueueContinuation(ctx, token)

	service.mu.Lock()
	service.fetchingMoreMixSongs = false
	if err != nil {
		service.mu.Unlock()
		return err
	}
	existing := make(map[string]struct{}, len(service.queue))
	for _, track := range service.queue {
		existing[track.VideoID] = struct{}{}
	}
	addedTracks := make([]Track, 0, len(result.Tracks))
	for _, track := range normalizeTracks(result.Tracks) {
		if _, ok := existing[track.VideoID]; ok {
			continue
		}
		service.queue = append(service.queue, track)
		existing[track.VideoID] = struct{}{}
		addedTracks = append(addedTracks, track)
	}
	service.mixContinuationToken = stringsTrim(result.ContinuationToken)
	service.mu.Unlock()
	service.requestTracksMetadataEnrichment(addedTracks)
	return nil
}

func (service *PlayerService) setShuffleEnabledLocked(enabled bool, recordUndo bool) {
	if service.shuffleEnabled == enabled {
		return
	}
	service.shuffleEnabled = enabled
	if enabled {
		service.materializeShuffleQueueLocked(service.queue, service.currentIndex, recordUndo, true)
		return
	}
	service.restoreQueueOrderBeforeShuffleLocked(recordUndo)
}

func (service *PlayerService) materializeShuffleQueueLocked(entries []Track, startingAt int, recordUndo bool, storesOriginalOrder bool) {
	if len(entries) <= 1 {
		service.queue = cloneTracks(entries)
		service.currentIndex = safeQueueIndex(startingAt, len(service.queue))
		if len(entries) == 0 {
			service.queueOrderBeforeShuffle = nil
		}
		return
	}
	service.clearForwardSkipNavigationStackLocked()
	if recordUndo {
		service.recordQueueStateForUndoLocked()
	}
	if storesOriginalOrder {
		service.queueOrderBeforeShuffle = cloneTracks(entries)
	}

	queue := cloneTracks(entries)
	index := safeQueueIndex(startingAt, len(queue))
	current := queue[index]
	queue = append(queue[:index], queue[index+1:]...)
	shuffleTracks(queue, service.random)
	service.queue = append([]Track{current}, queue...)
	service.currentIndex = 0
}

func (service *PlayerService) restoreQueueOrderBeforeShuffleLocked(recordUndo bool) {
	snapshot := cloneTracks(service.queueOrderBeforeShuffle)
	if len(snapshot) == 0 {
		service.queueOrderBeforeShuffle = nil
		return
	}
	currentQueue := cloneTracks(service.queue)
	if len(currentQueue) == 0 {
		service.queueOrderBeforeShuffle = nil
		service.queue = nil
		service.currentIndex = 0
		return
	}
	currentKey := ""
	if service.currentIndex >= 0 && service.currentIndex < len(currentQueue) {
		currentKey = queueTrackIdentity(currentQueue[service.currentIndex])
	}

	service.clearForwardSkipNavigationStackLocked()
	if recordUndo {
		service.recordQueueStateForUndoLocked()
	}

	used := make([]bool, len(currentQueue))
	restored := make([]Track, 0, len(currentQueue))
	for _, original := range snapshot {
		key := queueTrackIdentity(original)
		for index, track := range currentQueue {
			if used[index] || queueTrackIdentity(track) != key {
				continue
			}
			restored = append(restored, track)
			used[index] = true
			break
		}
	}
	for index, track := range currentQueue {
		if !used[index] {
			restored = append(restored, track)
		}
	}
	if len(restored) == 0 {
		service.queueOrderBeforeShuffle = nil
		return
	}
	service.queue = restored
	service.currentIndex = safeQueueIndex(service.currentIndex, len(service.queue))
	if currentKey != "" {
		for index, track := range service.queue {
			if queueTrackIdentity(track) == currentKey {
				service.currentIndex = index
				break
			}
		}
	}
	service.queueOrderBeforeShuffle = nil
}

func queueTrackIdentity(track Track) string {
	if track.ID != "" {
		return "id:" + track.ID
	}
	return "video:" + track.VideoID
}

func (service *PlayerService) playQueueIndexLocked(index int, rememberForwardSkip bool, strategy VideoLoadStrategy) (transportAction, error) {
	if index < 0 || index >= len(service.queue) {
		return transportAction{}, nil
	}
	if rememberForwardSkip {
		service.pushForwardSkipStackIfLeavingIndexLocked(index)
	}
	service.currentIndex = index
	return service.preparePlayTrackLocked(service.queue[index], strategy, PlayOptions{})
}

func (service *PlayerService) applyRadioQueue(ctx context.Context, seed Track, tracks []Track) {
	defer service.PublishSnapshot(ctx)
	normalized := normalizeTracks(tracks)
	if len(normalized) == 0 {
		return
	}
	queue := make([]Track, 0, len(normalized)+1)
	queue = append(queue, seed)
	for _, track := range normalized {
		if track.VideoID == seed.VideoID {
			continue
		}
		queue = append(queue, track)
	}
	service.mu.Lock()
	if !service.hasCurrentTrack || service.currentTrack.VideoID != seed.VideoID {
		service.mu.Unlock()
		return
	}
	service.clearForwardSkipNavigationStackLocked()
	service.recordQueueStateForUndoLocked()
	service.queueKind = QueueKindRadio
	if service.shuffleEnabled && len(queue) > 1 {
		service.materializeShuffleQueueLocked(queue, 0, false, true)
	} else {
		service.queueOrderBeforeShuffle = nil
		service.queue = queue
		service.currentIndex = 0
	}
	service.mu.Unlock()
	service.requestTracksMetadataEnrichment(queue)
	service.saveCurrentSession(ctx)
}

func (service *PlayerService) recordQueueStateForUndoLocked() {
	service.queueUndo = append(service.queueUndo, QueueSnapshot{
		Queue:        cloneTracks(service.queue),
		CurrentIndex: service.currentIndex,
	})
	if len(service.queueUndo) > queueUndoMaxCount {
		service.queueUndo = append([]QueueSnapshot(nil), service.queueUndo[len(service.queueUndo)-queueUndoMaxCount:]...)
	}
	service.queueRedo = nil
}

func (service *PlayerService) realignCurrentIndexLocked() {
	if len(service.queue) == 0 {
		service.currentIndex = 0
		return
	}
	if service.hasCurrentTrack {
		for index, track := range service.queue {
			if track.VideoID == service.currentTrack.VideoID {
				service.currentIndex = index
				return
			}
		}
	}
	service.currentIndex = safeQueueIndex(service.currentIndex, len(service.queue))
}

func (service *PlayerService) realignCurrentTrackLocked() {
	if len(service.queue) == 0 {
		return
	}
	service.currentIndex = safeQueueIndex(service.currentIndex, len(service.queue))
	service.currentTrack = service.queue[service.currentIndex]
	service.hasCurrentTrack = true
	service.pendingPlayVideoID = service.currentTrack.VideoID
}

func normalizeTracks(tracks []Track) []Track {
	normalized := make([]Track, 0, len(tracks))
	for _, track := range tracks {
		track = normalizeTrack(track)
		if track.VideoID == "" {
			continue
		}
		normalized = append(normalized, track)
	}
	return normalized
}

func shuffleTracks(tracks []Track, random func(limit int) int) {
	if len(tracks) <= 1 {
		return
	}
	for index := len(tracks) - 1; index > 0; index-- {
		swap := safeQueueIndex(random(index+1), index+1)
		tracks[index], tracks[swap] = tracks[swap], tracks[index]
	}
}
