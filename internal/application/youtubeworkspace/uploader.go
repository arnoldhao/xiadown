package youtubeworkspace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Uploader loads a regular YouTube channel through the same WEB InnerTube
// client and App Session used by the rest of the YouTube workspace.
func (service *Service) Uploader(
	ctx context.Context,
	request UploaderRequest,
) (UploaderPage, error) {
	if service == nil || service.requester == nil {
		return UploaderPage{}, errors.New("youtube uploader backend unavailable")
	}
	channelID := strings.TrimSpace(request.ChannelID)
	if !validYouTubeChannelID(channelID) {
		return UploaderPage{}, errors.New("invalid youtube channel id")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = withInnerTubeLocale(ctx, request.Locale)
	continuation := strings.TrimSpace(request.Continuation)
	if continuation != "" {
		data, err := service.requester.requestRead(
			ctx,
			"browse",
			map[string]any{"continuation": continuation},
			innerTubeAuthOptional,
		)
		if err != nil {
			return UploaderPage{}, fmt.Errorf("continue youtube uploader videos: %w", err)
		}
		parsed := parseInnerTubeItems(data, innerTubeItemsVideosOnly, 0)
		videos := completeUploaderVideos(parsed.Items, channelID, "")
		return UploaderPage{
			ChannelID:    channelID,
			Name:         channelID,
			WebURL:       youtubeChannelWebURL(channelID),
			Videos:       videos,
			Continuation: parsed.Continuation,
		}, nil
	}

	data, err := service.requester.requestRead(
		ctx,
		"browse",
		map[string]any{"browseId": channelID},
		innerTubeAuthOptional,
	)
	if err != nil {
		return UploaderPage{}, fmt.Errorf("get youtube uploader: %w", err)
	}
	page := parseInnerTubeUploader(data, channelID)
	parsed := parseInnerTubeItems(data, innerTubeItemsVideosOnly, 0)

	// Channel landing pages usually select Home. Follow the response's Videos
	// tab endpoint instead of hard-coding a locale-dependent title or a private
	// params token. This also tracks YouTube when the tab params generation
	// changes.
	if endpoint, ok := findInnerTubeUploaderVideosEndpoint(data); ok && endpoint.Params != "" {
		browseID := endpoint.BrowseID
		if !validYouTubeChannelID(browseID) {
			browseID = channelID
		}
		videoData, videoErr := service.requester.requestRead(
			ctx,
			"browse",
			map[string]any{
				"browseId": browseID,
				"params":   endpoint.Params,
			},
			innerTubeAuthOptional,
		)
		if videoErr == nil {
			parsed = parseInnerTubeItems(videoData, innerTubeItemsVideosOnly, 0)
		} else if len(parsed.Items) == 0 {
			return UploaderPage{}, fmt.Errorf("get youtube uploader videos: %w", videoErr)
		}
	}

	page.Videos = completeUploaderVideos(parsed.Items, channelID, page.Name)
	page.Continuation = parsed.Continuation
	return page, nil
}

func validYouTubeChannelID(channelID string) bool {
	channelID = strings.TrimSpace(channelID)
	return strings.HasPrefix(channelID, "UC") && len(channelID) >= 3 && len(channelID) <= 128
}

func youtubeChannelWebURL(channelID string) string {
	return "https://www.youtube.com/channel/" + url.PathEscape(strings.TrimSpace(channelID))
}

func completeUploaderVideos(videos []Video, channelID string, channelName string) []Video {
	result := make([]Video, len(videos))
	copy(result, videos)
	for index := range result {
		if strings.TrimSpace(result[index].Channel) == "" {
			result[index].Channel = channelName
		}
		if strings.TrimSpace(result[index].ChannelID) == "" {
			result[index].ChannelID = channelID
		}
	}
	if result == nil {
		return []Video{}
	}
	return result
}

func parseInnerTubeUploader(data map[string]any, requestedChannelID string) UploaderPage {
	metadata, _ := findInnerTubeRenderer(data, "channelMetadataRenderer")
	pageHeader, _ := findInnerTubeRenderer(data, "pageHeaderRenderer")
	legacyHeader, _ := findInnerTubeRenderer(data, "c4TabbedHeaderRenderer")
	viewModel := innerTubeNestedMap(pageHeader["content"], "pageHeaderViewModel")

	name := firstInnerTubeText(
		viewModel["title"],
		pageHeader["pageTitle"],
		metadata["title"],
		legacyHeader["title"],
	)
	if name == "" {
		name = requestedChannelID
	}

	rows := innerTubeMetadataRows(viewModel)
	handle := ""
	subscriberLabel := ""
	videoCountLabel := ""
	if len(rows) > 0 && len(rows[0]) > 0 {
		handle = strings.TrimSpace(rows[0][0])
	}
	if len(rows) > 1 {
		if len(rows[1]) > 0 {
			subscriberLabel = strings.TrimSpace(rows[1][0])
		}
		if len(rows[1]) > 1 {
			videoCountLabel = strings.TrimSpace(rows[1][1])
		}
	}
	if subscriberLabel == "" {
		subscriberLabel = firstInnerTubeText(
			legacyHeader["subscriberCountText"],
			metadata["subscriberCountText"],
		)
	}
	if videoCountLabel == "" {
		videoCountLabel = firstInnerTubeText(
			legacyHeader["videosCountText"],
			legacyHeader["videoCountText"],
			metadata["videosCountText"],
		)
	}

	avatarURL := innerTubeBestThumbnailURL(innerTubeNestedMap(
		viewModel["image"],
		"decoratedAvatarViewModel",
		"avatar",
		"avatarViewModel",
		"image",
	))
	if avatarURL == "" {
		avatarURL = innerTubeBestThumbnailURL(firstInnerTubeValue(
			legacyHeader["avatar"],
			metadata["avatar"],
		))
	}
	bannerURL := innerTubeBestThumbnailURL(innerTubeNestedMap(
		viewModel["banner"],
		"imageBannerViewModel",
		"image",
	))
	if bannerURL == "" {
		bannerURL = innerTubeBestThumbnailURL(firstInnerTubeValue(
			legacyHeader["banner"],
			legacyHeader["tvBanner"],
			legacyHeader["mobileBanner"],
		))
	}

	description := firstInnerTubeText(
		metadata["description"],
		innerTubeNestedMap(viewModel["description"], "descriptionPreviewViewModel")["description"],
	)
	webURL := firstNonEmptyString(
		innerTubeNormalizeURL(innerTubeString(metadata["vanityChannelUrl"])),
		innerTubeNormalizeURL(innerTubeString(metadata["channelUrl"])),
	)
	if webURL == "" {
		webURL = youtubeChannelWebURL(requestedChannelID)
	}
	if handle == "" {
		handle = uploaderHandleFromWebURL(webURL)
	}
	subscribed, _ := innerTubeNamedBool(data, "subscribed")
	return UploaderPage{
		ChannelID:       requestedChannelID,
		Name:            name,
		Handle:          handle,
		Description:     description,
		AvatarURL:       avatarURL,
		BannerURL:       bannerURL,
		SubscriberCount: innerTubeCount(subscriberLabel),
		SubscriberLabel: subscriberLabel,
		VideoCountLabel: videoCountLabel,
		IsSubscribed:    subscribed,
		WebURL:          webURL,
		Videos:          []Video{},
	}
}

func uploaderHandleFromWebURL(webURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(webURL))
	if err != nil {
		return ""
	}
	for _, part := range strings.Split(strings.Trim(parsed.Path, "/"), "/") {
		if strings.HasPrefix(part, "@") {
			return part
		}
	}
	return ""
}

