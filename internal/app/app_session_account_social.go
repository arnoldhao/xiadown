package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	appsessionidentity "xiadown/internal/application/appsessions"
	appsessionsdto "xiadown/internal/application/appsessions/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

const (
	tiktokAccountInfoURL        = "https://www.tiktok.com/passport/web/account/info/?aid=1988&app_name=tiktok_web&device_platform=web_pc"
	instagramCurrentUserURL     = "https://www.instagram.com/api/v1/accounts/current_user/?edit=true"
	instagramEditFormURL        = "https://www.instagram.com/api/v1/accounts/edit/web_form_data/"
	instagramWebProfileInfoURL  = "https://i.instagram.com/api/v1/users/web_profile_info/"
	xVerifyCredentialsURL       = "https://x.com/i/api/1.1/account/verify_credentials.json?include_entities=false&skip_status=true&include_email=false"
	xGraphQLViewerURL           = "https://x.com/i/api/graphql/u4ni7JqpqdAQxWQfkLsdUQ/Viewer"
	xGraphQLUserByRestIDURL     = "https://x.com/i/api/graphql/DaeC_2LfMgwCujE03HSZtw/UserByRestId"
	xAccountSettingsURL         = "https://x.com/i/api/1.1/account/settings.json"
	xUsersShowURL               = "https://x.com/i/api/1.1/users/show.json"
	xWebBearerToken             = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"
	facebookMeURL               = "https://www.facebook.com/me"
	vimeoViewerURL              = "https://vimeo.com/_next/viewer"
	twitchValidateURL           = "https://id.twitch.tv/oauth2/validate"
	twitchUsersURL              = "https://api.twitch.tv/helix/users"
	twitchGQLURL                = "https://gql.twitch.tv/gql"
	twitchWebClientID           = "kimne78kx3ncx6brgo4mv6wki5h1ko"
	niconicoUsersMeURL          = "https://nvapi.nicovideo.jp/v1/users/me"
	appSessionAccountMaxPayload = 2 << 20
	appSessionAvatarMaxPayload  = 512 << 10
)

