package wails

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"xiadown/internal/application/listenplayback"
	"xiadown/internal/application/youtubeworkspace"
)

type youtubeWorkspaceService interface {
	Browse(context.Context, youtubeworkspace.BrowseRequest) (youtubeworkspace.BrowsePage, error)
	PreparePlayback(youtubeworkspace.Video) (youtubeworkspace.PlaybackDescriptor, error)
}

type youtubeWorkspaceForceRefreshService interface {
	ForceRefresh()
}

type youtubeWorkspacePlayerResetter interface {
	Reset() error
}

// Kept separate from youtubeWorkspaceService so older test doubles and
// alternate workspace implementations remain source-compatible.
type youtubeWorkspaceVideoDetailsService interface {
	VideoDetails(context.Context, youtubeworkspace.VideoDetailsRequest) (youtubeworkspace.VideoDetails, error)
}

type youtubeWorkspaceVideoRatingService interface {
	RateVideo(context.Context, youtubeworkspace.VideoRatingRequest) error
}

type youtubeWorkspaceChannelSubscriptionService interface {
	SetChannelSubscription(context.Context, youtubeworkspace.ChannelSubscriptionRequest) error
}

type youtubeWorkspaceUploaderService interface {
	Uploader(context.Context, youtubeworkspace.UploaderRequest) (youtubeworkspace.UploaderPage, error)
}

type youtubeWorkspacePlayer interface {
	Play(ListenPlayerPlayRequest) error
}

type YouTubeWorkspacePlayVideoRequest struct {
	RequestID uint64                 `json:"requestId"`
	Video     youtubeworkspace.Video `json:"video"`
	Locale    string                 `json:"locale,omitempty"`
}

type YouTubeWorkspacePlayRequest struct {
	RequestID uint64 `json:"requestId"`
}

type youtubeWorkspaceCommittedPlay struct {
	RequestID uint64
	SessionID string
}

const (
	maxYouTubeWorkspacePlayTransactions         = 64
	youtubeWorkspacePlaybackLanguageMetadataKey = "language"
)

// YouTubeWorkspaceHandler exposes browse separately from playback. Browse is
// backed by the regular YouTube WEB InnerTube client and the YouTube App
// Session, while playback reuses the native watch-page surface and returns
// coordinator-ready media metadata to the workspace shell.
type YouTubeWorkspaceHandler struct {
	service                youtubeWorkspaceService
	player                 youtubeWorkspacePlayer
	coordinator            *listenplayback.PlaybackCoordinator
	browseMu               sync.Mutex
	browseCancel           context.CancelFunc
	browseGeneration       uint64
	playMu                 sync.Mutex
	playGeneration         uint64
	playLatestRequestID    uint64
	playSeen               map[uint64]struct{}
	playFinished           map[uint64]struct{}
	playCanceled           map[uint64]struct{}
	playCommitted          map[uint64]youtubeWorkspaceCommittedPlay
	playCommittedBySession map[string]uint64
}

func NewYouTubeWorkspaceHandler(
	service youtubeWorkspaceService,
	player youtubeWorkspacePlayer,
	coordinator ...*listenplayback.PlaybackCoordinator,
) *YouTubeWorkspaceHandler {
	handler := &YouTubeWorkspaceHandler{
		service:                service,
		player:                 player,
		playSeen:               make(map[uint64]struct{}),
		playFinished:           make(map[uint64]struct{}),
		playCanceled:           make(map[uint64]struct{}),
		playCommitted:          make(map[uint64]youtubeWorkspaceCommittedPlay),
		playCommittedBySession: make(map[string]uint64),
	}
	if len(coordinator) > 0 {
		handler.coordinator = coordinator[0]
	}
	return handler
}

func (handler *YouTubeWorkspaceHandler) ServiceName() string {
	return "YouTubeWorkspaceHandler"
}

