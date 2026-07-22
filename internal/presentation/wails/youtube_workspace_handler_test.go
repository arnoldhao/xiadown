package wails

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"xiadown/internal/application/listenplayback"
	"xiadown/internal/application/youtubeworkspace"
)

type youtubeWorkspaceServiceStub struct {
	page       youtubeworkspace.BrowsePage
	descriptor youtubeworkspace.PlaybackDescriptor
}

type youtubeWorkspaceVideoDetailsServiceStub struct {
	youtubeWorkspaceServiceStub
	request youtubeworkspace.VideoDetailsRequest
	result  youtubeworkspace.VideoDetails
}

type youtubeWorkspaceVideoRatingServiceStub struct {
	youtubeWorkspaceServiceStub
	request youtubeworkspace.VideoRatingRequest
}

type youtubeWorkspaceChannelSubscriptionServiceStub struct {
	youtubeWorkspaceServiceStub
	request youtubeworkspace.ChannelSubscriptionRequest
}

type youtubeWorkspaceUploaderServiceStub struct {
	youtubeWorkspaceServiceStub
	request youtubeworkspace.UploaderRequest
	result  youtubeworkspace.UploaderPage
}

func (stub *youtubeWorkspaceUploaderServiceStub) Uploader(
	_ context.Context,
	request youtubeworkspace.UploaderRequest,
) (youtubeworkspace.UploaderPage, error) {
	stub.request = request
	return stub.result, nil
}

func (stub *youtubeWorkspaceChannelSubscriptionServiceStub) SetChannelSubscription(
	_ context.Context,
	request youtubeworkspace.ChannelSubscriptionRequest,
) error {
	stub.request = request
	return nil
}

func (stub *youtubeWorkspaceVideoRatingServiceStub) RateVideo(
	_ context.Context,
	request youtubeworkspace.VideoRatingRequest,
) error {
	stub.request = request
	return nil
}

func (stub *youtubeWorkspaceVideoDetailsServiceStub) VideoDetails(
	_ context.Context,
	request youtubeworkspace.VideoDetailsRequest,
) (youtubeworkspace.VideoDetails, error) {
	stub.request = request
	return stub.result, nil
}

type supersedingYouTubeWorkspaceService struct {
	youtubeWorkspaceServiceStub
	calls   atomic.Int32
	started chan int
}

type supersedingYouTubePlaybackService struct {
	started      chan string
	releaseFirst chan struct{}
	firstVideoID string
}

func (stub *supersedingYouTubePlaybackService) Browse(
	context.Context,
	youtubeworkspace.BrowseRequest,
) (youtubeworkspace.BrowsePage, error) {
	return youtubeworkspace.BrowsePage{}, nil
}

func (stub *supersedingYouTubePlaybackService) PreparePlayback(
	video youtubeworkspace.Video,
) (youtubeworkspace.PlaybackDescriptor, error) {
	stub.started <- video.VideoID
	if video.VideoID == stub.firstVideoID {
		<-stub.releaseFirst
	}
	return youtubeworkspace.PlaybackDescriptor{
		Source: "youtube", MediaKind: "video", VideoID: video.VideoID, Title: video.Title,
	}, nil
}

func (stub *supersedingYouTubeWorkspaceService) Browse(
	ctx context.Context,
	_ youtubeworkspace.BrowseRequest,
) (youtubeworkspace.BrowsePage, error) {
	call := int(stub.calls.Add(1))
	stub.started <- call
	if call == 1 {
		<-ctx.Done()
		return youtubeworkspace.BrowsePage{}, ctx.Err()
	}
	return stub.page, nil
}