var (
	facebookNamePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<meta[^>]+(?:property|name)=["']og:title["'][^>]+content=["']([^"']+)["']`),
		regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']+)["'][^>]+(?:property|name)=["']og:title["']`),
		regexp.MustCompile(`"NAME"\s*:\s*"((?:\\.|[^"\\])*)"`),
		regexp.MustCompile(`"short_name"\s*:\s*"((?:\\.|[^"\\])*)"`),
		regexp.MustCompile(`"name"\s*:\s*"((?:\\.|[^"\\])*)"`),
		regexp.MustCompile(`<title[^>]*>(.*?)</title>`),
	}
	facebookAvatarPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<meta[^>]+(?:property|name)=["']og:image["'][^>]+content=["']([^"']+)["']`),
		regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']+)["'][^>]+(?:property|name)=["']og:image["']`),
		regexp.MustCompile(`"profile_picture"\s*:\s*\{[^{}]*"uri"\s*:\s*"((?:\\.|[^"\\])*)"`),
		regexp.MustCompile(`"profilePicture"\s*:\s*\{[^{}]*"uri"\s*:\s*"((?:\\.|[^"\\])*)"`),
		regexp.MustCompile(`"image"\s*:\s*\{[^{}]*"uri"\s*:\s*"((?:\\.|[^"\\])*)"`),
		regexp.MustCompile(`"uri"\s*:\s*"(https?:\\?/\\?/[^"\\]*(?:fbcdn|scontent)[^"\\]*)"`),
	}
	facebookURLPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<meta[^>]+(?:property|name)=["']og:url["'][^>]+content=["']([^"']+)["']`),
		regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']+)["'][^>]+(?:property|name)=["']og:url["']`),
		regexp.MustCompile(`(?is)<link[^>]+rel=["']canonical["'][^>]+href=["']([^"']+)["']`),
		regexp.MustCompile(`(?is)<link[^>]+href=["']([^"']+)["'][^>]+rel=["']canonical["']`),
	}
	facebookRedirectPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<meta[^>]+http-equiv=["']?refresh["']?[^>]+content=["'][^"']*url\s*=\s*([^"'>\s]+)`),
		regexp.MustCompile(`(?is)<meta[^>]+content=["'][^"']*url\s*=\s*([^"'>\s]+)[^"']*["'][^>]+http-equiv=["']?refresh["']?`),
		regexp.MustCompile(`(?is)(?:window\.)?location(?:\.href)?\s*=\s*["']([^"']+)["']`),
		regexp.MustCompile(`(?is)(?:window\.)?location\.replace\(\s*["']([^"']+)["']\s*\)`),
	}
)

type facebookAccountPage struct {
	body     string
	finalURL string
}

func fetchTikTokAppSessionAccount(ctx context.Context, client *http.Client, records []appcookies.Record) (appsessionsdto.AppSessionAccount, error) {
	return fetchTikTokAppSessionAccountFromURL(ctx, client, records, tiktokAccountInfoURL)
}

func fetchTikTokAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string) (appsessionsdto.AppSessionAccount, error) {
	if !appSessionHasAnyCookie(records, []string{"tiktok.com"}, "sessionid", "sessionid_ss", "sid_tt", "sid_guard", "uid_tt", "uid_tt_ss") {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	data, err := appSessionJSON(ctx, client, "tiktok", endpoint, records, []string{"tiktok.com"}, map[string]string{
		"Referer": "https://www.tiktok.com/",
	})
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	payload := appSessionMap(data, "data")
	if len(payload) == 0 {
		payload = data
	}
	if tiktokAccountPayloadLooksLoggedOut(data) ||
		tiktokAccountPayloadLooksLoggedOut(payload) ||
		appSessionInt(payload, "error_code") == 13 {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	userID := firstNonEmpty(
		appSessionString(payload, "user_id"),
		appSessionString(payload, "user_id_str"),
		appSessionString(payload, "uid"),
	)
	handle := firstNonEmpty(
		appSessionString(payload, "username"),
		appSessionString(payload, "unique_id"),
		appSessionString(payload, "uniqueId"),
	)
	displayName := firstNonEmpty(
		appSessionString(payload, "screen_name"),
		appSessionString(payload, "nickname"),
		handle,
	)
	avatarURL := normalizeAccountImageURL(firstNonEmpty(
		appSessionString(payload, "avatar_url"),
		appSessionString(payload, "avatar_url_168x168"),
		appSessionString(payload, "avatar_larger"),
		appSessionString(payload, "avatar_thumb"),
	))
	if displayName == "" && handle == "" && avatarURL == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	metadata := map[string]any{"accountEndpoint": "passport/web/account/info"}
	if userID != "" {
		metadata["userID"] = userID
	}
	return appsessionsdto.AppSessionAccount{
		DisplayName: displayName,
		Handle:      handle,
		AvatarURL:   avatarURL,
		Metadata:    metadata,
	}, nil
}

func fetchInstagramAppSessionAccount(ctx context.Context, client *http.Client, records []appcookies.Record) (appsessionsdto.AppSessionAccount, error) {
	return fetchInstagramAppSessionAccountFromURL(ctx, client, records, instagramCurrentUserURL)
}

func fetchInstagramAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string) (appsessionsdto.AppSessionAccount, error) {
	if !appSessionHasAnyCookie(records, []string{"instagram.com"}, "sessionid") {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	data, err := appSessionJSON(ctx, client, "instagram", endpoint, records, []string{"instagram.com"}, map[string]string{
		"Referer":          "https://www.instagram.com/",
		"X-IG-App-ID":      "936619743392459",
		"X-ASBD-ID":        "359341",
		"X-Requested-With": "XMLHttpRequest",
		"Sec-Fetch-Site":   "same-origin",
		"Sec-Fetch-Mode":   "cors",
		"Sec-Fetch-Dest":   "empty",
		"X-Instagram-AJAX": "1",
		"X-IG-WWW-Claim":   "0",
		"X-CSRFToken":      appSessionCookieValue(records, []string{"instagram.com"}, "csrftoken"),
	})
	if err != nil {
		if account, fallbackErr := fetchInstagramEditFormAppSessionAccountFromURL(ctx, client, records, instagramPeerEndpoint(endpoint, instagramEditFormURL)); fallbackErr == nil {
			return account, nil
		}
		return appsessionsdto.AppSessionAccount{}, err
	}
	user := appSessionMap(data, "user")
	if len(user) == 0 {
		user = data
	}
	if instagramAccountPayloadLooksLoggedOut(data) || instagramAccountPayloadLooksLoggedOut(user) {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	account, err := instagramAccountFromUser(user, "api/v1/accounts/current_user")
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	return embedInstagramAccountAvatar(ctx, client, account), nil
}

func fetchInstagramEditFormAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string) (appsessionsdto.AppSessionAccount, error) {
	data, err := appSessionJSON(ctx, client, "instagram", endpoint, records, []string{"instagram.com"}, map[string]string{
		"Referer":          "https://www.instagram.com/accounts/edit/",
		"X-IG-App-ID":      "936619743392459",
		"X-ASBD-ID":        "359341",
		"X-Requested-With": "XMLHttpRequest",
		"Sec-Fetch-Site":   "same-origin",
		"Sec-Fetch-Mode":   "cors",
		"Sec-Fetch-Dest":   "empty",
		"X-CSRFToken":      appSessionCookieValue(records, []string{"instagram.com"}, "csrftoken"),
	})
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	form := appSessionMap(data, "form_data")
	if len(form) == 0 {
		form = data
	}
	if instagramAccountPayloadLooksLoggedOut(data) || instagramAccountPayloadLooksLoggedOut(form) {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	user := map[string]any{
		"username":  appSessionString(form, "username"),
		"full_name": firstNonEmpty(appSessionString(form, "full_name"), appSessionString(form, "first_name")),
		"id":        firstNonEmpty(appSessionString(form, "user_id"), appSessionString(form, "pk"), appSessionString(form, "id")),
	}
	account, err := instagramAccountFromUser(user, "api/v1/accounts/edit/web_form_data")
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	username := strings.TrimSpace(account.Handle)
	if username == "" {
		return account, nil
	}
	profileEndpoint := instagramWebProfileEndpoint(endpoint, username)
	profile, err := fetchInstagramWebProfileAppSessionAccountFromURL(ctx, client, records, profileEndpoint, username)
	if err != nil {
		return account, nil
	}
	if account.DisplayName == "" {
		account.DisplayName = profile.DisplayName
	}
	if account.AvatarURL == "" {
		account.AvatarURL = profile.AvatarURL
	}
	if account.Metadata == nil {
		account.Metadata = map[string]any{}
	}
	for key, value := range profile.Metadata {
		if key == "accountEndpoint" {
			account.Metadata["profileEndpoint"] = value
			continue
		}
		if _, exists := account.Metadata[key]; !exists {
			account.Metadata[key] = value
		}
	}
	return embedInstagramAccountAvatar(ctx, client, account), nil
}

func fetchInstagramWebProfileAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string, username string) (appsessionsdto.AppSessionAccount, error) {
	data, err := appSessionJSON(ctx, client, "instagram", endpoint, records, []string{"instagram.com"}, map[string]string{
		"Referer":          "https://www.instagram.com/" + strings.Trim(strings.TrimSpace(username), "/") + "/",
		"X-IG-App-ID":      "936619743392459",
		"X-ASBD-ID":        "359341",
		"X-Requested-With": "XMLHttpRequest",
		"Sec-Fetch-Site":   "same-origin",
		"Sec-Fetch-Mode":   "cors",
		"Sec-Fetch-Dest":   "empty",
		"X-CSRFToken":      appSessionCookieValue(records, []string{"instagram.com"}, "csrftoken"),
	})
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	user := appSessionNestedMap(data, "data", "user")
	if len(user) == 0 {
		user = appSessionMap(data, "user")
	}
	if len(user) == 0 {
		user = data
	}
	if instagramAccountPayloadLooksLoggedOut(data) || instagramAccountPayloadLooksLoggedOut(user) {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	return instagramAccountFromUser(user, "api/v1/users/web_profile_info")
}

func instagramAccountFromUser(user map[string]any, accountEndpoint string) (appsessionsdto.AppSessionAccount, error) {
	handle := appSessionString(user, "username")
	displayName := firstNonEmpty(appSessionString(user, "full_name"), appSessionString(user, "name"), handle)
	avatarURL := normalizeAccountImageURL(firstNonEmpty(
		appSessionNestedString(user, "hd_profile_pic_url_info", "url"),
		appSessionNestedString(user, "profile_pic_url_info", "url"),
		appSessionNestedString(user, "hd_profile_pic_versions", "url"),
		appSessionNestedString(user, "profile_picture", "uri"),
		appSessionString(user, "profile_pic_url_hd"),
		appSessionString(user, "profile_pic_url"),
	))
	userID := firstNonEmpty(appSessionString(user, "pk"), appSessionString(user, "pk_id"), appSessionString(user, "id"))
	if displayName == "" && handle == "" && avatarURL == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	metadata := map[string]any{"accountEndpoint": accountEndpoint}
	if userID != "" {
		metadata["userID"] = userID
	}
	return appsessionsdto.AppSessionAccount{
		DisplayName: displayName,
		Handle:      handle,
		AvatarURL:   avatarURL,
		Metadata:    metadata,
	}, nil
}

func instagramPeerEndpoint(currentEndpoint string, fallbackEndpoint string) string {
	fallback, err := url.Parse(fallbackEndpoint)
	if err != nil {
		return fallbackEndpoint
	}
	current, err := url.Parse(strings.TrimSpace(currentEndpoint))
	if err != nil || current.Scheme == "" || current.Host == "" {
		return fallbackEndpoint
	}
	host := strings.ToLower(current.Hostname())
	if strings.HasSuffix(host, "instagram.com") {
		return fallbackEndpoint
	}
	fallback.Scheme = current.Scheme
	fallback.Host = current.Host
	return fallback.String()
}

func instagramWebProfileEndpoint(currentEndpoint string, username string) string {
	endpoint := instagramPeerEndpoint(currentEndpoint, instagramWebProfileInfoURL)
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return instagramWebProfileInfoURL + "?username=" + url.QueryEscape(username)
	}
	query := parsed.Query()
	query.Set("username", username)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func embedInstagramAccountAvatar(ctx context.Context, client *http.Client, account appsessionsdto.AppSessionAccount) appsessionsdto.AppSessionAccount {
	dataURL, err := appSessionImageDataURL(ctx, client, "instagram", account.AvatarURL, "https://www.instagram.com/")
	if err != nil || dataURL == "" {
		return account
	}
	account.AvatarURL = dataURL
	return account
}

func fetchXAppSessionAccount(ctx context.Context, client *http.Client, records []appcookies.Record) (appsessionsdto.AppSessionAccount, error) {
	return fetchXAppSessionAccountFromURL(ctx, client, records, xVerifyCredentialsURL)
}

func fetchXAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string) (appsessionsdto.AppSessionAccount, error) {
	return fetchXAppSessionAccountFromEndpoints(
		ctx,
		client,
		records,
		endpoint,
		xPeerEndpoint(endpoint, xGraphQLViewerURL),
		xPeerEndpoint(endpoint, xGraphQLUserByRestIDURL),
		xPeerEndpoint(endpoint, xAccountSettingsURL),
		xPeerEndpoint(endpoint, xUsersShowURL),
	)
}

func fetchXAppSessionAccountFromEndpoints(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string, viewerEndpoint string, userByRestIDEndpoint string, settingsEndpoint string, usersShowEndpoint string) (appsessionsdto.AppSessionAccount, error) {
	csrf := appSessionCookieValue(records, []string{"x.com", "twitter.com"}, "ct0")
	if !appSessionHasAnyCookie(records, []string{"x.com", "twitter.com"}, "auth_token") || csrf == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	var fallbackErrors []error
	if account, fallbackErr := fetchXUserByRestIDAppSessionAccountFromURL(ctx, client, records, userByRestIDEndpoint, csrf); fallbackErr == nil {
		return account, nil
	} else {
		fallbackErrors = append(fallbackErrors, fallbackErr)
	}
	if account, fallbackErr := fetchXViewerAppSessionAccountFromURL(ctx, client, records, viewerEndpoint, csrf); fallbackErr == nil {
		return account, nil
	} else {
		fallbackErrors = append(fallbackErrors, fallbackErr)
	}
	data, err := appSessionJSON(ctx, client, "x", endpoint, records, []string{"x.com", "twitter.com"}, xWebAPIHeaders(csrf))
	if err == nil {
		return xAccountFromUser(data, "1.1/account/verify_credentials")
	}
	if account, fallbackErr := fetchXSettingsAppSessionAccountFromURL(ctx, client, records, settingsEndpoint, usersShowEndpoint, csrf); fallbackErr == nil {
		return account, nil
	} else {
		fallbackErrors = append(fallbackErrors, fallbackErr)
	}
	return appsessionsdto.AppSessionAccount{}, firstActionableXAccountError(fallbackErrors, err)
}

func fetchXViewerAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string, csrf string) (appsessionsdto.AppSessionAccount, error) {
	data, err := appSessionJSON(ctx, client, "x", xGraphQLEndpoint(endpoint, map[string]any{}, xViewerFeatures(), xViewerFieldToggles()), records, []string{"x.com", "twitter.com"}, xWebAPIHeaders(csrf))
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, xGraphQLError("Viewer", err)
	}
	return xAccountFromGraphQLUser(xGraphQLUserResult(data), "graphql/Viewer")
}

func fetchXUserByRestIDAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string, csrf string) (appsessionsdto.AppSessionAccount, error) {
	userID := xUserIDFromTWIDCookie(records)
	if userID == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	data, err := appSessionJSON(ctx, client, "x", xGraphQLEndpoint(endpoint, map[string]any{
		"userId":                   userID,
		"withSafetyModeUserFields": true,
	}, xUserByRestIDFeatures(), xUserByRestIDFieldToggles()), records, []string{"x.com", "twitter.com"}, xWebAPIHeaders(csrf))
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, xGraphQLError("UserByRestId", err)
	}
	return xAccountFromGraphQLUser(xGraphQLUserResult(data), "graphql/UserByRestId")
}

func fetchXSettingsAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, settingsEndpoint string, usersShowEndpoint string, csrf string) (appsessionsdto.AppSessionAccount, error) {
	settings, err := appSessionJSON(ctx, client, "x", settingsEndpoint, records, []string{"x.com", "twitter.com"}, xWebAPIHeaders(csrf))
	if err != nil {
		if errors.Is(err, appsessions.ErrNoCookies) {
			return appsessionsdto.AppSessionAccount{}, fmt.Errorf("x account/settings rejected session")
		}
		return appsessionsdto.AppSessionAccount{}, fmt.Errorf("x account/settings failed: %w", err)
	}
	handle := firstNonEmpty(appSessionString(settings, "screen_name"), appSessionString(settings, "screenName"))
	if handle == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	showEndpoint := xUsersShowEndpoint(usersShowEndpoint, handle)
	user, err := appSessionJSON(ctx, client, "x", showEndpoint, records, []string{"x.com", "twitter.com"}, xWebAPIHeaders(csrf))
	if err == nil {
		account, accountErr := xAccountFromUser(user, "1.1/users/show")
		if accountErr == nil {
			if account.Handle == "" {
				account.Handle = handle
			}
			return account, nil
		}
	}
	userID := firstNonEmpty(appSessionString(settings, "user_id"), appSessionString(settings, "id_str"), appSessionString(settings, "id"))
	metadata := map[string]any{"accountEndpoint": "1.1/account/settings"}
	if userID != "" {
		metadata["userID"] = userID
	}
	return appsessionsdto.AppSessionAccount{
		DisplayName: handle,
		Handle:      handle,
		Metadata:    metadata,
	}, nil
}

