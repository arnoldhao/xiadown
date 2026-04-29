package browsercdp

import (
	"context"
	"sort"
	"strings"

	connectorsdto "xiadown/internal/application/connectors/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/sitepolicy"
)

type ConnectorsReader interface {
	ListConnectors(ctx context.Context) ([]connectorsdto.Connector, error)
}

type ConnectorCookieProvider interface {
	ResolveCookiesForURL(ctx context.Context, rawURL string) ([]appcookies.Record, error)
}

type ConnectorCookieProviderFunc func(ctx context.Context, rawURL string) ([]appcookies.Record, error)

func (fn ConnectorCookieProviderFunc) ResolveCookiesForURL(ctx context.Context, rawURL string) ([]appcookies.Record, error) {
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, rawURL)
}

func ResolveConnectorCookiesForURL(ctx context.Context, connectors ConnectorsReader, rawURL string) ([]appcookies.Record, error) {
	if connectors == nil {
		return nil, nil
	}
	items, err := connectors.ListConnectors(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		policy, ok := sitepolicy.ForConnectorType(item.Type)
		if !ok || !sitepolicy.MatchDomains(rawURL, policy.Domains) {
			continue
		}
		records := make([]appcookies.Record, 0, len(item.Cookies))
		for _, cookie := range item.Cookies {
			records = append(records, appcookies.Record{
				Name:     strings.TrimSpace(cookie.Name),
				Value:    cookie.Value,
				Domain:   strings.TrimSpace(cookie.Domain),
				Path:     strings.TrimSpace(cookie.Path),
				Expires:  cookie.Expires,
				HttpOnly: cookie.HttpOnly,
				Secure:   cookie.Secure,
				SameSite: strings.TrimSpace(cookie.SameSite),
			})
		}
		sort.Slice(records, func(i, j int) bool {
			left := records[i]
			right := records[j]
			switch {
			case left.Domain != right.Domain:
				return left.Domain < right.Domain
			case left.Path != right.Path:
				return left.Path < right.Path
			default:
				return left.Name < right.Name
			}
		})
		return records, nil
	}
	return nil, nil
}

func ConnectorTypeForURL(rawURL string) string {
	policy, ok := sitepolicy.ForURL(rawURL)
	if !ok {
		return ""
	}
	return policy.ConnectorType
}