func TestYouTubeWorkspaceHandlerSupersedesStaleBrowse(t *testing.T) {
	t.Parallel()
	service := &supersedingYouTubeWorkspaceService{
		youtubeWorkspaceServiceStub: youtubeWorkspaceServiceStub{
			page: youtubeworkspace.BrowsePage{RouteID: "search"},
		},
		started: make(chan int, 2),
	}
	handler := NewYouTubeWorkspaceHandler(service, nil)
	firstResult := make(chan error, 1)
	go func() {
		_, err := handler.Browse(context.Background(), youtubeworkspace.BrowseRequest{RouteID: "home"})
		firstResult <- err
	}()
	select {
	case call := <-service.started:
		if call != 1 {
			t.Fatalf("first browse call = %d", call)
		}
	case <-time.After(time.Second):
		t.Fatal("first browse did not start")
	}
	page, err := handler.Browse(context.Background(), youtubeworkspace.BrowseRequest{RouteID: "search"})
	if err != nil {
		t.Fatalf("replacement browse: %v", err)
	}
	if page.RouteID != "search" {
		t.Fatalf("replacement page = %+v", page)
	}
	select {
	case err := <-firstResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("stale browse error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stale browse was not canceled")
	}
}

func TestYouTubeWorkspaceHandlerLoadsOptionalVideoDetails(t *testing.T) {
	t.Parallel()
	service := &youtubeWorkspaceVideoDetailsServiceStub{
		result: youtubeworkspace.VideoDetails{
			VideoID: "AbCdEfGh123",
			Title:   "Detailed video",
		},
	}
	handler := NewYouTubeWorkspaceHandler(service, nil)
	request := youtubeworkspace.VideoDetailsRequest{VideoID: "AbCdEfGh123", Locale: "zh-CN"}
	details, err := handler.VideoDetails(context.Background(), request)
	if err != nil {
		t.Fatalf("VideoDetails: %v", err)
	}
	if details != service.result || service.request != request {
		t.Fatalf("details = %#v, request = %#v", details, service.request)
	}
}

func TestYouTubeWorkspaceHandlerKeepsLegacyServiceCompatibleWithoutDetails(t *testing.T) {
	t.Parallel()
	handler := NewYouTubeWorkspaceHandler(youtubeWorkspaceServiceStub{}, nil)
	_, err := handler.VideoDetails(
		context.Background(),
		youtubeworkspace.VideoDetailsRequest{VideoID: "AbCdEfGh123"},
	)
	if err == nil || err.Error() != "youtube video details unavailable" {
		t.Fatalf("legacy service error = %v", err)
	}
}

func TestYouTubeWorkspaceHandlerRatesVideoThroughOptionalService(t *testing.T) {
	t.Parallel()
	service := &youtubeWorkspaceVideoRatingServiceStub{}
	handler := NewYouTubeWorkspaceHandler(service, nil)
	request := youtubeworkspace.VideoRatingRequest{
		VideoID: "AbCdEfGh123",
		Rating:  youtubeworkspace.VideoRatingLike,
	}
	if err := handler.RateVideo(context.Background(), request); err != nil {
		t.Fatalf("RateVideo: %v", err)
	}
	if service.request != request {
		t.Fatalf("rating request = %#v", service.request)
	}
}

func TestYouTubeWorkspaceHandlerSetsChannelSubscriptionThroughOptionalService(t *testing.T) {
	t.Parallel()
	service := &youtubeWorkspaceChannelSubscriptionServiceStub{}
	handler := NewYouTubeWorkspaceHandler(service, nil)
	request := youtubeworkspace.ChannelSubscriptionRequest{
		ChannelID:  "UCabcdefghijklmnopqrstuv",
		Subscribed: true,
	}
	if err := handler.SetChannelSubscription(context.Background(), request); err != nil {
		t.Fatalf("SetChannelSubscription: %v", err)
	}
	if service.request != request {
		t.Fatalf("subscription request = %#v", service.request)
	}
}

func TestYouTubeWorkspaceHandlerLoadsOptionalUploaderPage(t *testing.T) {
	t.Parallel()
	service := &youtubeWorkspaceUploaderServiceStub{
		result: youtubeworkspace.UploaderPage{
			ChannelID: "UCabcdefghijklmnopqrstuv",
			Name:      "Workspace Creator",
		},
	}
	handler := NewYouTubeWorkspaceHandler(service, nil)
	request := youtubeworkspace.UploaderRequest{
		ChannelID: "UCabcdefghijklmnopqrstuv",
		Locale:    "zh-CN",
	}
	page, err := handler.Uploader(context.Background(), request)
	if err != nil {
		t.Fatalf("Uploader: %v", err)
	}
	if !reflect.DeepEqual(page, service.result) || service.request != request {
		t.Fatalf("uploader = %#v, request = %#v", page, service.request)
	}
}