func xAccountFromGraphQLUser(user map[string]any, endpoint string) (appsessionsdto.AppSessionAccount, error) {
	if len(user) == 0 {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	legacy := appSessionMap(user, "legacy")
	core := appSessionMap(user, "core")
	avatar := appSessionMap(user, "avatar")
	handle := firstNonEmpty(
		appSessionString(core, "screen_name"),
		appSessionString(legacy, "screen_name"),
		appSessionString(user, "screen_name"),
		appSessionString(user, "screenName"),
	)
	displayName := firstNonEmpty(
		appSessionString(core, "name"),
		appSessionString(legacy, "name"),
		appSessionString(user, "name"),
		handle,
	)
	avatarURL := normalizeAccountImageURL(firstNonEmpty(
		appSessionString(avatar, "image_url"),
		appSessionString(avatar, "imageUrl"),
		appSessionString(legacy, "profile_image_url_https"),
		appSessionString(legacy, "profile_image_url"),
		appSessionString(user, "profile_image_url_https"),
		appSessionString(user, "profile_image_url"),
	))
	if displayName == "" && handle == "" && avatarURL == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	account := appsessionsdto.AppSessionAccount{
		DisplayName: displayName,
		Handle:      handle,
		AvatarURL:   avatarURL,
		Metadata:    map[string]any{"accountEndpoint": endpoint},
	}
	if userID := firstNonEmpty(appSessionString(user, "rest_id"), appSessionString(legacy, "id_str"), appSessionString(legacy, "id")); userID != "" {
		account.Metadata["userID"] = userID
	}
	if appSessionBool(user, "is_blue_verified") {
		account.Metadata["blueVerified"] = true
	}
	return account, nil
}

func xAccountFromUser(user map[string]any, endpoint string) (appsessionsdto.AppSessionAccount, error) {
	handle := firstNonEmpty(appSessionString(user, "screen_name"), appSessionString(user, "screenName"))
	displayName := firstNonEmpty(appSessionString(user, "name"), handle)
	avatarURL := normalizeAccountImageURL(firstNonEmpty(
		appSessionString(user, "profile_image_url_https"),
		appSessionString(user, "profile_image_url"),
		appSessionString(user, "profileImageUrl"),
		appSessionString(user, "profileImageURL"),
	))
	userID := firstNonEmpty(appSessionString(user, "id_str"), appSessionString(user, "id"))
	if displayName == "" && handle == "" && avatarURL == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	metadata := map[string]any{"accountEndpoint": endpoint}
	if userID != "" {
		metadata["userID"] = userID
	}
	if appSessionBool(user, "verified") {
		metadata["verified"] = true
	}
	return appsessionsdto.AppSessionAccount{
		DisplayName: displayName,
		Handle:      handle,
		AvatarURL:   avatarURL,
		Metadata:    metadata,
	}, nil
}

func xGraphQLUserResult(data map[string]any) map[string]any {
	for _, path := range [][]string{
		{"data", "viewer", "user_results", "result"},
		{"data", "user", "result"},
		{"viewer", "user_results", "result"},
		{"user", "result"},
	} {
		if result := appSessionNestedMap(data, path...); len(result) > 0 {
			return result
		}
	}
	return nil
}

func xGraphQLEndpoint(endpoint string, variables map[string]any, features map[string]bool, fieldToggles map[string]bool) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return endpoint
	}
	query := parsed.Query()
	if variables == nil {
		variables = map[string]any{}
	}
	query.Set("variables", compactJSONString(variables))
	query.Set("features", compactJSONString(features))
	if len(fieldToggles) > 0 {
		query.Set("fieldToggles", compactJSONString(fieldToggles))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func compactJSONString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func xViewerFeatures() map[string]bool {
	return map[string]bool{
		"subscriptions_upsells_api_enabled":                                 true,
		"profile_label_improvements_pcf_label_in_post_enabled":              true,
		"responsive_web_profile_redirect_enabled":                           true,
		"rweb_tipjar_consumption_enabled":                                   true,
		"verified_phone_label_enabled":                                      false,
		"creator_subscriptions_tweet_preview_api_enabled":                   true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled": false,
		"responsive_web_graphql_timeline_navigation_enabled":                true,
	}
}

func xUserByRestIDFeatures() map[string]bool {
	return map[string]bool{
		"hidden_profile_subscriptions_enabled":                              true,
		"profile_label_improvements_pcf_label_in_post_enabled":              true,
		"responsive_web_profile_redirect_enabled":                           true,
		"rweb_tipjar_consumption_enabled":                                   true,
		"verified_phone_label_enabled":                                      false,
		"highlights_tweets_tab_ui_enabled":                                  true,
		"responsive_web_twitter_article_notes_tab_enabled":                  true,
		"subscriptions_feature_can_gift_premium":                            true,
		"creator_subscriptions_tweet_preview_api_enabled":                   true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled": false,
		"responsive_web_graphql_timeline_navigation_enabled":                true,
	}
}

func xViewerFieldToggles() map[string]bool {
	return map[string]bool{
		"isDelegate":              false,
		"withPayments":            false,
		"withAuxiliaryUserLabels": true,
	}
}

func xUserByRestIDFieldToggles() map[string]bool {
	return map[string]bool{
		"withPayments":            false,
		"withAuxiliaryUserLabels": true,
	}
}

func xUserIDFromTWIDCookie(records []appcookies.Record) string {
	value := appSessionCookieValue(records, []string{"x.com", "twitter.com"}, "twid")
	for i := 0; i < 2; i++ {
		unescaped, err := url.QueryUnescape(value)
		if err != nil || unescaped == value {
			break
		}
		value = unescaped
	}
	value = strings.Trim(value, ` "'`)
	if after, ok := strings.CutPrefix(value, "u="); ok {
		value = after
	}
	return regexp.MustCompile(`\d+`).FindString(value)
}

func xGraphQLError(operation string, err error) error {
	if errors.Is(err, appsessions.ErrNoCookies) {
		return fmt.Errorf("x graphql/%s rejected session", operation)
	}
	return fmt.Errorf("x graphql/%s failed: %w", operation, err)
}

func firstActionableXAccountError(errorsList []error, original error) error {
	for _, err := range errorsList {
		if err != nil && !errors.Is(err, appsessions.ErrNoCookies) {
			return err
		}
	}
	if original != nil {
		return original
	}
	return appsessions.ErrNoCookies
}

func xWebAPIHeaders(csrf string) map[string]string {
	return map[string]string{
		"Authorization":             "Bearer " + xWebBearerToken,
		"Origin":                    "https://x.com",
		"Referer":                   "https://x.com/home",
		"X-CSRF-Token":              csrf,
		"X-Twitter-Active-User":     "yes",
		"X-Twitter-Auth-Type":       "OAuth2Session",
		"X-Twitter-Client-Language": "en",
	}
}

func xPeerEndpoint(currentEndpoint string, fallbackEndpoint string) string {
	fallback, err := url.Parse(fallbackEndpoint)
	if err != nil {
		return fallbackEndpoint
	}
	current, err := url.Parse(strings.TrimSpace(currentEndpoint))
	if err != nil || current.Scheme == "" || current.Host == "" {
		return fallbackEndpoint
	}
	host := strings.ToLower(current.Hostname())
	if strings.HasSuffix(host, "x.com") || strings.HasSuffix(host, "twitter.com") {
		return fallbackEndpoint
	}
	fallback.Scheme = current.Scheme
	fallback.Host = current.Host
	return fallback.String()
}

func xUsersShowEndpoint(endpoint string, handle string) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return endpoint
	}
	query := parsed.Query()
	query.Set("screen_name", handle)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func fetchFacebookAppSessionAccount(ctx context.Context, client *http.Client, records []appcookies.Record) (appsessionsdto.AppSessionAccount, error) {
	return fetchFacebookAppSessionAccountFromURL(ctx, client, records, facebookMeURL)
}

func fetchFacebookAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string) (appsessionsdto.AppSessionAccount, error) {
	userID := appSessionCookieValue(records, []string{"facebook.com"}, "c_user")
	if userID == "" || !appSessionHasAnyCookie(records, []string{"facebook.com"}, "xs") {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}

	pages := make([]facebookAccountPage, 0, 3)
	visited := make(map[string]struct{})
	var lastErr error
	for _, candidate := range facebookAccountCandidateURLs(endpoint, userID) {
		page, err := fetchFacebookAccountPage(ctx, client, records, candidate)
		if err != nil {
			lastErr = err
			if errors.Is(err, appsessions.ErrNoCookies) {
				return appsessionsdto.AppSessionAccount{}, err
			}
			continue
		}
		if _, ok := visited[page.finalURL]; ok {
			continue
		}
		visited[page.finalURL] = struct{}{}
		pages = append(pages, page)
		for _, redirectURL := range extractFacebookRedirectURLs(page.body, page.finalURL) {
			if _, ok := visited[redirectURL]; ok {
				continue
			}
			redirectPage, err := fetchFacebookAccountPage(ctx, client, records, redirectURL)
			if err != nil {
				lastErr = err
				if errors.Is(err, appsessions.ErrNoCookies) {
					return appsessionsdto.AppSessionAccount{}, err
				}
				continue
			}
			visited[redirectPage.finalURL] = struct{}{}
			pages = append(pages, redirectPage)
		}
	}
	if len(pages) == 0 {
		if lastErr != nil {
			return appsessionsdto.AppSessionAccount{}, lastErr
		}
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}

	displayName := ""
	handle := ""
	avatarURL := ""
	accountEndpoint := ""
	for _, page := range pages {
		if accountEndpoint == "" {
			accountEndpoint = facebookEndpointLabel(page.finalURL)
		}
		if displayName == "" {
			displayName = extractFacebookDisplayName(page.body)
		}
		if handle == "" {
			handle = extractFacebookHandle(page.finalURL, page.body, userID)
		}
		if avatarURL == "" {
			avatarURL = extractFacebookAvatarURL(page.body)
		}
	}
	if accountEndpoint == "" {
		accountEndpoint = "www.facebook.com/me"
	}

	return appsessionsdto.AppSessionAccount{
		DisplayName: displayName,
		Handle:      handle,
		AvatarURL:   avatarURL,
		Metadata: map[string]any{
			"userID":          userID,
			"accountEndpoint": accountEndpoint,
		},
	}, nil
}

func fetchVimeoAppSessionAccount(ctx context.Context, client *http.Client, records []appcookies.Record) (appsessionsdto.AppSessionAccount, error) {
	return fetchVimeoAppSessionAccountFromURL(ctx, client, records, vimeoViewerURL)
}

func fetchVimeoAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string) (appsessionsdto.AppSessionAccount, error) {
	if !appSessionHasAnyCookie(records, []string{"vimeo.com"}, "vimeo", "is_logged_in") {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	data, err := appSessionJSON(ctx, client, "vimeo", endpoint, records, []string{"vimeo.com"}, map[string]string{
		"Referer": "https://vimeo.com/",
	})
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	user := appSessionMap(data, "user")
	if len(user) == 0 {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	displayName := firstNonEmpty(
		appSessionString(user, "name"),
		appSessionString(user, "display_name"),
	)
	handle := vimeoHandleFromUser(user)
	avatarURL := normalizeAccountImageURL(firstNonEmpty(
		appSessionNestedString(user, "pictures", "sizes", "link"),
		appSessionNestedString(user, "pictures", "base_link"),
		appSessionString(user, "portrait"),
		appSessionString(user, "avatar_url"),
	))
	userID := firstNonEmpty(appSessionString(user, "id"), appSessionString(user, "uri"))
	if displayName == "" && handle == "" && avatarURL == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	metadata := map[string]any{"accountEndpoint": "_next/viewer"}
	if userID != "" {
		metadata["userID"] = userID
	}
	return appsessionsdto.AppSessionAccount{
		DisplayName: displayName,
		Handle:      handle,
		AvatarURL:   avatarURL,
		Metadata:    metadata,
	}, nil
}

func fetchTwitchAppSessionAccount(ctx context.Context, client *http.Client, records []appcookies.Record) (appsessionsdto.AppSessionAccount, error) {
	return fetchTwitchAppSessionAccountFromURLs(ctx, client, records, twitchValidateURL, twitchUsersURL)
}

func fetchTwitchAppSessionAccountFromURLs(ctx context.Context, client *http.Client, records []appcookies.Record, validateEndpoint string, usersEndpoint string) (appsessionsdto.AppSessionAccount, error) {
	return fetchTwitchAppSessionAccountFromEndpoints(ctx, client, records, validateEndpoint, usersEndpoint, twitchGQLURL)
}

func fetchTwitchAppSessionAccountFromEndpoints(ctx context.Context, client *http.Client, records []appcookies.Record, validateEndpoint string, usersEndpoint string, gqlEndpoint string) (appsessionsdto.AppSessionAccount, error) {
	token := twitchAuthToken(records)
	if token == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	validate, err := appSessionJSON(ctx, client, "twitch", validateEndpoint, records, []string{"twitch.tv"}, map[string]string{
		"Authorization": "OAuth " + token,
		"Referer":       "https://www.twitch.tv/",
	})
	if err != nil {
		if account, fallbackErr := fetchTwitchGQLAppSessionAccountFromURL(ctx, client, records, gqlEndpoint, token); fallbackErr == nil {
			return account, nil
		}
		return appsessionsdto.AppSessionAccount{}, err
	}
	clientID := appSessionString(validate, "client_id")
	userID := appSessionString(validate, "user_id")
	handle := appSessionString(validate, "login")
	if clientID == "" || userID == "" {
		if account, fallbackErr := fetchTwitchGQLAppSessionAccountFromURL(ctx, client, records, gqlEndpoint, token); fallbackErr == nil {
			return account, nil
		}
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	usersURL := usersEndpoint
	if !strings.Contains(usersURL, "?") {
		usersURL += "?id=" + userID
	}
	users, err := appSessionJSON(ctx, client, "twitch", usersURL, records, []string{"twitch.tv"}, map[string]string{
		"Authorization": "Bearer " + token,
		"Client-Id":     clientID,
		"Referer":       "https://www.twitch.tv/",
	})
	if err != nil {
		if account, fallbackErr := fetchTwitchGQLAppSessionAccountFromURL(ctx, client, records, gqlEndpoint, token); fallbackErr == nil {
			return account, nil
		}
		return appsessionsdto.AppSessionAccount{}, err
	}
	user := firstMapInSlice(users["data"])
	if len(user) == 0 {
		if account, fallbackErr := fetchTwitchGQLAppSessionAccountFromURL(ctx, client, records, gqlEndpoint, token); fallbackErr == nil {
			return account, nil
		}
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	displayName := firstNonEmpty(appSessionString(user, "display_name"), appSessionString(user, "displayName"), handle)
	avatarURL := normalizeAccountImageURL(firstNonEmpty(appSessionString(user, "profile_image_url"), appSessionString(user, "profileImageURL")))
	return appsessionsdto.AppSessionAccount{
		DisplayName: displayName,
		Handle:      firstNonEmpty(appSessionString(user, "login"), handle),
		AvatarURL:   avatarURL,
		Metadata: map[string]any{
			"userID":          userID,
			"accountEndpoint": "oauth2/validate + helix/users",
		},
	}, nil
}

func fetchTwitchGQLAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string, token string) (appsessionsdto.AppSessionAccount, error) {
	if strings.TrimSpace(token) == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	payload := map[string]any{
		"operationName": "CurrentUser",
		"variables":     map[string]any{},
		"query":         "query CurrentUser { currentUser { id login displayName profileImageURL(width: 300) } }",
	}
	data, err := appSessionPostJSON(ctx, client, "twitch", endpoint, records, []string{"twitch.tv"}, map[string]string{
		"Authorization": "OAuth " + token,
		"Client-ID":     twitchWebClientID,
		"Origin":        "https://www.twitch.tv",
		"Referer":       "https://www.twitch.tv/",
	}, payload)
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	user := appSessionNestedMap(data, "data", "currentUser")
	if len(user) == 0 {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	handle := appSessionString(user, "login")
	displayName := firstNonEmpty(appSessionString(user, "displayName"), appSessionString(user, "display_name"), handle)
	avatarURL := normalizeAccountImageURL(firstNonEmpty(appSessionString(user, "profileImageURL"), appSessionString(user, "profile_image_url")))
	userID := appSessionString(user, "id")
	if displayName == "" && handle == "" && avatarURL == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	metadata := map[string]any{"accountEndpoint": "gql/currentUser"}
	if userID != "" {
		metadata["userID"] = userID
	}
	return appsessionsdto.AppSessionAccount{
		DisplayName: displayName,
		Handle:      handle,
		AvatarURL:   avatarURL,
		Metadata:    metadata,
	}, nil
}

func twitchAuthToken(records []appcookies.Record) string {
	token := appSessionCookieValue(records, []string{"twitch.tv"}, "auth-token")
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if decoded, err := url.PathUnescape(token); err == nil && strings.TrimSpace(decoded) != "" {
		token = decoded
	}
	return strings.Trim(strings.TrimSpace(token), `"`)
}

func fetchNiconicoAppSessionAccount(ctx context.Context, client *http.Client, records []appcookies.Record) (appsessionsdto.AppSessionAccount, error) {
	return fetchNiconicoAppSessionAccountFromURL(ctx, client, records, niconicoUsersMeURL)
}

func fetchNiconicoAppSessionAccountFromURL(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string) (appsessionsdto.AppSessionAccount, error) {
	if !appSessionHasAnyCookie(records, []string{"nicovideo.jp"}, "user_session") {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	data, err := appSessionJSON(ctx, client, "niconico", endpoint, records, []string{"nicovideo.jp"}, map[string]string{
		"Referer":              "https://www.nicovideo.jp/",
		"X-Frontend-Id":        "6",
		"X-Frontend-Version":   "0",
		"X-Niconico-Language":  "en-us",
		"X-Requested-With":     "XMLHttpRequest",
		"Sec-Fetch-Site":       "same-site",
		"Sec-Fetch-Mode":       "cors",
		"Sec-Fetch-Dest":       "empty",
		"X-Request-With":       "https://www.nicovideo.jp",
		"X-Niconico-With-Cors": "1",
	})
	if err != nil {
		return appsessionsdto.AppSessionAccount{}, err
	}
	meta := appSessionMap(data, "meta")
	if status := appSessionInt(meta, "status"); status == http.StatusUnauthorized || strings.EqualFold(appSessionString(meta, "errorCode"), "UNAUTHORIZED") {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	payload := appSessionMap(data, "data")
	user := appSessionMap(payload, "user")
	if len(user) == 0 {
		user = payload
	}
	displayName := firstNonEmpty(appSessionString(user, "nickname"), appSessionString(user, "name"))
	userID := firstNonEmpty(appSessionString(user, "id"), appSessionString(user, "userId"), appSessionString(user, "userID"))
	avatarURL := normalizeAccountImageURL(firstNonEmpty(
		appSessionNestedString(user, "icons", "large"),
		appSessionNestedString(user, "icons", "small"),
		appSessionString(user, "iconUrl"),
		appSessionString(user, "iconURL"),
	))
	if displayName == "" && userID == "" && avatarURL == "" {
		return appsessionsdto.AppSessionAccount{}, appsessions.ErrNoCookies
	}
	tierKey := ""
	tierLabel := ""
	if appSessionBool(user, "isPremium") || appSessionBool(payload, "isPremium") {
		tierKey = "premium"
		tierLabel = "Premium"
	}
	metadata := map[string]any{"accountEndpoint": "nvapi/v1/users/me"}
	if userID != "" {
		metadata["userID"] = userID
	}
	return appsessionsdto.AppSessionAccount{
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		TierKey:     tierKey,
		TierLabel:   tierLabel,
		Metadata:    metadata,
	}, nil
}

func appSessionJSON(ctx context.Context, client *http.Client, siteKey string, endpoint string, records []appcookies.Record, domains []string, headers map[string]string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", appsessionidentity.HTTPUserAgent(siteKey))
	for key, value := range headers {
		value = strings.TrimSpace(value)
		if value != "" {
			req.Header.Set(key, value)
		}
	}
	appSessionAddCookies(req, records, domains, endpoint)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, appsessions.ErrNoCookies
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, appSessionAccountMaxPayload))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if message := appSessionJSONErrorMessage(data); message != "" {
			return nil, fmt.Errorf("%s account info status %d: %s", siteKey, resp.StatusCode, message)
		}
		return nil, fmt.Errorf("%s account info status %d", siteKey, resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "json") {
		return nil, appsessions.ErrNoCookies
	}
	decoded, err := decodeAppSessionJSONObject(data)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func appSessionPostJSON(ctx context.Context, client *http.Client, siteKey string, endpoint string, records []appcookies.Record, domains []string, headers map[string]string, payload any) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", appsessionidentity.HTTPUserAgent(siteKey))
	for key, value := range headers {
		value = strings.TrimSpace(value)
		if value != "" {
			req.Header.Set(key, value)
		}
	}
	appSessionAddCookies(req, records, domains, endpoint)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, appsessions.ErrNoCookies
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, appSessionAccountMaxPayload))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if message := appSessionJSONErrorMessage(data); message != "" {
			return nil, fmt.Errorf("%s account info status %d: %s", siteKey, resp.StatusCode, message)
		}
		return nil, fmt.Errorf("%s account info status %d", siteKey, resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "json") {
		return nil, appsessions.ErrNoCookies
	}
	decoded, err := decodeAppSessionJSONObject(data)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func decodeAppSessionJSONObject(data []byte) (map[string]any, error) {
	raw := strings.TrimSpace(string(data))
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "for (;;);"))
	raw = strings.TrimSpace(strings.TrimPrefix(raw, ")]}',"))
	var decoded map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func appSessionJSONErrorMessage(data []byte) string {
	decoded, err := decodeAppSessionJSONObject(data)
	if err != nil {
		return ""
	}
	return firstNonEmpty(
		appSessionString(decoded, "message"),
		appSessionString(decoded, "error"),
		appSessionString(decoded, "error_message"),
		appSessionNestedString(decoded, "error", "message"),
	)
}

func appSessionImageDataURL(ctx context.Context, client *http.Client, siteKey string, rawURL string, referer string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		return rawURL, nil
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("User-Agent", appsessionidentity.HTTPUserAgent(siteKey))
	if referer = strings.TrimSpace(referer); referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("%s account avatar status %d", siteKey, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, appSessionAvatarMaxPayload+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", appsessions.ErrNoCookies
	}
	if len(data) > appSessionAvatarMaxPayload {
		return "", fmt.Errorf("%s account avatar too large", siteKey)
	}
	contentType := appSessionImageContentType(resp.Header.Get("Content-Type"), data)
	if contentType == "" {
		return "", fmt.Errorf("%s account avatar content type unsupported", siteKey)
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func appSessionImageContentType(header string, data []byte) string {
	contentType := strings.ToLower(strings.TrimSpace(header))
	if before, _, ok := strings.Cut(contentType, ";"); ok {
		contentType = strings.TrimSpace(before)
	}
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif":
		return contentType
	}
	detected := strings.ToLower(http.DetectContentType(data))
	switch detected {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return detected
	default:
		return ""
	}
}

func appSessionHTML(ctx context.Context, client *http.Client, siteKey string, endpoint string, records []appcookies.Record, domains []string, headers map[string]string) (string, string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", appsessionidentity.HTTPUserAgent(siteKey))
	for key, value := range headers {
		value = strings.TrimSpace(value)
		if value != "" {
			req.Header.Set(key, value)
		}
	}
	appSessionAddCookies(req, records, domains, endpoint)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", "", appsessions.ErrNoCookies
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", "", fmt.Errorf("%s account info status %d", siteKey, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, appSessionAccountMaxPayload))
	if err != nil {
		return "", "", err
	}
	return string(data), resp.Request.URL.String(), nil
}

func appSessionAddCookies(req *http.Request, records []appcookies.Record, domains []string, endpoint string) {
	var pairs []string
	for _, record := range appSessionAccountCookies(records, domains, endpoint) {
		name := strings.TrimSpace(record.Name)
		value := record.Value
		if name == "" || strings.ContainsAny(name, "=\r\n;") || strings.ContainsAny(value, "\r\n;") {
			continue
		}
		pairs = append(pairs, name+"="+value)
	}
	if len(pairs) > 0 {
		req.Header.Set("Cookie", strings.Join(pairs, "; "))
	}
}

func appSessionAccountCookies(records []appcookies.Record, domains []string, endpoint string) []appcookies.Record {
	matched := appcookies.FilterByDomains(records, domains)
	if len(matched) > 0 {
		return matched
	}
	return appcookies.MatchURL(records, endpoint)
}

func appSessionHasAnyCookie(records []appcookies.Record, domains []string, names ...string) bool {
	return appSessionCookieValue(records, domains, names...) != ""
}

func appSessionCookieValue(records []appcookies.Record, domains []string, names ...string) string {
	if len(records) == 0 || len(names) == 0 {
		return ""
	}
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		if trimmed := strings.ToLower(strings.TrimSpace(name)); trimmed != "" {
			wanted[trimmed] = struct{}{}
		}
	}
	for _, record := range appSessionAccountCookies(records, domains, "") {
		if _, ok := wanted[strings.ToLower(strings.TrimSpace(record.Name))]; ok {
			value := strings.TrimSpace(record.Value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func appSessionMap(data map[string]any, key string) map[string]any {
	if data == nil {
		return nil
	}
	value, ok := data[key]
	if !ok {
		return nil
	}
	result, _ := value.(map[string]any)
	return result
}

func appSessionNestedMap(data map[string]any, path ...string) map[string]any {
	var current any = data
	for _, key := range path {
		mapped, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = mapped[key]
	}
	result, _ := current.(map[string]any)
	return result
}

func appSessionString(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	return appSessionAnyString(data[key])
}

func appSessionNestedString(data map[string]any, path ...string) string {
	var current any = data
	for _, key := range path {
		if values, ok := current.([]any); ok {
			if len(values) == 0 {
				return ""
			}
			current = values[len(values)-1]
		}
		mapped, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = mapped[key]
	}
	if values, ok := current.([]any); ok {
		for index := len(values) - 1; index >= 0; index-- {
			if value := appSessionAnyString(values[index]); value != "" {
				return value
			}
		}
		return ""
	}
	return appSessionAnyString(current)
}

func appSessionAnyString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strings.TrimSpace(strconv.FormatFloat(typed, 'f', -1, 64))
	case int64:
		return strconv.FormatInt(typed, 10)
	case int:
		return strconv.Itoa(typed)
	default:
		return ""
	}
}

func appSessionInt(data map[string]any, key string) int {
	if data == nil {
		return 0
	}
	switch value := data[key].(type) {
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	default:
		parsed, _ := strconv.Atoi(appSessionAnyString(value))
		return parsed
	}
}

func appSessionBool(data map[string]any, key string) bool {
	if data == nil {
		return false
	}
	switch value := data[key].(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1", "yes":
			return true
		default:
			return false
		}
	case json.Number:
		parsed, _ := value.Int64()
		return parsed != 0
	case float64:
		return value != 0
	default:
		return false
	}
}

func firstMapInSlice(value any) map[string]any {
	values, ok := value.([]any)
	if !ok || len(values) == 0 {
		return nil
	}
	result, _ := values[0].(map[string]any)
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeAccountImageURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "//") {
		return "https:" + trimmed
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "http://") {
		return "https://" + trimmed[len("http://"):]
	}
	return trimmed
}

func vimeoHandleFromUser(user map[string]any) string {
	link := firstNonEmpty(appSessionString(user, "link"), appSessionString(user, "url"))
	link = strings.TrimRight(link, "/")
	if slash := strings.LastIndex(link, "/"); slash >= 0 && slash < len(link)-1 {
		return link[slash+1:]
	}
	return ""
}

func tiktokAccountPayloadLooksLoggedOut(payload map[string]any) bool {
	if len(payload) == 0 {
		return false
	}
	for _, value := range []string{
		appSessionString(payload, "name"),
		appSessionString(payload, "message"),
		appSessionString(payload, "status_msg"),
		appSessionString(payload, "description"),
	} {
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "session_expired", "login_required", "not login", "not logged in":
			return true
		}
		if strings.Contains(normalized, "session expired") ||
			strings.Contains(normalized, "login required") ||
			strings.Contains(normalized, "not logged in") {
			return true
		}
	}
	return false
}

func instagramAccountPayloadLooksLoggedOut(payload map[string]any) bool {
	if len(payload) == 0 {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(appSessionString(payload, "status")))
	message := strings.ToLower(strings.TrimSpace(appSessionString(payload, "message")))
	if status == "fail" && (message == "login_required" || strings.Contains(message, "login")) {
		return true
	}
	return message == "login_required" || strings.Contains(message, "login required")
}

func facebookAccountCandidateURLs(endpoint string, userID string) []string {
	values := make([]string, 0, 4)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range values {
			if existing == value {
				return
			}
		}
		values = append(values, value)
	}
	add(endpoint)
	endpointHost := ""
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		endpointHost = strings.ToLower(parsed.Hostname())
		profile := *parsed
		profile.Path = "/profile.php"
		profile.RawQuery = "id=" + url.QueryEscape(userID)
		profile.Fragment = ""
		add(profile.String())
	}
	if endpointHost == "" || strings.HasSuffix(endpointHost, "facebook.com") {
		escapedID := url.QueryEscape(userID)
		add("https://m.facebook.com/profile.php?id=" + escapedID)
		add("https://mbasic.facebook.com/profile.php?id=" + escapedID)
	}
	return values
}

