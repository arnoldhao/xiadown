package service

import (
	"context"
	"testing"

	connectorsdto "xiadown/internal/application/connectors/dto"
	"xiadown/internal/application/library/dto"
)

func TestValidateDownloadURLAcceptsValidDomainAndAddsScheme(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input   string
		wantURL string
	}{
		{
			input:   "www.youtube.com/watch?v=BaW_jenozKc",
			wantURL: "https://www.youtube.com/watch?v=BaW_jenozKc",
		},
		{
			input:   "www.youtube.com:443/watch?v=BaW_jenozKc",
			wantURL: "https://www.youtube.com:443/watch?v=BaW_jenozKc",
		},
	}
	for _, tc := range cases {
		gotURL, gotDomain, err := validateDownloadURL(tc.input)
		if err != nil {
			t.Fatalf("validateDownloadURL(%q) error = %v", tc.input, err)
		}
		if gotURL != tc.wantURL {
			t.Fatalf("url = %q, want %q", gotURL, tc.wantURL)
		}
		if gotDomain != "youtube.com" {
			t.Fatalf("domain = %q", gotDomain)
		}
	}
}

func TestValidateDownloadURLRejectsInvalidDomain(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{
		"localhost/watch?v=BaW_jenozKc",
		"https://watch?v=BaW_jenozKc",
		"watch?v=too-short",
		"video/not-a-bilibili-id",
	} {
		if _, _, err := validateDownloadURL(rawURL); err == nil {
			t.Fatalf("expected %q to be rejected", rawURL)
		}
	}
}

func TestValidateDownloadURLCompletesKnownVideoSuffixes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		input      string
		wantURL    string
		wantDomain string
	}{
		{
			name:       "youtube watch path",
			input:      "watch?v=BaW_jenozKc&t=1s",
			wantURL:    "https://www.youtube.com/watch?v=BaW_jenozKc&t=1s",
			wantDomain: "youtube.com",
		},
		{
			name:       "youtube query suffix",
			input:      "?v=BaW_jenozKc",
			wantURL:    "https://www.youtube.com/watch?v=BaW_jenozKc",
			wantDomain: "youtube.com",
		},
		{
			name:       "youtube naked id",
			input:      "BaW_jenozKc",
			wantURL:    "https://www.youtube.com/watch?v=BaW_jenozKc",
			wantDomain: "youtube.com",
		},
		{
			name:       "youtube shorts path",
			input:      "shorts/BGQWPY4IigY",
			wantURL:    "https://www.youtube.com/shorts/BGQWPY4IigY",
			wantDomain: "youtube.com",
		},
		{
			name:       "bilibili video path",
			input:      "video/BV13x41117TL",
			wantURL:    "https://www.bilibili.com/video/BV13x41117TL",
			wantDomain: "bilibili.com",
		},
		{
			name:       "bilibili av path",
			input:      "/video/av1074402/",
			wantURL:    "https://www.bilibili.com/video/av1074402/",
			wantDomain: "bilibili.com",
		},
		{
			name:       "bilibili naked bvid",
			input:      "BV13x41117TL",
			wantURL:    "https://www.bilibili.com/video/BV13x41117TL",
			wantDomain: "bilibili.com",
		},
		{
			name:       "bilibili festival bvid",
			input:      "festival/bh3-7th?bvid=BV1tr4y1f7p2&",
			wantURL:    "https://www.bilibili.com/festival/bh3-7th?bvid=BV1tr4y1f7p2&",
			wantDomain: "bilibili.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotURL, gotDomain, err := validateDownloadURL(tc.input)
			if err != nil {
				t.Fatalf("validateDownloadURL() error = %v", err)
			}
			if gotURL != tc.wantURL {
				t.Fatalf("url = %q, want %q", gotURL, tc.wantURL)
			}
			if gotDomain != tc.wantDomain {
				t.Fatalf("domain = %q, want %q", gotDomain, tc.wantDomain)
			}
		})
	}
}

