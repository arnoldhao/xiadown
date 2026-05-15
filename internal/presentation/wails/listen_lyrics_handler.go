package wails

import (
	"context"
	"fmt"
	"strings"

	"xiadown/internal/application/listenlyrics"
	"xiadown/internal/application/youtubemusic"
)

type listenLyricsClient interface {
	TrackLyrics(ctx context.Context, info youtubemusic.LyricsSearchInfo) (youtubemusic.LyricsResult, error)
}

type listenLyricsAdapter struct {
	client listenLyricsClient
}

func NewListenLyricsClient(client listenLyricsClient) listenLyricsAdapter {
	return listenLyricsAdapter{client: client}
}

func (adapter listenLyricsAdapter) TrackLyrics(ctx context.Context, request listenlyrics.Request) (listenlyrics.Snapshot, error) {
	if adapter.client == nil {
		return listenlyrics.Snapshot{VideoID: request.VideoID, Kind: listenlyrics.KindUnavailable}, nil
	}
	if strings.TrimSpace(request.Language) != "" {
		ctx = youtubemusic.WithLocale(ctx, request.Language)
	}
	result, err := adapter.client.TrackLyrics(ctx, youtubemusic.LyricsSearchInfo{
		VideoID:         strings.TrimSpace(request.VideoID),
		Title:           strings.TrimSpace(request.Title),
		Artist:          strings.TrimSpace(request.Artist),
		DurationSeconds: request.DurationSeconds,
		PlainOnly:       request.PlainOnly,
	})
	if err != nil {
		return listenlyrics.Snapshot{}, err
	}
	return listenLyricsSnapshotFromYouTubeMusic(strings.TrimSpace(request.VideoID), result), nil
}

func listenLyricsSnapshotFromYouTubeMusic(videoID string, result youtubemusic.LyricsResult) listenlyrics.Snapshot {
	return listenlyrics.Snapshot{
		VideoID:        videoID,
		Kind:           strings.TrimSpace(result.Kind),
		Source:         strings.TrimSpace(result.Source),
		Text:           result.Text,
		Lines:          listenLyricsLinesFromYouTubeMusic(result.Lines),
		ActiveProvider: strings.TrimSpace(result.Source),
	}
}

func listenLyricsLinesFromYouTubeMusic(lines []youtubemusic.LyricLine) []listenlyrics.Line {
	if len(lines) == 0 {
		return nil
	}
	result := make([]listenlyrics.Line, 0, len(lines))
	for _, line := range lines {
		result = append(result, listenlyrics.Line{
			StartMs:       line.StartMs,
			DurationMs:    line.DurationMs,
			Text:          line.Text,
			RomanizedText: line.RomanizedText,
			RomanizedKind: line.RomanizedKind,
			Words:         listenLyricsWordsFromYouTubeMusic(line.Words),
		})
	}
	return result
}

func listenLyricsWordsFromYouTubeMusic(words []youtubemusic.TimedWord) []listenlyrics.Word {
	if len(words) == 0 {
		return nil
	}
	result := make([]listenlyrics.Word, 0, len(words))
	for _, word := range words {
		result = append(result, listenlyrics.Word{
			StartMs: word.StartMs,
			Text:    word.Text,
		})
	}
	return result
}

type ListenLyricsHandler struct {
	service *listenlyrics.Service
}

type ListenLyricsTrackRequest struct {
	VideoID         string  `json:"videoId,omitempty"`
	Title           string  `json:"title,omitempty"`
	Artist          string  `json:"artist,omitempty"`
	Channel         string  `json:"channel,omitempty"`
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
}

type ListenLyricsRequest struct {
	Track           ListenLyricsTrackRequest `json:"track"`
	VideoID         string                   `json:"videoId,omitempty"`
	Title           string                   `json:"title,omitempty"`
	Artist          string                   `json:"artist,omitempty"`
	DurationSeconds float64                  `json:"durationSeconds,omitempty"`
	PlainOnly       bool                     `json:"plainOnly,omitempty"`
	Language        string                   `json:"language,omitempty"`
}

func NewListenLyricsHandler(service *listenlyrics.Service) *ListenLyricsHandler {
	return &ListenLyricsHandler{service: service}
}

func (handler *ListenLyricsHandler) TrackLyrics(ctx context.Context, request ListenLyricsRequest) (listenlyrics.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenlyrics.Snapshot{}, fmt.Errorf("listen lyrics service unavailable")
	}
	return handler.service.TrackLyrics(ctx, listenlyrics.Request{
		VideoID:         firstNonEmpty(request.VideoID, request.Track.VideoID),
		Title:           firstNonEmpty(request.Title, request.Track.Title),
		Artist:          firstNonEmpty(request.Artist, request.Track.Artist, request.Track.Channel),
		DurationSeconds: firstPositive(request.DurationSeconds, request.Track.DurationSeconds),
		PlainOnly:       request.PlainOnly,
		Language:        request.Language,
	})
}

func (handler *ListenLyricsHandler) CurrentLyrics(_ context.Context) (listenlyrics.Snapshot, error) {
	if handler == nil || handler.service == nil {
		return listenlyrics.Snapshot{}, fmt.Errorf("listen lyrics service unavailable")
	}
	return handler.service.Current(), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