func fetchFacebookAccountPage(ctx context.Context, client *http.Client, records []appcookies.Record, endpoint string) (facebookAccountPage, error) {
	body, finalURL, err := appSessionHTML(ctx, client, "facebook", endpoint, records, []string{"facebook.com"}, map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.9",
		"Referer":         "https://www.facebook.com/",
	})
	if err != nil {
		return facebookAccountPage{}, err
	}
	if facebookLooksLoggedOut(finalURL, body) {
		return facebookAccountPage{}, appsessions.ErrNoCookies
	}
	return facebookAccountPage{
		body:     body,
		finalURL: finalURL,
	}, nil
}

func facebookLooksLoggedOut(finalURL string, body string) bool {
	lowerURL := strings.ToLower(finalURL)
	if strings.Contains(lowerURL, "/login") || strings.Contains(lowerURL, "login.php") {
		return true
	}
	lowerBody := strings.ToLower(body)
	return strings.Contains(lowerBody, "name=\"login\"") &&
		strings.Contains(lowerBody, "facebook.com/login")
}

func extractFacebookRedirectURLs(body string, baseURL string) []string {
	values := make([]string, 0, 2)
	for _, pattern := range facebookRedirectPatterns {
		matches := pattern.FindAllStringSubmatch(body, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			if resolved := resolveFacebookURL(match[1], baseURL); resolved != "" {
				values = append(values, resolved)
			}
		}
	}
	return values
}

