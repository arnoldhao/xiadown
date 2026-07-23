package youtubemusic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

const (
	browseHomeID            = "FEmusic_home"
	browseExploreID         = "FEmusic_explore"
	browseChartsID          = "FEmusic_charts"
	browseMoodsAndGenresID  = "FEmusic_moods_and_genres"
	browseNewReleasesID     = "FEmusic_new_releases"
	browseHistoryID         = "FEmusic_history"
	browseLibraryLandingID  = "FEmusic_library_landing"
	browsePodcastsID        = "FEmusic_podcasts"
	browseLibraryArtistsID  = "FEmusic_library_corpus_artists"
	browseLikedPlaylistsID  = "FEmusic_liked_playlists"
	browseLikedSongsID      = "VLLM"
	defaultPlaylistQueueMax = 100
	defaultShelfLimit       = 8
)

type ShelfKind string

const (
	ShelfTracks     ShelfKind = "tracks"
	ShelfPlaylists  ShelfKind = "playlists"
	ShelfCategories ShelfKind = "categories"
	ShelfArtists    ShelfKind = "artists"
)

type Playlist struct {
	ID           string
	Title        string
	Channel      string
	Description  string
	ThumbnailURL string
}

type Artist struct {
	ID           string
	Name         string
	Subtitle     string
	ThumbnailURL string
}

type Category struct {
	ID           string
	BrowseID     string
	Params       string
	Title        string
	ColorHex     string
	ThumbnailURL string
}

type Shelf struct {
	ID           string
	Title        string
	Kind         ShelfKind
	Continuation string
	BrowseID     string
	Params       string
	Tracks       []Track
	Playlists    []Playlist
	Categories   []Category
	Artists      []Artist
}

type BrowseTab struct {
	Title    string
	BrowseID string
	Params   string
	Selected bool
}

type BrowsePage struct {
	Shelves         []Shelf
	Continuation    string
	Tabs            []BrowseTab
	Title           string
	Author          string
	AuthorBrowseID  string
	TrackCountLabel string
	DurationLabel   string
	Description     string
	ThumbnailURL    string
}

type TrackListPage struct {
	Tracks          []Track
	Continuation    string
	Title           string
	Author          string
	AuthorBrowseID  string
	TrackCountLabel string
	DurationLabel   string
	Description     string
	ThumbnailURL    string
}

type ArtistPage struct {
	ID               string
	Title            string
	Subtitle         string
	Description      string
	ThumbnailURL     string
	HeroThumbnailURL string
	ChannelID        string
	IsSubscribed     bool
	MixPlaylistID    string
	MixVideoID       string
	Tracks           []Track
	Shelves          []Shelf
	Continuation     string
}

func (client *Client) HomeRecommendations(ctx context.Context, limit int) ([]Track, error) {
	data, err := client.requestRead(ctx, "browse", map[string]any{
		"browseId": browseHomeID,
	})
	if err != nil {
		return nil, err
	}
	return parseHomeRecommendationTracks(data, normalizeLimit(limit)), nil
}

func (client *Client) HomeShelves(ctx context.Context, sectionLimit int, itemLimit int) ([]Shelf, error) {
	return client.BrowseShelves(ctx, browseHomeID, sectionLimit, itemLimit)
}

func (client *Client) BrowseShelves(ctx context.Context, browseID string, sectionLimit int, itemLimit int) ([]Shelf, error) {
	page, err := client.BrowseShelvesPage(ctx, browseID, "", "", sectionLimit, itemLimit)
	if err != nil {
		return nil, err
	}
	return page.Shelves, nil
}

func (client *Client) BrowseShelvesPage(ctx context.Context, browseID string, params string, continuation string, sectionLimit int, itemLimit int) (BrowsePage, error) {
	trimmedContinuation := strings.TrimSpace(continuation)
	if trimmedContinuation != "" {
		return client.readBrowseShelvesPage(ctx, map[string]any{
			"continuation": trimmedContinuation,
		}, sectionLimit, itemLimit, false)
	}

	cleanedBrowseID, err := cleanBrowseID(browseID)
	if err != nil {
		return BrowsePage{}, err
	}
	cleanedParams, err := cleanBrowseParams(params)
	if err != nil {
		return BrowsePage{}, err
	}
	body := map[string]any{
		"browseId": cleanedBrowseID,
	}
	if cleanedParams != "" {
		body["params"] = cleanedParams
	}
	return client.readBrowseShelvesPage(ctx, body, sectionLimit, itemLimit, true)
}

func (client *Client) readBrowseShelvesPage(ctx context.Context, body map[string]any, sectionLimit int, itemLimit int, includeMetadata bool) (BrowsePage, error) {
	normalizedSectionLimit := normalizeShelfLimit(sectionLimit)
	normalizedItemLimit := normalizeLimit(itemLimit)
	data, err := client.requestRead(ctx, "browse", body)
	if errors.Is(err, ErrBrowseUnavailable) {
		// Availability interstitials are occasionally transient. Retry once while
		// bypassing the read cache; requestWithOptions deliberately never caches
		// either interstitial response.
		data, err = client.requestReadFresh(ctx, "browse", body)
	}
	if err != nil {
		return BrowsePage{}, err
	}
	page := parseBrowseShelvesPage(data, normalizedSectionLimit, normalizedItemLimit, includeMetadata)
	if len(page.Shelves) > 0 {
		return page, nil
	}

	zap.L().Debug(
		"youtube music browse parsed empty; refreshing without read cache",
		zap.Bool("continuationRequest", !includeMetadata),
		zap.Int("initialSectionCount", len(browseSections(data))),
		zap.Int("initialShelfCount", len(page.Shelves)),
		zap.Int("initialTabCount", len(page.Tabs)),
	)
	freshData, err := client.requestReadFresh(ctx, "browse", body)
	if err != nil {
		zap.L().Warn(
			"youtube music browse empty refresh failed",
			zap.Bool("continuationRequest", !includeMetadata),
			zap.Int("initialSectionCount", len(browseSections(data))),
			zap.Error(err),
		)
		return BrowsePage{}, err
	}
	freshPage := parseBrowseShelvesPage(freshData, normalizedSectionLimit, normalizedItemLimit, includeMetadata)
	zap.L().Debug(
		"youtube music browse empty refresh completed",
		zap.Bool("continuationRequest", !includeMetadata),
		zap.Int("freshSectionCount", len(browseSections(freshData))),
		zap.Int("freshShelfCount", len(freshPage.Shelves)),
		zap.Int("freshTabCount", len(freshPage.Tabs)),
	)
	return freshPage, nil
}

// homeBrowseInterstitialError recognizes YouTube Music's successful HTTP
// response that contains only an itemSectionRenderer/messageRenderer
// interstitial instead of Home shelves. Known region-availability messages are
// classified separately because retrying cannot resolve them. Other messages
// remain opaque and transient; upstream text is never included in the error.
func homeBrowseInterstitialError(endpoint string, body map[string]any, data map[string]any) error {
	if strings.Trim(strings.TrimSpace(endpoint), "/") != "browse" ||
		strings.TrimSpace(stringInMap(body, "browseId")) != browseHomeID {
		return nil
	}
	sections := browseSections(data)
	if len(sections) == 0 {
		return nil
	}
	messageTexts := make([]string, 0, len(sections))
	regionUnavailableIcon := false
	for _, section := range sections {
		itemSection := asMap(section["itemSectionRenderer"])
		if itemSection == nil {
			return nil
		}
		items := mapsFromArray(itemSection["contents"])
		if len(items) == 0 {
			return nil
		}
		for _, item := range items {
			messageRenderer := asMap(item["messageRenderer"])
			if messageRenderer == nil {
				return nil
			}
			messageTexts = append(messageTexts, strings.Join(runsText(asMap(messageRenderer["text"])), " "))
			iconType := strings.TrimSpace(stringInMap(asMap(messageRenderer["icon"]), "iconType"))
			if strings.EqualFold(iconType, "MUSIC_UNAVAILABLE") {
				regionUnavailableIcon = true
			}
		}
	}
	if regionUnavailableIcon || isRegionUnavailableMessage(messageTexts) {
		return ErrRegionUnavailable
	}
	return ErrBrowseUnavailable
}

func isRegionUnavailableMessage(messages []string) bool {
	for _, message := range messages {
		normalized := strings.ToLower(strings.Join(strings.Fields(message), " "))
		for _, marker := range []string{
			"youtube music is not available in your area",
			"youtube music isn't available in your area",
			"youtube music is not available in your country",
			"youtube music isn't available in your country",
			"youtube music is not currently available in your country",
		} {
			if strings.Contains(normalized, marker) {
				return true
			}
		}
	}
	return false
}

