package browsercdp

import (
	"context"
	"errors"
	"testing"

	connectorsdto "xiadown/internal/application/connectors/dto"
)

type connectorsReaderStub struct {
	items []connectorsdto.Connector
	err   error
}

func (stub connectorsReaderStub) ListConnectors(context.Context) ([]connectorsdto.Connector, error) {
	if stub.err != nil {
		return nil, stub.err
	}
	return append([]connectorsdto.Connector(nil), stub.items...), nil
}

func TestResolveConnectorCookiesForURL_MatchesConnectorPolicy(t *testing.T) {
	t.Parallel()

	cookies, err := ResolveConnectorCookiesForURL(context.Background(), connectorsReaderStub{
		items: []connectorsdto.Connector{
			{
				Type: "youtube",
				Cookies: []connectorsdto.ConnectorCookie{
					{Name: "SID", Value: "youtube-cookie", Domain: ".youtube.com", Path: "/"},
				},
			},
			{
				Type: "bilibili",
				Cookies: []connectorsdto.ConnectorCookie{
					{Name: "SESSDATA", Value: "yes", Domain: ".bilibili.com", Path: "/"},
				},
			},
		},
	}, "https://www.youtube.com/watch?v=test")
	if err != nil {
		t.Fatalf("resolve cookies: %v", err)
	}
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "SID" || cookies[0].Value != "youtube-cookie" {
		t.Fatalf("unexpected cookie: %#v", cookies[0])
	}
}

func TestResolveConnectorCookiesForURL_ReturnsNilWhenNoMatch(t *testing.T) {
	t.Parallel()

	cookies, err := ResolveConnectorCookiesForURL(context.Background(), connectorsReaderStub{
		items: []connectorsdto.Connector{
			{
				Type: "bilibili",
				Cookies: []connectorsdto.ConnectorCookie{
					{Name: "SESSDATA", Value: "yes", Domain: ".bilibili.com", Path: "/"},
				},
			},
		},
	}, "https://example.com/")
	if err != nil {
		t.Fatalf("resolve cookies: %v", err)
	}
	if len(cookies) != 0 {
		t.Fatalf("expected no cookies, got %#v", cookies)
	}
}

func TestResolveConnectorCookiesForURL_PropagatesReaderError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("connectors unavailable")
	_, err := ResolveConnectorCookiesForURL(context.Background(), connectorsReaderStub{
		err: expectedErr,
	}, "https://example.com/repository")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}
