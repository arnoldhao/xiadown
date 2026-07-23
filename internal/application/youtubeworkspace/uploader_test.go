package youtubeworkspace

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestUploaderLoadsHeaderThenResponseDefinedVideosTab(t *testing.T) {
	t.Parallel()
	const channelID = "UCabcdefghijklmnopqrstuv"
	stub := &innerTubeRequesterStub{responses: []map[string]any{
		{
			"header": map[string]any{"pageHeaderRenderer": map[string]any{
				"pageTitle": "Workspace Creator",
				"content": map[string]any{"pageHeaderViewModel": map[string]any{
					"image": map[string]any{"decoratedAvatarViewModel": map[string]any{
						"avatar": map[string]any{"avatarViewModel": map[string]any{
							"image": uploaderThumbnail("https://yt3.example/avatar.jpg", 160, 160),
						}},
					}},
					"banner": map[string]any{"imageBannerViewModel": map[string]any{
						"image": uploaderThumbnail("https://yt3.example/banner.jpg", 2560, 424),
					}},
					"metadata": map[string]any{"contentMetadataViewModel": map[string]any{
						"metadataRows": []any{
							map[string]any{"metadataParts": []any{
								map[string]any{"text": map[string]any{"content": "@workspace"}},
							}},
							map[string]any{"metadataParts": []any{
								map[string]any{"text": map[string]any{"content": "34.9K subscribers"}},
								map[string]any{"text": map[string]any{"content": "173 videos"}},
							}},
						},
					}},
				}},
			}},
			"metadata": map[string]any{"channelMetadataRenderer": map[string]any{
				"title":            "Workspace Creator",
				"description":      "A complete channel description.\nSecond line.",
				"externalId":       channelID,
				"vanityChannelUrl": "https://www.youtube.com/@workspace",
			}},
			"subscribe": map[string]any{"subscribeButtonRenderer": map[string]any{
				"subscribed": true,
			}},
			"contents": []any{
				map[string]any{"tabRenderer": map[string]any{
					"title": "Startseite",
					"endpoint": map[string]any{"browseEndpoint": map[string]any{
						"browseId": channelID,
						"params":   "featured-params",
					}},
				}},
				map[string]any{"tabRenderer": map[string]any{
					"title": "Videos",
					"endpoint": map[string]any{
						"commandMetadata": map[string]any{"webCommandMetadata": map[string]any{
							"url": "/@workspace/videos",
						}},
						"browseEndpoint": map[string]any{
							"browseId": channelID,
							"params":   "videos%3Dparams",
						},
					},
				}},
				map[string]any{"videoRenderer": uploaderVideoRenderer("Featured001", "Featured item")},
			},
		},
		{
			"contents": []any{
				map[string]any{"videoRenderer": uploaderVideoRenderer("LatestVid01", "Latest upload")},
				map[string]any{"continuationItemRenderer": map[string]any{
					"continuationEndpoint": map[string]any{"continuationCommand": map[string]any{
						"token": "uploader-page-2",
					}},
				}},
			},
		},
	}}

	page, err := newInnerTubeServiceForTest(stub).Uploader(
		context.Background(),
		UploaderRequest{ChannelID: " " + channelID + " ", Locale: "zh-Hant-TW"},
	)
	if err != nil {
		t.Fatalf("Uploader: %v", err)
	}
	if page.ChannelID != channelID || page.Name != "Workspace Creator" || page.Handle != "@workspace" ||
		page.Description != "A complete channel description.\nSecond line." ||
		page.AvatarURL != "https://yt3.example/avatar.jpg" ||
		page.BannerURL != "https://yt3.example/banner.jpg" ||
		page.SubscriberCount != 34900 || page.SubscriberLabel != "34.9K subscribers" ||
		page.VideoCountLabel != "173 videos" || !page.IsSubscribed ||
		page.WebURL != "https://www.youtube.com/@workspace" || page.Continuation != "uploader-page-2" {
		t.Fatalf("uploader header = %#v", page)
	}
	if len(page.Videos) != 1 || page.Videos[0].VideoID != "LatestVid01" ||
		page.Videos[0].Channel != "Workspace Creator" || page.Videos[0].ChannelID != channelID {
		t.Fatalf("uploader videos = %#v", page.Videos)
	}
	calls := stub.snapshotCalls()
	if len(calls) != 2 || calls[0].endpoint != "browse" || calls[1].endpoint != "browse" ||
		calls[0].authPolicy != innerTubeAuthOptional || calls[0].locale != "zh-TW" || calls[1].locale != "zh-TW" {
		t.Fatalf("uploader calls = %#v", calls)
	}
	if !reflect.DeepEqual(calls[0].body, map[string]any{"browseId": channelID}) ||
		!reflect.DeepEqual(calls[1].body, map[string]any{
			"browseId": channelID,
			"params":   "videos=params",
		}) {
		t.Fatalf("uploader bodies = %#v", calls)
	}
}

