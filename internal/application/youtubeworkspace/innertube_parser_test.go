package youtubeworkspace

import (
	"encoding/json"
	"testing"
)

func TestParseInnerTubeItemsLegacyRendererFractal(t *testing.T) {
	data := parseInnerTubeFixture(t, `{
		"contents": [
			{"richItemRenderer": {"content": {"videoRenderer": {
				"videoId": "VIDEOR00001",
				"title": {"runs": [{"text": "A "}, {"text": "full video"}]},
				"ownerText": {"runs": [{
					"text": "Fixture Channel",
					"navigationEndpoint": {"browseEndpoint": {"browseId": "UCfixtureowner"}}
				}]},
				"lengthText": {"simpleText": "1:02:03"},
				"shortViewCountText": {"simpleText": "1.2M views"},
				"publishedTimeText": {"simpleText": "3 days ago"},
				"thumbnail": {"thumbnails": [
					{"url": "//img.example/small.jpg", "width": 320, "height": 180},
					{"url": "https://img.example/large.jpg", "width": 1280, "height": 720}
				]},
				"navigationEndpoint": {
					"commandMetadata": {"webCommandMetadata": {"url": "/watch?v=VIDEOR00001"}},
					"watchEndpoint": {"videoId": "VIDEOR00001"}
				}
			}}}},
			{"gridVideoRenderer": {
				"videoId": "VIDEOR00001",
				"title": {"simpleText": "Duplicate should be ignored"}
			}},
			{"gridVideoRenderer": {
				"videoId": "GRIDVID0001",
				"title": {"simpleText": "Live grid video"},
				"thumbnailOverlays": [{"thumbnailOverlayTimeStatusRenderer": {
					"style": "LIVE", "text": {"simpleText": "LIVE"}
				}}]
			}},
			{"compactVideoRenderer": {
				"videoId": "COMPACT0001",
				"title": {"simpleText": "Compact Short"},
				"navigationEndpoint": {
					"reelWatchEndpoint": {"videoId": "COMPACT0001"},
					"commandMetadata": {"webCommandMetadata": {"url": "/shorts/COMPACT0001"}}
				}
			}},
			{"videoCardRenderer": {
				"videoId": "CARDVID0001",
				"headline": {"simpleText": "Video card"},
				"viewCountText": {"simpleText": "12,345 views"}
			}},
			{"playlistVideoRenderer": {
				"videoId": "PLAYVID0001",
				"title": {"simpleText": "Playlist row"},
				"shortBylineText": {"runs": [{"text": "Playlist Owner"}]},
				"lengthText": {"simpleText": "4:56"}
			}},
			{"continuationItemRenderer": {
				"continuationEndpoint": {"continuationCommand": {"token": "legacy-next"}}
			}}
		]
	}`)

	result := parseInnerTubeItems(data, innerTubeItemsAll, 0)
	if got, want := len(result.Items), 5; got != want {
		t.Fatalf("items = %d, want %d: %#v", got, want, result.Items)
	}
	if result.Continuation != "legacy-next" {
		t.Fatalf("continuation = %q, want legacy-next", result.Continuation)
	}

	full := requireInnerTubeItem(t, result.Items, "VIDEOR00001", "")
	if full.Title != "A full video" || full.Channel != "Fixture Channel" || full.ChannelID != "UCfixtureowner" {
		t.Fatalf("unexpected full video identity: %#v", full)
	}
	if full.ThumbnailURL != "https://img.example/large.jpg" {
		t.Fatalf("thumbnail = %q, want largest source", full.ThumbnailURL)
	}
	if full.DurationSeconds != 3723 || full.DurationLabel != "1:02:03" {
		t.Fatalf("unexpected duration: %#v", full)
	}
	if full.ViewCount != 1_200_000 || full.PublishedLabel != "3 days ago" {
		t.Fatalf("unexpected statistics: %#v", full)
	}
	if full.WebURL != "https://www.youtube.com/watch?v=VIDEOR00001" {
		t.Fatalf("web URL = %q", full.WebURL)
	}

	live := requireInnerTubeItem(t, result.Items, "GRIDVID0001", "")
	if !live.Live || live.DurationLabel != "LIVE" || live.DurationSeconds != 0 {
		t.Fatalf("live renderer not normalized: %#v", live)
	}
	short := requireInnerTubeItem(t, result.Items, "COMPACT0001", "")
	if !short.Short || short.WebURL != "https://www.youtube.com/shorts/COMPACT0001" {
		t.Fatalf("compact short not detected: %#v", short)
	}
	card := requireInnerTubeItem(t, result.Items, "CARDVID0001", "")
	if card.Title != "Video card" || card.ViewCount != 12_345 {
		t.Fatalf("videoCardRenderer not parsed: %#v", card)
	}
	playlistRow := requireInnerTubeItem(t, result.Items, "PLAYVID0001", "")
	if playlistRow.Channel != "Playlist Owner" || playlistRow.DurationSeconds != 296 {
		t.Fatalf("playlistVideoRenderer not parsed: %#v", playlistRow)
	}
}