func extractFacebookDisplayName(body string) string {
	for _, pattern := range facebookNamePatterns {
		matches := pattern.FindStringSubmatch(body)
		if len(matches) < 2 {
			continue
		}
		value := normalizeFacebookDisplayName(matches[1])
		if value != "" {
			return value
		}
	}
	return ""
}

func extractFacebookAvatarURL(body string) string {
	for _, pattern := range facebookAvatarPatterns {
		matches := pattern.FindStringSubmatch(body)
		if len(matches) < 2 {
			continue
		}
		if value := normalizeFacebookAvatarURL(matches[1]); value != "" {
			return value
		}
	}
	return ""
}

func extractFacebookHandle(finalURL string, body string, userID string) string {
	if handle := facebookHandleFromURL(finalURL, userID); handle != "" {
		return handle
	}
	for _, pattern := range facebookURLPatterns {
		matches := pattern.FindStringSubmatch(body)
		if len(matches) < 2 {
			continue
		}
		if handle := facebookHandleFromURL(decodeFacebookText(matches[1]), userID); handle != "" {
			return handle
		}
	}
	return ""
}

func normalizeFacebookDisplayName(value string) string {
	value = decodeFacebookText(value)
	value = strings.TrimSuffix(value, " | Facebook")
	value = strings.TrimSuffix(value, " | Meta")
	value = strings.TrimSpace(value)
	if value == "" || !facebookDisplayNameLooksValid(value) {
		return ""
	}
	return value
}

func facebookDisplayNameLooksValid(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	lower = strings.Trim(lower, ".。!！ ")
	switch lower {
	case "", "facebook", "meta", "redirecting", "login", "log in", "log into facebook",
		"facebook - log in or sign up", "content not available", "page not found":
		return false
	}
	if strings.Contains(lower, "redirecting") ||
		strings.Contains(lower, "log in to facebook") ||
		strings.Contains(lower, "log into facebook") ||
		strings.Contains(lower, "facebook helps you connect") {
		return false
	}
	if _, err := url.ParseRequestURI(value); err == nil && strings.Contains(value, "://") {
		return false
	}
	return true
}

func normalizeFacebookAvatarURL(value string) string {
	value = normalizeAccountImageURL(decodeFacebookText(value))
	value = strings.ReplaceAll(value, `\/`, `/`)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if !strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "http://") {
		return ""
	}
	if strings.Contains(lower, "static.xx.fbcdn.net") ||
		strings.Contains(lower, "/rsrc.php/") {
		return ""
	}
	return value
}

func facebookHandleFromURL(rawURL string, userID string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	path := strings.Trim(parsed.EscapedPath(), "/")
	if path == "" || strings.Contains(path, "/") {
		return ""
	}
	value, err := url.PathUnescape(path)
	if err != nil {
		return ""
	}
	value = strings.TrimSpace(value)
	if value == "" || value == userID || facebookHandleLooksInternal(value) {
		return ""
	}
	return value
}

func facebookHandleLooksInternal(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	switch lower {
	case "me", "profile.php", "login", "login.php", "home.php", "checkpoint", "recover",
		"people", "pages", "groups", "watch", "marketplace", "messages", "notifications":
		return true
	}
	_, err := strconv.ParseInt(lower, 10, 64)
	return err == nil
}

func facebookEndpointLabel(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	path := strings.Trim(parsed.EscapedPath(), "/")
	if path == "" {
		path = "me"
	}
	return host + "/" + path
}

func resolveFacebookURL(rawValue string, baseURL string) string {
	value := decodeFacebookText(rawValue)
	value = strings.ReplaceAll(value, `\/`, `/`)
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "" && parsed.Host != "" {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func decodeFacebookText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, `\/`, `/`)
	var decoded string
	if err := json.NewDecoder(bytes.NewBufferString(`"` + value + `"`)).Decode(&decoded); err == nil {
		value = decoded
	}
	return strings.TrimSpace(html.UnescapeString(value))
}
