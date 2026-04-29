package youtubemusic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	browseHomeID            = "FEmusic_home"
	browseExploreID         = "FEmusic_explore"
	browseChartsID          = "FEmusic_charts"
	browseMoodsAndGenresID  = "FEmusic_moods_and_genres"
	browseNewReleasesID     = "FEmusic_new_releases"
	browsePodcastsID        = "FEmusic_podcasts"
	browseHistoryID         = "FEmusic_history"
	browseLibraryLandingID  = "FEmusic_library_landing"
	browseLibraryArtistsID  = "FEmusic_library_corpus_artists"
	browseLibraryPodcastsID = "FEmusic_library_non_music_audio_list"
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
	ShelfPodcasts   ShelfKind = "podcasts"
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

type PodcastShow struct {
	ID           string
	Title        string
	Author       string
	Description  string
	ThumbnailURL string
}

type Shelf struct {
	ID         string
	Title      string
	Kind       ShelfKind
	Tracks     []Track
	Playlists  []Playlist
	Categories []Category
	Podcasts   []PodcastShow
	Artists    []Artist
}

type BrowseTab struct {
	Title    string
	BrowseID string
	Params   string
	Selected bool
}

type BrowsePage struct {
	Shelves      []Shelf
	Continuation string
	Tabs         []BrowseTab
	Title        string
	Author       string
}

type TrackListPage struct {
	Tracks       []Track
	Continuation string
	Title        string
	Author       string
}

type ArtistPage struct {
	ID            string
	Title         string
	Subtitle      string
	ChannelID     string
	IsSubscribed  bool
	MixPlaylistID string
	MixVideoID    string
	Tracks        []Track
	Shelves       []Shelf
	Continuation  string
}