func TestPrepareYTDLPDownloadUsesConnectorModeForNormalizedURL(t *testing.T) {
	t.Parallel()

	service := &LibraryService{
		connectors: resourceConnectorReaderStub{
			items: []connectorsdto.Connector{
				{
					ID:             "connector-youtube",
					Type:           "youtube",
					Status:         "connected",
					CredentialMode: "cookies",
					CookiesCount:   2,
				},
				{
					ID:              "connector-china-private",
					Type:            "china_private",
					CredentialMode:  "profile",
					CredentialState: "profile",
					ProfilePath:     "/tmp/xiadown-profile",
				},
			},
		},
	}

	youtube, err := service.PrepareYTDLPDownload(context.Background(), dtoPrepareRequest("watch?v=BaW_jenozKc"))
	if err != nil {
		t.Fatalf("prepare youtube: %v", err)
	}
	if youtube.URL != "https://www.youtube.com/watch?v=BaW_jenozKc" || youtube.Domain != "youtube.com" {
		t.Fatalf("unexpected youtube normalization: %#v", youtube)
	}
	if youtube.ConnectorID != "connector-youtube" || !youtube.ConnectorAvailable || youtube.ConnectorCredentialMode != "cookies" {
		t.Fatalf("unexpected youtube connector: %#v", youtube)
	}

	chinaPrivate, err := service.PrepareYTDLPDownload(context.Background(), dtoPrepareRequest("douyin.com/video/123"))
	if err != nil {
		t.Fatalf("prepare china private: %v", err)
	}
	if chinaPrivate.URL != "https://douyin.com/video/123" || chinaPrivate.Domain != "douyin.com" {
		t.Fatalf("unexpected china private normalization: %#v", chinaPrivate)
	}
	if chinaPrivate.ConnectorID != "connector-china-private" || !chinaPrivate.ConnectorAvailable || chinaPrivate.ConnectorCredentialMode != "profile" {
		t.Fatalf("unexpected china private connector: %#v", chinaPrivate)
	}

	for _, rawURL := range []string{
		"https://www.iesdouyin.com/share/video/123/",
		"https://www.rednote.com/explore/123",
		"https://xhslink.com/a/example",
	} {
		result, err := service.PrepareYTDLPDownload(context.Background(), dtoPrepareRequest(rawURL))
		if err != nil {
			t.Fatalf("prepare china private alias %s: %v", rawURL, err)
		}
		if result.ConnectorID != "connector-china-private" || !result.ConnectorAvailable || result.ConnectorCredentialMode != "profile" {
			t.Fatalf("unexpected china private alias connector for %s: %#v", rawURL, result)
		}
	}
}

func TestPrepareYTDLPDownloadMarksUnreadyProfileConnectorUnavailable(t *testing.T) {
	t.Parallel()

	service := &LibraryService{
		connectors: resourceConnectorReaderStub{
			items: []connectorsdto.Connector{
				{
					ID:              "connector-china-private",
					Type:            "china_private",
					CredentialMode:  "profile",
					CredentialState: "disconnected",
					ProfilePath:     "/tmp/xiadown-profile",
				},
			},
		},
	}

	chinaPrivate, err := service.PrepareYTDLPDownload(context.Background(), dtoPrepareRequest("douyin.com/video/123"))
	if err != nil {
		t.Fatalf("prepare china private: %v", err)
	}
	if chinaPrivate.ConnectorID != "connector-china-private" || chinaPrivate.ConnectorAvailable || chinaPrivate.ConnectorCredentialMode != "profile" {
		t.Fatalf("unexpected china private connector availability: %#v", chinaPrivate)
	}
	if chinaPrivate.ConnectorCredentialState != "disconnected" {
		t.Fatalf("expected disconnected credential state, got %#v", chinaPrivate)
	}
}

func dtoPrepareRequest(rawURL string) dto.PrepareYTDLPDownloadRequest {
	return dto.PrepareYTDLPDownloadRequest{URL: rawURL}
}
