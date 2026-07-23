package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"

	"xiadown/internal/application/youtubemusic"
)

const (
	listenLibraryRecommendationLimit = 18
	listenLibraryHomeShelfLimit      = 20
	listenLibraryShelfLimit          = 8
	listenLibraryShelfItemLimit      = 12
	listenLibraryTimeout             = 25 * time.Second
	listenLibraryLogDetailLimit      = 512
)

var listenLibrarySensitiveDetailPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)(https?://[^?\s"]+)\?[^\s"]+`), `${1}?[REDACTED]`},
	{regexp.MustCompile(`(?i)("(?:authorization|proxy-authorization|cookie|set-cookie|key|token|access_token|refresh_token|id_token)"\s*:\s*")[^"]*(")`), `${1}[REDACTED]${2}`},
	{regexp.MustCompile(`(?i)(\b(?:authorization|proxy-authorization)\s*[:=]\s*)(?:bearer\s+|sapisidhash\s+)?[^,;]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(\b(?:cookie|set-cookie)\s*[:=]\s*)[^\r\n]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)([?&](?:key|token|access_token|refresh_token|id_token|auth|authorization)=)[^&\s]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(\b(?:key|token|access_token|refresh_token|id_token)\s*=\s*)[^\s,;&]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(\b(?:SAPISID|APISID|SID|HSID|SSID|__Secure-[^=\s]+)=)[^\s,;]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)\bSAPISIDHASH\s+\S+`), `SAPISIDHASH [REDACTED]`},
}

const (
	listenLibrarySourceHome      = "home"
	listenLibrarySourceExplore   = "explore"
	listenLibrarySourceCharts    = "charts"
	listenLibrarySourceMoods     = "moods"
	listenLibrarySourceNew       = "new"
	listenLibrarySourceHistory   = "history"
	listenLibrarySourceRecent    = "recent"
	listenLibrarySourcePodcasts  = "podcasts"
	listenLibrarySourcePlaylists = "playlists"
)

type listenYouTubeMusicLibraryClient interface {
	HomeRecommendations(ctx context.Context, limit int) ([]youtubemusic.Track, error)
	HomeShelves(ctx context.Context, sectionLimit int, itemLimit int) ([]youtubemusic.Shelf, error)
	BrowseShelves(ctx context.Context, browseID string, sectionLimit int, itemLimit int) ([]youtubemusic.Shelf, error)
	BrowseShelvesPage(ctx context.Context, browseID string, params string, continuation string, sectionLimit int, itemLimit int) (youtubemusic.BrowsePage, error)
}

type ListenLibraryHandler struct {
	ytMusic listenYouTubeMusicLibraryClient
}

type ListenLibraryResponse struct {
	Playlists       []ListenPlaylistItem `json:"playlists"`
	Artists         []ListenArtistItem   `json:"artists,omitempty"`
	Recommendations []ListenSearchItem   `json:"recommendations"`
	Shelves         []ListenLibraryShelf `json:"shelves"`
	Continuation    string               `json:"continuation,omitempty"`
}

type ListenLibraryShelf struct {
	ID           string               `json:"id"`
	Title        string               `json:"title"`
	Kind         string               `json:"kind"`
	Continuation string               `json:"continuation,omitempty"`
	BrowseID     string               `json:"browseId,omitempty"`
	Params       string               `json:"params,omitempty"`
	Tracks       []ListenSearchItem   `json:"tracks,omitempty"`
	Playlists    []ListenPlaylistItem `json:"playlists,omitempty"`
	Categories   []ListenCategoryItem `json:"categories,omitempty"`
	Artists      []ListenArtistItem   `json:"artists,omitempty"`
}

