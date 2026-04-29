package service

import (
	"context"
	"testing"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/connectors"
)

func TestCookiesForConnectorType(t *testing.T) {
	cookiesJSON, err := encodeCookies([]appcookies.Record{
		{Name: "SID", Value: "test-sid", Domain: ".youtube.com", Path: "/"},
	})
	if err != nil {
		t.Fatalf("encode cookies: %v", err)
	}
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:          "connector-youtube",
		Type:        string(connectors.ConnectorYouTube),
		Status:      string(connectors.StatusConnected),
		CookiesJSON: cookiesJSON,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	service := NewConnectorsService(newMemoryConnectorRepo(connector))

	records, err := service.CookiesForConnectorType(context.Background(), connectors.ConnectorYouTube)
	if err != nil {
		t.Fatalf("cookies for connector: %v", err)
	}
	if len(records) != 1 || records[0].Name != "SID" || records[0].Value != "test-sid" {
		t.Fatalf("unexpected records: %#v", records)
	}
}
