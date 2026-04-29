package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"xiadown/internal/application/youtubemusic"
	"xiadown/internal/domain/connectors"
)

const (
	dreamFMLibraryPlaylistLimit       = 18
	dreamFMLibraryArtistLimit         = 12
	dreamFMLibraryLikedSongLimit      = 50
	dreamFMLibraryRecommendationLimit = 18
	dreamFMLibraryShelfLimit          = 8
	dreamFMLibraryShelfItemLimit      = 12
	dreamFMLibraryTimeout             = 25 * time.Second
)

const (
	dreamFMLibrarySourceHome    = "home"
	dreamFMLibrarySourceExplore = "explore"
	dreamFMLibrarySourceCharts  = "charts"
	dreamFMLibrarySourceMoods   = "moods"
	dreamFMLibrarySourceNew     = "new"
	dreamFMLibrarySourceHistory = "history"
)

type dreamFMYouTubeMusicLibraryClient interface {
	LibraryPlaylists(ctx context.Context, limit int) ([]youtubemusic.Playlist, error)
	LibraryArtists(ctx context.Context, limit int) ([]youtubemusic.Artist, error)
	LikedSongs(ctx context.Context, limit int) ([]youtubemusic.Track, error)
	HomeRecommendations(ctx context.Context, limit int) ([]youtubemusic.Track, error)
	HomeShelves(ctx context.Context, sectionLimit int, itemLimit int) ([]youtubemusic.Shelf, error)
	BrowseShelves(ctx context.Context, browseID string, sectionLimit int, itemLimit int) ([]youtubemusic.Shelf, error)
	BrowseShelvesPage(ctx context.Context, browseID string, params string, continuation string, sectionLimit int, itemLimit int) (youtubemusic.BrowsePage, error)
}

type DreamFMLibraryHandler struct {
	ytMusic dreamFMYouTubeMusicLibraryClient
}

type dreamFMErrorResponse struct {
	Error dreamFMErrorBody `json:"error"`
}

type dreamFMErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
	Source    string `json:"source,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

type DreamFMLibraryResponse struct {
	Playlists       []DreamFMPlaylistItem `json:"playlists"`
	Artists         []DreamFMArtistItem   `json:"artists,omitempty"`
	Podcasts        []DreamFMPlaylistItem `json:"podcasts,omitempty"`
	Recommendations []DreamFMSearchItem   `json:"recommendations"`
	Shelves         []DreamFMLibraryShelf `json:"shelves"`
	Continuation    string                `json:"continuation,omitempty"`
}

type DreamFMLibraryShelf struct {
	ID         string                `json:"id"`
	Title      string                `json:"title"`
	Kind       string                `json:"kind"`
	Tracks     []DreamFMSearchItem   `json:"tracks,omitempty"`
	Playlists  []DreamFMPlaylistItem `json:"playlists,omitempty"`
	Categories []DreamFMCategoryItem `json:"categories,omitempty"`
	Podcasts   []DreamFMPlaylistItem `json:"podcasts,omitempty"`
	Artists    []DreamFMArtistItem   `json:"artists,omitempty"`
}

type DreamFMCategoryItem struct {
	ID           string `json:"id"`
	BrowseID     string `json:"browseId"`
	Params       string `json:"params,omitempty"`
	Title        string `json:"title"`
	ColorHex     string `json:"colorHex,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