func TestUploaderContinuationUsesBrowseTokenWithoutReloadingHeader(t *testing.T) {
	t.Parallel()
	const channelID = "UCabcdefghijklmnopqrstuv"
	stub := &innerTubeRequesterStub{responses: []map[string]any{{
		"contents": []any{map[string]any{
			"videoRenderer": uploaderVideoRenderer("MoreVideo02", "More upload"),
		}},
	}}}
	page, err := newInnerTubeServiceForTest(stub).Uploader(
		context.Background(),
		UploaderRequest{ChannelID: channelID, Continuation: " next-token "},
	)
	if err != nil {
		t.Fatalf("Uploader continuation: %v", err)
	}
	if page.ChannelID != channelID || page.Name != channelID || len(page.Videos) != 1 ||
		page.Videos[0].ChannelID != channelID {
		t.Fatalf("continuation page = %#v", page)
	}
	calls := stub.snapshotCalls()
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].body, map[string]any{"continuation": "next-token"}) {
		t.Fatalf("continuation calls = %#v", calls)
	}
}

func TestUploaderModernVideoLockupKeepsFirstMetadataRowAsStatistics(t *testing.T) {
	t.Parallel()
	const channelID = "UCabcdefghijklmnopqrstuv"
	stub := &innerTubeRequesterStub{responses: []map[string]any{
		{
			"header": map[string]any{"pageHeaderRenderer": map[string]any{
				"pageTitle": "Workspace Creator",
			}},
			"contents": []any{map[string]any{"tabRenderer": map[string]any{
				"tabIdentifier": "videos",
				"endpoint": map[string]any{"browseEndpoint": map[string]any{
					"browseId": channelID,
					"params":   "videos-params",
				}},
			}}},
		},
		{
			"contents": []any{uploaderVideoLockup("ModernVid01", "Modern upload")},
		},
	}}

	page, err := newInnerTubeServiceForTest(stub).Uploader(
		context.Background(),
		UploaderRequest{ChannelID: channelID},
	)
	if err != nil {
		t.Fatalf("Uploader: %v", err)
	}
	if len(page.Videos) != 1 {
		t.Fatalf("uploader videos = %#v", page.Videos)
	}
	video := page.Videos[0]
	if video.Channel != "Workspace Creator" || video.ChannelID != channelID {
		t.Fatalf("modern uploader identity = %#v", video)
	}
	if video.ViewCount != 169 || video.PublishedLabel != "2 weeks ago" {
		t.Fatalf("modern uploader statistics = %#v", video)
	}
}