func (client *Client) HomeRecommendations(ctx context.Context, limit int) ([]Track, error) {
	data, err := client.request(ctx, "browse", map[string]any{
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
		data, err := client.request(ctx, "browse", map[string]any{
			"continuation": trimmedContinuation,
		})
		if err != nil {
			return BrowsePage{}, err
		}
		return BrowsePage{
			Shelves:      parseHomeShelves(data, normalizeShelfLimit(sectionLimit), normalizeLimit(itemLimit)),
			Continuation: extractBrowseContinuationToken(data),
		}, nil
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
	data, err := client.request(ctx, "browse", body)
	if err != nil {
		return BrowsePage{}, err
	}
	header := playlistHeaderFromBrowseData(data)
	return BrowsePage{
		Shelves:      parseHomeShelves(data, normalizeShelfLimit(sectionLimit), normalizeLimit(itemLimit)),
		Continuation: extractBrowseContinuationToken(data),
		Tabs:         extractBrowseTabs(data),
		Title:        header.Title,
		Author:       header.Author,
	}, nil
}

func (client *Client) LibraryPlaylists(ctx context.Context, limit int) ([]Playlist, error) {
	data, err := client.request(ctx, "browse", map[string]any{
		"browseId": browseLikedPlaylistsID,
	})
	if err != nil {
		return nil, err
	}
	return parseLibraryBrowsePlaylists(data, normalizeLimit(limit)), nil
}

func (client *Client) LibraryArtists(ctx context.Context, limit int) ([]Artist, error) {
	data, err := client.request(ctx, "browse", map[string]any{
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
	landing, landingErr := client.request(ctx, "browse", map[string]any{
		"browseId": browseLibraryLandingID,
	})
	if landingErr != nil {
		return nil, err
	}
	return parseLibraryBrowseArtists(landing, normalizeLimit(limit)), nil
}

func (client *Client) LibraryPodcasts(ctx context.Context, limit int) ([]PodcastShow, error) {
	data, err := client.request(ctx, "browse", map[string]any{
		"browseId": browseLibraryPodcastsID,
	})
	if err == nil {
		podcasts := parseLibraryBrowsePodcasts(data, normalizeLimit(limit))
		if len(podcasts) > 0 {
			return podcasts, nil
		}
	}
	landing, landingErr := client.request(ctx, "browse", map[string]any{
		"browseId": browseLibraryLandingID,
	})
	if landingErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, landingErr
	}
	return parseLibraryBrowsePodcasts(landing, normalizeLimit(limit)), nil
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
	if isPodcastBrowseID(trimmedPlaylistID) || isMoodCategoryBrowseID(trimmedPlaylistID) {
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
	if isPodcastBrowseID(browseID) || isMoodCategoryBrowseID(browseID) || isAlbumBrowseID(browseID) || isPlaylistBrowseID(browseID) {
		page, err := client.browseTracksPage(ctx, browseID, "", "", itemLimit)
		if err == nil && len(page.Tracks) > 0 {
			return page, nil
		}
		if isPodcastBrowseID(browseID) || isMoodCategoryBrowseID(browseID) || isAlbumBrowseID(browseID) {
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
		strings.HasPrefix(trimmed, "UC") ||
		isPodcastBrowseID(trimmed) ||
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
	data, err := client.request(ctx, "browse", map[string]any{
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
		Tracks:       tracksFromShelves(page.Shelves, itemLimit),
		Continuation: page.Continuation,
		Title:        page.Title,
		Author:       page.Author,
	}, nil
}

func (client *Client) ArtistPage(ctx context.Context, browseID string, limit int) (ArtistPage, error) {
	artistBrowseID, err := cleanBrowseID(browseID)
	if err != nil {
		return ArtistPage{}, err
	}
	itemLimit := normalizeLimit(limit)
	data, err := client.request(ctx, "browse", map[string]any{
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
		ID:            artistBrowseID,
		Title:         firstNonEmpty(header.Title, browsePageTitle(data)),
		Subtitle:      header.Subtitle,
		ChannelID:     header.ChannelID,
		IsSubscribed:  header.IsSubscribed,
		MixPlaylistID: header.MixPlaylistID,
		MixVideoID:    header.MixVideoID,
		Tracks:        tracks,
		Shelves:       shelves,
		Continuation:  extractBrowseContinuationToken(data),
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

func parseLibraryBrowsePodcasts(data map[string]any, limit int) []PodcastShow {
	items := browseSectionItems(data)
	podcasts := make([]PodcastShow, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		podcast, ok := podcastShowFromLibraryItem(item)
		if !ok {
			continue
		}
		if _, exists := seen[podcast.ID]; exists {
			continue
		}
		seen[podcast.ID] = struct{}{}
		podcasts = append(podcasts, podcast)
		if len(podcasts) >= limit {
			break
		}
	}
	return podcasts
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
	items := sectionItems(section)
	if len(items) == 0 {
		return nil
	}

	tracks := make([]Track, 0, min(len(items), itemLimit))
	playlists := make([]Playlist, 0, min(len(items), itemLimit))
	categories := make([]Category, 0, min(len(items), itemLimit))
	podcasts := make([]PodcastShow, 0, min(len(items), itemLimit))
	artists := make([]Artist, 0, min(len(items), itemLimit))
	seenTracks := make(map[string]struct{}, len(items))
	seenPlaylists := make(map[string]struct{}, len(items))
	seenCategories := make(map[string]struct{}, len(items))
	seenPodcasts := make(map[string]struct{}, len(items))
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
		if len(podcasts) < itemLimit {
			if podcast, ok := podcastShowFromLibraryItem(item); ok {
				if _, exists := seenPodcasts[podcast.ID]; !exists {
					seenPodcasts[podcast.ID] = struct{}{}
					podcasts = append(podcasts, podcast)
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
		if len(tracks) >= itemLimit && len(playlists) >= itemLimit && len(categories) >= itemLimit && len(podcasts) >= itemLimit && len(artists) >= itemLimit {
			break
		}
	}

	result := make([]Shelf, 0, 5)
	if len(categories) > 0 {
		result = append(result, Shelf{
			ID:         buildShelfID(title, ShelfCategories, categories[0].ID),
			Title:      fallbackShelfTitle(title, ShelfCategories),
			Kind:       ShelfCategories,
			Categories: categories,
		})
	}
	if len(podcasts) > 0 {
		result = append(result, Shelf{
			ID:       buildShelfID(title, ShelfPodcasts, podcasts[0].ID),
			Title:    fallbackShelfTitle(title, ShelfPodcasts),
			Kind:     ShelfPodcasts,
			Podcasts: podcasts,
		})
	}
	if len(artists) > 0 {
		result = append(result, Shelf{
			ID:      buildShelfID(title, ShelfArtists, artists[0].ID),
			Title:   fallbackShelfTitle(title, ShelfArtists),
			Kind:    ShelfArtists,
			Artists: artists,
		})
	}
	if len(tracks) > 0 {
		result = append(result, Shelf{
			ID:     buildShelfID(title, ShelfTracks, tracks[0].VideoID),
			Title:  fallbackShelfTitle(title, ShelfTracks),
			Kind:   ShelfTracks,
			Tracks: tracks,
		})
	}
	if len(playlists) > 0 {
		result = append(result, Shelf{
			ID:        buildShelfID(title, ShelfPlaylists, playlists[0].ID),
			Title:     fallbackShelfTitle(title, ShelfPlaylists),
			Kind:      ShelfPlaylists,
			Playlists: playlists,
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
	if renderer := asMap(item["musicTwoRowItemRenderer"]); renderer != nil {
		return trackFromHomeTwoRowRenderer(renderer)
	}
	if renderer := asMap(item["musicMultiRowListItemRenderer"]); renderer != nil {
		return trackFromPodcastMultiRowRenderer(renderer)
	}
	return Track{}, false
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
	channel := firstCreatorText(subtitleRuns)
	channelFromRun, artistBrowseID := firstCreatorRun(subtitleNavigationRuns)
	if channelFromRun != "" {
		channel = channelFromRun
	}
	duration := firstDurationLabel(subtitleRuns)
	return Track{
		ID:             videoID,
		VideoID:        videoID,
		Title:          title,
		Channel:        fallbackString(channel, "YouTube Music"),
		ArtistBrowseID: artistBrowseID,
		DurationLabel:  duration,
		ThumbnailURL:   lastThumbnailURL(renderer),
		MusicVideoType: musicVideoTypeFromWatchEndpoint(watchEndpoint),
	}, true
}

func trackFromPodcastMultiRowRenderer(renderer map[string]any) (Track, bool) {
	watchEndpoint := asMap(asMap(renderer["onTap"])["watchEndpoint"])
	videoID := stringInMap(watchEndpoint, "videoId")
	if videoID == "" {
		videoID = findFirstStringByKey(renderer, "videoId")
	}
	if !videoIDPattern.MatchString(videoID) {
		return Track{}, false
	}
	title := firstUsefulText(runsText(asMap(renderer["title"])))
	if title == "" {
		title = videoID
	}
	subtitleNavigationRuns := textRunsWithNavigation(asMap(renderer["subtitle"]))
	subtitleRuns := textValuesFromRuns(subtitleNavigationRuns)
	channel := firstCreatorText(subtitleRuns)
	channelFromRun, artistBrowseID := firstCreatorRun(subtitleNavigationRuns)
	if channelFromRun != "" {
		channel = channelFromRun
	}
	duration := firstUsefulText(runsText(asMap(renderer["durationText"])))
	if duration == "" {
		duration = firstDurationLabel(collectTextRuns(renderer))
	}
	description := firstUsefulText(runsText(asMap(renderer["description"])))
	return Track{
		ID:             videoID,
		VideoID:        videoID,
		Title:          title,
		Channel:        fallbackString(channel, "YouTube Music"),
		ArtistBrowseID: artistBrowseID,
		DurationLabel:  duration,
		ThumbnailURL:   lastThumbnailURL(renderer),
		RawDescription: description,
		MusicVideoType: musicVideoTypeFromWatchEndpoint(watchEndpoint),
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

func podcastShowFromLibraryItem(item map[string]any) (PodcastShow, bool) {
	if renderer := asMap(item["musicTwoRowItemRenderer"]); renderer != nil {
		return podcastShowFromTwoRowRenderer(renderer)
	}
	if renderer := asMap(item["musicResponsiveListItemRenderer"]); renderer != nil {
		return podcastShowFromResponsiveRenderer(renderer)
	}
	return PodcastShow{}, false
}

func podcastShowFromTwoRowRenderer(renderer map[string]any) (PodcastShow, bool) {
	browseID := browseIDFromNavigationEndpoint(asMap(renderer["navigationEndpoint"]))
	if !isPodcastBrowseID(browseID) {
		return PodcastShow{}, false
	}
	title := firstUsefulText(runsText(asMap(renderer["title"])))
	if title == "" {
		title = browseID
	}
	return PodcastShow{
		ID:           browseID,
		Title:        title,
		Author:       firstUsefulText(runsText(asMap(renderer["subtitle"]))),
		ThumbnailURL: lastThumbnailURL(renderer),
	}, true
}

func podcastShowFromResponsiveRenderer(renderer map[string]any) (PodcastShow, bool) {
	browseID := browseIDFromNavigationEndpoint(asMap(renderer["navigationEndpoint"]))
	if !isPodcastBrowseID(browseID) {
		return PodcastShow{}, false
	}
	flexColumns := mapsFromArray(renderer["flexColumns"])
	title := ""
	if len(flexColumns) > 0 {
		title = firstUsefulText(textRunsFromFlexColumn(flexColumns[0]))
	}
	if title == "" {
		title = firstUsefulText(collectTextRuns(renderer))
	}
	if title == "" {
		title = browseID
	}
	subtitleRuns := make([]string, 0, 4)
	for _, column := range flexColumns[1:] {
		subtitleRuns = append(subtitleRuns, textRunsFromFlexColumn(column)...)
	}
	return PodcastShow{
		ID:           browseID,
		Title:        title,
		Author:       firstUsefulText(subtitleRuns),
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
	channel := firstUsefulText(runsText(asMap(renderer["subtitle"])))
	return Playlist{
		ID:           browseID,
		Title:        title,
		Channel:      channel,
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
	channelRuns := make([]string, 0, 4)
	for _, column := range flexColumns[1:] {
		channelRuns = append(channelRuns, textRunsFromFlexColumn(column)...)
	}
	channel := firstUsefulText(channelRuns)
	return Playlist{
		ID:           browseID,
		Title:        title,
		Channel:      channel,
		ThumbnailURL: lastThumbnailURL(renderer),
	}, true
}

type playlistHeaderData struct {
	Title        string
	Description  string
	ThumbnailURL string
	Author       string
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
	if runs := runsText(asMap(renderer["subtitle"])); len(runs) > 0 {
		header.Author = runs[0]
	}
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
	if header.Author == "" {
		if runs := runsText(asMap(renderer["subtitle"])); len(runs) > 0 {
			header.Author = runs[0]
		}
	}
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
	if header.Author == "" {
		if runs := runsText(asMap(renderer["subtitle"])); len(runs) > 0 {
			header.Author = runs[0]
		}
	}
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
	if header.Author == "" {
		facepile := asMap(renderer["facepile"])
		avatarStack := asMap(facepile["avatarStackViewModel"])
		text := asMap(avatarStack["text"])
		header.Author = stringInMap(text, "content")
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

func isPodcastBrowseID(browseID string) bool {
	return strings.HasPrefix(strings.TrimSpace(browseID), "MPSPP")
}

func isPlaylistBrowseID(browseID string) bool {
	switch {
	case strings.HasPrefix(browseID, "VL"),
		strings.HasPrefix(browseID, "PL"),
		strings.HasPrefix(browseID, "RD"),
		strings.HasPrefix(browseID, "OLAK"),
		strings.HasPrefix(browseID, "MPRE"):
		return true
	default:
		return false
	}
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
	case ShelfPodcasts:
		return "Podcasts"
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
	return firstUsefulText(collectTextRuns(header))
}

func browsePageTitle(data map[string]any) string {
	return firstNonEmpty(
		firstArtistTitleText(collectTextRuns(asMap(data["header"]))),
	)
}

type artistHeader struct {
	Title         string
	Subtitle      string
	ChannelID     string
	IsSubscribed  bool
	MixPlaylistID string
	MixVideoID    string
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
	return result
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
		"subtitle",
		"secondSubtitle",
		"subscriberCountText",
		"shortSubscriberCountText",
	} {
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
