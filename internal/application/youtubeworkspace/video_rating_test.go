package youtubeworkspace

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type videoRatingRequesterStub struct {
	innerTubeRequesterStub
	mutationEndpoint string
	mutationBody     map[string]any
	mutationErr      error
}

func (stub *videoRatingRequesterStub) requestMutation(
	_ context.Context,
	endpoint string,
	body map[string]any,
) (map[string]any, error) {
	stub.mutationEndpoint = endpoint
	stub.mutationBody = body
	return map[string]any{}, stub.mutationErr
}

func TestRateVideoUsesAuthenticatedInnerTubeRatingEndpoints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rating   VideoRating
		endpoint string
	}{
		{rating: VideoRatingLike, endpoint: "like/like"},
		{rating: VideoRatingDislike, endpoint: "like/dislike"},
		{rating: VideoRatingNone, endpoint: "like/removelike"},
	}
	for _, test := range tests {
		test := test
		t.Run(string(test.rating), func(t *testing.T) {
			t.Parallel()
			requester := &videoRatingRequesterStub{}
			service := newInnerTubeServiceForTest(&requester.innerTubeRequesterStub)
			service.requester = requester
			err := service.RateVideo(context.Background(), VideoRatingRequest{
				VideoID: "AbCdEfGh123",
				Rating:  test.rating,
			})
			if err != nil {
				t.Fatalf("RateVideo: %v", err)
			}
			if requester.mutationEndpoint != test.endpoint {
				t.Fatalf("endpoint = %q", requester.mutationEndpoint)
			}
			wantBody := map[string]any{
				"target": map[string]any{"videoId": "AbCdEfGh123"},
			}
			if !reflect.DeepEqual(requester.mutationBody, wantBody) {
				t.Fatalf("body = %#v", requester.mutationBody)
			}
		})
	}
}

func TestRateVideoValidatesInputAndPreservesMutationErrors(t *testing.T) {
	t.Parallel()
	requester := &videoRatingRequesterStub{}
	service := newInnerTubeServiceForTest(&requester.innerTubeRequesterStub)
	service.requester = requester
	if err := service.RateVideo(context.Background(), VideoRatingRequest{
		VideoID: "invalid",
		Rating:  VideoRatingLike,
	}); err == nil {
		t.Fatal("invalid id was accepted")
	}
	if err := service.RateVideo(context.Background(), VideoRatingRequest{
		VideoID: "AbCdEfGh123",
		Rating:  VideoRating("favorite"),
	}); err == nil {
		t.Fatal("invalid rating was accepted")
	}

	mutationErr := errors.New("youtube is not authenticated")
	requester.mutationErr = mutationErr
	err := service.RateVideo(context.Background(), VideoRatingRequest{
		VideoID: "AbCdEfGh123",
		Rating:  VideoRatingDislike,
	})
	if !errors.Is(err, mutationErr) {
		t.Fatalf("mutation error = %v", err)
	}
}
