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
	dreamFMSearchLimit         = 12
	dreamFMSearchArtistLimit   = 6
	dreamFMSearchPlaylistLimit = 6
	dreamFMSearchTimeout       = 25 * time.Second
)

type dreamFMYouTubeMusicClient interface {
	SearchSongs(ctx context.Context, query string, limit int) ([]youtubemusic.Track, error)
	SearchArtists(ctx context.Context, query string, limit int) ([]youtubemusic.Artist, error)
	SearchPlaylists(ctx context.Context, query string, limit int) ([]youtubemusic.Playlist, error)
}

type dreamFMTrackDurationClient interface {
	TrackDurations(ctx context.Context, videoIDs []string) (map[string]string, error)
}

type DreamFMSearchHandler struct {
	ytMusic dreamFMYouTubeMusicClient
}

type DreamFMSearchResponse struct {
	Items        []DreamFMSearchItem   `json:"items"`
	Artists      []DreamFMArtistItem   `json:"artists,omitempty"`
	Playlists    []DreamFMPlaylistItem `json:"playlists,omitempty"`
	Continuation string                `json:"continuation,omitempty"`
	Title        string                `json:"title,omitempty"`
	Author       string                `json:"author,omitempty"`
}

type DreamFMSearchItem struct {
	ID                     string `json:"id"`
	Group                  string `json:"group"`
	VideoID                string `json:"videoId"`
	Title                  string `json:"title"`
	Channel                string `json:"channel"`
	ArtistBrowseID         string `json:"artistBrowseId,omitempty"`
	Description            string `json:"description"`
	DurationLabel          string `json:"durationLabel"`
	PlayCountLabel         string `json:"playCountLabel,omitempty"`
	ThumbnailURL           string `json:"thumbnailUrl,omitempty"`
	MusicVideoType         string `json:"musicVideoType,omitempty"`
	HasVideo               bool   `json:"hasVideo"`
	VideoAvailabilityKnown bool   `json:"videoAvailabilityKnown,omitempty"`
}

type DreamFMArtistItem struct {
	ID           string `json:"id"`
	BrowseID     string `json:"browseId"`
	Name         string `json:"name"`
	Subtitle     string `json:"subtitle"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

func NewDreamFMSearchHandler(ytMusic dreamFMYouTubeMusicClient) *DreamFMSearchHandler {
	return &DreamFMSearchHandler{ytMusic: ytMusic}
}

func (handler *DreamFMSearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setCORSHeaders(w, r)

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeDreamFMSearchJSON(w, r, DreamFMSearchResponse{Items: []DreamFMSearchItem{}})
		return
	}
	if len([]rune(query)) < 2 {
		http.Error(w, "query is too short", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dreamFMSearchTimeout)
	defer cancel()

	if handler.ytMusic == nil {
		http.Error(w, "youtube music client unavailable", http.StatusServiceUnavailable)
		return
	}

	tracks, trackErr := handler.ytMusic.SearchSongs(ctx, query, dreamFMSearchLimit)
	artists, artistErr := handler.ytMusic.SearchArtists(ctx, query, dreamFMSearchArtistLimit)
	playlists, playlistErr := handler.ytMusic.SearchPlaylists(ctx, query, dreamFMSearchPlaylistLimit)
	if trackErr != nil && artistErr != nil && playlistErr != nil {
		http.Error(w, "youtube music search unavailable", http.StatusServiceUnavailable)
		return
	}
	if trackErr == nil {
		tracks = enrichDreamFMTrackDurations(ctx, handler.ytMusic, tracks)
	}
	writeDreamFMSearchJSON(w, r, DreamFMSearchResponse{
		Items:     mapYouTubeMusicTracksToDreamFMItems(tracks, "ytmusic-search"),
		Artists:   mapYouTubeMusicArtistsToDreamFMArtistItems(artists, "ytmusic-search-artist"),
		Playlists: mapYouTubeMusicPlaylistsToDreamFMPlaylistItems(playlists, "ytmusic-search-playlist"),
	})
}

func enrichDreamFMTrackDurations(ctx context.Context, client any, tracks []youtubemusic.Track) []youtubemusic.Track {
	durationClient, ok := client.(dreamFMTrackDurationClient)
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

func mapYouTubeMusicTracksToDreamFMItems(tracks []youtubemusic.Track, prefix string) []DreamFMSearchItem {
	items := make([]DreamFMSearchItem, 0, len(tracks))
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
		items = append(items, DreamFMSearchItem{
			ID:                     idPrefix + "-" + videoID,
			Group:                  "playlist",
			VideoID:                videoID,
			Title:                  title,
			Channel:                channel,
			ArtistBrowseID:         strings.TrimSpace(track.ArtistBrowseID),
			Description:            strings.TrimSpace(track.RawDescription),
			DurationLabel:          strings.TrimSpace(track.DurationLabel),
			PlayCountLabel:         strings.TrimSpace(track.PlayCountLabel),
			ThumbnailURL:           strings.TrimSpace(track.ThumbnailURL),
			MusicVideoType:         musicVideoType,
			HasVideo:               dreamFMMusicVideoTypeHasVideo(musicVideoType),
			VideoAvailabilityKnown: musicVideoType != "",
		})
	}
	return items
}

func dreamFMMusicVideoTypeHasVideo(value string) bool {
	return strings.TrimSpace(value) == "MUSIC_VIDEO_TYPE_OMV"
}

func mapYouTubeMusicArtistsToDreamFMArtistItems(artists []youtubemusic.Artist, prefix string) []DreamFMArtistItem {
	items := make([]DreamFMArtistItem, 0, len(artists))
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
		items = append(items, DreamFMArtistItem{
			ID:           itemPrefix + "-" + browseID,
			BrowseID:     browseID,
			Name:         name,
			Subtitle:     strings.TrimSpace(artist.Subtitle),
			ThumbnailURL: strings.TrimSpace(artist.ThumbnailURL),
		})
	}
	return items
}

func writeDreamFMSearchJSON(w http.ResponseWriter, r *http.Request, response DreamFMSearchResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
