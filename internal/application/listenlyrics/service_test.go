package listenlyrics

import (
	"context"
	"testing"
)

type fakeClient struct {
	requests []Request
	results  []Snapshot
	err      error
}

func (client *fakeClient) TrackLyrics(_ context.Context, request Request) (Snapshot, error) {
	client.requests = append(client.requests, request)
	if len(client.results) == 0 {
		return Snapshot{VideoID: request.VideoID, Kind: KindUnavailable}, client.err
	}
	result := client.results[0]
	client.results = client.results[1:]
	return result, client.err
}

func TestTrackLyricsCachesSyncedResult(t *testing.T) {
	client := &fakeClient{
		results: []Snapshot{{
			VideoID: "video-one",
			Kind:    KindSynced,
			Source:  "YTMusic",
			Lines:   []Line{{StartMs: 100, Text: "hello"}},
		}},
	}
	service := NewService(client)

	first, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one", Title: "One"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one", Title: "One"})
	if err != nil {
		t.Fatal(err)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected synced lyrics to be served from cache, got %d requests", len(client.requests))
	}
	if first.Kind != KindSynced || second.Kind != KindSynced || len(second.Lines) != 1 {
		t.Fatalf("unexpected synced lyrics results: first=%#v second=%#v", first, second)
	}
}

func TestTrackLyricsKeepsCachedPlainWhenRefreshUnavailable(t *testing.T) {
	client := &fakeClient{
		results: []Snapshot{
			{VideoID: "video-one", Kind: KindPlain, Source: "YTMusic", Text: "plain"},
			{VideoID: "video-one", Kind: KindUnavailable},
		},
	}
	service := NewService(client)

	first, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one", Title: "One"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.TrackLyrics(context.Background(), Request{VideoID: "video-one", Title: "One"})
	if err != nil {
		t.Fatal(err)
	}

	if first.Kind != KindPlain || second.Kind != KindPlain || second.Text != "plain" {
		t.Fatalf("expected cached plain fallback, first=%#v second=%#v", first, second)
	}
}