func TestParseInnerTubeItemsModernLockupFractalAndFilters(t *testing.T) {
	data := parseInnerTubeFixture(t, `{
		"onResponseReceivedActions": [{
			"appendContinuationItemsAction": {"continuationItems": [
				{"lockupViewModel": {
					"contentId": "LOCKUP00001",
					"contentType": "LOCKUP_CONTENT_TYPE_VIDEO",
					"contentImage": {"thumbnailViewModel": {
						"image": {"sources": [
							{"url": "https://img.example/lockup-small.jpg", "width": 320, "height": 180},
							{"url": "https://img.example/lockup-large.jpg", "width": 1920, "height": 1080}
						]},
						"overlays": [{"thumbnailBottomOverlayViewModel": {
							"badges": [{"thumbnailBadgeViewModel": {"text": "17:10"}}]
						}}]
					}},
					"metadata": {"lockupMetadataViewModel": {
						"title": {"content": "Modern lockup video"},
						"metadata": {"contentMetadataViewModel": {"metadataRows": [
							{"metadataParts": [{"text": {
								"content": "Modern Channel",
								"commandRuns": [{"onTap": {"innertubeCommand": {
									"browseEndpoint": {"browseId": "UCmodernchannel"}
								}}}]
							}}]},
							{"metadataParts": [
								{"text": {"content": "1.5M views"}},
								{"text": {"content": "9 months ago"}}
							]}
						]}}
					}},
					"rendererContext": {"commandContext": {"onTap": {"innertubeCommand": {
						"watchEndpoint": {"videoId": "LOCKUP00001"}
					}}}}
				}},
				{"shortsLockupViewModel": {
					"entityId": "shorts-shelf-item-SHORTS00001",
					"onTap": {"innertubeCommand": {"reelWatchEndpoint": {"videoId": "SHORTS00001"}}},
					"overlayMetadata": {
						"primaryText": {"content": "A fixture Short"},
						"secondaryText": {"content": "2.1M views"}
					},
					"thumbnailViewModel": {"image": {"sources": [
						{"url": "//img.example/short.jpg", "width": 405, "height": 720}
					]}}
				}},
				{"lockupViewModel": {
					"contentId": "PLfixture00001",
					"contentType": "LOCKUP_CONTENT_TYPE_PLAYLIST",
					"contentImage": {"thumbnailViewModel": {
						"image": {"sources": [{"url": "https://img.example/playlist.jpg", "width": 640, "height": 360}]},
						"overlays": [{"thumbnailBottomOverlayViewModel": {
							"badges": [{"thumbnailBadgeViewModel": {"text": "120 videos"}}]
						}}]
					}},
					"metadata": {"lockupMetadataViewModel": {
						"title": {"content": "Modern playlist"},
						"metadata": {"contentMetadataViewModel": {"metadataRows": [
							{"metadataParts": [{"text": {"content": "Playlist Channel"}}]}
						]}}
					}},
					"rendererContext": {"commandContext": {"onTap": {"innertubeCommand": {
						"watchEndpoint": {"playlistId": "PLfixture00001", "videoId": "LOCKUP00001"}
					}}}}
				}},
				{"lockupViewModel": {
					"contentId": "LOCKUP00001",
					"contentType": "LOCKUP_CONTENT_TYPE_VIDEO",
					"metadata": {"lockupMetadataViewModel": {"title": {"content": "Duplicate"}}}
				}},
				{"continuationItemRenderer": {
					"continuationEndpoint": {"continuationCommand": {"token": "modern-next"}}
				}}
			]}
		}]
	}`)

	all := parseInnerTubeItems(data, innerTubeItemsAll, 0)
	if got, want := len(all.Items), 3; got != want {
		t.Fatalf("all items = %d, want %d: %#v", got, want, all.Items)
	}
	if all.Continuation != "modern-next" {
		t.Fatalf("continuation = %q, want modern-next", all.Continuation)
	}
	video := requireInnerTubeItem(t, all.Items, "LOCKUP00001", "")
	if video.Channel != "Modern Channel" || video.ChannelID != "UCmodernchannel" {
		t.Fatalf("lockup channel not parsed: %#v", video)
	}
	if video.ThumbnailURL != "https://img.example/lockup-large.jpg" || video.DurationSeconds != 1030 {
		t.Fatalf("lockup media not parsed: %#v", video)
	}
	if video.ViewCount != 1_500_000 || video.PublishedLabel != "9 months ago" {
		t.Fatalf("lockup statistics not parsed: %#v", video)
	}

	shorts := parseInnerTubeItems(data, innerTubeItemsShortsOnly, 0)
	if got, want := len(shorts.Items), 1; got != want {
		t.Fatalf("shorts items = %d, want %d: %#v", got, want, shorts.Items)
	}
	short := shorts.Items[0]
	if short.VideoID != "SHORTS00001" || !short.Short || short.ViewCount != 2_100_000 {
		t.Fatalf("shortsLockupViewModel not parsed: %#v", short)
	}
	if short.ThumbnailURL != "https://img.example/short.jpg" {
		t.Fatalf("short thumbnail = %q", short.ThumbnailURL)
	}
	videos := parseInnerTubeItems(data, innerTubeItemsVideosOnly, 0)
	if got, want := len(videos.Items), 1; got != want {
		t.Fatalf("regular video items = %d, want %d: %#v", got, want, videos.Items)
	}
	if videos.Items[0].VideoID != "LOCKUP00001" || videos.Items[0].Short {
		t.Fatalf("videos-only filter leaked a non-video item: %#v", videos.Items[0])
	}

	playlists := parseInnerTubeItems(data, innerTubeItemsPlaylistsOnly, 0)
	if got, want := len(playlists.Items), 1; got != want {
		t.Fatalf("playlist items = %d, want %d: %#v", got, want, playlists.Items)
	}
	playlist := playlists.Items[0]
	if playlist.ItemKind != "playlist" || playlist.PlaylistID != "PLfixture00001" {
		t.Fatalf("playlist lockup identity not parsed: %#v", playlist)
	}
	if playlist.DurationLabel != "120 videos" || playlist.WebURL != "https://www.youtube.com/playlist?list=PLfixture00001" {
		t.Fatalf("playlist lockup metadata not parsed: %#v", playlist)
	}
}

