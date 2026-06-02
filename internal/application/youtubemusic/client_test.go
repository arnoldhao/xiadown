package youtubemusic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/lyricsromanization"
	"xiadown/internal/domain/appsessions"
)

type fakeCookieProvider struct {
	records []appcookies.Record
	err     error
}

func (provider fakeCookieProvider) RecordsForSiteKey(context.Context, string) ([]appcookies.Record, error) {
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
	client := NewClient(fakeCookieProvider{err: appsessions.ErrNoCookies})

	_, err := client.authHeaders(context.Background())
	if !errors.Is(err, ErrNotAuthenticated) || !errors.Is(err, appsessions.ErrNoCookies) {
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

func TestRequestUsesLocaleFromContext(t *testing.T) {
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	var requestBody map[string]any
	var acceptLanguage string
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		acceptLanguage = request.Header.Get("Accept-Language")
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return testHTTPResponse(request, http.StatusOK, `{}`), nil
	})}

	if _, err := client.SearchSongs(WithLocale(context.Background(), "zh-CN"), "lofi", 1); err != nil {
		t.Fatalf("request with locale: %v", err)
	}
	contextPayload, _ := requestBody["context"].(map[string]any)
	clientPayload, _ := contextPayload["client"].(map[string]any)
	if got := clientPayload["hl"]; got != "zh-CN" {
		t.Fatalf("expected zh-CN hl, got %#v", got)
	}
	if !strings.HasPrefix(acceptLanguage, "zh-CN") {
		t.Fatalf("expected zh-CN accept language, got %q", acceptLanguage)
	}
}

func TestRequestUsesConfiguredUserAgentInHeadersAndContext(t *testing.T) {
	const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36 Edg/124.0.0.0"
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.SetUserAgent(userAgent)
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	var requestBody map[string]any
	var headerUserAgent string
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		headerUserAgent = request.Header.Get("User-Agent")
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return testHTTPResponse(request, http.StatusOK, `{}`), nil
	})}

	if _, err := client.AccountInfo(context.Background()); err != nil {
		t.Fatalf("request with user agent: %v", err)
	}
	if headerUserAgent != userAgent {
		t.Fatalf("expected header user agent %q, got %q", userAgent, headerUserAgent)
	}
	contextPayload, _ := requestBody["context"].(map[string]any)
	clientPayload, _ := contextPayload["client"].(map[string]any)
	if got := clientPayload["userAgent"]; got != userAgent {
		t.Fatalf("expected context user agent %q, got %#v", userAgent, got)
	}
	if got := clientPayload["browserName"]; got != "Edge" {
		t.Fatalf("expected Edge browser name, got %#v", got)
	}
	if got := clientPayload["osName"]; got != "Windows" {
		t.Fatalf("expected Windows os name, got %#v", got)
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

func TestListTrackRendererPromotesAudioTrackMusicVideoType(t *testing.T) {
	track, ok := trackFromMusicResponsiveRenderer(map[string]any{
		"playlistItemData": map[string]any{"videoId": "TESTVID007G"},
		"navigationEndpoint": map[string]any{"watchEndpoint": map[string]any{
			"watchEndpointMusicSupportedConfigs": map[string]any{
				"watchEndpointMusicConfig": map[string]any{
					"musicVideoType": "MUSIC_VIDEO_TYPE_ATV",
				},
			},
		}},
		"flexColumns": []any{
			map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Lofi Mix"}}}}},
			map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{
				map[string]any{"text": "Super Lofi World"},
			}}}},
		},
	})
	if !ok {
		t.Fatal("expected track")
	}
	if track.MusicVideoType != "MUSIC_VIDEO_TYPE_ATV" {
		t.Fatalf("expected ATV music video type, got %q", track.MusicVideoType)
	}
}

