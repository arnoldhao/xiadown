package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	appsessionidentity "xiadown/internal/application/appsessions"
	appsessionsdto "xiadown/internal/application/appsessions/dto"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubemusic"
	"xiadown/internal/domain/appsessions"
)

const bilibiliNavURL = "https://api.bilibili.com/x/web-interface/nav"

type appSessionAccountCookieProvider struct {
	records []appcookies.Record
}

func (provider appSessionAccountCookieProvider) RecordsForSiteKey(_ context.Context, siteKey string) ([]appcookies.Record, error) {
	if strings.TrimSpace(siteKey) != "youtube" {
		return nil, appsessions.ErrSessionNotFound
	}
	if len(provider.records) == 0 {
		return nil, appsessions.ErrNoCookies
	}
	return append([]appcookies.Record(nil), provider.records...), nil
}

func newAppSessionAccountFetcher(httpClientProvider youtubemusic.HTTPClientProvider) appsessionsservice.AppSessionAccountFetcher {
	return func(ctx context.Context, siteKey string, records []appcookies.Record) (appsessionsdto.AppSessionAccount, error) {
		switch strings.TrimSpace(siteKey) {
		case "youtube":
			return fetchYouTubeMusicAppSessionAccount(ctx, httpClientProvider, records)
		case "bilibili":
			return fetchBilibiliAppSessionAccount(ctx, appSessionHTTPClient(httpClientProvider), records)
		case "tiktok":
			return fetchTikTokAppSessionAccount(ctx, appSessionHTTPClient(httpClientProvider), records)
		case "instagram":
			return fetchInstagramAppSessionAccount(ctx, appSessionHTTPClient(httpClientProvider), records)
		case "x":
			return fetchXAppSessionAccount(ctx, appSessionHTTPClient(httpClientProvider), records)
		case "facebook":
			return fetchFacebookAppSessionAccount(ctx, appSessionHTTPClient(httpClientProvider), records)
		case "vimeo":
			return fetchVimeoAppSessionAccount(ctx, appSessionHTTPClient(httpClientProvider), records)
		case "twitch":
			return fetchTwitchAppSessionAccount(ctx, appSessionHTTPClient(httpClientProvider), records)
		case "niconico":
			return fetchNiconicoAppSessionAccount(ctx, appSessionHTTPClient(httpClientProvider), records)
		default:
			return appsessionsdto.AppSessionAccount{}, appsessions.ErrUnsupported
		}
	}
}

func fetchYouTubeMusicAppSessionAccount(ctx context.Context, httpClientProvider youtubemusic.HTTPClientProvider, records []appcookies.Record) (appsessionsdto.AppSessionAccount, error) {
	client := youtubemusic.NewClientWithHTTPClientProvider(appSessionAccountCookieProvider{records: records}, httpClientProvider)
	client.SetUserAgent(appsessionidentity.HTTPUserAgent("youtube"))
	account, err := client.AccountInfo(ctx)
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	return appsessionsdto.AppSessionAccount{
		DisplayName: account.DisplayName,
		Handle:      account.Handle,
		AvatarURL:   account.AvatarURL,
	}, nil
}

func appSessionHTTPClient(httpClientProvider youtubemusic.HTTPClientProvider) *http.Client {
	if httpClientProvider != nil {
		if client := httpClientProvider.HTTPClient(); client != nil {
			return client
		}
	}
	return http.DefaultClient
}

type bilibiliNavResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		IsLogin bool   `json:"isLogin"`
		Mid     int64  `json:"mid"`
		Uname   string `json:"uname"`
		Face    string `json:"face"`
		Vip     struct {
			Type   int `json:"type"`
			Status int `json:"status"`
			Label  struct {
				Text       string `json:"text"`
				LabelTheme string `json:"label_theme"`
			} `json:"label"`
		} `json:"vip"`
		LevelInfo struct {
			CurrentLevel int `json:"current_level"`
		} `json:"level_info"`
	} `json:"data"`
}