func parseBrowseShelvesPage(data map[string]any, sectionLimit int, itemLimit int, includeMetadata bool) BrowsePage {
	page := BrowsePage{
		Shelves:      parseHomeShelves(data, sectionLimit, itemLimit),
		Continuation: extractBrowseContinuationToken(data),
	}
	if !includeMetadata {
		return page
	}
	header := playlistHeaderFromBrowseData(data)
	page.Tabs = extractBrowseTabs(data)
	page.Title = header.Title
	page.Author = header.Author
	page.AuthorBrowseID = header.AuthorBrowseID
	page.TrackCountLabel = header.TrackCountLabel
	page.DurationLabel = header.DurationLabel
	page.Description = header.Description
	page.ThumbnailURL = header.ThumbnailURL
	return page
}

func (client *Client) LibraryPlaylists(ctx context.Context, limit int) ([]Playlist, error) {
	data, err := client.requestRead(ctx, "browse", map[string]any{
		"browseId": browseLikedPlaylistsID,
	})
	if err != nil {
		return nil, err
	}
	return parseLibraryBrowsePlaylists(data, normalizeLimit(limit)), nil
}

func (client *Client) LibraryArtists(ctx context.Context, limit int) ([]Artist, error) {
	data, err := client.requestRead(ctx, "browse", map[string]any{
		"browseId": browseLibraryArtistsID,
		"params":   "ggMCCAU=",
	})
	if err != nil {
		return nil, err
	}
	artists := parseLibraryBrowseArtists(data, normalizeLimit(limit))
	if len(artists) > 0 {
		return artists, nil
	}
	landing, landingErr := client.requestRead(ctx, "browse", map[string]any{
		"browseId": browseLibraryLandingID,
	})
	if landingErr != nil {
		return nil, err
	}
	return parseLibraryBrowseArtists(landing, normalizeLimit(limit)), nil
}

func (client *Client) LikedSongs(ctx context.Context, limit int) ([]Track, error) {
	return client.browseTracks(ctx, browseLikedSongsID, limit)
}

func (client *Client) PlaylistQueue(ctx context.Context, playlistID string, limit int) ([]Track, error) {
	trimmedPlaylistID := strings.TrimSpace(playlistID)
	queueLimit := limit
	if queueLimit <= 0 {
		queueLimit = defaultPlaylistQueueMax
	}
	if isMoodCategoryBrowseID(trimmedPlaylistID) {
		return client.browseTracks(ctx, trimmedPlaylistID, queueLimit)
	}
	if isAlbumBrowseID(trimmedPlaylistID) {
		tracks, err := client.browseTracks(ctx, trimmedPlaylistID, queueLimit)
		if err == nil && len(tracks) > 0 {
			return tracks, nil
		}
	}

	rawPlaylistID, err := cleanPlaylistID(playlistID)
	if err != nil {
		return nil, err
	}
	data, err := client.request(ctx, "music/get_queue", map[string]any{
		"playlistId": rawPlaylistID,
	})
	if err != nil {
		if isPlaylistBrowseID(trimmedPlaylistID) {
			if fallback, fallbackErr := client.browseTracks(ctx, trimmedPlaylistID, queueLimit); fallbackErr == nil && len(fallback) > 0 {
				return fallback, nil
			}
		}
		return nil, err
	}
	tracks := parseQueueTracks(data, queueLimit)
	if len(tracks) > 0 || !isPlaylistBrowseID(trimmedPlaylistID) {
		return tracks, nil
	}
	if fallback, err := client.browseTracks(ctx, trimmedPlaylistID, queueLimit); err == nil && len(fallback) > 0 {
		return fallback, nil
	}
	return tracks, nil
}

func (client *Client) PlaylistPage(ctx context.Context, playlistID string, continuation string, limit int) (TrackListPage, error) {
	itemLimit := normalizeLimit(limit)
	if strings.TrimSpace(continuation) != "" {
		return client.browseTracksPage(ctx, "", "", continuation, itemLimit)
	}

	trimmedPlaylistID := strings.TrimSpace(playlistID)
	browseID := playlistBrowseID(trimmedPlaylistID)
	if isMoodCategoryBrowseID(browseID) || isAlbumBrowseID(browseID) || isPlaylistBrowseID(browseID) {
		page, err := client.browseTracksPage(ctx, browseID, "", "", itemLimit)
		if err == nil && len(page.Tracks) > 0 {
			return page, nil
		}
		if isMoodCategoryBrowseID(browseID) || isAlbumBrowseID(browseID) || isPodcastBrowseID(browseID) {
			return page, err
		}
	}

	tracks, err := client.PlaylistQueue(ctx, trimmedPlaylistID, limit)
	if err != nil {
		return TrackListPage{}, err
	}
	return TrackListPage{Tracks: tracks}, nil
}

func playlistBrowseID(playlistID string) string {
	trimmed := strings.TrimSpace(playlistID)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "VL") ||
		strings.HasPrefix(trimmed, "RD") ||
		strings.HasPrefix(trimmed, "OLAK") ||
		strings.HasPrefix(trimmed, "MPRE") ||
		isPodcastBrowseID(trimmed) ||
		strings.HasPrefix(trimmed, "UC") ||
		isMoodCategoryBrowseID(trimmed) {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "PL") {
		return "VL" + trimmed
	}
	return "VL" + trimmed
}

func (client *Client) browseTracks(ctx context.Context, browseID string, limit int) ([]Track, error) {
	cleanedBrowseID, err := cleanBrowseID(browseID)
	if err != nil {
		return nil, err
	}
	itemLimit := normalizeLimit(limit)
	data, err := client.requestRead(ctx, "browse", map[string]any{
		"browseId": cleanedBrowseID,
	})
	if err != nil {
		return nil, err
	}
	shelves := parseHomeShelves(data, defaultShelfLimit, itemLimit)
	tracks := tracksFromShelves(shelves, itemLimit)
	if len(tracks) == 0 {
		tracks = parseHomeRecommendationTracks(data, itemLimit)
	}
	return tracks, nil
}

func (client *Client) browseTracksPage(ctx context.Context, browseID string, params string, continuation string, limit int) (TrackListPage, error) {
	page, err := client.BrowseShelvesPage(ctx, browseID, params, continuation, defaultShelfLimit, limit)
	if err != nil {
		return TrackListPage{}, err
	}
	itemLimit := normalizeLimit(limit)
	return TrackListPage{
		Tracks:          tracksFromShelves(page.Shelves, itemLimit),
		Continuation:    page.Continuation,
		Title:           page.Title,
		Author:          page.Author,
		AuthorBrowseID:  page.AuthorBrowseID,
		TrackCountLabel: page.TrackCountLabel,
		DurationLabel:   page.DurationLabel,
		Description:     page.Description,
		ThumbnailURL:    page.ThumbnailURL,
	}, nil
}

func (client *Client) ArtistPage(ctx context.Context, browseID string, limit int) (ArtistPage, error) {
	artistBrowseID, err := cleanBrowseID(browseID)
	if err != nil {
		return ArtistPage{}, err
	}
	itemLimit := normalizeLimit(limit)
	data, err := client.requestRead(ctx, "browse", map[string]any{
		"browseId": artistBrowseID,
	})
	if err != nil {
		return ArtistPage{}, err
	}
	header := artistHeaderFromBrowseData(data, artistBrowseID)
	shelves := parseHomeShelves(data, defaultShelfLimit, itemLimit)
	tracks := tracksFromShelves(shelves, itemLimit)
	if len(tracks) == 0 {
		tracks = parseHomeRecommendationTracks(data, itemLimit)
	}
	return ArtistPage{
		ID:               artistBrowseID,
		Title:            firstNonEmpty(header.Title, browsePageTitle(data)),
		Subtitle:         header.Subtitle,
		Description:      header.Description,
		ThumbnailURL:     header.ThumbnailURL,
		HeroThumbnailURL: header.HeroThumbnailURL,
		ChannelID:        header.ChannelID,
		IsSubscribed:     header.IsSubscribed,
		MixPlaylistID:    header.MixPlaylistID,
		MixVideoID:       header.MixVideoID,
		Tracks:           tracks,
		Shelves:          shelves,
		Continuation:     extractBrowseContinuationToken(data),
	}, nil
}

func (client *Client) SubscribePlaylist(ctx context.Context, playlistID string) error {
	return client.editPlaylistLibrary(ctx, playlistID, "like/like")
}

func (client *Client) UnsubscribePlaylist(ctx context.Context, playlistID string) error {
	return client.editPlaylistLibrary(ctx, playlistID, "like/removelike")
}

func (client *Client) SubscribeArtist(ctx context.Context, channelID string) error {
	return client.editArtistSubscription(ctx, channelID, "subscription/subscribe")
}

func (client *Client) UnsubscribeArtist(ctx context.Context, channelID string) error {
	return client.editArtistSubscription(ctx, channelID, "subscription/unsubscribe")
}

