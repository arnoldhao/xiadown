package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
)

const (
	listenSearchLimit         = 12
	listenSearchArtistLimit   = 6
	listenSearchPlaylistLimit = 6
	listenSearchTimeout       = 25 * time.Second
)

type listenYouTubeMusicClient interface {
	SearchSongs(ctx context.Context, query string, limit int) ([]youtubemusic.Track, error)
	SearchArtists(ctx context.Context, query string, limit int) ([]youtubemusic.Artist, error)
	SearchPlaylists(ctx context.Context, query string, limit int) ([]youtubemusic.Playlist, error)
}

type listenTrackDurationClient interface {
	TrackDurations(ctx context.Context, videoIDs []string) (map[string]string, error)
}

type listenSearchSongsPageClient interface {
	SearchSongsPage(ctx context.Context, query string, continuation string, limit int) (youtubemusic.TrackListPage, error)
}

type ListenSearchHandler struct {
	ytMusic listenYouTubeMusicClient
}

type ListenSearchResponse struct {
	Items        []ListenSearchItem   `json:"items"`
	Artists      []ListenArtistItem   `json:"artists,omitempty"`
	Playlists    []ListenPlaylistItem `json:"playlists,omitempty"`
	Continuation string               `json:"continuation,omitempty"`
	Title        string               `json:"title,omitempty"`
	Author       string               `json:"author,omitempty"`
}

type ListenSearchItem struct {
	ID                     string             `json:"id"`
	Group                  string             `json:"group"`
	VideoID                string             `json:"videoId"`
	Title                  string             `json:"title"`
	Channel                string             `json:"channel"`
	Artists                []ListenArtistItem `json:"artists,omitempty"`
	ArtistBrowseID         string             `json:"artistBrowseId,omitempty"`
	ArtistSource           string             `json:"artistSource,omitempty"`
	Description            string             `json:"description"`
	DurationLabel          string             `json:"durationLabel"`
	PlayCountLabel         string             `json:"playCountLabel,omitempty"`
	ThumbnailURL           string             `json:"thumbnailUrl,omitempty"`
	MusicVideoType         string             `json:"musicVideoType,omitempty"`
	HasVideo               bool               `json:"hasVideo,omitempty"`
	VideoAvailabilityKnown bool               `json:"videoAvailabilityKnown,omitempty"`
}

