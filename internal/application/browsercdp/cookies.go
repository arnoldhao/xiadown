package browsercdp

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"

	appcookies "xiadown/internal/application/cookies"
)

func SetCookies(ctx context.Context, targetURL string, records []appcookies.Record) error {
	params := buildCookieParams(targetURL, records)
	if len(params) == 0 {
		return nil
	}
	return network.SetCookies(params).Do(ctx)
}

func SetCookiesOnBrowser(ctx context.Context, targetURL string, records []appcookies.Record) error {
	params := buildCookieParams(targetURL, records)
	if len(params) == 0 {
		return nil
	}
	return storage.SetCookies(params).Do(ctx)
}

func GetAllCookies(ctx context.Context) ([]appcookies.Record, error) {
	items, err := network.GetCookies().Do(ctx)
	if err != nil {
		return nil, err
	}
	return mapCDPCookies(items), nil
}

func GetStorageCookies(ctx context.Context) ([]appcookies.Record, error) {
	items, err := storage.GetCookies().Do(ctx)
	if err != nil {
		return nil, err
	}
	return mapCDPCookies(items), nil
}

func buildCookieParams(targetURL string, records []appcookies.Record) []*network.CookieParam {
	if len(records) == 0 {
		return nil
	}
	params := make([]*network.CookieParam, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.Name) == "" {
			continue
		}
		param := &network.CookieParam{
			Name:     strings.TrimSpace(record.Name),
			Value:    record.Value,
			Domain:   strings.TrimSpace(record.Domain),
			Path:     strings.TrimSpace(record.Path),
			HTTPOnly: record.HttpOnly,
			Secure:   record.Secure,
		}
		if param.Path == "" {
			param.Path = "/"
		}
		if record.Expires > 0 {
			expires := cdp.TimeSinceEpoch(time.Unix(record.Expires, 0))
			param.Expires = &expires
		}
		if param.Domain == "" {
			if parsed, err := url.Parse(strings.TrimSpace(targetURL)); err == nil && parsed.Hostname() != "" {
				param.URL = strings.TrimSpace(parsed.Scheme) + "://" + parsed.Hostname()
			}
		}
		switch strings.ToLower(strings.TrimSpace(record.SameSite)) {
		case "lax":
			param.SameSite = network.CookieSameSiteLax
		case "strict":
			param.SameSite = network.CookieSameSiteStrict
		case "none":
			param.SameSite = network.CookieSameSiteNone
		}
		params = append(params, param)
	}
	return params
}

func mapCDPCookies(items []*network.Cookie) []appcookies.Record {
	result := make([]appcookies.Record, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		sameSite := ""
		switch item.SameSite {
		case network.CookieSameSiteLax:
			sameSite = "lax"
		case network.CookieSameSiteStrict:
			sameSite = "strict"
		case network.CookieSameSiteNone:
			sameSite = "none"
		}
		result = append(result, appcookies.Record{
			Name:     item.Name,
			Value:    item.Value,
			Domain:   item.Domain,
			Path:     item.Path,
			Expires:  int64(item.Expires),
			HttpOnly: item.HTTPOnly,
			Secure:   item.Secure,
			SameSite: sameSite,
		})
	}
	return result
}
