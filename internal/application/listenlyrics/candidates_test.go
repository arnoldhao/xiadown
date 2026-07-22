package listenlyrics

import (
	"context"
	"testing"
)

type fakeCandidateClient struct {
	fakeClient
	candidates []Candidate
	preview    Snapshot
	previews   int
}

func (client *fakeCandidateClient) SearchLyricsCandidates(_ context.Context, request Request) ([]Candidate, error) {
	client.requests = append(client.requests, request)
	return append([]Candidate(nil), client.candidates...), nil
}

func (client *fakeCandidateClient) TrackLyricsCandidate(_ context.Context, _ CandidateRequest) (Snapshot, error) {
	client.previews++
	return cloneSnapshot(client.preview), nil
}

func TestSearchCandidatesNormalizesIdentityMetadata(t *testing.T) {
	client := &fakeCandidateClient{candidates: []Candidate{{ProviderID: "lrclib", ProviderTrackID: "42"}}}
	service := NewService(client)

	candidates, err := service.SearchCandidates(context.Background(), Request{
		Title:  " Song ",
		Artist: " Artist ",
		Album:  " Album ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || len(client.requests) != 1 {
		t.Fatalf("unexpected candidates: %#v", candidates)
	}
	request := client.requests[0]
	if request.Title != "Song" || request.Artist != "Artist" || request.Album != "Album" {
		t.Fatalf("candidate request not normalized: %#v", request)
	}
}

func TestTrackCandidateKeepsManualCacheSeparateFromAutomaticKey(t *testing.T) {
	client := &fakeCandidateClient{preview: Snapshot{
		Kind:   KindSynced,
		Source: "LRCLib",
		Lines:  []Line{{StartMs: 1000, Text: "chosen"}},
	}}
	service := NewService(client)
	service.current = Snapshot{VideoID: "current-video", Kind: KindPlain, Text: "current"}
	request := CandidateRequest{
		Track:           Request{VideoID: "video-one", Title: "Song"},
		ProviderID:      " LRCLIB ",
		ProviderTrackID: " 42 ",
	}

	result, err := service.TrackCandidate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderID != "lrclib" || result.ProviderTrackID != "42" {
		t.Fatalf("manual identity was not preserved: %#v", result)
	}
	if _, ok := service.cache[cacheKey(normalizeRequest(request.Track))]; ok {
		t.Fatal("manual preview polluted the automatic lyrics cache")
	}
	if len(service.cache) != 1 {
		t.Fatalf("expected one isolated manual cache entry, got %d", len(service.cache))
	}
	second, err := service.TrackCandidate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if second.ProviderTrackID != "42" || client.previews != 1 {
		t.Fatalf("expected manual preview cache hit, previews=%d result=%#v", client.previews, second)
	}
	if current := service.Current(); current.VideoID != "current-video" || current.Text != "current" {
		t.Fatalf("candidate preview changed active lyrics: %#v", current)
	}
}
