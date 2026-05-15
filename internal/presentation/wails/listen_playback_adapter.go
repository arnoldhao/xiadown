package wails

import (
	"context"
	"strings"

	"xiadown/internal/application/listenplayback"
	"xiadown/internal/application/youtubemusic"
)

const listenMixQueueLimit = 100

type listenPlaybackTransport struct {
	player *ListenYouTubeMusicPlayer
}

func NewListenPlaybackTransport(player *ListenYouTubeMusicPlayer) listenPlaybackTransport {
	return listenPlaybackTransport{player: player}
}

func (transport listenPlaybackTransport) LoadVideo(_ context.Context, request listenplayback.PlayRequest, strategy listenplayback.VideoLoadStrategy) error {
	if transport.player == nil {
		return nil
	}
	return transport.player.Play(ListenPlayerPlayRequest{
		VideoID:          request.Track.VideoID,
		Title:            request.Track.Title,
		Artist:           request.Track.Artist,
		StartSeconds:     request.StartSeconds,
		RestartFromStart: request.RestartFromStart,
		ForceReload:      request.ForceReload || strategy == listenplayback.VideoLoadForceFullPageWhenSameVideoID,
		Volume:           request.Volume,
		Muted:            request.Muted,
	})
}

func (transport listenPlaybackTransport) Play(context.Context) error {
	if transport.player == nil {
		return nil
	}
	return transport.player.Resume()
}

func (transport listenPlaybackTransport) Pause(context.Context) error {
	if transport.player == nil {
		return nil
	}
	return transport.player.Pause()
}

func (transport listenPlaybackTransport) Seek(_ context.Context, seconds float64) error {
	if transport.player == nil {
		return nil
	}
	return transport.player.Seek(seconds)
}

func (transport listenPlaybackTransport) SetVolume(_ context.Context, volume float64, muted bool) error {
	if transport.player == nil {
		return nil
	}
	return transport.player.SetVolume(volume, muted)
}

func (transport listenPlaybackTransport) Next(context.Context) error {
	if transport.player == nil {
		return nil
	}
	return transport.player.Next()
}

func (transport listenPlaybackTransport) Previous(context.Context) error {
	if transport.player == nil {
		return nil
	}
	return transport.player.Previous()
}

func (transport listenPlaybackTransport) CurrentVideoID(context.Context) string {
	if transport.player == nil {
		return ""
	}
	status := transport.player.Status()
	if strings.TrimSpace(status.ObservedVideoID) != "" {
		return strings.TrimSpace(status.ObservedVideoID)
	}
	return strings.TrimSpace(status.VideoID)
}

type listenPlaybackLibraryClient interface {
	PlaylistPage(ctx context.Context, playlistID string, continuation string, limit int) (youtubemusic.TrackListPage, error)
	Radio(ctx context.Context, videoID string, limit int) ([]youtubemusic.Track, error)
	TrackMetadata(ctx context.Context, videoID string) (youtubemusic.TrackMetadata, error)
}

type listenPlaybackLibraryAdapter struct {
	client listenPlaybackLibraryClient
}

func NewListenPlaybackLibraryClient(client listenPlaybackLibraryClient) listenPlaybackLibraryAdapter {
	return listenPlaybackLibraryAdapter{client: client}
}

func (adapter listenPlaybackLibraryAdapter) Radio(ctx context.Context, videoID string, limit int) ([]listenplayback.Track, error) {
	if adapter.client == nil {
		return nil, nil
	}
	tracks, err := adapter.client.Radio(ctx, videoID, limit)
	if err != nil {
		return nil, err
	}
	return listenPlaybackTracksFromYouTubeMusic(tracks), nil
}

func (adapter listenPlaybackLibraryAdapter) MixQueue(ctx context.Context, playlistID string, startVideoID string) (listenplayback.MixQueueResult, error) {
	if adapter.client == nil {
		return listenplayback.MixQueueResult{}, nil
	}
	page, err := adapter.client.PlaylistPage(ctx, playlistID, "", listenMixQueueLimit)
	if err != nil {
		return listenplayback.MixQueueResult{}, err
	}
	tracks := listenPlaybackTracksFromYouTubeMusic(page.Tracks)
	if startVideoID = strings.TrimSpace(startVideoID); startVideoID != "" {
		tracks = moveListenPlaybackTrackToFront(tracks, startVideoID)
	}
	return listenplayback.MixQueueResult{
		Tracks:            tracks,
		ContinuationToken: strings.TrimSpace(page.Continuation),
	}, nil
}

