package youtubeworkspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type innerTubeMutationRequester interface {
	requestMutation(context.Context, string, map[string]any) (map[string]any, error)
}

// RateVideo applies the signed-in YouTube account's rating through the same
// App Session-backed InnerTube client used by browse and video details.
func (service *Service) RateVideo(
	ctx context.Context,
	request VideoRatingRequest,
) error {
	if service == nil || service.requester == nil {
		return errors.New("youtube video rating backend unavailable")
	}
	requester, ok := service.requester.(innerTubeMutationRequester)
	if !ok {
		return errors.New("youtube video rating backend unavailable")
	}
	videoID := strings.TrimSpace(request.VideoID)
	if !youtubeVideoIDPattern.MatchString(videoID) {
		return errors.New("invalid youtube video id")
	}
	var endpoint string
	switch request.Rating {
	case VideoRatingLike:
		endpoint = "like/like"
	case VideoRatingDislike:
		endpoint = "like/dislike"
	case VideoRatingNone:
		endpoint = "like/removelike"
	default:
		return fmt.Errorf("invalid youtube video rating %q", request.Rating)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := requester.requestMutation(
		ctx,
		endpoint,
		map[string]any{"target": map[string]any{"videoId": videoID}},
	); err != nil {
		return fmt.Errorf("rate youtube video: %w", err)
	}
	return nil
}