func (client *Client) editArtistSubscription(ctx context.Context, channelID string, endpoint string) error {
	trimmed := strings.TrimSpace(channelID)
	if !strings.HasPrefix(trimmed, "UC") || len(trimmed) > 128 {
		return fmt.Errorf("invalid youtube music artist channel id")
	}
	_, err := client.request(ctx, endpoint, map[string]any{
		"channelIds": []string{trimmed},
	})
	if err == nil {
		client.clearRequestCache()
	}
	return err
}

func (client *Client) editPlaylistLibrary(ctx context.Context, playlistID string, endpoint string) error {
	rawPlaylistID, err := cleanPlaylistID(playlistID)
	if err != nil {
		return err
	}
	_, err = client.request(ctx, endpoint, map[string]any{
		"target": map[string]any{
			"playlistId": rawPlaylistID,
		},
	})
	if err == nil {
		client.clearRequestCache()
	}
	return err
}

func cleanPlaylistID(playlistID string) (string, error) {
	trimmed := strings.TrimSpace(playlistID)
	if trimmed == "" {
		return "", fmt.Errorf("invalid youtube music playlist id")
	}
	return strings.TrimPrefix(trimmed, "VL"), nil
}

func cleanBrowseID(browseID string) (string, error) {
	trimmed := strings.TrimSpace(browseID)
	if trimmed == "" || len(trimmed) > 256 {
		return "", fmt.Errorf("invalid youtube music browse id")
	}
	for _, character := range trimmed {
		if character >= 'a' && character <= 'z' {
			continue
		}
		if character >= 'A' && character <= 'Z' {
			continue
		}
		if character >= '0' && character <= '9' {
			continue
		}
		switch character {
		case '_', '-', '=':
			continue
		default:
			return "", fmt.Errorf("invalid youtube music browse id")
		}
	}
	return trimmed, nil
}

func cleanBrowseParams(params string) (string, error) {
	trimmed := strings.TrimSpace(params)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > 512 {
		return "", fmt.Errorf("invalid youtube music browse params")
	}
	for _, character := range trimmed {
		if character >= 'a' && character <= 'z' {
			continue
		}
		if character >= 'A' && character <= 'Z' {
			continue
		}
		if character >= '0' && character <= '9' {
			continue
		}
		switch character {
		case '_', '-', '=', '%':
			continue
		default:
			return "", fmt.Errorf("invalid youtube music browse params")
		}
	}
	return trimmed, nil
}

func parseHomeShelves(data map[string]any, sectionLimit int, itemLimit int) []Shelf {
	sections := browseSections(data)
	shelves := make([]Shelf, 0, min(len(sections), sectionLimit))
	seen := make(map[string]struct{}, len(sections))
	for _, section := range sections {
		for _, shelf := range shelvesFromSection(section, itemLimit) {
			if shelf.ID == "" {
				continue
			}
			if _, exists := seen[shelf.ID]; exists {
				continue
			}
			seen[shelf.ID] = struct{}{}
			shelves = append(shelves, shelf)
			if len(shelves) >= sectionLimit {
				return shelves
			}
		}
	}
	return shelves
}

func parseHomeRecommendationTracks(data map[string]any, limit int) []Track {
	items := browseSectionItems(data)
	tracks := make([]Track, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		track, ok := trackFromHomeItem(item)
		if !ok {
			continue
		}
		if _, exists := seen[track.VideoID]; exists {
			continue
		}
		seen[track.VideoID] = struct{}{}
		tracks = append(tracks, track)
		if len(tracks) >= limit {
			break
		}
	}
	return tracks
}