func (handler *YouTubeWorkspaceHandler) ForceRefresh(_ context.Context) error {
	if handler == nil {
		return fmt.Errorf("youtube workspace unavailable")
	}
	handler.browseMu.Lock()
	if handler.browseCancel != nil {
		handler.browseCancel()
		handler.browseCancel = nil
	}
	handler.browseGeneration++
	handler.browseMu.Unlock()
	if refresher, ok := handler.service.(youtubeWorkspaceForceRefreshService); ok {
		refresher.ForceRefresh()
	}
	if resetter, ok := handler.player.(youtubeWorkspacePlayerResetter); ok {
		return resetter.Reset()
	}
	return nil
}

func (handler *YouTubeWorkspaceHandler) Browse(
	ctx context.Context,
	request youtubeworkspace.BrowseRequest,
) (youtubeworkspace.BrowsePage, error) {
	if handler == nil || handler.service == nil {
		return youtubeworkspace.BrowsePage{}, fmt.Errorf("youtube workspace unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	browseCtx, cancel := context.WithCancel(ctx)
	handler.browseMu.Lock()
	if handler.browseCancel != nil {
		handler.browseCancel()
	}
	handler.browseGeneration++
	generation := handler.browseGeneration
	handler.browseCancel = cancel
	handler.browseMu.Unlock()
	defer func() {
		cancel()
		handler.browseMu.Lock()
		if handler.browseGeneration == generation {
			handler.browseCancel = nil
		}
		handler.browseMu.Unlock()
	}()
	return handler.service.Browse(browseCtx, request)
}

func (handler *YouTubeWorkspaceHandler) VideoDetails(
	ctx context.Context,
	request youtubeworkspace.VideoDetailsRequest,
) (youtubeworkspace.VideoDetails, error) {
	if handler == nil || handler.service == nil {
		return youtubeworkspace.VideoDetails{}, fmt.Errorf("youtube workspace unavailable")
	}
	detailService, ok := handler.service.(youtubeWorkspaceVideoDetailsService)
	if !ok {
		return youtubeworkspace.VideoDetails{}, fmt.Errorf("youtube video details unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return detailService.VideoDetails(ctx, request)
}

func (handler *YouTubeWorkspaceHandler) RateVideo(
	ctx context.Context,
	request youtubeworkspace.VideoRatingRequest,
) error {
	if handler == nil || handler.service == nil {
		return fmt.Errorf("youtube workspace unavailable")
	}
	ratingService, ok := handler.service.(youtubeWorkspaceVideoRatingService)
	if !ok {
		return fmt.Errorf("youtube video rating unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return ratingService.RateVideo(ctx, request)
}

func (handler *YouTubeWorkspaceHandler) SetChannelSubscription(
	ctx context.Context,
	request youtubeworkspace.ChannelSubscriptionRequest,
) error {
	if handler == nil || handler.service == nil {
		return fmt.Errorf("youtube workspace unavailable")
	}
	subscriptionService, ok := handler.service.(youtubeWorkspaceChannelSubscriptionService)
	if !ok {
		return fmt.Errorf("youtube channel subscription unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return subscriptionService.SetChannelSubscription(ctx, request)
}

func (handler *YouTubeWorkspaceHandler) Uploader(
	ctx context.Context,
	request youtubeworkspace.UploaderRequest,
) (youtubeworkspace.UploaderPage, error) {
	if handler == nil || handler.service == nil {
		return youtubeworkspace.UploaderPage{}, fmt.Errorf("youtube workspace unavailable")
	}
	uploaderService, ok := handler.service.(youtubeWorkspaceUploaderService)
	if !ok {
		return youtubeworkspace.UploaderPage{}, fmt.Errorf("youtube uploader unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return uploaderService.Uploader(ctx, request)
}

func (handler *YouTubeWorkspaceHandler) PlayVideo(
	ctx context.Context,
	video youtubeworkspace.Video,
) (youtubeworkspace.PlaybackDescriptor, error) {
	return handler.playVideo(ctx, video, 0, "")
}

func (handler *YouTubeWorkspaceHandler) PlayVideoRequest(
	ctx context.Context,
	request YouTubeWorkspacePlayVideoRequest,
) (youtubeworkspace.PlaybackDescriptor, error) {
	if request.RequestID == 0 {
		return youtubeworkspace.PlaybackDescriptor{}, fmt.Errorf("youtube play request id is required")
	}
	return handler.playVideo(ctx, request.Video, request.RequestID, request.Locale)
}

func (handler *YouTubeWorkspaceHandler) playVideo(
	ctx context.Context,
	video youtubeworkspace.Video,
	requestID uint64,
	language string,
) (youtubeworkspace.PlaybackDescriptor, error) {
	if handler == nil || handler.service == nil || (handler.coordinator == nil && handler.player == nil) {
		return youtubeworkspace.PlaybackDescriptor{}, fmt.Errorf("youtube workspace player unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	handler.playMu.Lock()
	handler.ensurePlayStateLocked()
	if requestID != 0 {
		if _, finished := handler.playFinished[requestID]; finished {
			handler.playMu.Unlock()
			return youtubeworkspace.PlaybackDescriptor{}, context.Canceled
		}
		handler.playSeen[requestID] = struct{}{}
		handler.prunePlayStateLocked()
		if _, canceled := handler.playCanceled[requestID]; canceled {
			delete(handler.playCanceled, requestID)
			delete(handler.playSeen, requestID)
			handler.playFinished[requestID] = struct{}{}
			handler.prunePlayStateLocked()
			handler.playMu.Unlock()
			return youtubeworkspace.PlaybackDescriptor{}, context.Canceled
		}
		handler.playLatestRequestID = requestID
	}
	handler.playGeneration++
	generation := handler.playGeneration
	handler.playMu.Unlock()
	descriptor, err := handler.service.PreparePlayback(video)
	if err != nil {
		if requestID != 0 {
			handler.playMu.Lock()
			delete(handler.playSeen, requestID)
			handler.playFinished[requestID] = struct{}{}
			handler.prunePlayStateLocked()
			if handler.playLatestRequestID == requestID {
				handler.playLatestRequestID = 0
			}
			handler.playMu.Unlock()
		}
		return youtubeworkspace.PlaybackDescriptor{}, err
	}
	language = normalizeListenPlayerLanguage(language)
	metadata := map[string]string{
		"channelId":      descriptor.ChannelID,
		"viewCount":      strconv.FormatInt(descriptor.ViewCount, 10),
		"publishedLabel": descriptor.PublishedLabel,
	}
	if language != "" {
		metadata[youtubeWorkspacePlaybackLanguageMetadataKey] = language
	}
	// Keep the generation check and focus acquisition in one critical section.
	// Otherwise an older, slower RPC can pass the check, resume after a newer
	// request has started, and replace the coordinator with stale playback.
	handler.playMu.Lock()
	defer handler.playMu.Unlock()
	_, canceled := handler.playCanceled[requestID]
	if generation != handler.playGeneration || (requestID != 0 && canceled) {
		if requestID != 0 {
			delete(handler.playCanceled, requestID)
			delete(handler.playSeen, requestID)
			handler.playFinished[requestID] = struct{}{}
			if handler.playLatestRequestID == requestID {
				handler.playLatestRequestID = 0
			}
		}
		return youtubeworkspace.PlaybackDescriptor{}, context.Canceled
	}
	if handler.coordinator != nil {
		snapshot, err := handler.coordinator.StartSession(ctx, listenplayback.PlaybackSessionRequest{
			Focus:          listenplayback.PlaybackFocusPersistent,
			RetainRollback: requestID != 0,
			Item: listenplayback.MediaItem{
				ID:           descriptor.VideoID,
				Kind:         listenplayback.MediaKindVideo,
				Source:       listenplayback.PlaybackSource{Provider: listenplayback.PlaybackProviderYouTube, ID: descriptor.VideoID},
				Title:        descriptor.Title,
				Artist:       descriptor.Artist,
				ArtworkURL:   descriptor.ThumbnailURL,
				CanonicalURL: descriptor.WebURL,
				Duration:     descriptor.DurationSeconds,
				Metadata:     metadata,
			},
		})
		if err != nil {
			handler.finishPlayRequestLocked(requestID)
			return youtubeworkspace.PlaybackDescriptor{}, err
		}
		if snapshot.Active == nil ||
			snapshot.Active.Item.Source.Provider != listenplayback.PlaybackProviderYouTube ||
			snapshot.Active.ID == "" {
			if snapshot.Active != nil {
				handler.coordinator.AcceptSessionRollback(snapshot.Active.ID)
			}
			handler.finishPlayRequestLocked(requestID)
			return youtubeworkspace.PlaybackDescriptor{}, fmt.Errorf("youtube playback session identity unavailable")
		}
		descriptor.SessionID = snapshot.Active.ID
		if requestID != 0 {
			record := youtubeWorkspaceCommittedPlay{
				RequestID: requestID,
				SessionID: descriptor.SessionID,
			}
			handler.playCommitted[requestID] = record
			handler.playCommittedBySession[record.SessionID] = requestID
			handler.prunePlayStateLocked()
		}
		return descriptor, nil
	}
	if err := handler.player.Play(ListenPlayerPlayRequest{
		VideoID:      descriptor.VideoID,
		Title:        descriptor.Title,
		Artist:       descriptor.Artist,
		Language:     language,
		Volume:       1,
		StartSeconds: 0,
	}); err != nil {
		handler.finishPlayRequestLocked(requestID)
		return youtubeworkspace.PlaybackDescriptor{}, err
	}
	return descriptor, nil
}

func (handler *YouTubeWorkspaceHandler) AcceptPlay(
	_ context.Context,
	request YouTubeWorkspacePlayRequest,
) error {
	if handler == nil {
		return fmt.Errorf("youtube workspace player unavailable")
	}
	if request.RequestID == 0 {
		return fmt.Errorf("youtube play request id is required")
	}
	handler.playMu.Lock()
	defer handler.playMu.Unlock()
	handler.ensurePlayStateLocked()
	if _, finished := handler.playFinished[request.RequestID]; finished {
		return nil
	}
	if record, ok := handler.playCommitted[request.RequestID]; ok && handler.coordinator != nil {
		handler.coordinator.AcceptSessionRollback(record.SessionID)
	}
	handler.forgetAcceptedPlayLocked(request.RequestID)
	delete(handler.playCanceled, request.RequestID)
	delete(handler.playSeen, request.RequestID)
	handler.playFinished[request.RequestID] = struct{}{}
	handler.prunePlayStateLocked()
	if handler.playLatestRequestID == request.RequestID {
		handler.playLatestRequestID = 0
	}
	return nil
}

func (handler *YouTubeWorkspaceHandler) CancelPlay(
	ctx context.Context,
	request YouTubeWorkspacePlayRequest,
) error {
	if handler == nil {
		return fmt.Errorf("youtube workspace player unavailable")
	}
	if request.RequestID == 0 {
		return fmt.Errorf("youtube play request id is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	handler.playMu.Lock()
	defer handler.playMu.Unlock()
	handler.ensurePlayStateLocked()
	if _, finished := handler.playFinished[request.RequestID]; finished {
		return nil
	}
	_, seen := handler.playSeen[request.RequestID]
	handler.playCanceled[request.RequestID] = struct{}{}
	handler.prunePlayStateLocked()
	if handler.playLatestRequestID == request.RequestID {
		handler.playGeneration++
		handler.playLatestRequestID = 0
	}
	record, committed := handler.playCommitted[request.RequestID]
	if !committed || handler.coordinator == nil {
		if seen {
			delete(handler.playSeen, request.RequestID)
			delete(handler.playCanceled, request.RequestID)
			handler.playFinished[request.RequestID] = struct{}{}
			handler.prunePlayStateLocked()
		}
		return nil
	}
	active := handler.coordinator.Snapshot().Active
	if active == nil || active.ID != record.SessionID {
		_, activeIsPendingPlay := handler.playCommittedBySession[playbackSessionID(active)]
		if !activeIsPendingPlay {
			handler.coordinator.AcceptSessionRollback(record.SessionID)
			handler.removeCommittedPlayLocked(record)
			delete(handler.playSeen, request.RequestID)
			delete(handler.playCanceled, request.RequestID)
			handler.playFinished[request.RequestID] = struct{}{}
			handler.prunePlayStateLocked()
		}
		return nil
	}

	_, err := handler.coordinator.RollbackSession(ctx, record.SessionID)
	if err != nil &&
		!errors.Is(err, listenplayback.ErrPlaybackSessionNotActive) &&
		!errors.Is(err, listenplayback.ErrPlaybackRollbackUnavailable) {
		// Keep the committed transaction intact for retryable failures such as a
		// canceled RPC context. The coordinator still owns both the active
		// session and its rollback point in that case, so consuming the request
		// here would make the cancellation impossible to retry.
		return err
	}
	handler.removeCommittedPlayLocked(record)
	delete(handler.playCanceled, record.RequestID)
	delete(handler.playSeen, record.RequestID)
	handler.playFinished[record.RequestID] = struct{}{}
	handler.prunePlayStateLocked()
	return nil
}

func (handler *YouTubeWorkspaceHandler) ensurePlayStateLocked() {
	if handler.playSeen == nil {
		handler.playSeen = make(map[uint64]struct{})
	}
	if handler.playFinished == nil {
		handler.playFinished = make(map[uint64]struct{})
	}
	if handler.playCanceled == nil {
		handler.playCanceled = make(map[uint64]struct{})
	}
	if handler.playCommitted == nil {
		handler.playCommitted = make(map[uint64]youtubeWorkspaceCommittedPlay)
	}
	if handler.playCommittedBySession == nil {
		handler.playCommittedBySession = make(map[string]uint64)
	}
}

func (handler *YouTubeWorkspaceHandler) finishPlayRequestLocked(requestID uint64) {
	if requestID == 0 {
		return
	}
	delete(handler.playSeen, requestID)
	delete(handler.playCanceled, requestID)
	handler.playFinished[requestID] = struct{}{}
	if handler.playLatestRequestID == requestID {
		handler.playLatestRequestID = 0
	}
	handler.prunePlayStateLocked()
}

func (handler *YouTubeWorkspaceHandler) removeCommittedPlayLocked(
	record youtubeWorkspaceCommittedPlay,
) {
	delete(handler.playCommitted, record.RequestID)
	if handler.playCommittedBySession[record.SessionID] == record.RequestID {
		delete(handler.playCommittedBySession, record.SessionID)
	}
}

func (handler *YouTubeWorkspaceHandler) forgetAcceptedPlayLocked(requestID uint64) {
	for candidateID, record := range handler.playCommitted {
		if candidateID > requestID {
			continue
		}
		handler.removeCommittedPlayLocked(record)
		delete(handler.playCanceled, candidateID)
		delete(handler.playSeen, candidateID)
		handler.playFinished[candidateID] = struct{}{}
	}
}

func (handler *YouTubeWorkspaceHandler) prunePlayStateLocked() {
	ids := make(map[uint64]struct{}, len(handler.playSeen)+len(handler.playFinished)+len(handler.playCanceled)+len(handler.playCommitted))
	for requestID := range handler.playSeen {
		ids[requestID] = struct{}{}
	}
	for requestID := range handler.playCanceled {
		ids[requestID] = struct{}{}
	}
	for requestID := range handler.playFinished {
		ids[requestID] = struct{}{}
	}
	for requestID := range handler.playCommitted {
		ids[requestID] = struct{}{}
	}
	if len(ids) <= maxYouTubeWorkspacePlayTransactions {
		return
	}
	ordered := make([]uint64, 0, len(ids))
	for requestID := range ids {
		ordered = append(ordered, requestID)
	}
	sort.Slice(ordered, func(left int, right int) bool {
		return ordered[left] < ordered[right]
	})
	for _, requestID := range ordered[:len(ordered)-maxYouTubeWorkspacePlayTransactions] {
		if requestID == handler.playLatestRequestID {
			continue
		}
		if record, ok := handler.playCommitted[requestID]; ok {
			if handler.coordinator != nil {
				handler.coordinator.AcceptSessionRollback(record.SessionID)
			}
			handler.removeCommittedPlayLocked(record)
		}
		delete(handler.playSeen, requestID)
		delete(handler.playFinished, requestID)
		delete(handler.playCanceled, requestID)
	}
}

func playbackSessionID(snapshot *listenplayback.PlaybackSessionSnapshot) string {
	if snapshot == nil {
		return ""
	}
	return snapshot.ID
}