func TestYouTubeWorkspaceHandlerKeepsLegacyServiceCompatibleWithoutUploader(t *testing.T) {
	t.Parallel()
	handler := NewYouTubeWorkspaceHandler(youtubeWorkspaceServiceStub{}, nil)
	_, err := handler.Uploader(
		context.Background(),
		youtubeworkspace.UploaderRequest{ChannelID: "UCabcdefghijklmnopqrstuv"},
	)
	if err == nil || err.Error() != "youtube uploader unavailable" {
		t.Fatalf("legacy service error = %v", err)
	}
}

func TestYouTubeWorkspaceHandlerSupersedesStalePlaybackBeforeFocusAcquisition(t *testing.T) {
	transport := newFakeListenLivePlaybackPlayer()
	backend := NewListenLivePlayerBackend(listenplayback.PlaybackProviderYouTube, transport)
	coordinator, err := listenplayback.NewPlaybackCoordinator(backend)
	if err != nil {
		t.Fatalf("NewPlaybackCoordinator: %v", err)
	}
	const firstVideoID = "AbCdEfGh123"
	const secondVideoID = "ZyXwVuTs987"
	service := &supersedingYouTubePlaybackService{
		started:      make(chan string, 2),
		releaseFirst: make(chan struct{}),
		firstVideoID: firstVideoID,
	}
	handler := NewYouTubeWorkspaceHandler(service, nil, coordinator)
	firstResult := make(chan error, 1)
	go func() {
		_, err := handler.PlayVideo(context.Background(), youtubeworkspace.Video{VideoID: firstVideoID, Title: "First"})
		firstResult <- err
	}()
	select {
	case videoID := <-service.started:
		if videoID != firstVideoID {
			t.Fatalf("first playback = %q", videoID)
		}
	case <-time.After(time.Second):
		t.Fatal("first playback did not start preparing")
	}
	second, err := handler.PlayVideo(context.Background(), youtubeworkspace.Video{VideoID: secondVideoID, Title: "Second"})
	if err != nil {
		t.Fatalf("second playback: %v", err)
	}
	if second.VideoID != secondVideoID {
		t.Fatalf("second descriptor = %+v", second)
	}
	close(service.releaseFirst)
	select {
	case err := <-firstResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("stale playback error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stale playback did not finish")
	}
	snapshot := coordinator.Snapshot()
	if snapshot.Active == nil || snapshot.Active.Item.ID != secondVideoID ||
		snapshot.Active.Item.Title != "Second" || snapshot.Active.State != listenplayback.PlaybackStatePlaying {
		t.Fatalf("stale RPC replaced latest coordinator focus: %+v", snapshot)
	}
}