type ListenArtistItem struct {
	ID           string `json:"id"`
	BrowseID     string `json:"browseId"`
	Name         string `json:"name"`
	Subtitle     string `json:"subtitle"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

func NewListenSearchHandler(ytMusic listenYouTubeMusicClient) *ListenSearchHandler {
	return &ListenSearchHandler{ytMusic: ytMusic}
}

func (handler *ListenSearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeListenMethodNotAllowed(w, r)
		return
	}
	setCORSHeaders(w, r)

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	continuation := strings.TrimSpace(r.URL.Query().Get("continuation"))
	if query == "" {
		writeListenSearchJSON(w, r, ListenSearchResponse{Items: []ListenSearchItem{}})
		return
	}
	if len([]rune(query)) < 2 {
		writeListenBadRequest(w, r, "query_too_short", "Search query is too short.", "")
		return
	}
	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenSearchTimeout)
	defer cancel()

	if handler.ytMusic == nil {
		writeListenServiceUnavailable(w, r, "youtube_music_client_unavailable", "YouTube Music client unavailable.", "")
		return
	}

	tracks, trackContinuation, trackErr := handler.searchSongs(ctx, query, continuation)
	artists, artistErr := handler.ytMusic.SearchArtists(ctx, query, listenSearchArtistLimit)
	playlists, playlistErr := handler.ytMusic.SearchPlaylists(ctx, query, listenSearchPlaylistLimit)
	if trackErr != nil && artistErr != nil && playlistErr != nil {
		writeListenYouTubeMusicUnavailable(
			w,
			r,
			trackErr,
			"YouTube Music search unavailable.",
			"search",
		)
		return
	}
	if trackErr == nil {
		tracks = enrichListenTrackDurations(ctx, handler.ytMusic, tracks)
	}
	writeListenSearchJSON(w, r, ListenSearchResponse{
		Items:        mapYouTubeMusicTracksToListenItems(tracks, "ytmusic-search"),
		Artists:      mapYouTubeMusicArtistsToListenArtistItems(artists, "ytmusic-search-artist"),
		Playlists:    mapYouTubeMusicPlaylistsToListenPlaylistItems(playlists, "ytmusic-search-playlist"),
		Continuation: trackContinuation,
	})
}

func (handler *ListenSearchHandler) searchSongs(ctx context.Context, query string, continuation string) ([]youtubemusic.Track, string, error) {
	if pageClient, ok := handler.ytMusic.(listenSearchSongsPageClient); ok {
		page, err := pageClient.SearchSongsPage(ctx, query, continuation, listenSearchLimit)
		return page.Tracks, strings.TrimSpace(page.Continuation), err
	}
	tracks, err := handler.ytMusic.SearchSongs(ctx, query, listenSearchLimit)
	return tracks, "", err
}

func enrichListenTrackDurations(ctx context.Context, client any, tracks []youtubemusic.Track) []youtubemusic.Track {
	durationClient, ok := client.(listenTrackDurationClient)
	if !ok || len(tracks) == 0 {
		return tracks
	}
	missingVideoIDs := make([]string, 0, len(tracks))
	for _, track := range tracks {
		if strings.TrimSpace(track.DurationLabel) != "" {
			continue
		}
		if videoID := strings.TrimSpace(track.VideoID); youtubeVideoIDPattern.MatchString(videoID) {
			missingVideoIDs = append(missingVideoIDs, videoID)
		}
	}
	if len(missingVideoIDs) == 0 {
		return tracks
	}
	durations, err := durationClient.TrackDurations(ctx, missingVideoIDs)
	if err != nil || len(durations) == 0 {
		return tracks
	}
	enriched := append([]youtubemusic.Track(nil), tracks...)
	for index := range enriched {
		if strings.TrimSpace(enriched[index].DurationLabel) != "" {
			continue
		}
		if duration := strings.TrimSpace(durations[strings.TrimSpace(enriched[index].VideoID)]); duration != "" {
			enriched[index].DurationLabel = duration
		}
	}
	return enriched
}

func mapYouTubeMusicTracksToListenItems(tracks []youtubemusic.Track, prefix string) []ListenSearchItem {
	items := make([]ListenSearchItem, 0, len(tracks))
	seen := make(map[string]struct{}, len(tracks))
	for _, track := range tracks {
		videoID := strings.TrimSpace(track.VideoID)
		if !youtubeVideoIDPattern.MatchString(videoID) {
			continue
		}
		if _, ok := seen[videoID]; ok {
			continue
		}
		seen[videoID] = struct{}{}
		idPrefix := strings.TrimSpace(prefix)
		if idPrefix == "" {
			idPrefix = "ytmusic"
		}
		title := strings.TrimSpace(track.Title)
		if title == "" {
			title = videoID
		}
		channel := strings.TrimSpace(track.Channel)
		musicVideoType := strings.TrimSpace(track.MusicVideoType)
		thumbnailURL := strings.TrimSpace(track.ThumbnailURL)
		hasVideo, videoAvailabilityKnown := listenTrackVideoAvailability(musicVideoType, videoID, thumbnailURL)
		items = append(items, ListenSearchItem{
			ID:                     idPrefix + "-" + videoID,
			Group:                  "playlist",
			VideoID:                videoID,
			Title:                  title,
			Channel:                channel,
			Artists:                mapYouTubeMusicTrackArtistsToListenArtistItems(track.Artists, idPrefix+"-artist"),
			ArtistBrowseID:         strings.TrimSpace(track.ArtistBrowseID),
			ArtistSource:           strings.TrimSpace(track.ArtistSource),
			Description:            strings.TrimSpace(track.RawDescription),
			DurationLabel:          strings.TrimSpace(track.DurationLabel),
			PlayCountLabel:         strings.TrimSpace(track.PlayCountLabel),
			ThumbnailURL:           thumbnailURL,
			MusicVideoType:         musicVideoType,
			HasVideo:               hasVideo,
			VideoAvailabilityKnown: videoAvailabilityKnown,
		})
	}
	return items
}

func mapYouTubeMusicTrackArtistsToListenArtistItems(artists []youtubemusic.TrackArtist, prefix string) []ListenArtistItem {
	items := make([]ListenArtistItem, 0, len(artists))
	seen := make(map[string]struct{}, len(artists))
	for _, artist := range artists {
		name := strings.TrimSpace(artist.Name)
		browseID := strings.TrimSpace(artist.BrowseID)
		if name == "" || browseID == "" {
			continue
		}
		if _, ok := seen[browseID]; ok {
			continue
		}
		seen[browseID] = struct{}{}
		itemPrefix := strings.TrimSpace(prefix)
		if itemPrefix == "" {
			itemPrefix = "ytmusic-track-artist"
		}
		items = append(items, ListenArtistItem{
			ID:       itemPrefix + "-" + browseID,
			BrowseID: browseID,
			Name:     name,
			Subtitle: "YouTube Music",
		})
	}
	return items
}

func listenTrackVideoAvailability(musicVideoType string, videoID string, thumbnailURL string) (bool, bool) {
	return youtubemusic.TrackVideoAvailability(musicVideoType, videoID, thumbnailURL)
}

func mapYouTubeMusicArtistsToListenArtistItems(artists []youtubemusic.Artist, prefix string) []ListenArtistItem {
	items := make([]ListenArtistItem, 0, len(artists))
	seen := make(map[string]struct{}, len(artists))
	for _, artist := range artists {
		browseID := strings.TrimSpace(artist.ID)
		if browseID == "" {
			continue
		}
		if _, ok := seen[browseID]; ok {
			continue
		}
		seen[browseID] = struct{}{}
		itemPrefix := strings.TrimSpace(prefix)
		if itemPrefix == "" {
			itemPrefix = "ytmusic-artist"
		}
		name := strings.TrimSpace(artist.Name)
		if name == "" {
			name = browseID
		}
		items = append(items, ListenArtistItem{
			ID:           itemPrefix + "-" + browseID,
			BrowseID:     browseID,
			Name:         name,
			Subtitle:     strings.TrimSpace(artist.Subtitle),
			ThumbnailURL: strings.TrimSpace(artist.ThumbnailURL),
		})
	}
	return items
}

func writeListenSearchJSON(w http.ResponseWriter, r *http.Request, response ListenSearchResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