func fetchBilibiliAppSessionAccount(ctx context.Context, client *http.Client, records []appcookies.Record) (appsessionsdto.AppSessionAccount, error) {
	return fetchBilibiliAppSessionAccountFromURL(ctx, client, records, bilibiliNavURL)
}

func fetchBilibiliAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string) (appsessionsdto.AppSessionAccount, error) {
	if client == nil {
		client = http.DefaultClient
	}
	matched := bilibiliAccountCookies(records, endpoint)
	if len(matched) == 0 {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("User-Agent", appsessionidentity.HTTPUserAgent("bilibili"))
	for _, record := range matched {
		name := strings.TrimSpace(record.Name)
		if name == "" {
			continue
		}
		req.AddCookie(&http.Cookie{
			Name:  name,
			Value: record.Value,
			Path:  record.Path,
		})
	}
	if len(req.Cookies()) == 0 {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	resp, err := client.Do(req)
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		_, _ = io.Copy(io.Discard, resp.Body)
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return appsessionsdto.AppSessionAccount{}, fmt.Errorf("bilibili account info status %d", resp.StatusCode)
	}
	var decoded bilibiliNavResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&decoded); err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	if decoded.Code != 0 {
		if decoded.Code == -101 {
			return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
		}
		message := strings.TrimSpace(decoded.Message)
		if message == "" {
			message = strconv.Itoa(decoded.Code)
		}
		return appsessionsdto.AppSessionAccount{}, fmt.Errorf("bilibili account info failed: %s", message)
	}
	if !decoded.Data.IsLogin || strings.TrimSpace(decoded.Data.Uname) == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	return mapBilibiliNavAccount(decoded), nil
}

func bilibiliAccountCookies(records []appcookies.Record, endpoint string) []appcookies.Record {
	matched := appcookies.FilterByDomains(records, []string{"bilibili.com"})
	if len(matched) > 0 {
		return matched
	}
	return appcookies.MatchURL(records, endpoint)
}

func mapBilibiliNavAccount(decoded bilibiliNavResponse) appsessionsdto.AppSessionAccount {
	vipLabel := strings.TrimSpace(decoded.Data.Vip.Label.Text)
	tierKey := ""
	tierLabel := ""
	if decoded.Data.Vip.Status == 1 {
		switch decoded.Data.Vip.Type {
		case 2:
			tierKey = "vip_annual"
		case 1:
			tierKey = "vip"
		default:
			tierKey = "vip_active"
		}
		tierLabel = vipLabel
	}
	badges := make([]appsessionsdto.AppSessionBadge, 0, 1)
	if decoded.Data.LevelInfo.CurrentLevel > 0 {
		levelLabel := fmt.Sprintf("LV%d", decoded.Data.LevelInfo.CurrentLevel)
		badges = append(badges, appsessionsdto.AppSessionBadge{
			Key:   "level_" + strconv.Itoa(decoded.Data.LevelInfo.CurrentLevel),
			Label: levelLabel,
		})
	}
	metadata := map[string]any{
		"mid":             decoded.Data.Mid,
		"level":           decoded.Data.LevelInfo.CurrentLevel,
		"vipType":         decoded.Data.Vip.Type,
		"vipStatus":       decoded.Data.Vip.Status,
		"vipLabelTheme":   strings.TrimSpace(decoded.Data.Vip.Label.LabelTheme),
		"accountEndpoint": "x/web-interface/nav",
	}
	return appsessionsdto.AppSessionAccount{
		DisplayName: strings.TrimSpace(decoded.Data.Uname),
		AvatarURL:   normalizeBilibiliAvatarURL(decoded.Data.Face),
		TierKey:     tierKey,
		TierLabel:   tierLabel,
		Badges:      badges,
		Metadata:    metadata,
	}
}

func normalizeBilibiliAvatarURL(value string) string {
	return normalizeAccountImageURL(value)
}