func tracksFromShelves(shelves []Shelf, limit int) []Track {
	tracks := make([]Track, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, shelf := range shelves {
		for _, track := range shelf.Tracks {
			if track.VideoID == "" {
				continue
			}
			if _, exists := seen[track.VideoID]; exists {
				continue
			}
			seen[track.VideoID] = struct{}{}
			tracks = append(tracks, track)
			if len(tracks) >= limit {
				return tracks
			}
		}
	}
	return tracks
}

func parseLibraryBrowsePlaylists(data map[string]any, limit int) []Playlist {
	items := browseSectionItems(data)
	playlists := make([]Playlist, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		playlist, ok := playlistFromLibraryItem(item)
		if !ok {
			continue
		}
		if _, exists := seen[playlist.ID]; exists {
			continue
		}
		seen[playlist.ID] = struct{}{}
		playlists = append(playlists, playlist)
		if len(playlists) >= limit {
			break
		}
	}
	return playlists
}

func parseLibraryBrowseArtists(data map[string]any, limit int) []Artist {
	items := browseSectionItems(data)
	artists := make([]Artist, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		artist, ok := artistFromLibraryItem(item)
		if !ok {
			continue
		}
		if _, exists := seen[artist.ID]; exists {
			continue
		}
		seen[artist.ID] = struct{}{}
		artists = append(artists, artist)
		if len(artists) >= limit {
			break
		}
	}
	return artists
}

func parseQueueTracks(data map[string]any, limit int) []Track {
	queueDatas, ok := data["queueDatas"].([]any)
	if !ok {
		return nil
	}
	tracks := make([]Track, 0, len(queueDatas))
	seen := make(map[string]struct{}, len(queueDatas))
	for _, item := range queueDatas {
		queueData := asMap(item)
		if queueData == nil {
			continue
		}
		content := asMap(queueData["content"])
		if content == nil {
			continue
		}
		renderer := extractQueueRenderer(content)
		if renderer == nil {
			continue
		}
		track, ok := trackFromPlaylistPanelRenderer(renderer)
		if !ok {
			continue
		}
		if _, exists := seen[track.VideoID]; exists {
			continue
		}
		seen[track.VideoID] = struct{}{}
		tracks = append(tracks, track)
		if len(tracks) >= limit {
			break
		}
	}
	return tracks
}

func shelvesFromSection(section map[string]any, itemLimit int) []Shelf {
	title := sectionTitle(section)
	continuation := sectionContinuationToken(section)
	browseRef := sectionBrowseEndpoint(section)
	items := sectionItems(section)
	if len(items) == 0 {
		return nil
	}

	tracks := make([]Track, 0, min(len(items), itemLimit))
	playlists := make([]Playlist, 0, min(len(items), itemLimit))
	categories := make([]Category, 0, min(len(items), itemLimit))
	artists := make([]Artist, 0, min(len(items), itemLimit))
	seenTracks := make(map[string]struct{}, len(items))
	seenPlaylists := make(map[string]struct{}, len(items))
	seenCategories := make(map[string]struct{}, len(items))
	seenArtists := make(map[string]struct{}, len(items))
	for _, item := range items {
		if len(tracks) < itemLimit {
			if track, ok := trackFromHomeItem(item); ok {
				if _, exists := seenTracks[track.VideoID]; !exists {
					seenTracks[track.VideoID] = struct{}{}
					tracks = append(tracks, track)
				}
			}
		}
		if len(playlists) < itemLimit {
			if playlist, ok := playlistFromLibraryItem(item); ok {
				if _, exists := seenPlaylists[playlist.ID]; !exists {
					seenPlaylists[playlist.ID] = struct{}{}
					playlists = append(playlists, playlist)
				}
			}
		}
		if len(categories) < itemLimit {
			if category, ok := categoryFromLibraryItem(item); ok {
				if _, exists := seenCategories[category.ID]; !exists {
					seenCategories[category.ID] = struct{}{}
					categories = append(categories, category)
				}
			}
		}
		if len(artists) < itemLimit {
			if artist, ok := artistFromLibraryItem(item); ok {
				if _, exists := seenArtists[artist.ID]; !exists {
					seenArtists[artist.ID] = struct{}{}
					artists = append(artists, artist)
				}
			}
		}
		if len(tracks) >= itemLimit && len(playlists) >= itemLimit && len(categories) >= itemLimit && len(artists) >= itemLimit {
			break
		}
	}

	result := make([]Shelf, 0, 4)
	if len(categories) > 0 {
		result = append(result, Shelf{
			ID:           buildShelfID(title, ShelfCategories, categories[0].ID),
			Title:        fallbackShelfTitle(title, ShelfCategories),
			Kind:         ShelfCategories,
			Continuation: continuation,
			BrowseID:     browseRef.BrowseID,
			Params:       browseRef.Params,
			Categories:   categories,
		})
	}
	if len(artists) > 0 {
		result = append(result, Shelf{
			ID:           buildShelfID(title, ShelfArtists, artists[0].ID),
			Title:        fallbackShelfTitle(title, ShelfArtists),
			Kind:         ShelfArtists,
			Continuation: continuation,
			BrowseID:     browseRef.BrowseID,
			Params:       browseRef.Params,
			Artists:      artists,
		})
	}
	if len(tracks) > 0 {
		result = append(result, Shelf{
			ID:           buildShelfID(title, ShelfTracks, tracks[0].VideoID),
			Title:        fallbackShelfTitle(title, ShelfTracks),
			Kind:         ShelfTracks,
			Continuation: continuation,
			BrowseID:     browseRef.BrowseID,
			Params:       browseRef.Params,
			Tracks:       tracks,
		})
	}
	if len(playlists) > 0 {
		result = append(result, Shelf{
			ID:           buildShelfID(title, ShelfPlaylists, playlists[0].ID),
			Title:        fallbackShelfTitle(title, ShelfPlaylists),
			Kind:         ShelfPlaylists,
			Continuation: continuation,
			BrowseID:     browseRef.BrowseID,
			Params:       browseRef.Params,
			Playlists:    playlists,
		})
	}
	return result
}

func extractBrowseTabs(data map[string]any) []BrowseTab {
	contents := asMap(data["contents"])
	if contents == nil {
		return nil
	}
	if tabs := browseTabsFromSingleColumnRenderer(asMap(contents["singleColumnBrowseResultsRenderer"])); len(tabs) > 0 {
		return tabs
	}
	if tabs := browseTabsFromSingleColumnRenderer(asMap(contents["twoColumnBrowseResultsRenderer"])); len(tabs) > 0 {
		return tabs
	}
	return nil
}

func browseTabsFromSingleColumnRenderer(singleColumn map[string]any) []BrowseTab {
	if singleColumn == nil {
		return nil
	}
	rawTabs := mapsFromArray(singleColumn["tabs"])
	if len(rawTabs) == 0 {
		return nil
	}
	tabs := make([]BrowseTab, 0, len(rawTabs))
	seen := make(map[string]struct{}, len(rawTabs))
	for _, rawTab := range rawTabs {
		tabRenderer := asMap(rawTab["tabRenderer"])
		if tabRenderer == nil {
			continue
		}
		endpoint := asMap(tabRenderer["endpoint"])
		if endpoint == nil {
			endpoint = asMap(tabRenderer["navigationEndpoint"])
		}
		browseEndpoint := asMap(endpoint["browseEndpoint"])
		browseID := stringInMap(browseEndpoint, "browseId")
		if browseID == "" {
			continue
		}
		params := stringInMap(browseEndpoint, "params")
		key := browseID + "\x00" + params
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		selected, _ := tabRenderer["selected"].(bool)
		tabs = append(tabs, BrowseTab{
			Title: firstNonEmpty(
				firstUsefulText(runsText(asMap(tabRenderer["title"]))),
				stringInMap(tabRenderer, "title"),
				stringInMap(tabRenderer, "tabIdentifier"),
			),
			BrowseID: browseID,
			Params:   params,
			Selected: selected,
		})
	}
	return tabs
}

func browseSectionItems(data map[string]any) []map[string]any {
	sections := browseSections(data)
	items := make([]map[string]any, 0, len(sections)*4)
	for _, section := range sections {
		items = append(items, sectionItems(section)...)
	}
	return items
}

func browseSections(data map[string]any) []map[string]any {
	contents := asMap(data["contents"])
	if contents != nil {
		if sections := sectionsFromSingleColumnRenderer(asMap(contents["singleColumnBrowseResultsRenderer"])); len(sections) > 0 {
			return sections
		}
		if sections := sectionsFromTwoColumnRenderer(asMap(contents["twoColumnBrowseResultsRenderer"])); len(sections) > 0 {
			return sections
		}
	}
	if sections := sectionsFromContinuation(data); len(sections) > 0 {
		return sections
	}
	return nil
}

func sectionsFromContinuation(data map[string]any) []map[string]any {
	continuationContents := asMap(data["continuationContents"])
	if continuationContents != nil {
		if sectionList := asMap(continuationContents["sectionListContinuation"]); sectionList != nil {
			if sections := mapsFromArray(sectionList["contents"]); len(sections) > 0 {
				return sections
			}
		}
		if shelf := asMap(continuationContents["musicShelfContinuation"]); shelf != nil {
			return []map[string]any{{"musicShelfRenderer": shelf}}
		}
		if shelf := asMap(continuationContents["musicPlaylistShelfContinuation"]); shelf != nil {
			return []map[string]any{{"musicPlaylistShelfRenderer": shelf}}
		}
		if shelf := asMap(continuationContents["musicCarouselShelfContinuation"]); shelf != nil {
			return []map[string]any{{"musicCarouselShelfRenderer": shelf}}
		}
	}
	for _, action := range mapsFromArray(data["onResponseReceivedActions"]) {
		appendAction := asMap(action["appendContinuationItemsAction"])
		items := mapsFromArray(appendAction["continuationItems"])
		if len(items) == 0 {
			continue
		}
		if containsSectionRenderer(items) {
			return items
		}
		return []map[string]any{{
			"musicShelfRenderer": map[string]any{
				"title":    map[string]any{"runs": []any{map[string]any{"text": "More"}}},
				"contents": items,
			},
		}}
	}
	return nil
}

func containsSectionRenderer(items []map[string]any) bool {
	for _, item := range items {
		for _, key := range []string{
			"musicCarouselShelfRenderer",
			"musicShelfRenderer",
			"musicPlaylistShelfRenderer",
			"musicCardShelfRenderer",
			"musicImmersiveCarouselShelfRenderer",
			"gridRenderer",
			"itemSectionRenderer",
		} {
			if asMap(item[key]) != nil {
				return true
			}
		}
	}
	return false
}

func sectionsFromSingleColumnRenderer(singleColumn map[string]any) []map[string]any {
	if singleColumn == nil {
		return nil
	}
	tabs := mapsFromArray(singleColumn["tabs"])
	if len(tabs) == 0 {
		return nil
	}
	sections := make([]map[string]any, 0, len(tabs))
	for _, tab := range tabs {
		tabRenderer := asMap(tab["tabRenderer"])
		if tabRenderer == nil {
			continue
		}
		tabContent := asMap(tabRenderer["content"])
		if tabContent == nil {
			continue
		}
		sectionList := asMap(tabContent["sectionListRenderer"])
		if sectionList == nil {
			continue
		}
		sections = append(sections, mapsFromArray(sectionList["contents"])...)
	}
	return sections
}

func sectionsFromTwoColumnRenderer(twoColumn map[string]any) []map[string]any {
	if twoColumn == nil {
		return nil
	}
	if secondaryContents := asMap(twoColumn["secondaryContents"]); secondaryContents != nil {
		if sectionList := asMap(secondaryContents["sectionListRenderer"]); sectionList != nil {
			if sections := mapsFromArray(sectionList["contents"]); len(sections) > 0 {
				return sections
			}
		}
	}
	return sectionsFromSingleColumnRenderer(twoColumn)
}

func extractBrowseContinuationToken(data map[string]any) string {
	if contents := asMap(data["contents"]); contents != nil {
		if token := continuationTokenFromSingleColumnRenderer(asMap(contents["singleColumnBrowseResultsRenderer"])); token != "" {
			return token
		}
		if token := continuationTokenFromTwoColumnRenderer(asMap(contents["twoColumnBrowseResultsRenderer"])); token != "" {
			return token
		}
	}
	if continuationContents := asMap(data["continuationContents"]); continuationContents != nil {
		for _, key := range []string{
			"sectionListContinuation",
			"musicShelfContinuation",
			"musicPlaylistShelfContinuation",
			"musicCarouselShelfContinuation",
		} {
			renderer := asMap(continuationContents[key])
			if token := continuationTokenFromRenderer(renderer); token != "" {
				return token
			}
			if token := continuationTokenFromContents(mapsFromArray(renderer["contents"])); token != "" {
				return token
			}
		}
	}
	for _, action := range mapsFromArray(data["onResponseReceivedActions"]) {
		appendAction := asMap(action["appendContinuationItemsAction"])
		if token := continuationTokenFromContents(mapsFromArray(appendAction["continuationItems"])); token != "" {
			return token
		}
	}
	return ""
}

func continuationTokenFromSingleColumnRenderer(singleColumn map[string]any) string {
	if singleColumn == nil {
		return ""
	}
	tabs := mapsFromArray(singleColumn["tabs"])
	if len(tabs) == 0 {
		return ""
	}
	for _, tab := range tabs {
		tabRenderer := asMap(tab["tabRenderer"])
		tabContent := asMap(tabRenderer["content"])
		sectionList := asMap(tabContent["sectionListRenderer"])
		if token := continuationTokenFromRenderer(sectionList); token != "" {
			return token
		}
		if token := continuationTokenFromSections(mapsFromArray(sectionList["contents"])); token != "" {
			return token
		}
	}
	return ""
}

func continuationTokenFromTwoColumnRenderer(twoColumn map[string]any) string {
	if twoColumn == nil {
		return ""
	}
	if secondaryContents := asMap(twoColumn["secondaryContents"]); secondaryContents != nil {
		if sectionList := asMap(secondaryContents["sectionListRenderer"]); sectionList != nil {
			if token := continuationTokenFromSections(mapsFromArray(sectionList["contents"])); token != "" {
				return token
			}
			if token := continuationTokenFromRenderer(sectionList); token != "" {
				return token
			}
		}
	}
	return continuationTokenFromSingleColumnRenderer(twoColumn)
}

func continuationTokenFromSections(sections []map[string]any) string {
	for _, section := range sections {
		for _, key := range []string{
			"musicPlaylistShelfRenderer",
			"musicShelfRenderer",
			"musicCarouselShelfRenderer",
			"gridRenderer",
		} {
			renderer := asMap(section[key])
			if token := continuationTokenFromRenderer(renderer); token != "" {
				return token
			}
			if token := continuationTokenFromContents(mapsFromArray(renderer["contents"])); token != "" {
				return token
			}
			if token := continuationTokenFromContents(mapsFromArray(renderer["items"])); token != "" {
				return token
			}
		}
		if itemSection := asMap(section["itemSectionRenderer"]); itemSection != nil {
			if token := continuationTokenFromSections(mapsFromArray(itemSection["contents"])); token != "" {
				return token
			}
		}
	}
	return ""
}

func continuationTokenFromRenderer(renderer map[string]any) string {
	if renderer == nil {
		return ""
	}
	for _, continuation := range mapsFromArray(renderer["continuations"]) {
		if token := stringInMap(asMap(continuation["nextContinuationData"]), "continuation"); token != "" {
			return token
		}
	}
	return ""
}

func continuationTokenFromContents(contents []map[string]any) string {
	for index := len(contents) - 1; index >= 0; index-- {
		renderer := asMap(contents[index]["continuationItemRenderer"])
		if renderer == nil {
			continue
		}
		endpoint := asMap(renderer["continuationEndpoint"])
		command := asMap(endpoint["continuationCommand"])
		if token := stringInMap(command, "token"); token != "" {
			return token
		}
	}
	return ""
}

func sectionTitle(section map[string]any) string {
	if section == nil {
		return ""
	}
	switch {
	case asMap(section["musicCarouselShelfRenderer"]) != nil:
		renderer := asMap(section["musicCarouselShelfRenderer"])
		return firstNonEmpty(
			headerTitle(asMap(asMap(renderer["header"])["musicCarouselShelfBasicHeaderRenderer"])),
			headerTitle(asMap(renderer["header"])),
			firstUsefulText(runsText(asMap(renderer["title"]))),
		)
	case asMap(section["musicShelfRenderer"]) != nil:
		renderer := asMap(section["musicShelfRenderer"])
		return firstNonEmpty(
			firstUsefulText(runsText(asMap(renderer["title"]))),
			headerTitle(asMap(renderer["header"])),
		)
	case asMap(section["musicPlaylistShelfRenderer"]) != nil:
		renderer := asMap(section["musicPlaylistShelfRenderer"])
		return firstNonEmpty(
			firstUsefulText(runsText(asMap(renderer["title"]))),
			headerTitle(asMap(renderer["header"])),
		)
	case asMap(section["musicCardShelfRenderer"]) != nil:
		renderer := asMap(section["musicCardShelfRenderer"])
		return firstNonEmpty(
			headerTitle(asMap(asMap(renderer["header"])["musicCardShelfHeaderBasicRenderer"])),
			headerTitle(asMap(renderer["header"])),
		)
	case asMap(section["musicImmersiveCarouselShelfRenderer"]) != nil:
		renderer := asMap(section["musicImmersiveCarouselShelfRenderer"])
		return firstNonEmpty(
			headerTitle(asMap(asMap(renderer["header"])["musicCarouselShelfBasicHeaderRenderer"])),
			headerTitle(asMap(renderer["header"])),
		)
	case asMap(section["gridRenderer"]) != nil:
		renderer := asMap(section["gridRenderer"])
		return firstNonEmpty(
			headerTitle(asMap(asMap(renderer["header"])["gridHeaderRenderer"])),
			headerTitle(asMap(renderer["header"])),
		)
	case asMap(section["itemSectionRenderer"]) != nil:
		for _, child := range mapsFromArray(asMap(section["itemSectionRenderer"])["contents"]) {
			if title := sectionTitle(child); title != "" {
				return title
			}
		}
	}
	return ""
}

func sectionContinuationToken(section map[string]any) string {
	renderer := sectionRenderer(section)
	if token := continuationTokenFromRenderer(renderer); token != "" {
		return token
	}
	return continuationTokenFromContents(sectionItems(section))
}

type browseEndpointRef struct {
	BrowseID string
	Params   string
}

func sectionBrowseEndpoint(section map[string]any) browseEndpointRef {
	return browseEndpointFromRenderer(sectionRenderer(section))
}

func browseEndpointFromRenderer(renderer map[string]any) browseEndpointRef {
	if renderer == nil {
		return browseEndpointRef{}
	}
	candidates := []map[string]any{
		asMap(renderer["bottomEndpoint"]),
		asMap(renderer["navigationEndpoint"]),
		asMap(renderer["moreContentButton"]),
	}
	if title := asMap(renderer["title"]); title != nil {
		candidates = append(candidates, title)
	}
	if header := asMap(renderer["header"]); header != nil {
		candidates = append(candidates, header)
		for _, key := range []string{
			"musicCarouselShelfBasicHeaderRenderer",
			"musicShelfBasicHeaderRenderer",
			"musicCardShelfHeaderBasicRenderer",
			"gridHeaderRenderer",
		} {
			candidates = append(candidates, asMap(header[key]))
		}
	}
	for _, candidate := range candidates {
		if ref := browseEndpointFromCandidate(candidate); ref.BrowseID != "" {
			return ref
		}
	}
	return browseEndpointRef{}
}

func browseEndpointFromCandidate(candidate map[string]any) browseEndpointRef {
	if candidate == nil {
		return browseEndpointRef{}
	}
	if ref := browseEndpointFromEndpoint(candidate); ref.BrowseID != "" {
		return ref
	}
	for _, key := range []string{"navigationEndpoint", "command", "clickCommand"} {
		if ref := browseEndpointFromEndpoint(asMap(candidate[key])); ref.BrowseID != "" {
			return ref
		}
	}
	for _, key := range []string{"buttonRenderer", "musicPlayButtonRenderer", "toggleButtonRenderer"} {
		if ref := browseEndpointFromCandidate(asMap(candidate[key])); ref.BrowseID != "" {
			return ref
		}
	}
	for _, key := range []string{"moreContentButton", "moreButton"} {
		if ref := browseEndpointFromCandidate(asMap(candidate[key])); ref.BrowseID != "" {
			return ref
		}
	}
	for _, run := range mapsFromArray(candidate["runs"]) {
		if ref := browseEndpointFromCandidate(run); ref.BrowseID != "" {
			return ref
		}
	}
	for _, item := range mapsFromArray(candidate["items"]) {
		if ref := browseEndpointFromCandidate(item); ref.BrowseID != "" {
			return ref
		}
	}
	return browseEndpointRef{}
}

func browseEndpointFromEndpoint(endpoint map[string]any) browseEndpointRef {
	if endpoint == nil {
		return browseEndpointRef{}
	}
	browseEndpoint := asMap(endpoint["browseEndpoint"])
	browseID := stringInMap(browseEndpoint, "browseId")
	if browseID == "" {
		return browseEndpointRef{}
	}
	return browseEndpointRef{
		BrowseID: browseID,
		Params:   stringInMap(browseEndpoint, "params"),
	}
}

func sectionRenderer(section map[string]any) map[string]any {
	if section == nil {
		return nil
	}
	for _, key := range []string{
		"musicCarouselShelfRenderer",
		"musicShelfRenderer",
		"musicPlaylistShelfRenderer",
		"musicCardShelfRenderer",
		"musicImmersiveCarouselShelfRenderer",
		"gridRenderer",
		"itemSectionRenderer",
	} {
		if renderer := asMap(section[key]); renderer != nil {
			return renderer
		}
	}
	return nil
}

func sectionItems(section map[string]any) []map[string]any {
	if section == nil {
		return nil
	}
	switch {
	case asMap(section["musicCarouselShelfRenderer"]) != nil:
		return mapsFromArray(asMap(section["musicCarouselShelfRenderer"])["contents"])
	case asMap(section["musicShelfRenderer"]) != nil:
		return mapsFromArray(asMap(section["musicShelfRenderer"])["contents"])
	case asMap(section["musicPlaylistShelfRenderer"]) != nil:
		return mapsFromArray(asMap(section["musicPlaylistShelfRenderer"])["contents"])
	case asMap(section["musicCardShelfRenderer"]) != nil:
		return mapsFromArray(asMap(section["musicCardShelfRenderer"])["contents"])
	case asMap(section["musicImmersiveCarouselShelfRenderer"]) != nil:
		return mapsFromArray(asMap(section["musicImmersiveCarouselShelfRenderer"])["contents"])
	case asMap(section["gridRenderer"]) != nil:
		return mapsFromArray(asMap(section["gridRenderer"])["items"])
	case asMap(section["itemSectionRenderer"]) != nil:
		result := make([]map[string]any, 0)
		for _, child := range mapsFromArray(asMap(section["itemSectionRenderer"])["contents"]) {
			result = append(result, sectionItems(child)...)
		}
		return result
	default:
		return nil
	}
}

func trackFromHomeItem(item map[string]any) (Track, bool) {
	if renderer := asMap(item["musicResponsiveListItemRenderer"]); renderer != nil {
		return trackFromMusicResponsiveRenderer(renderer)
	}
	if renderer := asMap(item["musicMultiRowListItemRenderer"]); renderer != nil {
		return trackFromMusicMultiRowRenderer(renderer)
	}
	if renderer := asMap(item["musicTwoRowItemRenderer"]); renderer != nil {
		return trackFromHomeTwoRowRenderer(renderer)
	}
	return Track{}, false
}

// Podcast episode shelves use musicMultiRowListItemRenderer in some YouTube
// Music responses. Keep their projection deliberately conservative: require a
// valid playable video ID, then reuse only visible metadata that is stable
// across renderer revisions.
func trackFromMusicMultiRowRenderer(renderer map[string]any) (Track, bool) {
	videoID := findFirstStringByKey(renderer, "videoId")
	if !videoIDPattern.MatchString(videoID) {
		return Track{}, false
	}
	title := firstUsefulText(runsText(asMap(renderer["title"])))
	if title == "" {
		title = firstUsefulText(collectTextRuns(renderer))
	}
	if title == "" {
		title = videoID
	}
	subtitleRuns := runsText(asMap(renderer["subtitle"]))
	channel := firstCreatorText(subtitleRuns)
	allText := collectTextRuns(renderer)
	return Track{
		ID:             videoID,
		VideoID:        videoID,
		Title:          title,
		Channel:        fallbackString(channel, "YouTube Music"),
		DurationLabel:  firstDurationLabel(allText),
		ThumbnailURL:   lastThumbnailURL(renderer),
		MusicVideoType: positiveMusicVideoTypeFromRenderer(renderer),
		RawDescription: firstUsefulText(runsText(asMap(renderer["description"]))),
	}, true
}

func trackFromHomeTwoRowRenderer(renderer map[string]any) (Track, bool) {
	navigationEndpoint := asMap(renderer["navigationEndpoint"])
	watchEndpoint := asMap(navigationEndpoint["watchEndpoint"])
	videoID := stringInMap(watchEndpoint, "videoId")
	if !videoIDPattern.MatchString(videoID) {
		return Track{}, false
	}
	title := firstUsefulText(runsText(asMap(renderer["title"])))
	if title == "" {
		title = videoID
	}
	subtitleNavigationRuns := textRunsWithNavigation(asMap(renderer["subtitle"]))
	subtitleRuns := textValuesFromRuns(subtitleNavigationRuns)
	channel, artistBrowseID, artistSource, artists := artistRunFromBylineRuns(subtitleNavigationRuns)
	if channel == "" {
		channel = firstCreatorText(subtitleRuns)
	}
	duration := firstDurationLabel(subtitleRuns)
	return Track{
		ID:             videoID,
		VideoID:        videoID,
		Title:          title,
		Channel:        fallbackString(channel, "YouTube Music"),
		Artists:        artists,
		ArtistBrowseID: artistBrowseID,
		ArtistSource:   artistSource,
		DurationLabel:  duration,
		ThumbnailURL:   lastThumbnailURL(renderer),
	}, true
}

func playlistFromLibraryItem(item map[string]any) (Playlist, bool) {
	if renderer := asMap(item["musicTwoRowItemRenderer"]); renderer != nil {
		return playlistFromTwoRowRenderer(renderer)
	}
	if renderer := asMap(item["musicResponsiveListItemRenderer"]); renderer != nil {
		return playlistFromResponsiveRenderer(renderer)
	}
	return Playlist{}, false
}

func categoryFromLibraryItem(item map[string]any) (Category, bool) {
	if renderer := asMap(item["musicNavigationButtonRenderer"]); renderer != nil {
		return categoryFromNavigationButtonRenderer(renderer)
	}
	return Category{}, false
}

func categoryFromNavigationButtonRenderer(renderer map[string]any) (Category, bool) {
	title := firstUsefulText(runsText(asMap(renderer["buttonText"])))
	if title == "" {
		return Category{}, false
	}
	browseEndpoint := asMap(asMap(renderer["clickCommand"])["browseEndpoint"])
	browseID := stringInMap(browseEndpoint, "browseId")
	if !isMoodCategoryBrowseID(browseID) {
		return Category{}, false
	}
	params := stringInMap(browseEndpoint, "params")
	id := browseID
	if params != "" {
		id += "_" + params
	}
	return Category{
		ID:           id,
		BrowseID:     browseID,
		Params:       params,
		Title:        title,
		ColorHex:     navigationButtonColorHex(renderer),
		ThumbnailURL: lastThumbnailURL(renderer),
	}, true
}

func artistFromLibraryItem(item map[string]any) (Artist, bool) {
	if renderer := asMap(item["musicResponsiveListItemRenderer"]); renderer != nil {
		return artistFromSearchResponsiveRenderer(renderer)
	}
	if renderer := asMap(item["musicTwoRowItemRenderer"]); renderer != nil {
		return artistFromTwoRowRenderer(renderer)
	}
	return Artist{}, false
}

func artistFromTwoRowRenderer(renderer map[string]any) (Artist, bool) {
	navigationEndpoint := asMap(renderer["navigationEndpoint"])
	browseEndpoint := asMap(navigationEndpoint["browseEndpoint"])
	browseID := stringInMap(browseEndpoint, "browseId")
	if browseID == "" || (!isArtistBrowseID(browseID) && !isArtistPageType(pageTypeFromBrowseEndpoint(browseEndpoint))) {
		return Artist{}, false
	}
	title := firstUsefulText(runsText(asMap(renderer["title"])))
	if title == "" {
		title = browseID
	}
	return Artist{
		ID:           browseID,
		Name:         title,
		Subtitle:     firstUsefulText(runsText(asMap(renderer["subtitle"]))),
		ThumbnailURL: lastThumbnailURL(renderer),
	}, true
}

func playlistFromTwoRowRenderer(renderer map[string]any) (Playlist, bool) {
	browseID := browseIDFromNavigationEndpoint(asMap(renderer["navigationEndpoint"]))
	if !isPlaylistBrowseID(browseID) {
		return Playlist{}, false
	}
	title := firstUsefulText(runsText(asMap(renderer["title"])))
	if title == "" {
		title = browseID
	}
	channel, description := playlistMetadataFromValues(runsText(asMap(renderer["subtitle"])))
	return Playlist{
		ID:           browseID,
		Title:        title,
		Channel:      channel,
		Description:  description,
		ThumbnailURL: lastThumbnailURL(renderer),
	}, true
}

func playlistFromResponsiveRenderer(renderer map[string]any) (Playlist, bool) {
	browseID := browseIDFromNavigationEndpoint(asMap(renderer["navigationEndpoint"]))
	if !isPlaylistBrowseID(browseID) {
		return Playlist{}, false
	}
	title := ""
	flexColumns := mapsFromArray(renderer["flexColumns"])
	if len(flexColumns) > 0 {
		title = firstUsefulText(textRunsFromFlexColumn(flexColumns[0]))
	}
	if title == "" {
		title = firstUsefulText(collectTextRuns(renderer))
	}
	if title == "" {
		title = browseID
	}
	metadataRuns := make([]string, 0, 4)
	for _, column := range flexColumns[1:] {
		metadataRuns = append(metadataRuns, textRunsFromFlexColumn(column)...)
	}
	channel, description := playlistMetadataFromValues(metadataRuns)
	return Playlist{
		ID:           browseID,
		Title:        title,
		Channel:      channel,
		Description:  description,
		ThumbnailURL: lastThumbnailURL(renderer),
	}, true
}

type playlistHeaderData struct {
	Title           string
	Description     string
	ThumbnailURL    string
	Author          string
	AuthorBrowseID  string
	TrackCountLabel string
	DurationLabel   string
}

func playlistHeaderFromBrowseData(data map[string]any) playlistHeaderData {
	header := playlistHeaderData{}
	headerDict := asMap(data["header"])
	if headerDict != nil {
		applyPlaylistDetailHeaderRenderer(headerDict, &header)
		applyPlaylistImmersiveHeaderRenderer(headerDict, &header)
		applyPlaylistVisualHeaderRenderer(headerDict, &header)
		applyPlaylistEditableHeaderRenderer(headerDict, &header)
	}
	if responsiveHeader := extractPlaylistResponsiveHeaderRenderer(data); responsiveHeader != nil {
		applyPlaylistResponsiveHeaderRenderer(responsiveHeader, &header)
	}
	return header
}

func applyPlaylistDetailHeaderRenderer(headerDict map[string]any, header *playlistHeaderData) {
	renderer := asMap(headerDict["musicDetailHeaderRenderer"])
	if renderer == nil {
		return
	}
	if title := firstUsefulText(runsText(asMap(renderer["title"]))); title != "" {
		header.Title = title
	}
	if description := strings.Join(runsText(asMap(renderer["description"])), ""); description != "" {
		header.Description = description
	}
	if thumbnailURL := lastThumbnailURL(renderer); thumbnailURL != "" {
		header.ThumbnailURL = thumbnailURL
	}
	applyPlaylistAuthorText(asMap(renderer["subtitle"]), header)
	applyPlaylistStructuredMetadata(renderer, header)
}

func applyPlaylistImmersiveHeaderRenderer(headerDict map[string]any, header *playlistHeaderData) {
	renderer := asMap(headerDict["musicImmersiveHeaderRenderer"])
	if renderer == nil {
		return
	}
	if header.Title == "" {
		header.Title = firstUsefulText(runsText(asMap(renderer["title"])))
	}
	if header.ThumbnailURL == "" {
		header.ThumbnailURL = lastThumbnailURL(renderer)
	}
	if header.Description == "" {
		header.Description = strings.Join(runsText(asMap(renderer["description"])), "")
	}
	applyPlaylistAuthorText(asMap(renderer["subtitle"]), header)
	applyPlaylistStructuredMetadata(renderer, header)
}

func applyPlaylistVisualHeaderRenderer(headerDict map[string]any, header *playlistHeaderData) {
	renderer := asMap(headerDict["musicVisualHeaderRenderer"])
	if renderer == nil {
		return
	}
	if header.Title == "" {
		header.Title = firstUsefulText(runsText(asMap(renderer["title"])))
	}
	if header.ThumbnailURL == "" {
		header.ThumbnailURL = lastThumbnailURL(renderer)
	}
}

func applyPlaylistEditableHeaderRenderer(headerDict map[string]any, header *playlistHeaderData) {
	editableHeader := asMap(headerDict["musicEditablePlaylistDetailHeaderRenderer"])
	nestedHeader := asMap(editableHeader["header"])
	renderer := asMap(nestedHeader["musicDetailHeaderRenderer"])
	if renderer == nil {
		return
	}
	if header.Title == "" {
		header.Title = firstUsefulText(runsText(asMap(renderer["title"])))
	}
	if header.ThumbnailURL == "" {
		header.ThumbnailURL = lastThumbnailURL(renderer)
	}
	applyPlaylistAuthorText(asMap(renderer["subtitle"]), header)
	applyPlaylistStructuredMetadata(renderer, header)
}

func extractPlaylistResponsiveHeaderRenderer(data map[string]any) map[string]any {
	for _, sections := range extractPlaylistHeaderSections(data) {
		for _, section := range sections {
			if renderer := asMap(section["musicResponsiveHeaderRenderer"]); renderer != nil {
				return renderer
			}
		}
	}
	return nil
}

func extractPlaylistHeaderSections(data map[string]any) [][]map[string]any {
	contents := asMap(data["contents"])
	if contents == nil {
		return nil
	}
	sectionGroups := make([][]map[string]any, 0, 3)
	if singleColumn := asMap(contents["singleColumnBrowseResultsRenderer"]); singleColumn != nil {
		if sections := sectionsFromSingleColumnRenderer(singleColumn); len(sections) > 0 {
			sectionGroups = append(sectionGroups, sections)
		}
	}
	if twoColumn := asMap(contents["twoColumnBrowseResultsRenderer"]); twoColumn != nil {
		if secondaryContents := asMap(twoColumn["secondaryContents"]); secondaryContents != nil {
			if sectionList := asMap(secondaryContents["sectionListRenderer"]); sectionList != nil {
				if sections := mapsFromArray(sectionList["contents"]); len(sections) > 0 {
					sectionGroups = append(sectionGroups, sections)
				}
			}
		}
		if sections := sectionsFromSingleColumnRenderer(twoColumn); len(sections) > 0 {
			sectionGroups = append(sectionGroups, sections)
		}
	}
	return sectionGroups
}

func applyPlaylistResponsiveHeaderRenderer(renderer map[string]any, header *playlistHeaderData) {
	if header.Title == "" {
		header.Title = firstUsefulText(runsText(asMap(renderer["title"])))
	}
	if header.ThumbnailURL == "" {
		header.ThumbnailURL = lastThumbnailURL(renderer)
	}
	if header.Description == "" {
		descriptionShelf := asMap(asMap(renderer["description"])["musicDescriptionShelfRenderer"])
		header.Description = strings.Join(runsText(asMap(descriptionShelf["description"])), "")
	}
	applyPlaylistAuthorText(asMap(renderer["straplineTextOne"]), header)
	if header.Author == "" {
		facepile := asMap(renderer["facepile"])
		avatarStack := asMap(facepile["avatarStackViewModel"])
		text := asMap(avatarStack["text"])
		header.Author = firstCreatorText([]string{stringInMap(text, "content")})
	}
	applyPlaylistStructuredMetadata(renderer, header)
}

func applyPlaylistAuthorText(text map[string]any, header *playlistHeaderData) {
	runs := textRunsWithNavigation(text)
	_, _, _, artists := artistRunFromBylineRuns(runs)
	if len(artists) > 0 {
		if header.Author == "" {
			header.Author = artists[0].Name
		}
		if header.AuthorBrowseID == "" {
			header.AuthorBrowseID = artists[0].BrowseID
		}
		return
	}
	if header.Author == "" {
		header.Author = firstCreatorText(textValuesFromRuns(runs))
	}
}

func applyPlaylistStructuredMetadata(renderer map[string]any, header *playlistHeaderData) {
	runs := textRunsWithNavigation(asMap(renderer["secondSubtitle"]))
	if len(runs) >= 3 && isSeparatorText(runs[1].Text) {
		if header.TrackCountLabel == "" {
			header.TrackCountLabel = strings.TrimSpace(runs[0].Text)
		}
		if header.DurationLabel == "" {
			header.DurationLabel = strings.TrimSpace(runs[2].Text)
		}
		return
	}
	if len(runs) == 1 && header.DurationLabel == "" {
		header.DurationLabel = strings.TrimSpace(runs[0].Text)
	}
}

func extractQueueRenderer(content map[string]any) map[string]any {
	if renderer := asMap(content["playlistPanelVideoRenderer"]); renderer != nil {
		return renderer
	}
	if wrapper := asMap(content["playlistPanelVideoWrapperRenderer"]); wrapper != nil {
		primary := asMap(wrapper["primaryRenderer"])
		if renderer := asMap(primary["playlistPanelVideoRenderer"]); renderer != nil {
			return renderer
		}
	}
	return nil
}

func browseIDFromNavigationEndpoint(navigationEndpoint map[string]any) string {
	if navigationEndpoint == nil {
		return ""
	}
	return stringInMap(asMap(navigationEndpoint["browseEndpoint"]), "browseId")
}

func pageTypeFromBrowseEndpoint(browseEndpoint map[string]any) string {
	if browseEndpoint == nil {
		return ""
	}
	configs := asMap(browseEndpoint["browseEndpointContextSupportedConfigs"])
	musicConfig := asMap(configs["browseEndpointContextMusicConfig"])
	return stringInMap(musicConfig, "pageType")
}

func isArtistPageType(pageType string) bool {
	return strings.Contains(strings.ToUpper(strings.TrimSpace(pageType)), "ARTIST")
}

func isArtistBrowseID(browseID string) bool {
	trimmed := strings.TrimSpace(browseID)
	return strings.HasPrefix(trimmed, "UC") || strings.HasPrefix(trimmed, "MPLAUC")
}

func isMoodCategoryBrowseID(browseID string) bool {
	return strings.HasPrefix(strings.TrimSpace(browseID), browseMoodsAndGenresID)
}

func isPlaylistBrowseID(browseID string) bool {
	browseID = strings.TrimSpace(browseID)
	switch {
	case strings.HasPrefix(browseID, "VL"),
		strings.HasPrefix(browseID, "PL"),
		strings.HasPrefix(browseID, "RD"),
		strings.HasPrefix(browseID, "OLAK"),
		strings.HasPrefix(browseID, "MPRE"),
		isPodcastBrowseID(browseID):
		return true
	default:
		return false
	}
}

func isPodcastBrowseID(browseID string) bool {
	return strings.HasPrefix(strings.TrimSpace(browseID), "MPSP")
}

func isAlbumBrowseID(browseID string) bool {
	trimmed := strings.TrimSpace(browseID)
	return strings.HasPrefix(trimmed, "MPRE") || strings.HasPrefix(trimmed, "OLAK")
}

func fallbackShelfTitle(title string, kind ShelfKind) string {
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		return trimmed
	}
	switch kind {
	case ShelfPlaylists:
		return "Playlists"
	case ShelfCategories:
		return "Categories"
	case ShelfArtists:
		return "Artists"
	default:
		return "Recommended"
	}
}