func TestListTrackRendererPromotesOfficialVideoMusicVideoType(t *testing.T) {
	track, ok := trackFromMusicResponsiveRenderer(map[string]any{
		"playlistItemData": map[string]any{"videoId": "TESTVID007G"},
		"navigationEndpoint": map[string]any{"watchEndpoint": map[string]any{
			"watchEndpointMusicSupportedConfigs": map[string]any{
				"watchEndpointMusicConfig": map[string]any{
					"musicVideoType": "MUSIC_VIDEO_TYPE_OMV",
				},
			},
		}},
		"flexColumns": []any{
			map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Fan Upload"}}}}},
			map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{
				map[string]any{"text": "Uploader"},
			}}}},
		},
	})
	if !ok {
		t.Fatal("expected track")
	}
	if track.MusicVideoType != "MUSIC_VIDEO_TYPE_OMV" {
		t.Fatalf("expected OMV video type, got %q", track.MusicVideoType)
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

func TestParseAccountInfoPrefersSelectedAccount(t *testing.T) {
	account := parseAccountInfo(accountsListResponse(
		accountItemWithHandle("Personal Account", "@personal", "//lh3.googleusercontent.com/personal", false),
		accountItemWithHandle("Selected Channel", "@selected", "//lh3.googleusercontent.com/selected", true),
	))
	if account.DisplayName != "Selected Channel" {
		t.Fatalf("unexpected account name: %q", account.DisplayName)
	}
	if account.Handle != "@selected" {
		t.Fatalf("unexpected account handle: %q", account.Handle)
	}
	if account.AvatarURL != "https://lh3.googleusercontent.com/selected" {
		t.Fatalf("unexpected account avatar: %q", account.AvatarURL)
	}
}

func TestParseAccountInfoFallsBackToFirstAccount(t *testing.T) {
	account := parseAccountInfo(accountsListResponse(
		accountItem("First Account", "//lh3.googleusercontent.com/first", false),
		accountItem("Second Account", "//lh3.googleusercontent.com/second", false),
	))
	if account.DisplayName != "First Account" {
		t.Fatalf("unexpected fallback account name: %q", account.DisplayName)
	}
	if account.AvatarURL != "https://lh3.googleusercontent.com/first" {
		t.Fatalf("unexpected fallback account avatar: %q", account.AvatarURL)
	}
}

func accountsListResponse(items ...map[string]any) map[string]any {
	contents := make([]any, 0, len(items))
	for _, item := range items {
		contents = append(contents, map[string]any{"accountItem": item})
	}
	return map[string]any{
		"actions": []any{
			map[string]any{
				"getMultiPageMenuAction": map[string]any{
					"menu": map[string]any{
						"multiPageMenuRenderer": map[string]any{
							"sections": []any{
								map[string]any{
									"accountSectionListRenderer": map[string]any{
										"contents": []any{
											map[string]any{
												"accountItemSectionRenderer": map[string]any{
													"contents": contents,
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
}

func accountItem(name string, avatarURL string, selected bool) map[string]any {
	return accountItemWithHandle(name, "", avatarURL, selected)
}

func accountItemWithHandle(name string, handle string, avatarURL string, selected bool) map[string]any {
	return map[string]any{
		"accountName": map[string]any{
			"runs": []any{map[string]any{"text": name}},
		},
		"channelHandle": map[string]any{
			"runs": []any{map[string]any{"text": handle}},
		},
		"accountPhoto": map[string]any{
			"thumbnails": []any{
				map[string]any{"url": "//lh3.googleusercontent.com/small"},
				map[string]any{"url": avatarURL},
			},
		},
		"isSelected": selected,
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

func TestParseSearchPlaylistsPreservesAlbumArtist(t *testing.T) {
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
											"browseId": "MPREalbum123",
											"browseEndpointContextSupportedConfigs": map[string]any{
												"browseEndpointContextMusicConfig": map[string]any{"pageType": "MUSIC_PAGE_TYPE_ALBUM"},
											},
										}},
										"flexColumns": []any{
											map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{map[string]any{"text": "Morning Album"}}}}},
											map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{
												map[string]any{"text": "Album"},
												map[string]any{"text": " • "},
												map[string]any{"text": "Dawn Artist"},
											}}}},
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

	playlists := parseSearchPlaylists(data, 10)
	if len(playlists) != 1 {
		t.Fatalf("expected one album, got %d", len(playlists))
	}
	if playlists[0].ID != "MPREalbum123" ||
		playlists[0].Title != "Morning Album" ||
		playlists[0].Channel != "Album" ||
		playlists[0].Description != "Dawn Artist" {
		t.Fatalf("unexpected album metadata: %#v", playlists[0])
	}
}

func TestPlaylistMetadataSkipsReleaseYearAsAuthor(t *testing.T) {
	channel, description := playlistMetadataFromValues([]string{
		"Album",
		" • ",
		"2009",
		" • ",
		"Dawn Artist",
	})
	if channel != "Album" || description != "Dawn Artist" {
		t.Fatalf("unexpected album metadata: channel=%q description=%q", channel, description)
	}

	channel, description = playlistMetadataFromValues([]string{
		"2009年",
		" • ",
		"Dawn Artist",
	})
	if channel != "Dawn Artist" || description != "" {
		t.Fatalf("unexpected year-first metadata: channel=%q description=%q", channel, description)
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

func TestParseRadioQueuePageExtractsNextRadioContinuation(t *testing.T) {
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
														map[string]any{"playlistPanelVideoRenderer": map[string]any{
															"videoId": "TESTVID008H",
															"title":   map[string]any{"runs": []any{map[string]any{"text": "Mix Track"}}},
														}},
													},
													"continuations": []any{
														map[string]any{"nextRadioContinuationData": map[string]any{"continuation": "next-radio-token"}},
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

	page := parseRadioQueuePage(data, 10)
	if len(page.Tracks) != 1 {
		t.Fatalf("expected one track, got %d", len(page.Tracks))
	}
	if page.Continuation != "next-radio-token" {
		t.Fatalf("unexpected continuation token: %q", page.Continuation)
	}
}

func TestParseRadioTracksMarksLinkedMultiArtistSource(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{"playlistPanelVideoRenderer": map[string]any{
				"videoId": "TESTVID008H",
				"title":   map[string]any{"runs": []any{map[string]any{"text": "Collab Track"}}},
				"longBylineText": map[string]any{"runs": []any{
					map[string]any{
						"text":               "Artist A",
						"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UCartistA"}},
					},
					map[string]any{"text": ", "},
					map[string]any{
						"text":               "Artist B",
						"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UCartistB"}},
					},
				}},
			}},
		},
	}

	tracks := parseRadioTracks(data, 10)
	if len(tracks) != 1 {
		t.Fatalf("expected one track, got %d", len(tracks))
	}
	if tracks[0].Channel != "Artist A, Artist B" || tracks[0].ArtistBrowseID != "UCartistA" {
		t.Fatalf("unexpected multi-artist metadata: %#v", tracks[0])
	}
	if tracks[0].ArtistSource != trackArtistSourceAPILinkedMultiple {
		t.Fatalf("expected multi-artist source, got %q", tracks[0].ArtistSource)
	}
	if len(tracks[0].Artists) != 2 ||
		tracks[0].Artists[0].Name != "Artist A" ||
		tracks[0].Artists[0].BrowseID != "UCartistA" ||
		tracks[0].Artists[1].Name != "Artist B" ||
		tracks[0].Artists[1].BrowseID != "UCartistB" {
		t.Fatalf("unexpected structured artists: %#v", tracks[0].Artists)
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

func TestHomeTwoRowTrackSkipsReleaseYearCreator(t *testing.T) {
	track, ok := trackFromHomeTwoRowRenderer(map[string]any{
		"title": map[string]any{"runs": []any{map[string]any{"text": "Moonlight"}}},
		"subtitle": map[string]any{"runs": []any{
			map[string]any{"text": "2009年"},
			map[string]any{"text": " • "},
			map[string]any{
				"text":               "Dreamy Producer",
				"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UCdreamy"}},
			},
			map[string]any{"text": " • "},
			map[string]any{"text": "2:49"},
		}},
		"navigationEndpoint": map[string]any{"watchEndpoint": map[string]any{"videoId": "a1b2c3d4e5F"}},
	})
	if !ok {
		t.Fatal("expected track")
	}
	if track.Channel != "Dreamy Producer" || track.ArtistBrowseID != "UCdreamy" || track.DurationLabel != "2:49" {
		t.Fatalf("unexpected two-row track: %#v", track)
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

func TestParseHomeShelvesSupportsCategoriesAndSkipsPodcasts(t *testing.T) {
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
	if len(shelves) != 1 {
		t.Fatalf("expected one shelf, got %d", len(shelves))
	}
	if shelves[0].Kind != ShelfCategories || len(shelves[0].Categories) != 1 || shelves[0].Categories[0].ColorHex != "#336699" {
		t.Fatalf("unexpected category shelf: %#v", shelves[0])
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
												"bottomEndpoint": map[string]any{"browseEndpoint": map[string]any{
													"browseId": "MPLAUCsuperlofi",
													"params":   "wAEB",
												}},
												"continuations": []any{
													map[string]any{"nextContinuationData": map[string]any{"continuation": "top-songs-more"}},
												},
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
	if shelves[1].Continuation != "top-songs-more" {
		t.Fatalf("unexpected top songs continuation: %q", shelves[1].Continuation)
	}
	if shelves[1].BrowseID != "MPLAUCsuperlofi" || shelves[1].Params != "wAEB" {
		t.Fatalf("unexpected top songs browse endpoint: %#v", shelves[1])
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
				"foregroundThumbnail": map[string]any{"musicThumbnailRenderer": map[string]any{"thumbnail": map[string]any{"thumbnails": []any{
					map[string]any{"url": "https://lh3.googleusercontent.com/artist-small"},
					map[string]any{"url": "https://lh3.googleusercontent.com/artist-large"},
				}}}},
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
	if header.Subtitle != "1.2M monthly listeners" || header.ThumbnailURL != "https://lh3.googleusercontent.com/artist-large" || header.ChannelID != "UCsuperlofi" || !header.IsSubscribed || header.MixPlaylistID != "RDARTISTsuperlofi" {
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
																"longBylineText": map[string]any{"runs": []any{
																	map[string]any{"text": "专为"},
																	map[string]any{
																		"text": "Rick Astley",
																		"navigationEndpoint": map[string]any{
																			"browseEndpoint": map[string]any{"browseId": "UCuAXFkgsw1L7xaCfnd5JJOw"},
																		},
																	},
																}},
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

func TestParseTrackMetadataUsesPlainArtistBeforeAlbumByline(t *testing.T) {
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
																"videoId": "CPONUbyJ3YM",
																"title":   map[string]any{"runs": []any{map[string]any{"text": "You are my magic"}}},
																"longBylineText": map[string]any{"runs": []any{
																	map[string]any{"text": "Accusefive"},
																	map[string]any{"text": " • "},
																	map[string]any{
																		"text": "Rose Says",
																		"navigationEndpoint": map[string]any{
																			"browseEndpoint": map[string]any{
																				"browseId": "MPREb_rWRwXVPHvuX",
																				"browseEndpointContextSupportedConfigs": map[string]any{
																					"browseEndpointContextMusicConfig": map[string]any{
																						"pageType": "MUSIC_PAGE_TYPE_ALBUM",
																					},
																				},
																			},
																		},
																	},
																	map[string]any{"text": " • "},
																	map[string]any{"text": "2022"},
																}},
																"shortBylineText": map[string]any{"runs": []any{
																	map[string]any{"text": "Accusefive"},
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
					},
				},
			},
		},
	}

	metadata := parseTrackMetadata(data, "CPONUbyJ3YM")
	if metadata.Channel != "Accusefive" || metadata.ArtistBrowseID != "" || metadata.ArtistSource != trackArtistSourceAPIText {
		t.Fatalf("unexpected metadata artist: %#v", metadata)
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
	if tracks[2].RawDescription != "Album Name" {
		t.Fatalf("expected album metadata in description, got %#v", tracks[2])
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
	if track.RawDescription != "Album Name" {
		t.Fatalf("unexpected album metadata: %#v", track)
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

func TestPlaylistHeaderFromBrowseDataSkipsReleaseYearAuthor(t *testing.T) {
	data := map[string]any{
		"header": map[string]any{
			"musicDetailHeaderRenderer": map[string]any{
				"title": map[string]any{"runs": []any{map[string]any{"text": "Midnight Album"}}},
				"subtitle": map[string]any{"runs": []any{
					map[string]any{"text": "2009"},
					map[string]any{"text": " • "},
					map[string]any{"text": "Album Artist"},
					map[string]any{"text": " • "},
					map[string]any{"text": "Album"},
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

func TestExtractTimedLyricsRomanizesJapaneseKanji(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	result := extractTimedLyrics(map[string]any{
		"contents": map[string]any{
			"nested": []any{
				map[string]any{
					"timedLyricsModel": map[string]any{
						"lyricsData": []any{
							map[string]any{
								"lyricLine":   "東京に行く",
								"startTimeMs": "1200",
							},
						},
					},
				},
			},
		},
	}, "YTMusic")
	if result.Kind != lyricsResultSynced || len(result.Lines) != 1 {
		t.Fatalf("expected synced lyrics, got %+v", result)
	}
	if got := result.Lines[0].RomanizedText; got == "" || strings.Contains(got, "東京") {
		t.Fatalf("expected kanji romanization, got %+v", result.Lines[0])
	}
	if result.Lines[0].RomanizedKind != string(lyricsromanization.KindRomanized) {
		t.Fatalf("expected romanized kind, got %+v", result.Lines[0])
	}
}

func TestPlainLyricLinesRomanizesJapaneseKanji(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system romanization is only available on darwin")
	}
	lines := plainLyricLines("東京に行く\nHello")
	if len(lines) != 2 {
		t.Fatalf("expected two plain lines, got %+v", lines)
	}
	if got := lines[0].RomanizedText; got == "" || strings.Contains(got, "東京") {
		t.Fatalf("expected kanji romanization, got %+v", lines[0])
	}
	if lines[0].RomanizedKind != string(lyricsromanization.KindRomanized) {
		t.Fatalf("expected romanized kind, got %+v", lines[0])
	}
	if got := lines[1].RomanizedText; got != "" {
		t.Fatalf("expected latin line to skip romanization, got %+v", lines[1])
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

func TestBestLRCLibModelPrefersSyncedOverPlain(t *testing.T) {
	plainDuration := 213.0
	syncedDuration := 260.0
	model, ok := bestLRCLibModelForInfo([]lrcLibModel{
		{
			ID:          1,
			TrackName:   "Track",
			ArtistName:  "Artist",
			Duration:    &plainDuration,
			PlainLyrics: "Plain",
		},
		{
			ID:           2,
			TrackName:    "Track",
			ArtistName:   "Artist",
			Duration:     &syncedDuration,
			SyncedLyrics: "[00:01.00]Synced",
		},
	}, LyricsSearchInfo{
		Title:           "Track",
		Artist:          "Artist",
		DurationSeconds: plainDuration,
	})
	if !ok {
		t.Fatalf("expected a lyric model")
	}
	if model.ID != 2 {
		t.Fatalf("expected synced model to win over plain fallback, got %+v", model)
	}
}

func TestBestLRCLibModelCanChooseSyncedTitleVariantByDuration(t *testing.T) {
	plainDuration := 260.0
	syncedDuration := 213.0
	model, ok := bestLRCLibModelForInfo([]lrcLibModel{
		{
			ID:          1,
			TrackName:   "Track",
			ArtistName:  "Artist",
			Duration:    &plainDuration,
			PlainLyrics: "Plain",
		},
		{
			ID:           2,
			TrackName:    "Track - Radio Edit",
			ArtistName:   "Artist",
			Duration:     &syncedDuration,
			SyncedLyrics: "[00:01.00]Synced",
		},
	}, LyricsSearchInfo{
		Title:           "Track",
		Artist:          "Artist",
		DurationSeconds: syncedDuration,
	})
	if !ok {
		t.Fatalf("expected a lyric model")
	}
	if model.ID != 2 {
		t.Fatalf("expected closer synced model with title variant to win, got %+v", model)
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
		case "/api/get":
			return testHTTPResponse(request, http.StatusNotFound, `{}`), nil
		case "/api/search":
			body = `[{"id":1,"trackName":"Track","artistName":"Artist","duration":260,"plainLyrics":"Plain only"},{"id":2,"trackName":"Track","artistName":"Artist","duration":213,"syncedLyrics":"[00:01.00]Synced line"}]`
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
		t.Fatalf("unexpected synced lyrics: %+v", result.Lines)
	}
}

func TestTrackLyricsPlainOnlyUsesYouTubePlainFallback(t *testing.T) {
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body string
		switch request.URL.Path {
		case "/youtubei/v1/next":
			body = `{"contents":{"singleColumnMusicWatchNextResultsRenderer":{"tabbedRenderer":{"watchNextTabbedResultsRenderer":{"tabs":[{"tabRenderer":{"endpoint":{"browseEndpoint":{"browseId":"MPLYt_lyrics"}}}}]}}},"nested":[{"timedLyricsModel":{"lyricsData":[{"lyricLine":"Synced should not win","startTimeMs":"1000"}]}}]}}`
		case "/youtubei/v1/browse":
			body = `{"contents":{"sectionListRenderer":{"contents":[{"musicDescriptionShelfRenderer":{"description":{"runs":[{"text":"Official plain lyrics"}]},"footer":{"runs":[{"text":"YouTube Music"}]}}}]}}}`
		case "/api/get", "/api/search":
			t.Fatalf("plain-only video lyrics should not request LRCLib: %s", request.URL.Path)
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
		PlainOnly:       true,
	})
	if err != nil {
		t.Fatalf("track lyrics: %v", err)
	}
	if result.Kind != lyricsResultPlain || result.Source != "YouTube Music" || result.Text != "Official plain lyrics" {
		t.Fatalf("expected YouTube plain lyrics, got %+v", result)
	}
}

func TestTrackLyricsPlainOnlyFallsBackToLRCLibPlain(t *testing.T) {
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	lrcGetCalls := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body string
		switch request.URL.Path {
		case "/youtubei/v1/next":
			body = `{"contents":{"singleColumnMusicWatchNextResultsRenderer":{"tabbedRenderer":{"watchNextTabbedResultsRenderer":{"tabs":[{"tabRenderer":{"endpoint":{"browseEndpoint":{"browseId":"MPLYt_lyrics"}}}}]}}}}}`
		case "/youtubei/v1/browse":
			body = `{}`
		case "/api/get":
			lrcGetCalls++
			body = `{"id":3,"trackName":"Track","artistName":"Artist","duration":213,"plainLyrics":"LRCLib plain","syncedLyrics":"[00:01.00]Should not win"}`
		case "/api/search":
			body = `[]`
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
		PlainOnly:       true,
	})
	if err != nil {
		t.Fatalf("track lyrics: %v", err)
	}
	if result.Kind != lyricsResultPlain || result.Source != "LRCLib" || result.Text != "LRCLib plain" || len(result.Lines) != 1 {
		t.Fatalf("expected LRCLib plain lyrics, got %+v", result)
	}
	if lrcGetCalls != 1 {
		t.Fatalf("expected LRCLib exact lookup, got %d calls", lrcGetCalls)
	}
}

func TestTrackLyricsSearchesLRCLibWithoutVideoID(t *testing.T) {
	client := NewClient(fakeCookieProvider{})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/get" {
			return testHTTPResponse(request, http.StatusNotFound, `{}`), nil
		}
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

func TestTrackLyricsUsesLRCLibExactLookup(t *testing.T) {
	client := NewClient(fakeCookieProvider{})
	searchCalls := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/get":
			if got := request.URL.Query().Get("track_name"); got != "Track" {
				t.Fatalf("unexpected track_name query: %q", got)
			}
			if got := request.URL.Query().Get("artist_name"); got != "Artist" {
				t.Fatalf("unexpected artist_name query: %q", got)
			}
			if got := request.URL.Query().Get("duration"); got != "213" {
				t.Fatalf("unexpected duration query: %q", got)
			}
			body := `{"id":2,"trackName":"Track","artistName":"Artist","duration":213,"syncedLyrics":"[00:01.00]Exact synced"}`
			return testHTTPResponse(request, http.StatusOK, body), nil
		case "/api/search":
			searchCalls++
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := client.TrackLyrics(context.Background(), LyricsSearchInfo{
		Title:           "Track",
		Artist:          "Artist",
		DurationSeconds: 213,
	})
	if err != nil {
		t.Fatalf("track lyrics: %v", err)
	}
	if result.Kind != lyricsResultSynced || result.Source != "LRCLib" || len(result.Lines) != 2 || result.Lines[1].Text != "Exact synced" {
		t.Fatalf("expected exact synced lyrics, got %+v", result)
	}
	if searchCalls > 1 {
		t.Fatalf("expected at most one concurrent search lookup, got %d search calls", searchCalls)
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
		switch request.URL.Path {
		case "/youtubei/v1/next":
			nextCalls++
			if nextCalls == 1 {
				return nil, io.EOF
			}
			body := `{"contents":{"nested":[{"timedLyricsModel":{"lyricsData":[{"lyricLine":"Recovered line","startTimeMs":"1200","durationMs":"3000"}]}}]}}`
			return testHTTPResponse(request, http.StatusOK, body), nil
		case "/api/get":
			return testHTTPResponse(request, http.StatusNotFound, `{}`), nil
		case "/api/search":
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
			return nil, nil
		}
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

func TestTrackLyricsCachesSyncedLyrics(t *testing.T) {
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	nextCalls := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/youtubei/v1/next":
			nextCalls++
			body := `{"contents":{"nested":[{"timedLyricsModel":{"lyricsData":[{"lyricLine":"Cached line","startTimeMs":"1200","durationMs":"3000"}]}}]}}`
			return testHTTPResponse(request, http.StatusOK, body), nil
		case "/api/get":
			return testHTTPResponse(request, http.StatusNotFound, `{}`), nil
		case "/api/search":
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	for i := 0; i < 2; i++ {
		result, err := client.TrackLyrics(context.Background(), LyricsSearchInfo{
			VideoID: "TESTVID007G",
			Title:   "Track",
			Artist:  "Artist",
		})
		if err != nil {
			t.Fatalf("track lyrics call %d: %v", i+1, err)
		}
		if result.Kind != lyricsResultSynced || len(result.Lines) != 1 || result.Lines[0].Text != "Cached line" {
			t.Fatalf("unexpected lyrics result on call %d: %+v", i+1, result)
		}
	}
	if nextCalls != 1 {
		t.Fatalf("expected synced lyrics cache to avoid second network call, got %d calls", nextCalls)
	}
}

func TestTrackLyricsRefreshesCachedPlainAndUpgradesToSynced(t *testing.T) {
	client := NewClient(fakeCookieProvider{records: []appcookies.Record{
		{Name: "__Secure-3PAPISID", Value: "test-sapisid", Domain: ".youtube.com", Path: "/", Expires: 4102444800, Secure: true},
	}})
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	nextCalls := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/youtubei/v1/next":
			nextCalls++
			if nextCalls == 1 {
				body := `{"contents":{"singleColumnMusicWatchNextResultsRenderer":{"tabbedRenderer":{"watchNextTabbedResultsRenderer":{"tabs":[{"tabRenderer":{"endpoint":{"browseEndpoint":{"browseId":"MPLYt_lyrics"}}}}]}}}}}`
				return testHTTPResponse(request, http.StatusOK, body), nil
			}
			body := `{"contents":{"nested":[{"timedLyricsModel":{"lyricsData":[{"lyricLine":"Upgraded synced line","startTimeMs":"1200","durationMs":"3000"}]}}]}}`
			return testHTTPResponse(request, http.StatusOK, body), nil
		case "/youtubei/v1/browse":
			body := `{"contents":{"sectionListRenderer":{"contents":[{"musicDescriptionShelfRenderer":{"description":{"runs":[{"text":"Cached plain lyrics"}]}}}]}}}`
			return testHTTPResponse(request, http.StatusOK, body), nil
		case "/api/get":
			return testHTTPResponse(request, http.StatusNotFound, `{}`), nil
		case "/api/search":
			return testHTTPResponse(request, http.StatusOK, `[]`), nil
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	first, err := client.TrackLyrics(context.Background(), LyricsSearchInfo{
		VideoID:         "TESTVID007G",
		Title:           "Track",
		Artist:          "Artist",
		DurationSeconds: 213,
	})
	if err != nil {
		t.Fatalf("first track lyrics: %v", err)
	}
	if first.Kind != lyricsResultPlain || first.Text != "Cached plain lyrics" {
		t.Fatalf("expected first call to return plain lyrics, got %+v", first)
	}

	second, err := client.TrackLyrics(context.Background(), LyricsSearchInfo{
		VideoID:         "TESTVID007G",
		Title:           "Track",
		Artist:          "Artist",
		DurationSeconds: 213,
	})
	if err != nil {
		t.Fatalf("second track lyrics: %v", err)
	}
	if second.Kind != lyricsResultSynced || len(second.Lines) != 1 || second.Lines[0].Text != "Upgraded synced line" {
		t.Fatalf("expected second call to upgrade to synced lyrics, got %+v", second)
	}
	if nextCalls != 2 {
		t.Fatalf("expected plain cache to refresh instead of short-circuiting, got %d next calls", nextCalls)
	}
}