type innerTubeUploaderVideosEndpoint struct {
	BrowseID string
	Params   string
}

func findInnerTubeUploaderVideosEndpoint(value any) (innerTubeUploaderVideosEndpoint, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if renderer, ok := innerTubeMap(typed["tabRenderer"]); ok {
			endpoint := innerTubeNestedMap(renderer["endpoint"], "browseEndpoint")
			metadata := innerTubeNestedMap(renderer["endpoint"], "commandMetadata", "webCommandMetadata")
			rawURL := strings.TrimSpace(innerTubeString(metadata["url"]))
			identifier := strings.ToLower(strings.TrimSpace(innerTubeString(renderer["tabIdentifier"])))
			isVideos := identifier == "videos" || strings.HasSuffix(strings.TrimSuffix(rawURL, "/"), "/videos")
			if isVideos && len(endpoint) > 0 {
				params := strings.TrimSpace(innerTubeString(endpoint["params"]))
				if decoded, err := url.QueryUnescape(params); err == nil {
					params = decoded
				}
				return innerTubeUploaderVideosEndpoint{
					BrowseID: strings.TrimSpace(innerTubeString(endpoint["browseId"])),
					Params:   params,
				}, true
			}
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if result, ok := findInnerTubeUploaderVideosEndpoint(typed[key]); ok {
				return result, true
			}
		}
	case []any:
		for _, item := range typed {
			if result, ok := findInnerTubeUploaderVideosEndpoint(item); ok {
				return result, true
			}
		}
	case []map[string]any:
		for _, item := range typed {
			if result, ok := findInnerTubeUploaderVideosEndpoint(item); ok {
				return result, true
			}
		}
	}
	return innerTubeUploaderVideosEndpoint{}, false
}

func findInnerTubeRenderer(value any, rendererName string) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if renderer, ok := innerTubeMap(typed[rendererName]); ok {
			return renderer, true
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if renderer, ok := findInnerTubeRenderer(typed[key], rendererName); ok {
				return renderer, true
			}
		}
	case []any:
		for _, item := range typed {
			if renderer, ok := findInnerTubeRenderer(item, rendererName); ok {
				return renderer, true
			}
		}
	case []map[string]any:
		for _, item := range typed {
			if renderer, ok := findInnerTubeRenderer(item, rendererName); ok {
				return renderer, true
			}
		}
	}
	return nil, false
}