type DreamFMPlaylistItem struct {
	ID           string `json:"id"`
	PlaylistID   string `json:"playlistId"`
	Title        string `json:"title"`
	Channel      string `json:"channel"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

func NewDreamFMLibraryHandler(ytMusic dreamFMYouTubeMusicLibraryClient) *DreamFMLibraryHandler {
	return &DreamFMLibraryHandler{ytMusic: ytMusic}
}

func (handler *DreamFMLibraryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if handler.ytMusic == nil {
		writeDreamFMLibraryError(
			w,
			r,
			http.StatusServiceUnavailable,
			"youtube_music_client_unavailable",
			"YouTube Music client unavailable.",
			"",
			"",
		)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dreamFMLibraryTimeout)
	defer cancel()

	source := normalizeDreamFMLibrarySource(r.URL.Query().Get("source"))
	if source != dreamFMLibrarySourceHome || strings.TrimSpace(r.URL.Query().Get("continuation")) != "" || strings.TrimSpace(r.URL.Query().Get("browseId")) != "" {
		handler.serveBrowseSource(w, r, ctx, source)
		return
	}

	playlists, playlistErr := handler.ytMusic.LibraryPlaylists(ctx, dreamFMLibraryPlaylistLimit)
	artists, artistErr := handler.ytMusic.LibraryArtists(ctx, dreamFMLibraryArtistLimit)
	likedSongs, likedErr := handler.ytMusic.LikedSongs(ctx, dreamFMLibraryLikedSongLimit)
	homePage, homeShelvesErr := handler.ytMusic.BrowseShelvesPage(ctx, dreamFMLibrarySourceBrowseID(dreamFMLibrarySourceHome), "", "", dreamFMLibraryShelfLimit, dreamFMLibraryShelfItemLimit)
	homeShelves := homePage.Shelves
	if homeShelvesErr != nil {
		homeShelves, homeShelvesErr = handler.ytMusic.HomeShelves(ctx, dreamFMLibraryShelfLimit, dreamFMLibraryShelfItemLimit)
	}

	var recommendations []youtubemusic.Track
	var recommendationErr error
	if homeShelvesErr != nil || len(homeShelves) == 0 {
		recommendations, recommendationErr = handler.ytMusic.HomeRecommendations(ctx, dreamFMLibraryRecommendationLimit)
	}
	if playlistErr != nil && artistErr != nil && likedErr != nil && homeShelvesErr != nil && recommendationErr != nil {
		err := firstDreamFMLibraryError(
			playlistErr,
			artistErr,
			likedErr,
			homeShelvesErr,
			recommendationErr,
		)
		writeDreamFMLibraryError(
			w,
			r,
			dreamFMLibraryErrorHTTPStatus(err),
			dreamFMLibraryErrorCode(err),
			dreamFMLibraryErrorMessage(err, source),
			joinDreamFMLibraryErrors([]dreamFMLibraryNamedError{
				{name: "playlists", err: playlistErr},
				{name: "artists", err: artistErr},
				{name: "liked songs", err: likedErr},
				{name: "home shelves", err: homeShelvesErr},
				{name: "recommendations", err: recommendationErr},
			}),
			source,
		)
		return
	}
	likedSongs = enrichDreamFMTrackDurations(ctx, handler.ytMusic, likedSongs)
	recommendations = enrichDreamFMTrackDurations(ctx, handler.ytMusic, recommendations)
	homeShelves = enrichDreamFMShelfTrackDurations(ctx, handler.ytMusic, homeShelves)

	responseShelves := mapYouTubeMusicShelvesToDreamFMShelves(homeShelves)
	responseRecommendations := flattenDreamFMLibraryTrackShelves(responseShelves, dreamFMLibraryRecommendationLimit)
	if len(responseRecommendations) == 0 {
		responseRecommendations = mapYouTubeMusicTracksToDreamFMItems(recommendations, "ytmusic-home")
	}
	if likedShelf := mapYouTubeMusicLikedSongsToDreamFMShelf(likedSongs); len(likedShelf.Tracks) > 0 {
		responseShelves = append([]DreamFMLibraryShelf{likedShelf}, responseShelves...)
	}
	if len(responseShelves) == 0 && len(responseRecommendations) > 0 {
		responseShelves = []DreamFMLibraryShelf{{
			ID:     "ytmusic-home-tracks",
			Kind:   string(youtubemusic.ShelfTracks),
			Tracks: responseRecommendations,
		}}
	}

	writeDreamFMLibraryJSON(w, r, DreamFMLibraryResponse{
		Playlists:       mapYouTubeMusicPlaylistsToDreamFMPlaylistItems(playlists, "ytmusic-library"),
		Artists:         mapYouTubeMusicArtistsToDreamFMArtistItems(artists, "ytmusic-library-artist"),
		Recommendations: responseRecommendations,
		Shelves:         responseShelves,
		Continuation:    homePage.Continuation,
	})
}

func (handler *DreamFMLibraryHandler) serveBrowseSource(w http.ResponseWriter, r *http.Request, ctx context.Context, source string) {
	browseID := dreamFMLibrarySourceBrowseID(source)
	if browseID == "" {
		writeDreamFMLibraryError(w, r, http.StatusBadRequest, "invalid_source", "Invalid DreamFM library source.", "", source)
		return
	}
	params := ""
	hasBrowseOverride := false
	if overrideBrowseID := strings.TrimSpace(r.URL.Query().Get("browseId")); overrideBrowseID != "" {
		if !dreamFMLibraryBrowseOverrideAllowed(source, overrideBrowseID) {
			writeDreamFMLibraryError(w, r, http.StatusBadRequest, "invalid_browse_id", "Invalid DreamFM library browse id.", "browseId: "+overrideBrowseID, source)
			return
		}
		hasBrowseOverride = true
		browseID = overrideBrowseID
		params = strings.TrimSpace(r.URL.Query().Get("params"))
	}
	continuation := strings.TrimSpace(r.URL.Query().Get("continuation"))
	page, err := handler.ytMusic.BrowseShelvesPage(ctx, browseID, params, continuation, dreamFMLibraryShelfLimit, dreamFMLibraryShelfItemLimit)
	if err != nil {
		writeDreamFMLibraryError(
			w,
			r,
			dreamFMLibraryErrorHTTPStatus(err),
			dreamFMLibraryErrorCode(err),
			dreamFMLibraryErrorMessage(err, source),
			strings.TrimSpace(err.Error()),
			source,
		)
		return
	}
	if source == dreamFMLibrarySourceCharts && continuation == "" && !hasBrowseOverride {
		page = handler.expandChartBrowsePage(ctx, page, browseID, params)
	}
	shelves := page.Shelves
	shelves = enrichDreamFMShelfTrackDurations(ctx, handler.ytMusic, shelves)
	responseShelves := mapYouTubeMusicShelvesToDreamFMShelvesWithPrefixes(shelves, "ytmusic-"+source, "ytmusic-"+source+"-playlist")
	writeDreamFMLibraryJSON(w, r, DreamFMLibraryResponse{
		Playlists:       []DreamFMPlaylistItem{},
		Recommendations: flattenDreamFMLibraryTrackShelves(responseShelves, dreamFMLibraryRecommendationLimit),
		Shelves:         responseShelves,
		Continuation:    page.Continuation,
	})
}

func (handler *DreamFMLibraryHandler) expandChartBrowsePage(ctx context.Context, page youtubemusic.BrowsePage, initialBrowseID string, initialParams string) youtubemusic.BrowsePage {
	if len(page.Tabs) == 0 {
		return page
	}
	shelves := append([]youtubemusic.Shelf(nil), page.Shelves...)
	seenRequests := map[string]struct{}{
		strings.TrimSpace(initialBrowseID) + "\x00" + strings.TrimSpace(initialParams): {},
	}
	for _, tab := range page.Tabs {
		browseID := strings.TrimSpace(tab.BrowseID)
		params := strings.TrimSpace(tab.Params)
		if browseID == "" {
			continue
		}
		key := browseID + "\x00" + params
		if _, exists := seenRequests[key]; exists {
			continue
		}
		seenRequests[key] = struct{}{}
		tabPage, err := handler.ytMusic.BrowseShelvesPage(ctx, browseID, params, "", dreamFMLibraryShelfLimit, dreamFMLibraryShelfItemLimit)
		if err != nil {
			continue
		}
		shelves = append(shelves, tabPage.Shelves...)
	}
	page.Shelves = dedupeYouTubeMusicShelves(shelves)
	return page
}

func dedupeYouTubeMusicShelves(shelves []youtubemusic.Shelf) []youtubemusic.Shelf {
	if len(shelves) == 0 {
		return nil
	}
	items := make([]youtubemusic.Shelf, 0, len(shelves))
	seen := make(map[string]struct{}, len(shelves))
	for _, shelf := range shelves {
		id := strings.TrimSpace(shelf.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		items = append(items, shelf)
	}
	return items
}

type dreamFMLibraryNamedError struct {
	name string
	err  error
}

func writeDreamFMLibraryError(w http.ResponseWriter, r *http.Request, status int, code string, message string, detail string, source string) {
	writeDreamFMLibraryJSONStatus(w, r, status, dreamFMErrorResponse{
		Error: dreamFMErrorBody{
			Code:    code,
			Message: message,
			Detail:  strings.TrimSpace(detail),
			Source:  strings.TrimSpace(source),
		},
	})
}

func firstDreamFMLibraryError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func dreamFMLibraryErrorCode(err error) string {
	switch {
	case isDreamFMLibraryMissingCookiesError(err):
		return "youtube_cookies_missing"
	case errors.Is(err, youtubemusic.ErrAuthExpired):
		return "youtube_auth_expired"
	case isDreamFMLibraryTimeoutError(err):
		return "youtube_timeout"
	case isDreamFMLibraryNetworkError(err):
		return "youtube_network_unavailable"
	default:
		return "youtube_music_unavailable"
	}
}

func dreamFMLibraryErrorHTTPStatus(err error) int {
	switch {
	case isDreamFMLibraryMissingCookiesError(err), errors.Is(err, youtubemusic.ErrAuthExpired):
		return http.StatusUnauthorized
	case isDreamFMLibraryTimeoutError(err):
		return http.StatusGatewayTimeout
	default:
		return http.StatusServiceUnavailable
	}
}

func dreamFMLibraryErrorMessage(err error, source string) string {
	switch {
	case isDreamFMLibraryMissingCookiesError(err):
		return "YouTube Music cookies are missing."
	case errors.Is(err, youtubemusic.ErrAuthExpired):
		return "YouTube Music authentication expired."
	case isDreamFMLibraryTimeoutError(err):
		return "YouTube Music request timed out."
	case isDreamFMLibraryNetworkError(err):
		return "YouTube Music network unavailable."
	default:
		return "YouTube Music library unavailable."
	}
}

func isDreamFMLibraryMissingCookiesError(err error) bool {
	return errors.Is(err, youtubemusic.ErrNotAuthenticated) ||
		errors.Is(err, connectors.ErrNoCookies) ||
		errors.Is(err, connectors.ErrConnectorNotFound)
}

func isDreamFMLibraryTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, youtubemusic.ErrRequestTimedOut) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "client.timeout") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "tls handshake timeout")
}

func isDreamFMLibraryNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, youtubemusic.ErrNetworkUnavailable) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, marker := range []string{
		"no such host",
		"network is unreachable",
		"connection refused",
		"connection reset",
		"temporary failure",
		"dial tcp",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func joinDreamFMLibraryErrors(items []dreamFMLibraryNamedError) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item.err == nil {
			continue
		}
		detail := strings.TrimSpace(item.err.Error())
		if detail == "" {
			continue
		}
		parts = append(parts, item.name+": "+detail)
	}
	return strings.Join(parts, "; ")
}

func normalizeDreamFMLibrarySource(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", dreamFMLibrarySourceHome:
		return dreamFMLibrarySourceHome
	case dreamFMLibrarySourceExplore:
		return dreamFMLibrarySourceExplore
	case dreamFMLibrarySourceCharts:
		return dreamFMLibrarySourceCharts
	case dreamFMLibrarySourceMoods:
		return dreamFMLibrarySourceMoods
	case dreamFMLibrarySourceNew:
		return dreamFMLibrarySourceNew
	case dreamFMLibrarySourceHistory:
		return dreamFMLibrarySourceHistory
	default:
		return dreamFMLibrarySourceHome
	}
}

func dreamFMLibrarySourceBrowseID(source string) string {
	switch source {
	case dreamFMLibrarySourceHome:
		return "FEmusic_home"
	case dreamFMLibrarySourceExplore:
		return "FEmusic_explore"
	case dreamFMLibrarySourceCharts:
		return "FEmusic_charts"
	case dreamFMLibrarySourceMoods:
		return "FEmusic_moods_and_genres"
	case dreamFMLibrarySourceNew:
		return "FEmusic_new_releases"
	case dreamFMLibrarySourceHistory:
		return "FEmusic_history"
	default:
		return ""
	}
}

func dreamFMLibraryBrowseOverrideAllowed(source string, browseID string) bool {
	switch source {
	case dreamFMLibrarySourceMoods:
		return strings.HasPrefix(strings.TrimSpace(browseID), "FEmusic_moods_and_genres")
	default:
		return false
	}
}

func mapYouTubeMusicLikedSongsToDreamFMShelf(tracks []youtubemusic.Track) DreamFMLibraryShelf {
	return DreamFMLibraryShelf{
		ID:     "ytmusic-liked-songs",
		Title:  "Liked Music",
		Kind:   string(youtubemusic.ShelfTracks),
		Tracks: mapYouTubeMusicTracksToDreamFMItems(tracks, "ytmusic-liked"),
	}
}

func enrichDreamFMShelfTrackDurations(ctx context.Context, client any, shelves []youtubemusic.Shelf) []youtubemusic.Shelf {
	if len(shelves) == 0 {
		return shelves
	}
	enriched := append([]youtubemusic.Shelf(nil), shelves...)
	for index := range enriched {
		enriched[index].Tracks = enrichDreamFMTrackDurations(ctx, client, enriched[index].Tracks)
	}
	return enriched
}

func mapYouTubeMusicShelvesToDreamFMShelves(shelves []youtubemusic.Shelf) []DreamFMLibraryShelf {
	return mapYouTubeMusicShelvesToDreamFMShelvesWithPrefixes(shelves, "ytmusic-home", "ytmusic-home-playlist")
}

func mapYouTubeMusicShelvesToDreamFMShelvesWithPrefixes(shelves []youtubemusic.Shelf, trackPrefix string, playlistPrefix string) []DreamFMLibraryShelf {
	items := make([]DreamFMLibraryShelf, 0, len(shelves))
	seen := make(map[string]struct{}, len(shelves))
	for _, shelf := range shelves {
		shelfID := strings.TrimSpace(shelf.ID)
		if shelfID == "" {
			continue
		}
		if _, exists := seen[shelfID]; exists {
			continue
		}
		seen[shelfID] = struct{}{}

		item := DreamFMLibraryShelf{
			ID:    shelfID,
			Title: strings.TrimSpace(shelf.Title),
			Kind:  string(shelf.Kind),
		}
		switch shelf.Kind {
		case youtubemusic.ShelfArtists:
			item.Artists = mapYouTubeMusicArtistsToDreamFMArtistItems(shelf.Artists, trackPrefix+"-artist")
		case youtubemusic.ShelfPlaylists:
			item.Playlists = mapYouTubeMusicPlaylistsToDreamFMPlaylistItems(shelf.Playlists, playlistPrefix)
		case youtubemusic.ShelfCategories:
			item.Categories = mapYouTubeMusicCategoriesToDreamFMCategoryItems(shelf.Categories)
		case youtubemusic.ShelfPodcasts:
			continue
		default:
			item.Kind = string(youtubemusic.ShelfTracks)
			item.Tracks = mapYouTubeMusicTracksToDreamFMItems(shelf.Tracks, trackPrefix)
		}
		if len(item.Tracks) == 0 && len(item.Playlists) == 0 && len(item.Categories) == 0 && len(item.Podcasts) == 0 && len(item.Artists) == 0 {
			continue
		}
		items = append(items, item)
	}
	return items
}

func mapYouTubeMusicCategoriesToDreamFMCategoryItems(categories []youtubemusic.Category) []DreamFMCategoryItem {
	items := make([]DreamFMCategoryItem, 0, len(categories))
	seen := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		browseID := strings.TrimSpace(category.BrowseID)
		title := strings.TrimSpace(category.Title)
		if browseID == "" || title == "" {
			continue
		}
		id := strings.TrimSpace(category.ID)
		if id == "" {
			id = browseID
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		items = append(items, DreamFMCategoryItem{
			ID:           id,
			BrowseID:     browseID,
			Params:       strings.TrimSpace(category.Params),
			Title:        title,
			ColorHex:     strings.TrimSpace(category.ColorHex),
			ThumbnailURL: strings.TrimSpace(category.ThumbnailURL),
		})
	}
	return items
}

func flattenDreamFMLibraryTrackShelves(shelves []DreamFMLibraryShelf, limit int) []DreamFMSearchItem {
	if len(shelves) == 0 {
		return nil
	}
	items := make([]DreamFMSearchItem, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, shelf := range shelves {
		for _, track := range shelf.Tracks {
			videoID := strings.TrimSpace(track.VideoID)
			if videoID == "" {
				continue
			}
			if _, exists := seen[videoID]; exists {
				continue
			}
			seen[videoID] = struct{}{}
			items = append(items, track)
			if len(items) >= limit {
				return items
			}
		}
	}
	return items
}

func mapYouTubeMusicPlaylistsToDreamFMPlaylistItems(playlists []youtubemusic.Playlist, prefix string) []DreamFMPlaylistItem {
	items := make([]DreamFMPlaylistItem, 0, len(playlists))
	seen := make(map[string]struct{}, len(playlists))
	for _, playlist := range playlists {
		playlistID := strings.TrimSpace(playlist.ID)
		if playlistID == "" {
			continue
		}
		if _, exists := seen[playlistID]; exists {
			continue
		}
		seen[playlistID] = struct{}{}
		itemPrefix := strings.TrimSpace(prefix)
		if itemPrefix == "" {
			itemPrefix = "ytmusic-playlist"
		}
		title := strings.TrimSpace(playlist.Title)
		if title == "" {
			title = playlistID
		}
		items = append(items, DreamFMPlaylistItem{
			ID:           itemPrefix + "-" + playlistID,
			PlaylistID:   playlistID,
			Title:        title,
			Channel:      strings.TrimSpace(playlist.Channel),
			Description:  strings.TrimSpace(playlist.Description),
			ThumbnailURL: strings.TrimSpace(playlist.ThumbnailURL),
		})
	}
	return items
}

func writeDreamFMLibraryJSON(w http.ResponseWriter, r *http.Request, response DreamFMLibraryResponse) {
	writeDreamFMLibraryJSONStatus(w, r, http.StatusOK, response)
}

func writeDreamFMLibraryJSONStatus(w http.ResponseWriter, r *http.Request, status int, response any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