func TestParseInnerTubeItemsLegacyPlaylistRenderers(t *testing.T) {
	data := parseInnerTubeFixture(t, `{
		"contents": [
			{"playlistRenderer": {
				"playlistId": "PLlegacy00001",
				"title": {"simpleText": "Legacy playlist"},
				"shortBylineText": {"runs": [{
					"text": "Legacy Owner",
					"navigationEndpoint": {"browseEndpoint": {"browseId": "UClegacyowner"}}
				}]},
				"videoCountText": {"simpleText": "48 videos"},
				"thumbnails": [{"thumbnails": [
					{"url": "https://img.example/pl-small.jpg", "width": 120, "height": 68},
					{"url": "https://img.example/pl-large.jpg", "width": 640, "height": 360}
				]}],
				"navigationEndpoint": {
					"commandMetadata": {"webCommandMetadata": {"url": "/playlist?list=PLlegacy00001"}},
					"browseEndpoint": {"browseId": "VLPLlegacy00001"}
				}
			}},
			{"gridPlaylistRenderer": {
				"title": {"runs": [{"text": "Grid playlist"}]},
				"videoCountShortText": {"simpleText": "9 videos"},
				"thumbnail": {"thumbnails": [{"url": "//img.example/grid-pl.jpg", "width": 480, "height": 270}]},
				"navigationEndpoint": {"browseEndpoint": {"browseId": "VLPLgrid000001"}}
			}},
			{"gridPlaylistRenderer": {
				"playlistId": "PLlegacy00001",
				"title": {"simpleText": "Duplicate playlist"}
			}}
		]
	}`)

	result := parseInnerTubeItems(data, innerTubeItemsPlaylistsOnly, 0)
	if got, want := len(result.Items), 2; got != want {
		t.Fatalf("playlist items = %d, want %d: %#v", got, want, result.Items)
	}
	legacy := requireInnerTubeItem(t, result.Items, "", "PLlegacy00001")
	if legacy.Channel != "Legacy Owner" || legacy.ChannelID != "UClegacyowner" {
		t.Fatalf("legacy playlist owner not parsed: %#v", legacy)
	}
	if legacy.ThumbnailURL != "https://img.example/pl-large.jpg" || legacy.DurationLabel != "48 videos" {
		t.Fatalf("legacy playlist media not parsed: %#v", legacy)
	}
	grid := requireInnerTubeItem(t, result.Items, "", "PLgrid000001")
	if grid.Title != "Grid playlist" || grid.ThumbnailURL != "https://img.example/grid-pl.jpg" {
		t.Fatalf("gridPlaylistRenderer not parsed: %#v", grid)
	}
}