func navigationButtonColorHex(renderer map[string]any) string {
	solid := asMap(renderer["solid"])
	if solid == nil {
		return ""
	}
	color, ok := int64FromAny(solid["leftStripeColor"])
	if !ok {
		return ""
	}
	return fmt.Sprintf("#%06X", color&0x00FFFFFF)
}

func int64FromAny(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case json.Number:
		number, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return number, true
	default:
		return 0, false
	}
}

func buildShelfID(title string, kind ShelfKind, firstItemID string) string {
	return strings.Join([]string{
		strings.TrimSpace(title),
		string(kind),
		strings.TrimSpace(firstItemID),
	}, "::")
}

func headerTitle(header map[string]any) string {
	if header == nil {
		return ""
	}
	// Shelf headers also contain action buttons such as "More" and "Play all".
	// Walking the whole renderer makes the selected text depend on Go's map
	// iteration order, so the action label can accidentally become the shelf
	// title. Prefer the renderer's explicit title field and keep the recursive
	// scan only as a compatibility fallback for older response shapes.
	return firstNonEmpty(
		firstUsefulText(runsText(asMap(header["title"]))),
		firstUsefulText(runsText(asMap(header["text"]))),
		firstUsefulText(collectTextRuns(header)),
	)
}

func browsePageTitle(data map[string]any) string {
	return firstNonEmpty(
		firstArtistTitleText(collectTextRuns(asMap(data["header"]))),
	)
}

