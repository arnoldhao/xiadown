package youtubeworkspace

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestVideoDetailsUsesPlayerEndpointAndParsesRichMetadata(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{responses: []map[string]any{
		{
			"videoDetails": map[string]any{
				"videoId":          "AbCdEfGh123",
				"title":            "Player title",
				"author":           "Player channel",
				"channelId":        "UCabcdefghijklmnopqrstuv",
				"lengthSeconds":    "3723",
				"viewCount":        json.Number("987654"),
				"shortDescription": "First line\nSecond line",
				"thumbnail": map[string]any{"thumbnails": []any{
					map[string]any{"url": "//i.ytimg.com/vi/AbCdEfGh123/mqdefault.jpg", "width": 320, "height": 180},
					map[string]any{"url": "https://i.ytimg.com/vi/AbCdEfGh123/maxresdefault.jpg", "width": 1280, "height": 720},
				}},
			},
			"microformat": map[string]any{"playerMicroformatRenderer": map[string]any{
				"publishDate":      "2026-07-02",
				"uploadDate":       "2026-07-01",
				"dateText":         map[string]any{"simpleText": "Jul 2, 2026"},
				"ownerChannelName": "Microformat channel",
			}},
			"frameworkUpdates": map[string]any{"entityBatchUpdate": map[string]any{
				"mutations": []any{map[string]any{"payload": map[string]any{
					"videoActionBarEntity": map[string]any{
						"likeCountText": map[string]any{"simpleText": "12.3K likes"},
					},
				}}},
			}},
		},
		{
			"contents": []any{map[string]any{
				"videoSecondaryInfoRenderer": map[string]any{
					"owner": map[string]any{"videoOwnerRenderer": map[string]any{
						"thumbnail": map[string]any{"thumbnails": []any{
							map[string]any{"url": "//yt3.example/channel-avatar.jpg", "width": 88, "height": 88},
						}},
					}},
					"subscribeButton": map[string]any{
						"subscribeButtonRenderer": map[string]any{
							"channelId":  "UCabcdefghijklmnopqrstuv",
							"subscribed": true,
						},
					},
				},
			}},
		},
	}}

	details, err := newInnerTubeServiceForTest(stub).VideoDetails(
		context.Background(),
		VideoDetailsRequest{VideoID: " AbCdEfGh123 ", Locale: "zh-Hant-TW"},
	)
	if err != nil {
		t.Fatalf("VideoDetails: %v", err)
	}
	want := VideoDetails{
		VideoID:          "AbCdEfGh123",
		Title:            "Player title",
		Channel:          "Player channel",
		ChannelID:        "UCabcdefghijklmnopqrstuv",
		ChannelAvatarURL: "https://yt3.example/channel-avatar.jpg",
		ThumbnailURL:     "https://i.ytimg.com/vi/AbCdEfGh123/maxresdefault.jpg",
		DurationSeconds:  3723,
		ViewCount:        987654,
		LikeCount:        12300,
		PublishedDate:    "2026-07-02",
		PublishedLabel:   "Jul 2, 2026",
		Description:      "First line\nSecond line",
		IsSubscribed:     true,
		WebURL:           "https://www.youtube.com/watch?v=AbCdEfGh123",
	}
	if !reflect.DeepEqual(details, want) {
		t.Fatalf("details = %#v, want %#v", details, want)
	}
	calls := stub.snapshotCalls()
	if len(calls) != 2 || calls[0].endpoint != "player" || calls[1].endpoint != "next" ||
		calls[0].authPolicy != innerTubeAuthOptional || calls[0].locale != "zh-TW" {
		t.Fatalf("player request = %#v", calls)
	}
	if !reflect.DeepEqual(calls[0].body, map[string]any{
		"videoId":        "AbCdEfGh123",
		"contentCheckOk": true,
		"racyCheckOk":    true,
	}) {
		t.Fatalf("player request body = %#v", calls[0].body)
	}
}