func TestYouTubeWorkspaceHandlerCancelsPreparedPlaybackBeforeFocusAcquisition(t *testing.T) {
	transport := newFakeListenLivePlaybackPlayer()
	backend := NewListenLivePlayerBackend(listenplayback.PlaybackProviderYouTube, transport)
	coordinator, err := listenplayback.NewPlaybackCoordinator(backend)
	if err != nil {
		t.Fatalf("NewPlaybackCoordinator: %v", err)
	}
	const previousVideoID = "AbCdEfGh123"
	const pendingVideoID = "ZyXwVuTs987"
	service := &supersedingYouTubePlaybackService{
		started:      make(chan string, 2),
		releaseFirst: make(chan struct{}),
		firstVideoID: pendingVideoID,
	}
	handler := NewYouTubeWorkspaceHandler(service, nil, coordinator)
	previous, err := handler.PlayVideo(
		context.Background(),
		youtubeworkspace.Video{VideoID: previousVideoID, Title: "Previous"},
	)
	if err != nil {
		t.Fatalf("play previous: %v", err)
	}
	<-service.started

	result := make(chan error, 1)
	go func() {
		_, requestErr := handler.PlayVideoRequest(
			context.Background(),
			YouTubeWorkspacePlayVideoRequest{
				RequestID: 41,
				Video: youtubeworkspace.Video{
					VideoID: pendingVideoID,
					Title:   "Pending",
				},
			},
		)
		result <- requestErr
	}()
	select {
	case videoID := <-service.started:
		if videoID != pendingVideoID {
			t.Fatalf("pending playback = %q", videoID)
		}
	case <-time.After(time.Second):
		t.Fatal("pending playback did not start preparing")
	}
	if err := handler.CancelPlay(context.Background(), YouTubeWorkspacePlayRequest{RequestID: 41}); err != nil {
		t.Fatalf("CancelPlay: %v", err)
	}
	close(service.releaseFirst)
	select {
	case requestErr := <-result:
		if !errors.Is(requestErr, context.Canceled) {
			t.Fatalf("canceled playback error = %v", requestErr)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled playback did not finish")
	}
	if snapshot := coordinator.Snapshot(); snapshot.Active == nil ||
		snapshot.Active.ID != previous.SessionID || snapshot.Active.Item.ID != previousVideoID {
		t.Fatalf("canceled preparation replaced previous playback: %+v", snapshot)
	}
}

func TestYouTubeWorkspaceHandlerRollsBackCommittedCanceledPlayback(t *testing.T) {
	transport := newFakeListenLivePlaybackPlayer()
	backend := NewListenLivePlayerBackend(listenplayback.PlaybackProviderYouTube, transport)
	coordinator, err := listenplayback.NewPlaybackCoordinator(backend)
	if err != nil {
		t.Fatalf("NewPlaybackCoordinator: %v", err)
	}
	service := &supersedingYouTubePlaybackService{
		started:      make(chan string, 3),
		releaseFirst: make(chan struct{}),
		firstVideoID: "never-block",
	}
	handler := NewYouTubeWorkspaceHandler(service, nil, coordinator)
	previous, err := handler.PlayVideo(
		context.Background(),
		youtubeworkspace.Video{VideoID: "AbCdEfGh123", Title: "Previous"},
	)
	if err != nil {
		t.Fatalf("play previous: %v", err)
	}
	<-service.started
	pending, err := handler.PlayVideoRequest(
		context.Background(),
		YouTubeWorkspacePlayVideoRequest{
			RequestID: 42,
			Video: youtubeworkspace.Video{
				VideoID: "ZyXwVuTs987",
				Title:   "Canceled replacement",
			},
		},
	)
	if err != nil {
		t.Fatalf("play replacement: %v", err)
	}
	<-service.started
	if pending.SessionID == previous.SessionID {
		t.Fatalf("replacement reused previous session: old=%q new=%q", previous.SessionID, pending.SessionID)
	}
	if err := handler.CancelPlay(context.Background(), YouTubeWorkspacePlayRequest{RequestID: 42}); err != nil {
		t.Fatalf("CancelPlay: %v", err)
	}
	if snapshot := coordinator.Snapshot(); snapshot.Active == nil ||
		snapshot.Active.ID != previous.SessionID ||
		snapshot.Active.Item.ID != previous.VideoID ||
		snapshot.Active.State != listenplayback.PlaybackStatePlaying {
		t.Fatalf("cancel did not restore previous playback: %+v", snapshot)
	}

	accepted, err := handler.PlayVideoRequest(
		context.Background(),
		YouTubeWorkspacePlayVideoRequest{
			RequestID: 43,
			Video: youtubeworkspace.Video{
				VideoID: "LmNoPqRs456",
				Title:   "Accepted replacement",
			},
		},
	)
	if err != nil {
		t.Fatalf("play accepted replacement: %v", err)
	}
	<-service.started
	if err := handler.AcceptPlay(context.Background(), YouTubeWorkspacePlayRequest{RequestID: 43}); err != nil {
		t.Fatalf("AcceptPlay: %v", err)
	}
	if err := handler.CancelPlay(context.Background(), YouTubeWorkspacePlayRequest{RequestID: 43}); err != nil {
		t.Fatalf("cancel accepted play: %v", err)
	}
	if snapshot := coordinator.Snapshot(); snapshot.Active == nil ||
		snapshot.Active.ID != accepted.SessionID || snapshot.Active.Item.ID != accepted.VideoID {
		t.Fatalf("accepted playback was rolled back: %+v", snapshot)
	}
	handler.playMu.Lock()
	defer handler.playMu.Unlock()
	if len(handler.playSeen) != 0 || len(handler.playCanceled) != 0 ||
		len(handler.playCommitted) != 0 || len(handler.playCommittedBySession) != 0 {
		t.Fatalf("accepted transaction leaked mutable state: seen=%d canceled=%d committed=%d sessions=%d",
			len(handler.playSeen), len(handler.playCanceled), len(handler.playCommitted), len(handler.playCommittedBySession))
	}
}

func TestYouTubeWorkspaceHandlerRetriesCanceledPlaybackRollback(t *testing.T) {
	transport := newFakeListenLivePlaybackPlayer()
	backend := NewListenLivePlayerBackend(listenplayback.PlaybackProviderYouTube, transport)
	coordinator, err := listenplayback.NewPlaybackCoordinator(backend)
	if err != nil {
		t.Fatalf("NewPlaybackCoordinator: %v", err)
	}
	service := &supersedingYouTubePlaybackService{
		started:      make(chan string, 2),
		releaseFirst: make(chan struct{}),
		firstVideoID: "never-block",
	}
	handler := NewYouTubeWorkspaceHandler(service, nil, coordinator)
	previous, err := handler.PlayVideo(
		context.Background(),
		youtubeworkspace.Video{VideoID: "AbCdEfGh123", Title: "Previous"},
	)
	if err != nil {
		t.Fatalf("play previous: %v", err)
	}
	<-service.started
	pending, err := handler.PlayVideoRequest(
		context.Background(),
		YouTubeWorkspacePlayVideoRequest{
			RequestID: 44,
			Video: youtubeworkspace.Video{
				VideoID: "ZyXwVuTs987",
				Title:   "Canceled replacement",
			},
		},
	)
	if err != nil {
		t.Fatalf("play replacement: %v", err)
	}
	<-service.started

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := handler.CancelPlay(canceledCtx, YouTubeWorkspacePlayRequest{RequestID: 44}); !errors.Is(err, context.Canceled) {
		t.Fatalf("first CancelPlay error = %v, want context.Canceled", err)
	}
	if snapshot := coordinator.Snapshot(); snapshot.Active == nil || snapshot.Active.ID != pending.SessionID {
		t.Fatalf("failed rollback changed active playback: %+v", snapshot)
	}
	handler.playMu.Lock()
	_, committed := handler.playCommitted[44]
	_, finished := handler.playFinished[44]
	handler.playMu.Unlock()
	if !committed || finished {
		t.Fatalf("retryable rollback consumed transaction: committed=%t finished=%t", committed, finished)
	}

	if err := handler.CancelPlay(context.Background(), YouTubeWorkspacePlayRequest{RequestID: 44}); err != nil {
		t.Fatalf("retry CancelPlay: %v", err)
	}
	if snapshot := coordinator.Snapshot(); snapshot.Active == nil ||
		snapshot.Active.ID != previous.SessionID ||
		snapshot.Active.Item.ID != previous.VideoID ||
		snapshot.Active.State != listenplayback.PlaybackStatePlaying {
		t.Fatalf("retry did not restore previous playback: %+v", snapshot)
	}
	handler.playMu.Lock()
	_, committed = handler.playCommitted[44]
	_, finished = handler.playFinished[44]
	handler.playMu.Unlock()
	if committed || !finished {
		t.Fatalf("successful retry did not finish transaction: committed=%t finished=%t", committed, finished)
	}
}

func TestYouTubeWorkspaceHandlerBoundsUnknownCancellationTombstones(t *testing.T) {
	handler := NewYouTubeWorkspaceHandler(youtubeWorkspaceServiceStub{}, &youtubeWorkspacePlayerStub{})
	for requestID := uint64(1); requestID <= maxYouTubeWorkspacePlayTransactions*3; requestID++ {
		if err := handler.CancelPlay(
			context.Background(),
			YouTubeWorkspacePlayRequest{RequestID: requestID},
		); err != nil {
			t.Fatalf("CancelPlay(%d): %v", requestID, err)
		}
	}
	handler.playMu.Lock()
	defer handler.playMu.Unlock()
	tracked := make(map[uint64]struct{})
	for requestID := range handler.playSeen {
		tracked[requestID] = struct{}{}
	}
	for requestID := range handler.playFinished {
		tracked[requestID] = struct{}{}
	}
	for requestID := range handler.playCanceled {
		tracked[requestID] = struct{}{}
	}
	for requestID := range handler.playCommitted {
		tracked[requestID] = struct{}{}
	}
	if len(tracked) > maxYouTubeWorkspacePlayTransactions {
		t.Fatalf("tracked play transactions = %d, want <= %d", len(tracked), maxYouTubeWorkspacePlayTransactions)
	}
}

func TestYouTubeWorkspaceHandlerAcquiresCoordinatorFocus(t *testing.T) {
	transport := newFakeListenLivePlaybackPlayer()
	backend := NewListenLivePlayerBackend(listenplayback.PlaybackProviderYouTube, transport)
	coordinator, err := listenplayback.NewPlaybackCoordinator(backend)
	if err != nil {
		t.Fatalf("NewPlaybackCoordinator: %v", err)
	}
	legacyPlayer := &youtubeWorkspacePlayerStub{}
	descriptor := youtubeworkspace.PlaybackDescriptor{
		Source:          "youtube",
		MediaKind:       "video",
		VideoID:         "AbCdEfGh123",
		Title:           "Workspace video",
		Artist:          "Creator",
		ChannelID:       "UCabcdefghijklmnopqrstuv",
		ThumbnailURL:    "https://example.test/thumb.jpg",
		DurationSeconds: 93,
		ViewCount:       4567,
		PublishedLabel:  "2 days ago",
		WebURL:          "https://www.youtube.com/watch?v=AbCdEfGh123",
	}
	handler := NewYouTubeWorkspaceHandler(
		youtubeWorkspaceServiceStub{descriptor: descriptor},
		legacyPlayer,
		coordinator,
	)
	const requestID = 57
	result, err := handler.PlayVideoRequest(
		context.Background(),
		YouTubeWorkspacePlayVideoRequest{
			RequestID: requestID,
			Video:     youtubeworkspace.Video{VideoID: descriptor.VideoID},
			Locale:    "zh-Hans-CN",
		},
	)
	if err != nil {
		t.Fatalf("PlayVideoRequest: %v", err)
	}
	if result.SessionID == "" {
		t.Fatalf("descriptor has no coordinator session: %+v", result)
	}
	descriptor.SessionID = result.SessionID
	if result != descriptor {
		t.Fatalf("descriptor = %+v", result)
	}
	_, provider, _, request := transport.snapshot()
	if provider != listenplayback.PlaybackProviderYouTube || request.VideoID != descriptor.VideoID ||
		request.Title != descriptor.Title || request.Language != "zh-CN" {
		t.Fatalf("coordinator transport request = %q %+v", provider, request)
	}
	if legacyPlayer.request.VideoID != "" {
		t.Fatalf("legacy player was called directly: %+v", legacyPlayer.request)
	}
	snapshot := coordinator.Snapshot()
	if snapshot.Active == nil || snapshot.Active.ID != result.SessionID || snapshot.Active.Item.ArtworkURL != descriptor.ThumbnailURL ||
		snapshot.Active.Item.CanonicalURL != descriptor.WebURL || snapshot.Active.Item.Duration != descriptor.DurationSeconds ||
		snapshot.Active.Item.Metadata["channelId"] != descriptor.ChannelID ||
		snapshot.Active.Item.Metadata["viewCount"] != "4567" ||
		snapshot.Active.Item.Metadata["publishedLabel"] != descriptor.PublishedLabel ||
		snapshot.Active.Item.Metadata["language"] != "zh-CN" {
		t.Fatalf("coordinator snapshot = %+v", snapshot)
	}
	if err := handler.AcceptPlay(context.Background(), YouTubeWorkspacePlayRequest{RequestID: requestID}); err != nil {
		t.Fatalf("AcceptPlay: %v", err)
	}
}

func TestYouTubeWorkspaceHandlerPreservesGlobalVolumeAcrossVideos(t *testing.T) {
	transport := newFakeListenLivePlaybackPlayer()
	backend := NewListenLivePlayerBackend(listenplayback.PlaybackProviderYouTube, transport)
	coordinator, err := listenplayback.NewPlaybackCoordinator(backend)
	if err != nil {
		t.Fatalf("NewPlaybackCoordinator: %v", err)
	}
	first := youtubeworkspace.PlaybackDescriptor{
		Source: "youtube", MediaKind: "video", VideoID: "AbCdEfGh123", Title: "First",
	}
	firstHandler := NewYouTubeWorkspaceHandler(
		youtubeWorkspaceServiceStub{descriptor: first},
		nil,
		coordinator,
	)
	if _, err := firstHandler.PlayVideo(context.Background(), youtubeworkspace.Video{VideoID: first.VideoID}); err != nil {
		t.Fatalf("play first video: %v", err)
	}
	if _, err := coordinator.SetVolume(context.Background(), 0.23, false); err != nil {
		t.Fatalf("set volume: %v", err)
	}

	second := youtubeworkspace.PlaybackDescriptor{
		Source: "youtube", MediaKind: "video", VideoID: "ZyXwVuTs987", Title: "Second",
	}
	secondHandler := NewYouTubeWorkspaceHandler(
		youtubeWorkspaceServiceStub{descriptor: second},
		nil,
		coordinator,
	)
	if _, err := secondHandler.PlayVideo(context.Background(), youtubeworkspace.Video{VideoID: second.VideoID}); err != nil {
		t.Fatalf("play second video: %v", err)
	}
	_, _, _, request := transport.snapshot()
	if request.VideoID != second.VideoID || request.Volume != 0.23 || request.Muted {
		t.Fatalf("second playback reset global volume: %+v", request)
	}
	if snapshot := coordinator.Snapshot(); snapshot.Active == nil || snapshot.Active.Volume != 0.23 || snapshot.Active.Muted {
		t.Fatalf("coordinator lost global volume: %+v", snapshot)
	}
}

func (stub youtubeWorkspaceServiceStub) Browse(
	context.Context,
	youtubeworkspace.BrowseRequest,
) (youtubeworkspace.BrowsePage, error) {
	return stub.page, nil
}

func (stub youtubeWorkspaceServiceStub) PreparePlayback(
	youtubeworkspace.Video,
) (youtubeworkspace.PlaybackDescriptor, error) {
	return stub.descriptor, nil
}

type youtubeWorkspacePlayerStub struct {
	request ListenPlayerPlayRequest
}

func (stub *youtubeWorkspacePlayerStub) Play(request ListenPlayerPlayRequest) error {
	stub.request = request
	return nil
}

func TestYouTubeWorkspaceHandlerPlaysThroughExistingNativePlayer(t *testing.T) {
	t.Parallel()
	player := &youtubeWorkspacePlayerStub{}
	descriptor := youtubeworkspace.PlaybackDescriptor{
		Source:    "youtube",
		MediaKind: "video",
		VideoID:   "AbCdEfGh123",
		Title:     "Workspace video",
		Artist:    "Creator",
	}
	handler := NewYouTubeWorkspaceHandler(
		youtubeWorkspaceServiceStub{descriptor: descriptor},
		player,
	)

	result, err := handler.playVideo(
		context.Background(),
		youtubeworkspace.Video{VideoID: descriptor.VideoID},
		0,
		"zh-Hant-TW",
	)
	if err != nil {
		t.Fatalf("play video: %v", err)
	}
	if result != descriptor {
		t.Fatalf("unexpected descriptor: %#v", result)
	}
	if player.request.VideoID != descriptor.VideoID || player.request.Title != descriptor.Title ||
		player.request.Artist != descriptor.Artist || player.request.Language != "zh-TW" {
		t.Fatalf("unexpected native player request: %#v", player.request)
	}
}