type artistHeader struct {
	Title            string
	Subtitle         string
	Description      string
	ThumbnailURL     string
	HeroThumbnailURL string
	ChannelID        string
	IsSubscribed     bool
	MixPlaylistID    string
	MixVideoID       string
}

func artistHeaderFromBrowseData(data map[string]any, browseID string) artistHeader {
	root := asMap(data["header"])
	if root == nil {
		return artistHeader{ChannelID: artistChannelIDFromBrowseID(browseID)}
	}
	result := artistHeader{ChannelID: artistChannelIDFromBrowseID(browseID)}
	for _, key := range []string{
		"musicImmersiveHeaderRenderer",
		"musicVisualHeaderRenderer",
		"musicHeaderRenderer",
	} {
		renderer := asMap(root[key])
		if renderer == nil {
			continue
		}
		if result.Title == "" {
			result.Title = firstArtistTitleText(runsText(asMap(renderer["title"])))
		}
		if result.Subtitle == "" {
			result.Subtitle = artistSubtitleFromHeader(renderer)
		}
		if result.Description == "" {
			result.Description = artistDescriptionFromHeader(renderer)
		}
		if result.ThumbnailURL == "" {
			result.ThumbnailURL = artistPortraitThumbnailURL(renderer)
		}
		if result.HeroThumbnailURL == "" {
			result.HeroThumbnailURL = artistHeroThumbnailURL(renderer)
		}
		if channelID, subscribed := artistSubscriptionFromHeader(renderer); channelID != "" || subscribed {
			if channelID != "" {
				result.ChannelID = channelID
			}
			result.IsSubscribed = subscribed
		}
		if result.MixPlaylistID == "" && result.MixVideoID == "" {
			result.MixPlaylistID, result.MixVideoID = artistMixFromHeader(renderer)
		}
	}
	if result.Subtitle == "" {
		result.Subtitle = firstArtistInfoText(collectTextRuns(root))
	}
	if result.HeroThumbnailURL == "" {
		result.HeroThumbnailURL = result.ThumbnailURL
	}
	if result.ThumbnailURL == "" {
		result.ThumbnailURL = result.HeroThumbnailURL
	}
	return result
}