type ListenCategoryItem struct {
	ID           string `json:"id"`
	BrowseID     string `json:"browseId"`
	Params       string `json:"params,omitempty"`
	Title        string `json:"title"`
	ColorHex     string `json:"colorHex,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

type ListenPlaylistItem struct {
	ID           string `json:"id"`
	PlaylistID   string `json:"playlistId"`
	Title        string `json:"title"`
	Channel      string `json:"channel"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

func NewListenLibraryHandler(ytMusic listenYouTubeMusicLibraryClient) *ListenLibraryHandler {
	return &ListenLibraryHandler{ytMusic: ytMusic}
}

func (handler *ListenLibraryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if handler.ytMusic == nil {
		writeListenLibraryError(
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

	ctx, cancel := context.WithTimeout(listenRequestContextWithLocale(r.Context(), r), listenLibraryTimeout)
	defer cancel()

	source := normalizeListenLibrarySource(r.URL.Query().Get("source"))
	if source != listenLibrarySourceHome || strings.TrimSpace(r.URL.Query().Get("continuation")) != "" || strings.TrimSpace(r.URL.Query().Get("browseId")) != "" {
		handler.serveBrowseSource(w, r, ctx, source)
		return
	}

	homePage, homeShelvesErr := handler.ytMusic.BrowseShelvesPage(ctx, listenLibrarySourceBrowseID(listenLibrarySourceHome), "", "", listenLibraryHomeShelfLimit, listenLibraryShelfItemLimit)
	homeShelves := homePage.Shelves
	if errors.Is(homeShelvesErr, youtubemusic.ErrBrowseUnavailable) ||
		errors.Is(homeShelvesErr, youtubemusic.ErrRegionUnavailable) {
		writeListenLibraryError(
			w,
			r,
			listenLibraryErrorHTTPStatus(homeShelvesErr),
			listenLibraryErrorCode(homeShelvesErr),
			listenLibraryErrorMessage(homeShelvesErr, source),
			strings.TrimSpace(homeShelvesErr.Error()),
			source,
		)
		return
	}
	if homeShelvesErr != nil {
		homeShelves, homeShelvesErr = handler.ytMusic.HomeShelves(ctx, listenLibraryHomeShelfLimit, listenLibraryShelfItemLimit)
	}

	var recommendations []youtubemusic.Track
	var recommendationErr error
	if homeShelvesErr != nil || len(homeShelves) == 0 {
		recommendations, recommendationErr = handler.ytMusic.HomeRecommendations(ctx, listenLibraryRecommendationLimit)
	}
	if homeShelvesErr != nil && recommendationErr != nil {
		err := firstListenLibraryError(homeShelvesErr, recommendationErr)
		writeListenLibraryError(
			w,
			r,
			listenLibraryErrorHTTPStatus(err),
			listenLibraryErrorCode(err),
			listenLibraryErrorMessage(err, source),
			joinListenLibraryErrors([]listenLibraryNamedError{
				{name: "home shelves", err: homeShelvesErr},
				{name: "recommendations", err: recommendationErr},
			}),
			source,
		)
		return
	}
	recommendations = enrichListenTrackDurations(ctx, handler.ytMusic, recommendations)
	homeShelves = enrichListenShelfTrackDurations(ctx, handler.ytMusic, homeShelves)

	responseShelves := mapYouTubeMusicShelvesToListenShelves(homeShelves)
	responseRecommendations := flattenListenLibraryTrackShelves(responseShelves, listenLibraryRecommendationLimit)
	if len(responseRecommendations) == 0 {
		responseRecommendations = mapYouTubeMusicTracksToListenItems(recommendations, "ytmusic-home")
	}
	if len(responseShelves) == 0 && len(responseRecommendations) > 0 {
		responseShelves = []ListenLibraryShelf{{
			ID:     "ytmusic-home-tracks",
			Kind:   string(youtubemusic.ShelfTracks),
			Tracks: responseRecommendations,
		}}
	}

	writeListenLibraryJSON(w, r, ListenLibraryResponse{
		Playlists:       []ListenPlaylistItem{},
		Artists:         []ListenArtistItem{},
		Recommendations: responseRecommendations,
		Shelves:         responseShelves,
		Continuation:    homePage.Continuation,
	})
}

func (handler *ListenLibraryHandler) serveBrowseSource(w http.ResponseWriter, r *http.Request, ctx context.Context, source string) {
	browseID := listenLibrarySourceBrowseID(source)
	if browseID == "" {
		writeListenLibraryError(w, r, http.StatusBadRequest, "invalid_source", "Invalid Listen library source.", "", source)
		return
	}
	params := ""
	hasBrowseOverride := false
	if overrideBrowseID := strings.TrimSpace(r.URL.Query().Get("browseId")); overrideBrowseID != "" {
		if !listenLibraryBrowseOverrideAllowed(source, overrideBrowseID) {
			writeListenLibraryError(w, r, http.StatusBadRequest, "invalid_browse_id", "Invalid Listen library browse id.", "browseId: "+overrideBrowseID, source)
			return
		}
		hasBrowseOverride = true
		browseID = overrideBrowseID
		params = strings.TrimSpace(r.URL.Query().Get("params"))
	}
	continuation := strings.TrimSpace(r.URL.Query().Get("continuation"))
	page, err := handler.ytMusic.BrowseShelvesPage(ctx, browseID, params, continuation, listenLibraryShelfLimit, listenLibraryShelfItemLimit)
	if err != nil {
		writeListenLibraryError(
			w,
			r,
			listenLibraryErrorHTTPStatus(err),
			listenLibraryErrorCode(err),
			listenLibraryErrorMessage(err, source),
			strings.TrimSpace(err.Error()),
			source,
		)
		return
	}
	if source == listenLibrarySourceCharts && continuation == "" && !hasBrowseOverride {
		page = handler.expandChartBrowsePage(ctx, page, browseID, params)
	}
	shelves := page.Shelves
	shelves = enrichListenShelfTrackDurations(ctx, handler.ytMusic, shelves)
	responseShelves := mapYouTubeMusicShelvesToListenShelvesWithPrefixes(shelves, "ytmusic-"+source, "ytmusic-"+source+"-playlist")
	responsePlaylists := []ListenPlaylistItem{}
	if source == listenLibrarySourcePlaylists {
		responsePlaylists = flattenListenLibraryShelfPlaylists(responseShelves)
	}
	writeListenLibraryJSON(w, r, ListenLibraryResponse{
		Playlists:       responsePlaylists,
		Recommendations: flattenListenLibraryTrackShelves(responseShelves, listenLibraryRecommendationLimit),
		Shelves:         responseShelves,
		Continuation:    page.Continuation,
	})
}

func (handler *ListenLibraryHandler) expandChartBrowsePage(ctx context.Context, page youtubemusic.BrowsePage, initialBrowseID string, initialParams string) youtubemusic.BrowsePage {
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
		tabPage, err := handler.ytMusic.BrowseShelvesPage(ctx, browseID, params, "", listenLibraryShelfLimit, listenLibraryShelfItemLimit)
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

type listenLibraryNamedError struct {
	name string
	err  error
}

func writeListenLibraryError(w http.ResponseWriter, r *http.Request, status int, code string, message string, detail string, source string) {
	if r == nil || !errors.Is(r.Context().Err(), context.Canceled) {
		zap.L().Warn(
			"listen youtube music library request failed",
			zap.Int("status", status),
			zap.String("code", strings.TrimSpace(code)),
			zap.String("source", strings.TrimSpace(source)),
			zap.String("detail", safeListenLibraryLogDetail(detail)),
		)
	}
	writeListenError(w, r, status, code, message, detail, source, listenYouTubeMusicErrorRetryableFromCode(code))
}

func safeListenLibraryLogDetail(detail string) string {
	safe := detail
	for _, sensitive := range listenLibrarySensitiveDetailPatterns {
		safe = sensitive.pattern.ReplaceAllString(safe, sensitive.replacement)
	}
	safe = strings.Join(strings.Fields(safe), " ")
	runes := []rune(safe)
	if len(runes) > listenLibraryLogDetailLimit {
		safe = string(runes[:listenLibraryLogDetailLimit]) + "…"
	}
	return safe
}

func firstListenLibraryError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func listenLibraryErrorCode(err error) string {
	return listenYouTubeMusicErrorCode(err)
}

func listenLibraryErrorHTTPStatus(err error) int {
	return listenYouTubeMusicErrorHTTPStatus(err)
}

func listenLibraryErrorMessage(err error, source string) string {
	return listenYouTubeMusicErrorMessage(err, "YouTube Music library unavailable.")
}

func joinListenLibraryErrors(items []listenLibraryNamedError) string {
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

func normalizeListenLibrarySource(value string) string {
	cleaned := strings.ToLower(strings.TrimSpace(value))
	switch cleaned {
	case "", listenLibrarySourceHome:
		return listenLibrarySourceHome
	case listenLibrarySourceExplore:
		return listenLibrarySourceExplore
	case listenLibrarySourceCharts:
		return listenLibrarySourceCharts
	case listenLibrarySourceMoods:
		return listenLibrarySourceMoods
	case listenLibrarySourceNew:
		return listenLibrarySourceNew
	case listenLibrarySourceHistory:
		return listenLibrarySourceHistory
	case listenLibrarySourceRecent:
		return listenLibrarySourceRecent
	case listenLibrarySourcePodcasts:
		return listenLibrarySourcePodcasts
	case listenLibrarySourcePlaylists:
		return listenLibrarySourcePlaylists
	default:
		return cleaned
	}
}

func listenLibrarySourceBrowseID(source string) string {
	switch source {
	case listenLibrarySourceHome:
		return "FEmusic_home"
	case listenLibrarySourceExplore:
		return "FEmusic_explore"
	case listenLibrarySourceCharts:
		return "FEmusic_charts"
	case listenLibrarySourceMoods:
		return "FEmusic_moods_and_genres"
	case listenLibrarySourceNew:
		return "FEmusic_new_releases"
	case listenLibrarySourceHistory:
		return "FEmusic_history"
	case listenLibrarySourceRecent:
		return "FEmusic_library_landing"
	case listenLibrarySourcePodcasts:
		return "FEmusic_podcasts"
	case listenLibrarySourcePlaylists:
		return "FEmusic_liked_playlists"
	default:
		return ""
	}
}

func listenLibraryBrowseOverrideAllowed(source string, browseID string) bool {
	switch source {
	case listenLibrarySourceHome, listenLibrarySourceMoods:
		return strings.HasPrefix(strings.TrimSpace(browseID), "FEmusic_moods_and_genres")
	default:
		return false
	}
}

func enrichListenShelfTrackDurations(ctx context.Context, client any, shelves []youtubemusic.Shelf) []youtubemusic.Shelf {
	if len(shelves) == 0 {
		return shelves
	}
	enriched := append([]youtubemusic.Shelf(nil), shelves...)
	for index := range enriched {
		enriched[index].Tracks = enrichListenTrackDurations(ctx, client, enriched[index].Tracks)
	}
	return enriched
}

func mapYouTubeMusicShelvesToListenShelves(shelves []youtubemusic.Shelf) []ListenLibraryShelf {
	return mapYouTubeMusicShelvesToListenShelvesWithPrefixes(shelves, "ytmusic-home", "ytmusic-home-playlist")
}

func mapYouTubeMusicShelvesToListenShelvesWithPrefixes(shelves []youtubemusic.Shelf, trackPrefix string, playlistPrefix string) []ListenLibraryShelf {
	items := make([]ListenLibraryShelf, 0, len(shelves))
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

		item := ListenLibraryShelf{
			ID:           shelfID,
			Title:        strings.TrimSpace(shelf.Title),
			Kind:         string(shelf.Kind),
			Continuation: strings.TrimSpace(shelf.Continuation),
			BrowseID:     strings.TrimSpace(shelf.BrowseID),
			Params:       strings.TrimSpace(shelf.Params),
		}
		switch shelf.Kind {
		case youtubemusic.ShelfArtists:
			item.Artists = mapYouTubeMusicArtistsToListenArtistItems(shelf.Artists, trackPrefix+"-artist")
		case youtubemusic.ShelfPlaylists:
			item.Playlists = mapYouTubeMusicPlaylistsToListenPlaylistItems(shelf.Playlists, playlistPrefix)
		case youtubemusic.ShelfCategories:
			item.Categories = mapYouTubeMusicCategoriesToListenCategoryItems(shelf.Categories)
		case youtubemusic.ShelfKind("podcasts"):
			// Keep the HTTP contract compatible with existing Music workspace
			// clients: podcast shows are navigable collections, so expose them
			// through the established playlists shape.
			item.Kind = string(youtubemusic.ShelfPlaylists)
			item.Playlists = mapYouTubeMusicPlaylistsToListenPlaylistItems(shelf.Playlists, playlistPrefix)
		default:
			item.Kind = string(youtubemusic.ShelfTracks)
			item.Tracks = mapYouTubeMusicTracksToListenItems(shelf.Tracks, trackPrefix)
		}
		if len(item.Tracks) == 0 && len(item.Playlists) == 0 && len(item.Categories) == 0 && len(item.Artists) == 0 {
			continue
		}
		items = append(items, item)
	}
	return items
}

func mapYouTubeMusicCategoriesToListenCategoryItems(categories []youtubemusic.Category) []ListenCategoryItem {
	items := make([]ListenCategoryItem, 0, len(categories))
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
		items = append(items, ListenCategoryItem{
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

func flattenListenLibraryTrackShelves(shelves []ListenLibraryShelf, limit int) []ListenSearchItem {
	if len(shelves) == 0 {
		return nil
	}
	items := make([]ListenSearchItem, 0, limit)
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

func flattenListenLibraryShelfPlaylists(shelves []ListenLibraryShelf) []ListenPlaylistItem {
	items := make([]ListenPlaylistItem, 0)
	seen := make(map[string]struct{})
	for _, shelf := range shelves {
		for _, playlist := range shelf.Playlists {
			playlistID := strings.TrimSpace(playlist.PlaylistID)
			if playlistID == "" {
				continue
			}
			if _, exists := seen[playlistID]; exists {
				continue
			}
			seen[playlistID] = struct{}{}
			items = append(items, playlist)
		}
	}
	return items
}

func mapYouTubeMusicPlaylistsToListenPlaylistItems(playlists []youtubemusic.Playlist, prefix string) []ListenPlaylistItem {
	items := make([]ListenPlaylistItem, 0, len(playlists))
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
		items = append(items, ListenPlaylistItem{
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

func writeListenLibraryJSON(w http.ResponseWriter, r *http.Request, response ListenLibraryResponse) {
	writeListenLibraryJSONStatus(w, r, http.StatusOK, response)
}

func writeListenLibraryJSONStatus(w http.ResponseWriter, r *http.Request, status int, response any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(response)
}
