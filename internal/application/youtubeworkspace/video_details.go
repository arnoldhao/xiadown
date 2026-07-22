package youtubeworkspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// VideoDetails loads canonical watch metadata without requiring sign-in. When
// a YouTube App Session is available the shared requester uses it; otherwise
// the same endpoint is queried as a guest.
func (service *Service) VideoDetails(
	ctx context.Context,
	request VideoDetailsRequest,
) (VideoDetails, error) {
	if service == nil || service.requester == nil {
		return VideoDetails{}, errors.New("youtube video details backend unavailable")
	}
	videoID := strings.TrimSpace(request.VideoID)
	if !youtubeVideoIDPattern.MatchString(videoID) {
		return VideoDetails{}, errors.New("invalid youtube video id")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = withInnerTubeLocale(ctx, request.Locale)
	data, err := service.requester.requestRead(
		ctx,
		"player",
		map[string]any{
			"videoId":        videoID,
			"contentCheckOk": true,
			"racyCheckOk":    true,
		},
		innerTubeAuthOptional,
	)
	if err != nil {
		return VideoDetails{}, fmt.Errorf("get youtube video details: %w", err)
	}
	details, err := parseInnerTubeVideoDetails(data, videoID)
	if err != nil {
		return VideoDetails{}, fmt.Errorf("get youtube video details: %w", err)
	}
	// Watch-next carries both the public like count and the signed-in channel
	// subscription state. Treat it as optional enrichment so an action-bar
	// failure never blocks the description/info dialog.
	if nextData, nextErr := service.requester.requestRead(
		ctx,
		"next",
		map[string]any{"videoId": videoID},
		innerTubeAuthOptional,
	); nextErr == nil {
		if owner, ok := findInnerTubeRenderer(nextData, "videoOwnerRenderer"); ok {
			details.ChannelAvatarURL = innerTubeBestThumbnailURL(firstInnerTubeValue(
				owner["thumbnail"],
				owner["avatar"],
			))
		}
		if details.LikeCount == 0 {
			details.LikeCount = innerTubeNamedCount(nextData, map[string]struct{}{
				"likeCount":           {},
				"likeCountText":       {},
				"likeCountIfLiked":    {},
				"likeCountIfDisliked": {},
			})
		}
		if subscribed, ok := innerTubeNamedBool(nextData, "subscribed"); ok {
			details.IsSubscribed = subscribed
		}
	}
	return details, nil
}

func parseInnerTubeVideoDetails(data map[string]any, requestedVideoID string) (VideoDetails, error) {
	videoDetails, _ := innerTubeMap(data["videoDetails"])
	microformat := innerTubeNestedMap(data["microformat"], "playerMicroformatRenderer")
	if len(videoDetails) == 0 && len(microformat) == 0 {
		reason := innerTubeText(innerTubeNestedMap(data["playabilityStatus"])["reason"])
		if reason == "" {
			reason = innerTubeString(innerTubeNestedMap(data["playabilityStatus"])["reason"])
		}
		if reason != "" {
			return VideoDetails{}, errors.New(reason)
		}
		return VideoDetails{}, errors.New("youtube player response has no video details")
	}

	videoID := firstNonEmptyString(
		innerTubeString(videoDetails["videoId"]),
		requestedVideoID,
	)
	if !youtubeVideoIDPattern.MatchString(videoID) {
		return VideoDetails{}, errors.New("youtube player response has an invalid video id")
	}

	publishedDate := firstNonEmptyString(
		innerTubeString(microformat["publishDate"]),
		innerTubeString(microformat["uploadDate"]),
		innerTubeString(videoDetails["publishDate"]),
	)
	publishedLabel := firstInnerTubeText(
		microformat["dateText"],
		videoDetails["publishedTimeText"],
	)
	if publishedLabel == "" {
		publishedLabel = publishedDate
	}

	thumbnailURL := innerTubeBestThumbnailURL(videoDetails["thumbnail"])
	if thumbnailURL == "" {
		thumbnailURL = innerTubeBestThumbnailURL(microformat["thumbnail"])
	}
	if thumbnailURL == "" {
		thumbnailURL = "https://i.ytimg.com/vi/" + url.PathEscape(videoID) + "/hqdefault.jpg"
	}

	viewCount := firstInnerTubeExactCount(videoDetails["viewCount"], microformat["viewCount"])
	likeCount := firstInnerTubeExactCount(videoDetails["likeCount"], microformat["likeCount"])
	if likeCount == 0 {
		likeCount = innerTubeNamedCount(data, map[string]struct{}{
			"likeCount":           {},
			"likeCountText":       {},
			"likeCountIfLiked":    {},
			"likeCountIfDisliked": {},
		})
	}

	title := firstInnerTubeText(videoDetails["title"], microformat["title"])
	if title == "" {
		title = videoID
	}
	return VideoDetails{
		VideoID:         videoID,
		Title:           title,
		Channel:         firstInnerTubeText(videoDetails["author"], microformat["ownerChannelName"]),
		ChannelID:       firstNonEmptyString(innerTubeString(videoDetails["channelId"]), innerTubeString(microformat["externalChannelId"])),
		ThumbnailURL:    thumbnailURL,
		DurationSeconds: float64(firstInnerTubeExactCount(videoDetails["lengthSeconds"], microformat["lengthSeconds"])),
		ViewCount:       viewCount,
		LikeCount:       likeCount,
		PublishedDate:   publishedDate,
		PublishedLabel:  publishedLabel,
		Description:     firstInnerTubeText(videoDetails["shortDescription"], microformat["description"]),
		WebURL:          "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID),
	}, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstInnerTubeExactCount(values ...any) int64 {
	for _, value := range values {
		if count, ok := innerTubeExactCount(value); ok {
			return count
		}
	}
	return 0
}

func innerTubeExactCount(value any) (int64, bool) {
	if value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case json.Number:
		result, err := strconv.ParseInt(string(typed), 10, 64)
		return max(0, result), err == nil
	case int:
		return max(0, int64(typed)), true
	case int32:
		return max(0, int64(typed)), true
	case int64:
		return max(0, typed), true
	case uint:
		if uint64(typed) > math.MaxInt64 {
			return math.MaxInt64, true
		}
		return int64(typed), true
	case uint64:
		if typed > math.MaxInt64 {
			return math.MaxInt64, true
		}
		return int64(typed), true
	case float32:
		return boundedInnerTubeCount(float64(typed))
	case float64:
		return boundedInnerTubeCount(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		if result, err := strconv.ParseInt(strings.ReplaceAll(trimmed, ",", ""), 10, 64); err == nil {
			return max(0, result), true
		}
		if result := innerTubeCount(trimmed); result > 0 {
			return result, true
		}
		return 0, false
	default:
		if text := innerTubeText(value); text != "" {
			return innerTubeExactCount(text)
		}
		return 0, false
	}
}

func boundedInnerTubeCount(value float64) (int64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	if value <= 0 {
		return 0, true
	}
	if value >= math.MaxInt64 {
		return math.MaxInt64, true
	}
	return int64(math.Round(value)), true
}

// innerTubeNamedCount is a narrow recursive fallback for player generations
// that move the public like count into a nested entity/view-model update.
func innerTubeNamedCount(value any, names map[string]struct{}) int64 {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if _, wanted := names[key]; wanted {
				if result, ok := innerTubeExactCount(typed[key]); ok {
					return result
				}
			}
		}
		for _, key := range keys {
			if result := innerTubeNamedCount(typed[key], names); result > 0 {
				return result
			}
		}
	case []any:
		for _, item := range typed {
			if result := innerTubeNamedCount(item, names); result > 0 {
				return result
			}
		}
	case []map[string]any:
		for _, item := range typed {
			if result := innerTubeNamedCount(item, names); result > 0 {
				return result
			}
		}
	}
	return 0
}

// innerTubeNamedBool finds a concrete boolean state in a renderer or entity
// tree. Subscription payloads have used both subscribeButtonRenderer and
// subscriptionStateEntity across WEB client generations, but both expose the
// canonical `subscribed` field.
func innerTubeNamedBool(value any, name string) (bool, bool) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if key != name {
				continue
			}
			if result, ok := typed[key].(bool); ok {
				return result, true
			}
		}
		for _, key := range keys {
			if result, ok := innerTubeNamedBool(typed[key], name); ok {
				return result, true
			}
		}
	case []any:
		for _, item := range typed {
			if result, ok := innerTubeNamedBool(item, name); ok {
				return result, true
			}
		}
	case []map[string]any:
		for _, item := range typed {
			if result, ok := innerTubeNamedBool(item, name); ok {
				return result, true
			}
		}
	}
	return false, false
}