func artistDescriptionFromHeader(renderer map[string]any) string {
	for _, key := range []string{"description", "descriptionText", "biography"} {
		candidate := asMap(renderer[key])
		if candidate == nil {
			continue
		}
		if description := strings.TrimSpace(strings.Join(runsText(candidate), "")); description != "" {
			return description
		}
		for _, rendererKey := range []string{"musicDescriptionShelfRenderer", "descriptionShelfRenderer"} {
			descriptionRenderer := asMap(candidate[rendererKey])
			if descriptionRenderer == nil {
				continue
			}
			if description := strings.TrimSpace(strings.Join(runsText(asMap(descriptionRenderer["description"])), "")); description != "" {
				return description
			}
		}
	}
	return ""
}

func artistPortraitThumbnailURL(renderer map[string]any) string {
	for _, key := range []string{"foregroundThumbnail", "avatar", "thumbnail"} {
		if thumbnailURL := lastThumbnailURL(renderer[key]); thumbnailURL != "" {
			return thumbnailURL
		}
	}
	return lastThumbnailURL(renderer)
}

func artistHeroThumbnailURL(renderer map[string]any) string {
	for _, key := range []string{"thumbnail", "backgroundThumbnail", "backgroundImage", "heroImage"} {
		if thumbnailURL := lastThumbnailURL(renderer[key]); thumbnailURL != "" {
			return thumbnailURL
		}
	}
	return artistPortraitThumbnailURL(renderer)
}

