package service

import (
	"context"
	"sync"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/connectors"
)

type memoryConnectorRepo struct {
	mu    sync.Mutex
	items map[string]connectors.Connector
}

func newMemoryConnectorRepo(items ...connectors.Connector) *memoryConnectorRepo {
	repo := &memoryConnectorRepo{items: make(map[string]connectors.Connector, len(items))}
	for _, item := range items {
		repo.items[item.ID] = item
	}
	return repo
}

func (repo *memoryConnectorRepo) List(context.Context) ([]connectors.Connector, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	result := make([]connectors.Connector, 0, len(repo.items))
	for _, item := range repo.items {
		result = append(result, item)
	}
	return result, nil
}

func (repo *memoryConnectorRepo) Get(_ context.Context, id string) (connectors.Connector, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	item, ok := repo.items[id]
	if !ok {
		return connectors.Connector{}, connectors.ErrConnectorNotFound
	}
	return item, nil
}

func (repo *memoryConnectorRepo) Save(_ context.Context, connector connectors.Connector) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.items[connector.ID] = connector
	return nil
}

func (repo *memoryConnectorRepo) Delete(_ context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	delete(repo.items, id)
	return nil
}

func TestFinalizeUsesCachedCookiesWhenCDPUnavailable(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:        "connector-youtube",
		Type:      string(connectors.ConnectorYouTube),
		Status:    string(connectors.StatusDisconnected),
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	repo := newMemoryConnectorRepo(connector)
	service := NewConnectorsService(repo)
	service.now = func() time.Time { return now.Add(time.Minute) }
	service.removeAll = nil

	session := &connectorSession{
		ID:                "session-1",
		ConnectorID:       connector.ID,
		ConnectorType:     connector.Type,
		State:             connectorSessionStateRunning,
		LastCookies:       []appcookies.Record{{Name: "SID", Value: "1", Domain: ".youtube.com", Path: "/"}, {Name: "HSID", Value: "2", Domain: ".youtube.com", Path: "/"}},
		LastCookiesAt:     now,
		ConnectorSnapshot: mapConnectorDTO(connector),
		finalizeDone:      make(chan struct{}),
	}
	service.putSession(session)

	result, triggered, err := service.finalizeConnectSession(context.Background(), session.ID, "browser_closed")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if !triggered {
		t.Fatalf("expected finalize to run")
	}
	if !result.Saved {
		t.Fatalf("expected cached cookies to be saved")
	}
	if result.RawCookiesCount != 2 || result.FilteredCookiesCount != 2 {
		t.Fatalf("expected 2 cached cookies, got raw=%d filtered=%d", result.RawCookiesCount, result.FilteredCookiesCount)
	}
	if result.Reason != "browser_closed" {
		t.Fatalf("expected browser_closed reason, got %q", result.Reason)
	}

	saved, err := repo.Get(context.Background(), connector.ID)
	if err != nil {
		t.Fatalf("get saved connector: %v", err)
	}
	if saved.Status != connectors.StatusConnected {
		t.Fatalf("expected connected status, got %q", saved.Status)
	}
	if stored := decodeCookies(saved.CookiesJSON); len(stored) != 2 {
		t.Fatalf("expected 2 stored cookies, got %d", len(stored))
	}
}