func (adapter listenPlaybackLibraryAdapter) MixQueueContinuation(ctx context.Context, continuation string) (listenplayback.MixQueueResult, error) {
	if adapter.client == nil {
		return listenplayback.MixQueueResult{}, nil
	}
	page, err := adapter.client.PlaylistPage(ctx, "", continuation, listenMixQueueLimit)
	if err != nil {
		return listenplayback.MixQueueResult{}, err
	}
	return listenplayback.MixQueueResult{
		Tracks:            listenPlaybackTracksFromYouTubeMusic(page.Tracks),
		ContinuationToken: strings.TrimSpace(page.Continuation),
	}, nil
}

func (adapter listenPlaybackLibraryAdapter) TrackMetadata(ctx context.Context, videoID string) (listenplayback.Track, error) {
	if adapter.client == nil {
		return listenplayback.Track{ID: videoID, VideoID: videoID, Title: videoID}, nil
	}
	metadata, err := adapter.client.TrackMetadata(ctx, videoID)
	if err != nil {
		return listenplayback.Track{}, err
	}
	return listenPlaybackTrackFromMetadata(metadata, videoID), nil
}

func listenPlaybackTracksFromYouTubeMusic(tracks []youtubemusic.Track) []listenplayback.Track {
	result := make([]listenplayback.Track, 0, len(tracks))
	for _, track := range tracks {
		converted := listenPlaybackTrackFromYouTubeMusic(track)
		if converted.VideoID == "" {
			continue
		}
		result = append(result, converted)
	}
	return result
}

func listenPlaybackTrackFromYouTubeMusic(track youtubemusic.Track) listenplayback.Track {
	videoID := strings.TrimSpace(track.VideoID)
	musicVideoType := strings.TrimSpace(track.MusicVideoType)
	hasVideo := listenMusicVideoTypeHasVideoForPlayback(musicVideoType)
	return listenplayback.Track{
		ID:                     strings.TrimSpace(track.ID),
		VideoID:                videoID,
		Title:                  strings.TrimSpace(track.Title),
		Artist:                 strings.TrimSpace(track.Channel),
		ArtistBrowseID:         strings.TrimSpace(track.ArtistBrowseID),
		DurationLabel:          strings.TrimSpace(track.DurationLabel),
		ThumbnailURL:           strings.TrimSpace(track.ThumbnailURL),
		MusicVideoType:         musicVideoType,
		HasVideo:               hasVideo,
		VideoAvailabilityKnown: hasVideo,
	}
}

func listenPlaybackTrackFromPlayRequest(request ListenPlayerPlayRequest) listenplayback.Track {
	videoID := strings.TrimSpace(request.VideoID)
	return listenplayback.Track{
		ID:      videoID,
		VideoID: videoID,
		Title:   strings.TrimSpace(request.Title),
		Artist:  strings.TrimSpace(request.Artist),
	}
}

func listenPlaybackTrackFromMetadata(metadata youtubemusic.TrackMetadata, fallbackVideoID string) listenplayback.Track {
	videoID := strings.TrimSpace(metadata.VideoID)
	if videoID == "" {
		videoID = strings.TrimSpace(fallbackVideoID)
	}
	musicVideoType := strings.TrimSpace(metadata.MusicVideoType)
	likeStatus := ""
	if metadata.LikeStatusKnown {
		likeStatus = string(metadata.LikeStatus)
	}
	return listenplayback.Track{
		ID:                     videoID,
		VideoID:                videoID,
		Title:                  strings.TrimSpace(metadata.Title),
		Artist:                 strings.TrimSpace(metadata.Channel),
		ArtistBrowseID:         strings.TrimSpace(metadata.ArtistBrowseID),
		DurationLabel:          strings.TrimSpace(metadata.DurationLabel),
		ThumbnailURL:           strings.TrimSpace(metadata.ThumbnailURL),
		MusicVideoType:         musicVideoType,
		HasVideo:               listenMusicVideoTypeHasVideoForPlayback(musicVideoType),
		VideoAvailabilityKnown: musicVideoType != "",
		LikeStatus:             likeStatus,
	}
}

func moveListenPlaybackTrackToFront(tracks []listenplayback.Track, videoID string) []listenplayback.Track {
	for index, track := range tracks {
		if track.VideoID != videoID {
			continue
		}
		if index == 0 {
			return tracks
		}
		result := make([]listenplayback.Track, 0, len(tracks))
		result = append(result, track)
		result = append(result, tracks[:index]...)
		result = append(result, tracks[index+1:]...)
		return result
	}
	return tracks
}

func listenMusicVideoTypeHasVideoForPlayback(value string) bool {
	return strings.TrimSpace(value) == "MUSIC_VIDEO_TYPE_OMV"
}
