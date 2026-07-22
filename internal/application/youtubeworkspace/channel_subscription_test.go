package youtubeworkspace

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSetChannelSubscriptionUsesAuthenticatedMutation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		subscribed bool
		endpoint   string
	}{
		{name: "subscribe", subscribed: true, endpoint: "subscription/subscribe"},
		{name: "unsubscribe", subscribed: false, endpoint: "subscription/unsubscribe"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requester := &videoRatingRequesterStub{}
			service := newInnerTubeServiceForTest(&requester.innerTubeRequesterStub)
			service.requester = requester
			err := service.SetChannelSubscription(
				context.Background(),
				ChannelSubscriptionRequest{
					ChannelID:  " UCabcdefghijklmnopqrstuv ",
					Subscribed: test.subscribed,
				},
			)
			if err != nil {
				t.Fatalf("SetChannelSubscription: %v", err)
			}
			if requester.mutationEndpoint != test.endpoint {
				t.Fatalf("subscription endpoint = %q", requester.mutationEndpoint)
			}
			if !reflect.DeepEqual(requester.mutationBody, map[string]any{
				"channelIds": []string{"UCabcdefghijklmnopqrstuv"},
			}) {
				t.Fatalf("subscription body = %#v", requester.mutationBody)
			}
		})
	}
}

func TestSetChannelSubscriptionValidatesAndPreservesErrors(t *testing.T) {
	t.Parallel()
	invalidRequester := &videoRatingRequesterStub{}
	invalidService := newInnerTubeServiceForTest(&invalidRequester.innerTubeRequesterStub)
	invalidService.requester = invalidRequester
	err := invalidService.SetChannelSubscription(
		context.Background(),
		ChannelSubscriptionRequest{ChannelID: "not-a-channel", Subscribed: true},
	)
	if err == nil || !strings.Contains(err.Error(), "invalid youtube channel id") {
		t.Fatalf("invalid channel error = %v", err)
	}
	if invalidRequester.mutationEndpoint != "" {
		t.Fatalf("invalid channel reached requester: %q", invalidRequester.mutationEndpoint)
	}

	backendErr := errors.New("authentication expired")
	requester := &videoRatingRequesterStub{mutationErr: backendErr}
	service := newInnerTubeServiceForTest(&requester.innerTubeRequesterStub)
	service.requester = requester
	err = service.SetChannelSubscription(
		context.Background(),
		ChannelSubscriptionRequest{
			ChannelID:  "UCabcdefghijklmnopqrstuv",
			Subscribed: true,
		},
	)
	if !errors.Is(err, backendErr) {
		t.Fatalf("backend error = %v", err)
	}
}
