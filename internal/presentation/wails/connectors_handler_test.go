package wails

import (
	"context"
	"testing"

	"xiadown/internal/application/connectors/dto"
	connectorsservice "xiadown/internal/application/connectors/service"
	"xiadown/internal/domain/connectors"
)

type memoryConnectorRepository struct {
	items map[string]connectors.Connector
}

func newMemoryConnectorRepository() *memoryConnectorRepository {
	return &memoryConnectorRepository{items: make(map[string]connectors.Connector)}
}

func (repo *memoryConnectorRepository) List(context.Context) ([]connectors.Connector, error) {
	items := make([]connectors.Connector, 0, len(repo.items))
	for _, item := range repo.items {
		items = append(items, item)
	}
	return items, nil
}

func (repo *memoryConnectorRepository) Get(_ context.Context, id string) (connectors.Connector, error) {
	item, ok := repo.items[id]
	if !ok {
		return connectors.Connector{}, connectors.ErrConnectorNotFound
	}
	return item, nil
}

func (repo *memoryConnectorRepository) Save(_ context.Context, connector connectors.Connector) error {
	repo.items[connector.ID] = connector
	return nil
}

func (repo *memoryConnectorRepository) Delete(_ context.Context, id string) error {
	delete(repo.items, id)
	return nil
}

type countingOnlinePlayerResetter struct {
	count int
}

func (resetter *countingOnlinePlayerResetter) Reset() error {
	resetter.count++
	return nil
}

func TestConnectorsHandlerResetsOnlinePlayerForYouTubeMutations(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryConnectorRepository()
	service := connectorsservice.NewConnectorsService(repo)
	resetter := &countingOnlinePlayerResetter{}
	handler := NewConnectorsHandler(service, nil, resetter)

	if _, err := handler.UpsertConnector(ctx, dto.UpsertConnectorRequest{
		ID:     "connector-youtube",
		Type:   string(connectors.ConnectorYouTube),
		Status: string(connectors.StatusConnected),
	}); err != nil {
		t.Fatalf("upsert youtube connector: %v", err)
	}
	if resetter.count != 1 {
		t.Fatalf("expected youtube upsert to reset player once, got %d", resetter.count)
	}

	if _, err := handler.UpsertConnector(ctx, dto.UpsertConnectorRequest{
		ID:     "connector-bilibili",
		Type:   string(connectors.ConnectorBilibili),
		Status: string(connectors.StatusConnected),
	}); err != nil {
		t.Fatalf("upsert bilibili connector: %v", err)
	}
	if resetter.count != 1 {
		t.Fatalf("expected non-youtube upsert to leave reset count at 1, got %d", resetter.count)
	}

	if err := handler.ClearConnector(ctx, dto.ClearConnectorRequest{ID: "connector-youtube"}); err != nil {
		t.Fatalf("clear youtube connector: %v", err)
	}
	if resetter.count != 2 {
		t.Fatalf("expected youtube clear to reset player twice, got %d", resetter.count)
	}

	if err := handler.ClearConnector(ctx, dto.ClearConnectorRequest{ID: "connector-bilibili"}); err != nil {
		t.Fatalf("clear bilibili connector: %v", err)
	}
	if resetter.count != 2 {
		t.Fatalf("expected non-youtube clear to leave reset count at 2, got %d", resetter.count)
	}
}
