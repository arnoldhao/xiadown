package youtubemusic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/connectors"
)

type fakeCookieProvider struct {
	records []appcookies.Record
	err     error
}

func (provider fakeCookieProvider) CookiesForConnectorType(context.Context, connectors.ConnectorType) ([]appcookies.Record, error) {
	return provider.records, provider.err
}

type testHTTPClientProvider struct {
	client *http.Client
}

func (provider *testHTTPClientProvider) HTTPClient() *http.Client {
	return provider.client
}

func TestAuthHeadersBuildSAPISIDHash(t *testing.T) {
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
		{Name: "SID", Value: "test-sid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }

	headers, err := client.authHeaders(context.Background())
	if err != nil {
		t.Fatalf("auth headers: %v", err)
	}
	if got := headers["Authorization"]; !strings.HasPrefix(got, "SAPISIDHASH 1700000000_") {
		t.Fatalf("unexpected auth header: %q", got)
	}
	if got := headers["Cookie"]; !strings.Contains(got, "__Secure-3PAPISID=test-sapisid") || !strings.Contains(got, "SID=test-sid") {
		t.Fatalf("unexpected cookie header: %q", got)
	}
}

func TestAuthHeadersWrapsMissingCookiesAsNotAuthenticated(t *testing.T) {
	client := NewClient(fakeCookieProvider{err: connectors.ErrNoCookies})

	_, err := client.authHeaders(context.Background())
	if !errors.Is(err, ErrNotAuthenticated) || !errors.Is(err, connectors.ErrNoCookies) {
		t.Fatalf("expected missing cookies auth error, got %v", err)
	}
}

func TestRequestWrapsTimeoutError(t *testing.T) {
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.httpClient = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}

	_, err := client.SearchSongs(context.Background(), "lofi", 1)
	if !errors.Is(err, ErrRequestTimedOut) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected wrapped timeout error, got %v", err)
	}
}

func TestRequestWrapsEOFAsNetworkError(t *testing.T) {
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.httpClient = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, io.EOF
	})}

	_, err := client.SearchSongs(context.Background(), "lofi", 1)
	if !errors.Is(err, ErrNetworkUnavailable) || !errors.Is(err, io.EOF) {
		t.Fatalf("expected wrapped EOF network error, got %v", err)
	}
}

func TestRequestUsesLatestHTTPClientProviderClient(t *testing.T) {
	provider := &testHTTPClientProvider{}
	client := NewClientWithHTTPClientProvider(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}}, provider)
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }

	firstCalls := 0
	secondCalls := 0
	provider.client = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		firstCalls++
		return nil, errors.New("dial tcp: connection refused")
	})}

	_, err := client.SearchSongs(context.Background(), "lofi", 1)
	if !errors.Is(err, ErrNetworkUnavailable) {
		t.Fatalf("expected network error from first provided client, got %v", err)
	}

	provider.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		secondCalls++
		return testHTTPResponse(request, http.StatusOK, `{}`), nil
	})}

	if _, err := client.SearchSongs(context.Background(), "lofi", 1); err != nil {
		t.Fatalf("expected second provided client to recover request, got %v", err)
	}
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("unexpected provider client calls: first=%d second=%d", firstCalls, secondCalls)
	}
}

func TestParseSearchSongs(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"sectionListRenderer": map[string]any{
				"contents": []any{
					map[string]any{
						"musicShelfRenderer": map[string]any{
							"contents": []any{
								map[string]any{
									"musicResponsiveListItemRenderer": map[string]any{
										"playlistItemData": map[string]any{"videoId": "TESTVID007G"},
										"flexColumns": []any{
											map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Lofi Mix"}}}}},
											map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{
												map[string]any{"text": "Song"},
												map[string]any{"text": " • "},
												map[string]any{
													"text":               "Super Lofi World",
													"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UCsuperlofi"}},
												},
											}}}},
										},
										"fixedColumns": []any{
											map[string]any{"musicResponsiveListItemFixedColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "3:21"}}}}},
										},
										"thumbnail": map[string]any{"musicThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{
											map[string]any{"url": "https://lh3.googleusercontent.com/small"},
											map[string]any{"url": "https://lh3.googleusercontent.com/large"},
										}}}},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	tracks := parseSearchSongs(data, 10)
	if len(tracks) != 1 {
		t.Fatalf("expected one track, got %d", len(tracks))
	}
	track := tracks[0]
	if track.VideoID != "TESTVID007G" || track.Title != "Lofi Mix" || track.Channel != "Super Lofi World" || track.DurationLabel != "3:21" {
		t.Fatalf("unexpected track: %#v", track)
	}
	if track.ArtistBrowseID != "UCsuperlofi" {
		t.Fatalf("unexpected artist browse id: %q", track.ArtistBrowseID)
	}
	if track.ThumbnailURL != "https://lh3.googleusercontent.com/large" {
		t.Fatalf("unexpected thumbnail: %q", track.ThumbnailURL)
	}
}

func TestTrackThumbnailMatchesKasetRendererThumbnail(t *testing.T) {
	track, ok := trackFromMusicResponsiveRenderer(map[string]any{
		"playlistItemData": map[string]any{"videoId": "TESTVID007G"},
		"flexColumns": []any{
			map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Lofi Mix"}}}}},
			map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{
				map[string]any{
					"text":               "Super Lofi World",
					"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UCsuperlofi"}},
				},
			}}}},
		},
		"thumbnail": map[string]any{"musicThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{
			map[string]any{"url": "https://lh3.googleusercontent.com/song-small"},
			map[string]any{"url": "https://lh3.googleusercontent.com/song-large"},
		}}}},
		"menu": map[string]any{"menuRenderer": map[string]any{"items": []any{
			map[string]any{"menuNavigationItemRenderer": map[string]any{"icon": map[string]any{"thumbnails": []any{
				map[string]any{"url": "https://lh3.googleusercontent.com/unrelated-menu-image"},
			}}}},
		}}},
	})
	if !ok {
		t.Fatal("expected track")
	}
	if track.ThumbnailURL != "https://lh3.googleusercontent.com/song-large" {
		t.Fatalf("unexpected thumbnail: %q", track.ThumbnailURL)
	}
}

func TestTrackThumbnailNormalizesProtocolRelativeURL(t *testing.T) {
	track, ok := trackFromMusicResponsiveRenderer(map[string]any{
		"playlistItemData": map[string]any{"videoId": "TESTVID007G"},
		"flexColumns": []any{
			map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Lofi Mix"}}}}},
			map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{
				map[string]any{"text": "Super Lofi World"},
			}}}},
		},
		"thumbnail": map[string]any{"musicThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{
			map[string]any{"url": "//lh3.googleusercontent.com/song-small"},
			map[string]any{"url": "//lh3.googleusercontent.com/song-large"},
		}}}},
	})
	if !ok {
		t.Fatal("expected track")
	}
	if track.ThumbnailURL != "https://lh3.googleusercontent.com/song-large" {
		t.Fatalf("unexpected thumbnail: %q", track.ThumbnailURL)
	}
}