func artistChannelIDFromBrowseID(browseID string) string {
	trimmed := strings.TrimSpace(browseID)
	if strings.HasPrefix(trimmed, "UC") {
		return trimmed
	}
	return ""
}

func firstArtistTitleText(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || isSeparatorText(trimmed) || isArtistActionText(trimmed) || isArtistInfoText(trimmed) {
			continue
		}
		return trimmed
	}
	return ""
}

func artistSubtitleFromHeader(renderer map[string]any) string {
	for _, key := range []string{
		"monthlyListenerCount",
		"subscriberCountText",
		"shortSubscriberCountText",
	} {
		// These fields have explicit count semantics, so their localized value is
		// useful even when it does not contain one of our English/Chinese hints.
		if subtitle := firstUsefulText(collectTextRuns(renderer[key])); subtitle != "" {
			return subtitle
		}
	}
	for _, key := range []string{"subtitle", "secondSubtitle"} {
		if subtitle := firstArtistInfoText(collectTextRuns(renderer[key])); subtitle != "" {
			return subtitle
		}
	}
	return firstArtistInfoText(collectTextRuns(renderer))
}

func firstArtistInfoText(values []string) string {
	for _, value := range values {
		if isArtistInfoText(value) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isArtistInfoText(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || isSeparatorText(trimmed) || isArtistActionText(trimmed) {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "monthly") ||
		strings.Contains(lower, "listeners") ||
		strings.Contains(lower, "subscribers") ||
		strings.Contains(trimmed, "听众") ||
		strings.Contains(trimmed, "聽眾") ||
		strings.Contains(trimmed, "订阅者") ||
		strings.Contains(trimmed, "訂閱者")
}

func isArtistActionText(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "artist", "youtube music", "subscribe", "subscribed", "unsubscribe", "shuffle", "radio", "mix":
		return true
	default:
		return false
	}
}

func artistSubscriptionFromHeader(renderer map[string]any) (string, bool) {
	subscribeButton := asMap(asMap(renderer["subscriptionButton"])["subscribeButtonRenderer"])
	if subscribeButton != nil {
		channelID := stringInMap(subscribeButton, "channelId")
		subscribed, _ := subscribeButton["subscribed"].(bool)
		return channelID, subscribed
	}
	for _, item := range mapsFromArray(asMap(asMap(renderer["menu"])["menuRenderer"])["items"]) {
		toggleItem := asMap(item["toggleMenuServiceItemRenderer"])
		iconType := stringInMap(asMap(toggleItem["defaultIcon"]), "iconType")
		switch iconType {
		case "SUBSCRIBED", "NOTIFICATION_ON":
			return "", true
		case "SUBSCRIBE", "NOTIFICATION_OFF":
			return "", false
		}
	}
	return "", false
}

func artistMixFromHeader(renderer map[string]any) (string, string) {
	startRadioButton := asMap(renderer["startRadioButton"])
	buttonRenderer := asMap(startRadioButton["buttonRenderer"])
	navigationEndpoint := asMap(buttonRenderer["navigationEndpoint"])
	if navigationEndpoint == nil {
		return "", ""
	}
	if endpoint := asMap(navigationEndpoint["watchPlaylistEndpoint"]); endpoint != nil {
		return stringInMap(endpoint, "playlistId"), ""
	}
	if endpoint := asMap(navigationEndpoint["watchEndpoint"]); endpoint != nil {
		return stringInMap(endpoint, "playlistId"), stringInMap(endpoint, "videoId")
	}
	return "", ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeShelfLimit(limit int) int {
	if limit <= 0 {
		return defaultShelfLimit
	}
	return max(1, min(limit, 20))
}