func TestUploaderParsesLegacyChannelHeader(t *testing.T) {
	t.Parallel()
	const channelID = "UCabcdefghijklmnopqrstuv"
	page := parseInnerTubeUploader(map[string]any{
		"header": map[string]any{"c4TabbedHeaderRenderer": map[string]any{
			"channelId":           channelID,
			"title":               "Legacy creator",
			"subscriberCountText": map[string]any{"simpleText": "1.2M subscribers"},
			"videosCountText":     map[string]any{"runs": []any{map[string]any{"text": "80 videos"}}},
			"avatar":              uploaderThumbnail("//yt3.example/legacy-avatar.jpg", 100, 100),
			"banner":              uploaderThumbnail("//yt3.example/legacy-banner.jpg", 1280, 212),
		}},
		"metadata": map[string]any{"channelMetadataRenderer": map[string]any{
			"description": "Legacy description",
			"channelUrl":  "https://www.youtube.com/channel/" + channelID,
		}},
	}, channelID)
	if page.Name != "Legacy creator" || page.SubscriberCount != 1_200_000 ||
		page.VideoCountLabel != "80 videos" || page.Description != "Legacy description" ||
		page.AvatarURL != "https://yt3.example/legacy-avatar.jpg" ||
		page.BannerURL != "https://yt3.example/legacy-banner.jpg" {
		t.Fatalf("legacy page = %#v", page)
	}
}

func TestUploaderRejectsInvalidChannelAndPreservesRequesterErrors(t *testing.T) {
	t.Parallel()
	invalid := &innerTubeRequesterStub{}
	_, err := newInnerTubeServiceForTest(invalid).Uploader(
		context.Background(),
		UploaderRequest{ChannelID: "not-a-channel"},
	)
	if err == nil || !strings.Contains(err.Error(), "invalid youtube channel id") || len(invalid.snapshotCalls()) != 0 {
		t.Fatalf("invalid uploader error = %v, calls = %#v", err, invalid.snapshotCalls())
	}

	backendErr := errors.New("network unavailable")
	_, err = newInnerTubeServiceForTest(&innerTubeRequesterStub{errors: []error{backendErr}}).Uploader(
		context.Background(),
		UploaderRequest{ChannelID: "UCabcdefghijklmnopqrstuv"},
	)
	if !errors.Is(err, backendErr) {
		t.Fatalf("requester error = %v", err)
	}
}

func uploaderThumbnail(rawURL string, width int, height int) map[string]any {
	return map[string]any{"sources": []any{map[string]any{
		"url": rawURL, "width": width, "height": height,
	}}}
}

func uploaderVideoRenderer(videoID string, title string) map[string]any {
	return map[string]any{
		"videoId": videoID,
		"title":   map[string]any{"simpleText": title},
		"thumbnail": map[string]any{"thumbnails": []any{map[string]any{
			"url":   "https://i.ytimg.com/vi/" + videoID + "/hqdefault.jpg",
			"width": 480, "height": 360,
		}}},
		"lengthText":        map[string]any{"simpleText": "3:42"},
		"viewCountText":     map[string]any{"simpleText": "12K views"},
		"publishedTimeText": map[string]any{"simpleText": "2 days ago"},
	}
}

func uploaderVideoLockup(videoID string, title string) map[string]any {
	return map[string]any{
		"lockupViewModel": map[string]any{
			"contentId":   videoID,
			"contentType": "LOCKUP_CONTENT_TYPE_VIDEO",
			"metadata": map[string]any{"lockupMetadataViewModel": map[string]any{
				"title": map[string]any{"content": title},
				"metadata": map[string]any{"contentMetadataViewModel": map[string]any{
					"metadataRows": []any{map[string]any{
						"metadataParts": []any{
							map[string]any{"text": map[string]any{"content": "169 views"}},
							map[string]any{"text": map[string]any{"content": "2 weeks ago"}},
						},
					}},
				}},
			}},
			"rendererContext": map[string]any{"commandContext": map[string]any{
				"onTap": map[string]any{"innertubeCommand": map[string]any{
					"watchEndpoint": map[string]any{"videoId": videoID},
				}},
			}},
		},
	}
}