func TestThumbnailExtractionMatchesKasetCroppedSquarePriority(t *testing.T) {
	thumbnailURL := lastThumbnailURL(map[string]any{
		"thumbnail": map[string]any{"croppedSquareThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{
			map[string]any{"url": "//lh3.googleusercontent.com/playlist-small"},
			map[string]any{"url": "//lh3.googleusercontent.com/playlist-large"},
		}}}},
		"menu": map[string]any{"menuRenderer": map[string]any{"items": []any{
			map[string]any{"menuNavigationItemRenderer": map[string]any{"icon": map[string]any{"thumbnails": []any{
				map[string]any{"url": "https://lh3.googleusercontent.com/unrelated-menu-image"},
			}}}},
		}}},
	})
	if thumbnailURL != "https://lh3.googleusercontent.com/playlist-large" {
		t.Fatalf("unexpected thumbnail: %q", thumbnailURL)
	}
}

func TestThumbnailExtractionMatchesKasetForegroundThumbnail(t *testing.T) {
	thumbnailURL := lastThumbnailURL(map[string]any{
		"foregroundThumbnail": map[string]any{"musicThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{
			map[string]any{"url": "//lh3.googleusercontent.com/header-small"},
			map[string]any{"url": "//lh3.googleusercontent.com/header-large"},
		}}}},
	})
	if thumbnailURL != "https://lh3.googleusercontent.com/header-large" {
		t.Fatalf("unexpected thumbnail: %q", thumbnailURL)
	}
}

func TestParseSearchArtistsAndPlaylists(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"sectionListRenderer": map[string]any{
				"contents": []any{
					map[string]any{
						"musicShelfRenderer": map[string]any{
							"contents": []any{
								map[string]any{
									"musicResponsiveListItemRenderer": map[string]any{
										"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{
											"browseId": "UCsuperlofi",
											"browseEndpointContextSupportedConfigs": map[string]any{
												"browseEndpointContextMusicConfig": map[string]any{"pageType": "MUSIC_PAGE_TYPE_ARTIST"},
											},
										}},
										"flexColumns": []any{
											map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Super Lofi World"}}}}},
											map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Artist"}}}}},
										},
										"thumbnail": map[string]any{"musicThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{
											map[string]any{"url": "https://lh3.googleusercontent.com/artist"},
										}}}},
									},
								},
								map[string]any{
									"musicResponsiveListItemRenderer": map[string]any{
										"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{
											"browseId": "VLPL1234567890",
											"browseEndpointContextSupportedConfigs": map[string]any{
												"browseEndpointContextMusicConfig": map[string]any{"pageType": "MUSIC_PAGE_TYPE_PLAYLIST"},
											},
										}},
										"flexColumns": []any{
											map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Focus Queue"}}}}},
											map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Dream FM"}}}}},
										},
										"thumbnail": map[string]any{"musicThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{
											map[string]any{"url": "https://i.ytimg.com/vi/focus/hqdefault.jpg"},
										}}}},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	artists := parseSearchArtists(data, 10)
	if len(artists) != 1 {
		t.Fatalf("expected one artist, got %d", len(artists))
	}
	if artists[0].ID != "UCsuperlofi" || artists[0].Name != "Super Lofi World" || artists[0].Subtitle != "Artist" {
		t.Fatalf("unexpected artists: %#v", artists)
	}
	if artists[0].ThumbnailURL != "https://lh3.googleusercontent.com/artist" {
		t.Fatalf("unexpected artist thumbnail: %q", artists[0].ThumbnailURL)
	}

	playlists := parseSearchPlaylists(data, 10)
	if len(playlists) != 1 {
		t.Fatalf("expected one playlist, got %d", len(playlists))
	}
	if playlists[0].ID != "VLPL1234567890" || playlists[0].Title != "Focus Queue" || playlists[0].Channel != "Dream FM" {
		t.Fatalf("unexpected playlists: %#v", playlists)
	}
}

func TestParseRadioTracks(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"singleColumnMusicWatchNextResultsRenderer": map[string]any{
				"tabbedRenderer": map[string]any{
					"watchNextTabbedResultsRenderer": map[string]any{
						"tabs": []any{
							map[string]any{"tabRenderer": map[string]any{"content": map[string]any{"musicQueueRenderer": map[string]any{"content": map[string]any{"playlistPanelRenderer": map[string]any{"contents": []any{
								map[string]any{"playlistPanelVideoRenderer": map[string]any{
									"videoId": "TESTVID008H",
									"title":   map[string]any{"runs": []any{map[string]any{"text": "Lofi Radio"}}},
									"longBylineText": map[string]any{"runs": []any{map[string]any{
										"text":               "Lofi Girl",
										"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UClufigirl"}},
									}}},
									"lengthText": map[string]any{"runs": []any{map[string]any{"text": "LIVE"}}},
								}},
							}}}}}}},
						},
					},
				},
			},
		},
	}

	tracks := parseRadioTracks(data, 10)
	if len(tracks) != 1 {
		t.Fatalf("expected one track, got %d", len(tracks))
	}
	if tracks[0].VideoID != "TESTVID008H" || tracks[0].Title != "Lofi Radio" || tracks[0].Channel != "Lofi Girl" || tracks[0].DurationLabel != "LIVE" {
		t.Fatalf("unexpected radio track: %#v", tracks[0])
	}
	if tracks[0].ArtistBrowseID != "UClufigirl" {
		t.Fatalf("unexpected radio artist browse id: %q", tracks[0].ArtistBrowseID)
	}
}