func TestParseInnerTubeItemsFindsContinuationBeyondLimit(t *testing.T) {
	data := parseInnerTubeFixture(t, `{
		"contents": [
			{"videoRenderer": {"videoId": "LIMITED0001", "title": {"simpleText": "First"}}},
			{"videoRenderer": {"videoId": "LIMITED0002", "title": {"simpleText": "Second"}}}
		],
		"continuations": [{"nextContinuationData": {"continuation": "legacy-array-next"}}]
	}`)
	result := parseInnerTubeItems(data, innerTubeItemsAll, 1)
	if got, want := len(result.Items), 1; got != want {
		t.Fatalf("limited items = %d, want %d", got, want)
	}
	if result.Continuation != "legacy-array-next" {
		t.Fatalf("continuation = %q, want legacy-array-next", result.Continuation)
	}
}

func TestInnerTubeCountVariants(t *testing.T) {
	tests := map[string]int64{
		"1,234 views": 1_234,
		"1.25K views": 1_250,
		"2.1M views":  2_100_000,
		"3.2万次观看":     32_000,
		"":            0,
	}
	for input, want := range tests {
		if got := innerTubeCount(input); got != want {
			t.Errorf("innerTubeCount(%q) = %d, want %d", input, got, want)
		}
	}
}

func parseInnerTubeFixture(t *testing.T, raw string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return result
}

func requireInnerTubeItem(t *testing.T, items []Video, videoID string, playlistID string) Video {
	t.Helper()
	for _, item := range items {
		if videoID != "" && item.VideoID == videoID {
			return item
		}
		if playlistID != "" && item.PlaylistID == playlistID {
			return item
		}
	}
	t.Fatalf("item not found: video=%q playlist=%q in %#v", videoID, playlistID, items)
	return Video{}
}