func TestVideoDetailsFallsBackToMicroformatAndCanonicalMetadata(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{responses: []map[string]any{{
		"microformat": map[string]any{"playerMicroformatRenderer": map[string]any{
			"title":             map[string]any{"simpleText": "Microformat title"},
			"ownerChannelName":  "Microformat channel",
			"externalChannelId": "UCzyxwvutsrqponmlkjihgfe",
			"lengthSeconds":     json.Number("75"),
			"viewCount":         "1,234",
			"likeCount":         0,
			"uploadDate":        "2025-12-31",
			"description":       map[string]any{"simpleText": "Microformat description"},
		}},
	}}}

	details, err := newInnerTubeServiceForTest(stub).VideoDetails(
		context.Background(),
		VideoDetailsRequest{VideoID: "ZyXwVuTs987"},
	)
	if err != nil {
		t.Fatalf("VideoDetails: %v", err)
	}
	if details.VideoID != "ZyXwVuTs987" || details.Title != "Microformat title" ||
		details.Channel != "Microformat channel" || details.ChannelID != "UCzyxwvutsrqponmlkjihgfe" ||
		details.DurationSeconds != 75 || details.ViewCount != 1234 || details.LikeCount != 0 ||
		details.PublishedDate != "2025-12-31" || details.PublishedLabel != "2025-12-31" ||
		details.Description != "Microformat description" ||
		details.ThumbnailURL != "https://i.ytimg.com/vi/ZyXwVuTs987/hqdefault.jpg" {
		t.Fatalf("microformat details = %#v", details)
	}
}

func TestVideoDetailsFallsBackToWatchNextLikeEntity(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{responses: []map[string]any{
		{
			"videoDetails": map[string]any{
				"videoId": "AbCdEfGh123",
				"title":   "Player title",
			},
		},
		{
			"frameworkUpdates": map[string]any{
				"entityBatchUpdate": map[string]any{
					"mutations": []any{map[string]any{
						"payload": map[string]any{
							"likeCountEntity": map[string]any{
								"likeCountIfLiked": map[string]any{"content": "98.7K"},
							},
						},
					}},
				},
			},
		},
	}}

	details, err := newInnerTubeServiceForTest(stub).VideoDetails(
		context.Background(),
		VideoDetailsRequest{VideoID: "AbCdEfGh123"},
	)
	if err != nil {
		t.Fatalf("VideoDetails: %v", err)
	}
	if details.LikeCount != 98700 {
		t.Fatalf("watch-next like count = %d", details.LikeCount)
	}
	calls := stub.snapshotCalls()
	if len(calls) != 2 || calls[0].endpoint != "player" || calls[1].endpoint != "next" {
		t.Fatalf("detail endpoints = %#v", calls)
	}
}

func TestVideoDetailsRejectsInvalidIDWithoutRequest(t *testing.T) {
	t.Parallel()
	stub := &innerTubeRequesterStub{}
	_, err := newInnerTubeServiceForTest(stub).VideoDetails(
		context.Background(),
		VideoDetailsRequest{VideoID: "invalid"},
	)
	if err == nil || !strings.Contains(err.Error(), "invalid youtube video id") {
		t.Fatalf("invalid id error = %v", err)
	}
	if len(stub.snapshotCalls()) != 0 {
		t.Fatalf("invalid id reached requester: %#v", stub.snapshotCalls())
	}
}

func TestVideoDetailsPreservesRequesterAndPlayabilityErrors(t *testing.T) {
	t.Parallel()
	backendErr := errors.New("network unavailable")
	_, err := newInnerTubeServiceForTest(&innerTubeRequesterStub{errors: []error{backendErr}}).VideoDetails(
		context.Background(),
		VideoDetailsRequest{VideoID: "AbCdEfGh123"},
	)
	if !errors.Is(err, backendErr) {
		t.Fatalf("requester error = %v", err)
	}

	_, err = newInnerTubeServiceForTest(&innerTubeRequesterStub{responses: []map[string]any{{
		"playabilityStatus": map[string]any{"status": "ERROR", "reason": "Video unavailable"},
	}}}).VideoDetails(context.Background(), VideoDetailsRequest{VideoID: "AbCdEfGh123"})
	if err == nil || !strings.Contains(err.Error(), "Video unavailable") {
		t.Fatalf("playability error = %v", err)
	}
}