func TestParseHomeRecommendationTracks(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"singleColumnBrowseResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"contents": []any{
										map[string]any{
											"musicShelfRenderer": map[string]any{
												"contents": []any{
													map[string]any{
														"musicResponsiveListItemRenderer": map[string]any{
															"playlistItemData": map[string]any{"videoId": "TESTVID007G"},
															"flexColumns": []any{
																map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Lofi Mix"}}}}},
																map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Super Lofi World"}}}}},
															},
														},
													},
													map[string]any{
														"musicTwoRowItemRenderer": map[string]any{
															"title": map[string]any{"runs": []any{map[string]any{"text": "Moonlight"}}},
															"subtitle": map[string]any{"runs": []any{
																map[string]any{
																	"text":               "Dreamy Producer",
																	"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UCdreamy"}},
																},
																map[string]any{"text": " • "},
																map[string]any{"text": "2:49"},
															}},
															"navigationEndpoint": map[string]any{"watchEndpoint": map[string]any{"videoId": "a1b2c3d4e5F"}},
															"thumbnailRenderer":  map[string]any{"musicThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{map[string]any{"url": "https://i.ytimg.com/vi/a1b2c3d4e5F/hqdefault.jpg"}}}}},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	tracks := parseHomeRecommendationTracks(data, 10)
	if len(tracks) != 2 {
		t.Fatalf("expected two tracks, got %d", len(tracks))
	}
	if tracks[0].VideoID != "TESTVID007G" || tracks[1].VideoID != "a1b2c3d4e5F" {
		t.Fatalf("unexpected tracks: %#v", tracks)
	}
	if tracks[1].Channel != "Dreamy Producer" || tracks[1].DurationLabel != "2:49" {
		t.Fatalf("unexpected two-row track: %#v", tracks[1])
	}
	if tracks[1].ArtistBrowseID != "UCdreamy" {
		t.Fatalf("unexpected two-row artist browse id: %q", tracks[1].ArtistBrowseID)
	}
}

func TestParseLibraryBrowsePlaylists(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"singleColumnBrowseResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"contents": []any{
										map[string]any{
											"gridRenderer": map[string]any{
												"items": []any{
													map[string]any{
														"musicTwoRowItemRenderer": map[string]any{
															"title":              map[string]any{"runs": []any{map[string]any{"text": "My Liked Mix"}}},
															"subtitle":           map[string]any{"runs": []any{map[string]any{"text": "You"}}},
															"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "VLPL1234567890"}},
															"thumbnailRenderer":  map[string]any{"musicThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{map[string]any{"url": "https://i.ytimg.com/vi/aaa/hqdefault.jpg"}}}}},
														},
													},
												},
											},
										},
										map[string]any{
											"musicShelfRenderer": map[string]any{
												"contents": []any{
													map[string]any{
														"musicResponsiveListItemRenderer": map[string]any{
															"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "VLPLabcdefghij"}},
															"flexColumns": []any{
																map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Focus Queue"}}}}},
																map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Arnold"}}}}},
															},
															"thumbnail": map[string]any{"musicThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{map[string]any{"url": "https://i.ytimg.com/vi/bbb/hqdefault.jpg"}}}}},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	playlists := parseLibraryBrowsePlaylists(data, 10)
	if len(playlists) != 2 {
		t.Fatalf("expected two playlists, got %d", len(playlists))
	}
	if playlists[0].ID != "VLPL1234567890" || playlists[1].ID != "VLPLabcdefghij" {
		t.Fatalf("unexpected playlists: %#v", playlists)
	}
	if playlists[0].Channel != "You" || playlists[1].Channel != "Arnold" {
		t.Fatalf("unexpected playlist channels: %#v", playlists)
	}
}

func TestParseHomeShelves(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"singleColumnBrowseResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"contents": []any{
										map[string]any{
											"musicShelfRenderer": map[string]any{
												"title": map[string]any{"runs": []any{map[string]any{"text": "Quick picks"}}},
												"contents": []any{
													map[string]any{
														"musicResponsiveListItemRenderer": map[string]any{
															"playlistItemData": map[string]any{"videoId": "TESTVID007G"},
															"flexColumns": []any{
																map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Lofi Mix"}}}}},
																map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Dream FM"}}}}},
															},
														},
													},
												},
											},
										},
										map[string]any{
											"musicCarouselShelfRenderer": map[string]any{
												"header": map[string]any{
													"musicCarouselShelfBasicHeaderRenderer": map[string]any{
														"title": map[string]any{"runs": []any{map[string]any{"text": "Featured playlists"}}},
													},
												},
												"contents": []any{
													map[string]any{
														"musicTwoRowItemRenderer": map[string]any{
															"title":              map[string]any{"runs": []any{map[string]any{"text": "Late Night"}}},
															"subtitle":           map[string]any{"runs": []any{map[string]any{"text": "Dream FM"}}},
															"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "VLPLfeedface"}},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	shelves := parseHomeShelves(data, 10, 10)
	if len(shelves) != 2 {
		t.Fatalf("expected two shelves, got %d", len(shelves))
	}
	if shelves[0].Kind != ShelfTracks || shelves[0].Title != "Quick picks" || len(shelves[0].Tracks) != 1 {
		t.Fatalf("unexpected track shelf: %#v", shelves[0])
	}
	if shelves[1].Kind != ShelfPlaylists || shelves[1].Title != "Featured playlists" || len(shelves[1].Playlists) != 1 {
		t.Fatalf("unexpected playlist shelf: %#v", shelves[1])
	}
}

func TestParseHomeShelvesSupportsCategoriesPodcastsAndContinuation(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"singleColumnBrowseResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"continuations": []any{
										map[string]any{"nextContinuationData": map[string]any{"continuation": "next-token"}},
									},
									"contents": []any{
										map[string]any{
											"gridRenderer": map[string]any{
												"header": map[string]any{
													"gridHeaderRenderer": map[string]any{
														"title": map[string]any{"runs": []any{map[string]any{"text": "Moods"}}},
													},
												},
												"items": []any{
													map[string]any{
														"musicNavigationButtonRenderer": map[string]any{
															"buttonText": map[string]any{"runs": []any{map[string]any{"text": "Focus"}}},
															"clickCommand": map[string]any{
																"browseEndpoint": map[string]any{
																	"browseId": "FEmusic_moods_and_genres_category",
																	"params":   "ggMPOg1uX1JlbGF4YXRpb24%3D",
																},
															},
															"solid": map[string]any{"leftStripeColor": 0xff336699},
														},
													},
												},
											},
										},
										map[string]any{
											"musicCarouselShelfRenderer": map[string]any{
												"header": map[string]any{
													"musicCarouselShelfBasicHeaderRenderer": map[string]any{
														"title": map[string]any{"runs": []any{map[string]any{"text": "Podcasts"}}},
													},
												},
												"contents": []any{
													map[string]any{
														"musicTwoRowItemRenderer": map[string]any{
															"title":              map[string]any{"runs": []any{map[string]any{"text": "Night Talks"}}},
															"subtitle":           map[string]any{"runs": []any{map[string]any{"text": "Dream FM"}}},
															"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "MPSPPpodcast"}},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	shelves := parseHomeShelves(data, 10, 10)
	if len(shelves) != 2 {
		t.Fatalf("expected two shelves, got %d", len(shelves))
	}
	if shelves[0].Kind != ShelfCategories || len(shelves[0].Categories) != 1 || shelves[0].Categories[0].ColorHex != "#336699" {
		t.Fatalf("unexpected category shelf: %#v", shelves[0])
	}
	if shelves[1].Kind != ShelfPodcasts || len(shelves[1].Podcasts) != 1 || shelves[1].Podcasts[0].ID != "MPSPPpodcast" {
		t.Fatalf("unexpected podcast shelf: %#v", shelves[1])
	}
	if token := extractBrowseContinuationToken(data); token != "next-token" {
		t.Fatalf("unexpected continuation token: %q", token)
	}
}

func TestParseHomeShelvesSupportsArtistShelves(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"singleColumnBrowseResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"contents": []any{
										map[string]any{
											"musicCarouselShelfRenderer": map[string]any{
												"header": map[string]any{
													"musicCarouselShelfBasicHeaderRenderer": map[string]any{
														"title": map[string]any{"runs": []any{map[string]any{"text": "Top artists"}}},
													},
												},
												"contents": []any{
													map[string]any{
														"musicTwoRowItemRenderer": map[string]any{
															"title":              map[string]any{"runs": []any{map[string]any{"text": "Super Lofi World"}}},
															"subtitle":           map[string]any{"runs": []any{map[string]any{"text": "Artist"}}},
															"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UCsuperlofi"}},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	shelves := parseHomeShelves(data, 10, 10)
	if len(shelves) != 1 || shelves[0].Kind != ShelfArtists || len(shelves[0].Artists) != 1 {
		t.Fatalf("unexpected artist shelves: %#v", shelves)
	}
	if shelves[0].Artists[0].ID != "UCsuperlofi" || shelves[0].Artists[0].Name != "Super Lofi World" {
		t.Fatalf("unexpected artist shelf item: %#v", shelves[0].Artists[0])
	}
}

func TestParseHomeShelvesCollectsAllSingleColumnTabs(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"singleColumnBrowseResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"contents": []any{
										map[string]any{
											"musicShelfRenderer": map[string]any{
												"title": map[string]any{"runs": []any{map[string]any{"text": "Video charts"}}},
												"contents": []any{
													map[string]any{
														"musicResponsiveListItemRenderer": map[string]any{
															"playlistItemData": map[string]any{"videoId": "VideoChart1"},
															"flexColumns": []any{
																map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Video Chart"}}}}},
																map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Dream FM"}}}}},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"contents": []any{
										map[string]any{
											"musicShelfRenderer": map[string]any{
												"title": map[string]any{"runs": []any{map[string]any{"text": "Top songs"}}},
												"contents": []any{
													map[string]any{
														"musicResponsiveListItemRenderer": map[string]any{
															"playlistItemData": map[string]any{"videoId": "TopSong0001"},
															"flexColumns": []any{
																map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Top Song"}}}}},
																map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Dream FM"}}}}},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	shelves := parseHomeShelves(data, 10, 10)
	if len(shelves) != 2 {
		t.Fatalf("expected shelves from both tabs, got %d: %#v", len(shelves), shelves)
	}
	if shelves[0].Title != "Video charts" || shelves[1].Title != "Top songs" {
		t.Fatalf("unexpected shelf titles: %#v", shelves)
	}
}

func TestExtractBrowseTabs(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"singleColumnBrowseResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"title":    "Videos",
							"selected": true,
							"endpoint": map[string]any{"browseEndpoint": map[string]any{
								"browseId": "FEmusic_charts",
							}},
						},
					},
					map[string]any{
						"tabRenderer": map[string]any{
							"title": "Top songs",
							"endpoint": map[string]any{"browseEndpoint": map[string]any{
								"browseId": "FEmusic_charts",
								"params":   "songs",
							}},
						},
					},
				},
			},
		},
	}

	tabs := extractBrowseTabs(data)
	if len(tabs) != 2 {
		t.Fatalf("expected two tabs, got %d: %#v", len(tabs), tabs)
	}
	if !tabs[0].Selected || tabs[0].BrowseID != "FEmusic_charts" || tabs[1].Params != "songs" {
		t.Fatalf("unexpected tabs: %#v", tabs)
	}
}

func TestParseContinuationShelves(t *testing.T) {
	data := map[string]any{
		"continuationContents": map[string]any{
			"sectionListContinuation": map[string]any{
				"continuations": []any{
					map[string]any{"nextContinuationData": map[string]any{"continuation": "after-more"}},
				},
				"contents": []any{
					map[string]any{
						"musicShelfRenderer": map[string]any{
							"title": map[string]any{"runs": []any{map[string]any{"text": "More picks"}}},
							"contents": []any{
								map[string]any{
									"musicResponsiveListItemRenderer": map[string]any{
										"playlistItemData": map[string]any{"videoId": "TESTVID009I"},
										"flexColumns": []any{
											map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Night Drive"}}}}},
											map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Dream FM"}}}}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	shelves := parseHomeShelves(data, 10, 10)
	if len(shelves) != 1 || shelves[0].Kind != ShelfTracks || len(shelves[0].Tracks) != 1 {
		t.Fatalf("unexpected continuation shelves: %#v", shelves)
	}
	if token := extractBrowseContinuationToken(data); token != "after-more" {
		t.Fatalf("unexpected continuation token: %q", token)
	}
}

func TestParseQueueTracks(t *testing.T) {
	data := map[string]any{
		"queueDatas": []any{
			map[string]any{
				"content": map[string]any{
					"playlistPanelVideoRenderer": map[string]any{
						"videoId":         "TESTVID009I",
						"title":           map[string]any{"runs": []any{map[string]any{"text": "Night Drive"}}},
						"shortBylineText": map[string]any{"runs": []any{map[string]any{"text": "Dream FM"}}},
						"lengthText":      map[string]any{"runs": []any{map[string]any{"text": "4:20"}}},
						"thumbnail":       map[string]any{"thumbnails": []any{map[string]any{"url": "https://i.ytimg.com/vi/TESTVID009I/hqdefault.jpg"}}},
					},
				},
			},
		},
	}

	tracks := parseQueueTracks(data, 10)
	if len(tracks) != 1 {
		t.Fatalf("expected one queue track, got %d", len(tracks))
	}
	if tracks[0].VideoID != "TESTVID009I" || tracks[0].Channel != "Dream FM" || tracks[0].DurationLabel != "4:20" {
		t.Fatalf("unexpected queue track: %#v", tracks[0])
	}
}

func TestParseQueueTracksHandlesWrappedRenderer(t *testing.T) {
	data := map[string]any{
		"queueDatas": []any{
			map[string]any{
				"content": map[string]any{
					"playlistPanelVideoWrapperRenderer": map[string]any{
						"primaryRenderer": map[string]any{
							"playlistPanelVideoRenderer": map[string]any{
								"videoId":         "TESTVID009I",
								"title":           map[string]any{"runs": []any{map[string]any{"text": "Night Drive"}}},
								"shortBylineText": map[string]any{"runs": []any{map[string]any{"text": "Dream FM"}}},
								"lengthText":      map[string]any{"runs": []any{map[string]any{"text": "4:20"}}},
							},
						},
					},
				},
			},
		},
	}

	tracks := parseQueueTracks(data, 10)
	if len(tracks) != 1 {
		t.Fatalf("expected one queue track, got %d", len(tracks))
	}
	if tracks[0].VideoID != "TESTVID009I" || tracks[0].Title != "Night Drive" {
		t.Fatalf("unexpected queue track: %#v", tracks[0])
	}
}

func TestArtistHeaderIgnoresSubscriptionTextForTitle(t *testing.T) {
	data := map[string]any{
		"header": map[string]any{
			"musicImmersiveHeaderRenderer": map[string]any{
				"title": map[string]any{"runs": []any{map[string]any{"text": "Super Lofi World"}}},
				"monthlyListenerCount": map[string]any{"runs": []any{
					map[string]any{"text": "1.2M monthly listeners"},
				}},
				"subscriptionButton": map[string]any{
					"subscribeButtonRenderer": map[string]any{
						"channelId":  "UCsuperlofi",
						"subscribed": true,
						"subscribedButtonText": map[string]any{"runs": []any{
							map[string]any{"text": "Unsubscribe"},
						}},
					},
				},
				"startRadioButton": map[string]any{
					"buttonRenderer": map[string]any{
						"navigationEndpoint": map[string]any{
							"watchPlaylistEndpoint": map[string]any{
								"playlistId": "RDARTISTsuperlofi",
							},
						},
					},
				},
			},
		},
	}

	header := artistHeaderFromBrowseData(data, "UCsuperlofi")
	if header.Title != "Super Lofi World" {
		t.Fatalf("unexpected artist title: %#v", header)
	}
	if header.Subtitle != "1.2M monthly listeners" || header.ChannelID != "UCsuperlofi" || !header.IsSubscribed || header.MixPlaylistID != "RDARTISTsuperlofi" {
		t.Fatalf("unexpected artist header: %#v", header)
	}
}

func TestParseTrackMetadata(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"singleColumnMusicWatchNextResultsRenderer": map[string]any{
				"tabbedRenderer": map[string]any{
					"watchNextTabbedResultsRenderer": map[string]any{
						"tabs": []any{
							map[string]any{
								"tabRenderer": map[string]any{
									"content": map[string]any{
										"musicQueueRenderer": map[string]any{
											"content": map[string]any{
												"playlistPanelRenderer": map[string]any{
													"contents": []any{
														map[string]any{
															"playlistPanelVideoRenderer": map[string]any{
																"videoId": "TESTVID007G",
																"title":   map[string]any{"runs": []any{map[string]any{"text": "Never Gonna Give You Up"}}},
																"navigationEndpoint": map[string]any{"watchEndpoint": map[string]any{
																	"watchEndpointMusicSupportedConfigs": map[string]any{
																		"watchEndpointMusicConfig": map[string]any{
																			"musicVideoType": "MUSIC_VIDEO_TYPE_OMV",
																		},
																	},
																}},
																"longBylineText": map[string]any{"runs": []any{map[string]any{
																	"text": "Rick Astley",
																	"navigationEndpoint": map[string]any{
																		"browseEndpoint": map[string]any{"browseId": "UCuAXFkgsw1L7xaCfnd5JJOw"},
																	},
																}}},
																"lengthText": map[string]any{"runs": []any{map[string]any{"text": "3:33"}}},
																"thumbnail":  map[string]any{"thumbnails": []any{map[string]any{"url": "https://i.ytimg.com/vi/TESTVID007G/hqdefault.jpg"}}},
																"menu": map[string]any{
																	"menuRenderer": map[string]any{
																		"topLevelButtons": []any{
																			map[string]any{
																				"likeButtonRenderer": map[string]any{
																					"likeStatus": "LIKE",
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	metadata := parseTrackMetadata(data, "TESTVID007G")
	if metadata.VideoID != "TESTVID007G" {
		t.Fatalf("unexpected metadata video id: %#v", metadata)
	}
	if metadata.Title != "Never Gonna Give You Up" {
		t.Fatalf("unexpected metadata title: %#v", metadata)
	}
	if metadata.Channel != "Rick Astley" || metadata.ArtistBrowseID != "UCuAXFkgsw1L7xaCfnd5JJOw" {
		t.Fatalf("unexpected metadata channel: %#v", metadata)
	}
	if metadata.DurationLabel != "3:33" {
		t.Fatalf("unexpected metadata duration: %#v", metadata)
	}
	if metadata.LikeStatus != LikeStatusLike {
		t.Fatalf("unexpected like status: %#v", metadata)
	}
	if !metadata.LikeStatusKnown {
		t.Fatalf("expected like status to be known: %#v", metadata)
	}
	if metadata.ThumbnailURL != "https://i.ytimg.com/vi/TESTVID007G/hqdefault.jpg" {
		t.Fatalf("unexpected thumbnail: %#v", metadata)
	}
	if metadata.MusicVideoType != "MUSIC_VIDEO_TYPE_OMV" {
		t.Fatalf("unexpected music video type: %#v", metadata)
	}
}

func TestParseTrackMetadataReportsUnknownLikeStatus(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"singleColumnMusicWatchNextResultsRenderer": map[string]any{
				"tabbedRenderer": map[string]any{
					"watchNextTabbedResultsRenderer": map[string]any{
						"tabs": []any{
							map[string]any{"tabRenderer": map[string]any{"content": map[string]any{"musicQueueRenderer": map[string]any{"content": map[string]any{"playlistPanelRenderer": map[string]any{"contents": []any{
								map[string]any{"playlistPanelVideoRenderer": map[string]any{
									"videoId": "TESTVID007G",
								}},
							}}}}}}},
						},
					},
				},
			},
		},
	}

	metadata := parseTrackMetadata(data, "TESTVID007G")
	if metadata.LikeStatusKnown {
		t.Fatalf("expected unknown like status: %#v", metadata)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func testHTTPResponse(request *http.Request, statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    request,
	}
}

func TestPlaylistQueueFallsBackToBrowseWhenQueueEndpointFails(t *testing.T) {
	var paths []string
	var browseRequestBody string
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.URL.Path)
		rawBody, _ := io.ReadAll(request.Body)
		switch request.URL.Path {
		case "/youtubei/v1/music/get_queue":
			if !strings.Contains(string(rawBody), `"playlistId":"PLfallback123"`) {
				t.Fatalf("unexpected get_queue body: %s", string(rawBody))
			}
			return testHTTPResponse(request, http.StatusInternalServerError, `queue unavailable`), nil
		case "/youtubei/v1/browse":
			browseRequestBody = string(rawBody)
			return testHTTPResponse(request, http.StatusOK, `{"contents":{"singleColumnBrowseResultsRenderer":{"tabs":[{"tabRenderer":{"content":{"sectionListRenderer":{"contents":[{"musicShelfRenderer":{"title":{"runs":[{"text":"Album songs"}]},"contents":[{"musicResponsiveListItemRenderer":{"playlistItemData":{"videoId":"AbCdEfGhI12"},"flexColumns":[{"musicResponsiveListItemFlexColumnRenderer":{"text":{"runs":[{"text":"Fallback Track"}]}}},{"musicResponsiveListItemFlexColumnRenderer":{"text":{"runs":[{"text":"Fallback Artist","navigationEndpoint":{"browseEndpoint":{"browseId":"UCfallbackartist"}}}]}}}],"fixedColumns":[{"musicResponsiveListItemFixedColumnRenderer":{"text":{"runs":[{"text":"3:45"}]}}}]}}]}}]}}}}]}}}}`), nil
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	tracks, err := client.PlaylistQueue(context.Background(), "VLPLfallback123", 10)
	if err != nil {
		t.Fatalf("playlist queue: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected one fallback track, got %d", len(tracks))
	}
	if tracks[0].VideoID != "AbCdEfGhI12" || tracks[0].Title != "Fallback Track" || tracks[0].Channel != "Fallback Artist" || tracks[0].ArtistBrowseID != "UCfallbackartist" || tracks[0].DurationLabel != "3:45" {
		t.Fatalf("unexpected fallback track: %#v", tracks[0])
	}
	if strings.Join(paths, ",") != "/youtubei/v1/music/get_queue,/youtubei/v1/browse" {
		t.Fatalf("unexpected request sequence: %#v", paths)
	}
	if !strings.Contains(browseRequestBody, `"browseId":"VLPLfallback123"`) {
		t.Fatalf("unexpected browse body: %s", browseRequestBody)
	}
}

func TestPlaylistQueueUsesBrowseForAlbumIDs(t *testing.T) {
	for _, albumID := range []string{"MPREalbum123", "OLAKalbum123"} {
		t.Run(albumID, func(t *testing.T) {
			var paths []string
			var browseRequestBody string
			client := NewClient(fakeCookieProvider{records: []appcookies.Record{
				{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
			}})
			client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
			client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				paths = append(paths, request.URL.Path)
				rawBody, _ := io.ReadAll(request.Body)
				switch request.URL.Path {
				case "/youtubei/v1/browse":
					browseRequestBody = string(rawBody)
					return testHTTPResponse(request, http.StatusOK, `{"contents":{"twoColumnBrowseResultsRenderer":{"secondaryContents":{"sectionListRenderer":{"contents":[{"musicPlaylistShelfRenderer":{"title":{"runs":[{"text":"Album songs"}]},"contents":[{"musicResponsiveListItemRenderer":{"playlistItemData":{"videoId":"AbCdEfGhI12"},"flexColumns":[{"musicResponsiveListItemFlexColumnRenderer":{"text":{"runs":[{"text":"Album Track"}]}}},{"musicResponsiveListItemFlexColumnRenderer":{"text":{"runs":[{"text":"Album Artist","navigationEndpoint":{"browseEndpoint":{"browseId":"UCalbumartist"}}}]}}}],"fixedColumns":[{"musicResponsiveListItemFixedColumnRenderer":{"text":{"runs":[{"text":"4:05"}]}}}]}}]}}]}}}}}`), nil
				default:
					t.Fatalf("unexpected request path: %s", request.URL.Path)
					return nil, nil
				}
			})}

			tracks, err := client.PlaylistQueue(context.Background(), albumID, 10)
			if err != nil {
				t.Fatalf("album queue: %v", err)
			}
			if len(tracks) != 1 {
				t.Fatalf("expected one album track, got %d", len(tracks))
			}
			if tracks[0].VideoID != "AbCdEfGhI12" || tracks[0].Title != "Album Track" || tracks[0].Channel != "Album Artist" || tracks[0].ArtistBrowseID != "UCalbumartist" || tracks[0].DurationLabel != "4:05" {
				t.Fatalf("unexpected album track: %#v", tracks[0])
			}
			if strings.Join(paths, ",") != "/youtubei/v1/browse" {
				t.Fatalf("unexpected request sequence: %#v", paths)
			}
			if !strings.Contains(browseRequestBody, `"browseId":"`+albumID+`"`) {
				t.Fatalf("unexpected browse body: %s", browseRequestBody)
			}
		})
	}
}

func TestAlbumBrowseTracksIgnoreNonArtistMetadataAsArtist(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"twoColumnBrowseResultsRenderer": map[string]any{
				"secondaryContents": map[string]any{
					"sectionListRenderer": map[string]any{
						"contents": []any{
							map[string]any{
								"musicPlaylistShelfRenderer": map[string]any{
									"title": map[string]any{"runs": []any{map[string]any{"text": "Album songs"}}},
									"contents": []any{
										map[string]any{
											"musicResponsiveListItemRenderer": map[string]any{
												"playlistItemData": map[string]any{"videoId": "AbCdEfGhI12"},
												"flexColumns": []any{
													map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Album Track"}}}}},
													map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "1.2M plays"}}}}},
												},
												"fixedColumns": []any{
													map[string]any{"musicResponsiveListItemFixedColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "4:05"}}}}},
												},
											},
										},
										map[string]any{
											"musicResponsiveListItemRenderer": map[string]any{
												"playlistItemData": map[string]any{"videoId": "BcDeFgHiJ34"},
												"flexColumns": []any{
													map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Localized Track"}}}}},
													map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "8,765 次播放"}}}}},
												},
												"fixedColumns": []any{
													map[string]any{"musicResponsiveListItemFixedColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "3:21"}}}}},
												},
											},
										},
										map[string]any{
											"musicResponsiveListItemRenderer": map[string]any{
												"playlistItemData": map[string]any{"videoId": "CdEfGhIjK56"},
												"flexColumns": []any{
													map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Album Link Track"}}}}},
													map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{
														"text":               "Album Name",
														"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "MPREalbum123"}},
													}}}}},
												},
												"fixedColumns": []any{
													map[string]any{"musicResponsiveListItemFixedColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "2:58"}}}}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	shelves := parseHomeShelves(data, 10, 10)
	tracks := tracksFromShelves(shelves, 10)
	if len(tracks) != 3 {
		t.Fatalf("expected three album tracks, got %d: %#v", len(tracks), tracks)
	}
	for _, track := range tracks {
		if track.Channel != "" {
			t.Fatalf("expected non-artist metadata to leave artist empty, got %#v", track)
		}
	}
	if tracks[0].PlayCountLabel != "1.2M plays" {
		t.Fatalf("expected English play count label, got %#v", tracks[0])
	}
	if tracks[1].PlayCountLabel != "8,765 次播放" {
		t.Fatalf("expected localized play count label, got %#v", tracks[1])
	}
	if tracks[2].PlayCountLabel != "" {
		t.Fatalf("expected album metadata not to become play count, got %#v", tracks[2])
	}
}

func TestAlbumBrowseTrackArtistMatchesKasetFlexColumnRules(t *testing.T) {
	track, ok := trackFromMusicResponsiveRenderer(map[string]any{
		"playlistItemData": map[string]any{"videoId": "AbCdEfGhI12"},
		"flexColumns": []any{
			map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{
				map[string]any{"text": "Album Track"},
			}}}},
			map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{
				map[string]any{"text": "Song"},
				map[string]any{"text": " • "},
				map[string]any{
					"text":               "Album Artist",
					"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UCalbumartist"}},
				},
				map[string]any{"text": " & "},
				map[string]any{
					"text":               "Featured Artist",
					"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "MPLAUCfeaturedartist"}},
				},
				map[string]any{"text": " • "},
				map[string]any{
					"text":               "Album Name",
					"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "MPREalbum123"}},
				},
			}}}},
		},
		"fixedColumns": []any{
			map[string]any{"musicResponsiveListItemFixedColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "4:05"}}}}},
		},
	})
	if !ok {
		t.Fatal("expected track")
	}
	if track.Channel != "Album Artist, Featured Artist" || track.ArtistBrowseID != "UCalbumartist" {
		t.Fatalf("unexpected artist: %#v", track)
	}
}

func TestPlaylistHeaderFromBrowseDataMatchesKasetAuthorRules(t *testing.T) {
	data := map[string]any{
		"header": map[string]any{
			"musicDetailHeaderRenderer": map[string]any{
				"title": map[string]any{"runs": []any{map[string]any{"text": "Midnight Album"}}},
				"subtitle": map[string]any{"runs": []any{
					map[string]any{"text": "Album Artist"},
					map[string]any{"text": " • "},
					map[string]any{"text": "Album"},
					map[string]any{"text": " • "},
					map[string]any{"text": "10 songs"},
				}},
			},
		},
	}

	header := playlistHeaderFromBrowseData(data)
	if header.Title != "Midnight Album" || header.Author != "Album Artist" {
		t.Fatalf("unexpected playlist header: %#v", header)
	}
}

func TestPlaylistHeaderFromBrowseDataUsesResponsiveFacepileAuthor(t *testing.T) {
	data := map[string]any{
		"contents": map[string]any{
			"twoColumnBrowseResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"contents": []any{
										map[string]any{
											"musicResponsiveHeaderRenderer": map[string]any{
												"title": map[string]any{"runs": []any{map[string]any{"text": "Responsive Album"}}},
												"facepile": map[string]any{"avatarStackViewModel": map[string]any{
													"text": map[string]any{"content": "Facepile Artist"},
												}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	header := playlistHeaderFromBrowseData(data)
	if header.Title != "Responsive Album" || header.Author != "Facepile Artist" {
		t.Fatalf("unexpected responsive playlist header: %#v", header)
	}
}

func TestSubscribePlaylistCallsLikeEndpoint(t *testing.T) {
	var requestPath string
	var requestBody string
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestPath = request.URL.Path
		rawBody, _ := io.ReadAll(request.Body)
		requestBody = string(rawBody)
		return testHTTPResponse(request, http.StatusOK, `{}`), nil
	})}

	if err := client.SubscribePlaylist(context.Background(), "VLPL1234567890"); err != nil {
		t.Fatalf("subscribe playlist: %v", err)
	}
	if requestPath != "/youtubei/v1/like/like" {
		t.Fatalf("unexpected request path: %q", requestPath)
	}
	if !strings.Contains(requestBody, `"playlistId":"PL1234567890"`) {
		t.Fatalf("unexpected request body: %s", requestBody)
	}
}

func TestUnsubscribePlaylistCallsRemoveLikeEndpoint(t *testing.T) {
	var requestPath string
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestPath = request.URL.Path
		return testHTTPResponse(request, http.StatusOK, `{}`), nil
	})}

	if err := client.UnsubscribePlaylist(context.Background(), "VLPL1234567890"); err != nil {
		t.Fatalf("unsubscribe playlist: %v", err)
	}
	if requestPath != "/youtubei/v1/like/removelike" {
		t.Fatalf("unexpected request path: %q", requestPath)
	}
}

func TestSubscribeArtistCallsSubscriptionEndpoint(t *testing.T) {
	var requestPath string
	var requestBody string
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestPath = request.URL.Path
		rawBody, _ := io.ReadAll(request.Body)
		requestBody = string(rawBody)
		return testHTTPResponse(request, http.StatusOK, `{}`), nil
	})}

	if err := client.SubscribeArtist(context.Background(), "UCsuperlofi"); err != nil {
		t.Fatalf("subscribe artist: %v", err)
	}
	if requestPath != "/youtubei/v1/subscription/subscribe" {
		t.Fatalf("unexpected request path: %q", requestPath)
	}
	if !strings.Contains(requestBody, `"channelIds":["UCsuperlofi"]`) {
		t.Fatalf("unexpected request body: %s", requestBody)
	}
}

func TestRateSongCallsLikeEndpoint(t *testing.T) {
	var requestPath string
	var requestBody string
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestPath = request.URL.Path
		rawBody, _ := io.ReadAll(request.Body)
		requestBody = string(rawBody)
		return testHTTPResponse(request, http.StatusOK, `{}`), nil
	})}

	if err := client.RateSong(context.Background(), "TESTVID007G", LikeStatusLike); err != nil {
		t.Fatalf("rate song: %v", err)
	}
	if requestPath != "/youtubei/v1/like/like" {
		t.Fatalf("unexpected request path: %q", requestPath)
	}
	if !strings.Contains(requestBody, `"videoId":"TESTVID007G"`) {
		t.Fatalf("unexpected request body: %s", requestBody)
	}
}

func TestRateSongCallsRemoveLikeEndpointForIndifferent(t *testing.T) {
	var requestPath string
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestPath = request.URL.Path
		return testHTTPResponse(request, http.StatusOK, `{}`), nil
	})}

	if err := client.RateSong(context.Background(), "TESTVID007G", LikeStatusIndifferent); err != nil {
		t.Fatalf("remove song like: %v", err)
	}
	if requestPath != "/youtubei/v1/like/removelike" {
		t.Fatalf("unexpected request path: %q", requestPath)
	}
}

func TestExtractTimedLyrics(t *testing.T) {
	result := extractTimedLyrics(map[string]any{
		"contents": map[string]any{
			"nested": []any{
				map[string]any{
					"timedLyricsModel": map[string]any{
						"lyricsData": []any{
							map[string]any{
								"lyricLine":   "First line",
								"startTimeMs": "1200",
								"durationMs":  "3000",
							},
							map[string]any{
								"lyricLine":   "Second line",
								"startTimeMs": "4200",
							},
						},
					},
				},
			},
		},
	}, "YTMusic")

	if result.Kind != lyricsResultSynced || result.Source != "YTMusic" || len(result.Lines) != 2 {
		t.Fatalf("expected synced lyrics, got %+v", result)
	}
	if result.Lines[0].StartMs != 1200 || result.Lines[0].DurationMs != 3000 || result.Lines[0].Text != "First line" {
		t.Fatalf("unexpected first line: %+v", result.Lines[0])
	}
	if result.Lines[1].DurationMs != 5000 {
		t.Fatalf("expected fallback duration on last line, got %+v", result.Lines[1])
	}
}

func TestParseLRCLines(t *testing.T) {
	lines := parseLRCLines("[offset:100]\n[00:01.20]First\n[00:03.50]<00:03.50>Sec<00:04.00>ond")
	if len(lines) != 3 {
		t.Fatalf("expected leading spacer and two lyric lines, got %+v", lines)
	}
	if lines[0].StartMs != 0 || lines[0].DurationMs != 1100 {
		t.Fatalf("unexpected spacer: %+v", lines[0])
	}
	if lines[1].StartMs != 1100 || lines[1].Text != "First" || lines[1].DurationMs != 2300 {
		t.Fatalf("unexpected first lyric line: %+v", lines[1])
	}
	if lines[2].Text != "Second" || len(lines[2].Words) != 2 {
		t.Fatalf("expected word timing to be parsed, got %+v", lines[2])
	}
}

func TestBestLRCLibModelPrefersSyncedLyrics(t *testing.T) {
	plainDuration := 213.0
	syncedDuration := 260.0
	model, ok := bestLRCLibModel([]lrcLibModel{
		{
			ID:          1,
			Duration:    &plainDuration,
			PlainLyrics: "Plain",
		},
		{
			ID:           2,
			Duration:     &syncedDuration,
			SyncedLyrics: "[00:01.00]Synced",
		},
	}, plainDuration)
	if !ok {
		t.Fatalf("expected a lyric model")
	}
	if model.ID != 2 {
		t.Fatalf("expected synced model to win over closer plain model, got %+v", model)
	}
}

func TestTrackLyricsPrefersLRCLibSyncedOverYouTubePlain(t *testing.T) {
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body string
		switch request.URL.Path {
		case "/youtubei/v1/next":
			body = `{"contents":{"singleColumnMusicWatchNextResultsRenderer":{"tabbedRenderer":{"watchNextTabbedResultsRenderer":{"tabs":[{"tabRenderer":{"endpoint":{"browseEndpoint":{"browseId":"MPLYt_lyrics"}}}}]}}}}}`
		case "/youtubei/v1/browse":
			body = `{"contents":{"sectionListRenderer":{"contents":[{"musicDescriptionShelfRenderer":{"description":{"runs":[{"text":"Official plain lyrics"}]}}}]}}}`
		case "/api/search":
			body = `[{"id":1,"trackName":"Track","artistName":"Artist","duration":213,"plainLyrics":"Plain only"},{"id":2,"trackName":"Track","artistName":"Artist","duration":260,"syncedLyrics":"[00:01.00]Synced line"}]`
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		return testHTTPResponse(request, http.StatusOK, body), nil
	})}

	result, err := client.TrackLyrics(context.Background(), LyricsSearchInfo{
		VideoID:         "TESTVID007G",
		Title:           "Track",
		Artist:          "Artist",
		DurationSeconds: 213,
	})
	if err != nil {
		t.Fatalf("track lyrics: %v", err)
	}
	if result.Kind != lyricsResultSynced || result.Source != "LRCLib" {
		t.Fatalf("expected LRCLib synced lyrics, got %+v", result)
	}
	if len(result.Lines) != 2 || result.Lines[1].Text != "Synced line" {
		t.Fatalf("unexpected synced lines: %+v", result.Lines)
	}
}

func TestTrackLyricsSearchesLRCLibWithoutVideoID(t *testing.T) {
	client := NewClient(fakeCookieProvider{})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/search" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		if got := request.URL.Query().Get("track_name"); got != "Track" {
			t.Fatalf("unexpected track_name query: %q", got)
		}
		if got := request.URL.Query().Get("artist_name"); got != "Artist" {
			t.Fatalf("unexpected artist_name query: %q", got)
		}
		body := `[{"id":2,"trackName":"Track","artistName":"Artist","duration":213,"syncedLyrics":"[00:01.00]Synced line"}]`
		return testHTTPResponse(request, http.StatusOK, body), nil
	})}

	result, err := client.TrackLyrics(context.Background(), LyricsSearchInfo{
		Title:           "Track",
		Artist:          "Artist",
		DurationSeconds: 213,
	})
	if err != nil {
		t.Fatalf("track lyrics: %v", err)
	}
	if result.Kind != lyricsResultSynced || result.Source != "LRCLib" {
		t.Fatalf("expected LRCLib synced lyrics, got %+v", result)
	}
	if len(result.Lines) != 2 || result.Lines[1].Text != "Synced line" {
		t.Fatalf("unexpected synced lines: %+v", result.Lines)
	}
}

func TestTrackLyricsRetriesTransientYouTubeMusicRequest(t *testing.T) {
	previousDelays := lyricsRequestRetryDelays
	lyricsRequestRetryDelays = []time.Duration{0, 0}
	defer func() { lyricsRequestRetryDelays = previousDelays }()

	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	nextCalls := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/youtubei/v1/next" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		nextCalls++
		if nextCalls == 1 {
			return nil, io.EOF
		}
		body := `{"contents":{"nested":[{"timedLyricsModel":{"lyricsData":[{"lyricLine":"Recovered line","startTimeMs":"1200","durationMs":"3000"}]}}]}}`
		return testHTTPResponse(request, http.StatusOK, body), nil
	})}

	result, err := client.TrackLyrics(context.Background(), LyricsSearchInfo{
		VideoID: "TESTVID007G",
		Title:   "Track",
		Artist:  "Artist",
	})
	if err != nil {
		t.Fatalf("track lyrics: %v", err)
	}
	if nextCalls != 2 {
		t.Fatalf("expected request retry, got %d calls", nextCalls)
	}
	if result.Kind != lyricsResultSynced || result.Source != "YTMusic" || len(result.Lines) != 1 || result.Lines[0].Text != "Recovered line" {
		t.Fatalf("unexpected lyrics result: %+v", result)
	}
}
