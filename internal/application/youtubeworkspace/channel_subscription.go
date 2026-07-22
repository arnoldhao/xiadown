package youtubeworkspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// SetChannelSubscription changes the signed-in YouTube account's channel
// subscription through the regular YouTube App Session.
func (service *Service) SetChannelSubscription(
	ctx context.Context,
	request ChannelSubscriptionRequest,
) error {
	if service == nil || service.requester == nil {
		return errors.New("youtube channel subscription backend unavailable")
	}
	requester, ok := service.requester.(innerTubeMutationRequester)
	if !ok {
		return errors.New("youtube channel subscription backend unavailable")
	}
	channelID := strings.TrimSpace(request.ChannelID)
	if !strings.HasPrefix(channelID, "UC") || len(channelID) < 3 || len(channelID) > 128 {
		return errors.New("invalid youtube channel id")
	}
	endpoint := "subscription/unsubscribe"
	if request.Subscribed {
		endpoint = "subscription/subscribe"
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := requester.requestMutation(ctx, endpoint, map[string]any{
		"channelIds": []string{channelID},
	}); err != nil {
		return fmt.Errorf("set youtube channel subscription: %w", err)
	}
	return nil
}
