package service

import (
	"context"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type workspaceStateLibraryRepoStub struct {
	item library.Library
}

func (repo *workspaceStateLibraryRepoStub) List(_ context.Context) ([]library.Library, error) {
	return []library.Library{repo.item}, nil
}

func (repo *workspaceStateLibraryRepoStub) Get(_ context.Context, id string) (library.Library, error) {
	if repo.item.ID != id {
		return library.Library{}, library.ErrLibraryNotFound
	}
	return repo.item, nil
}

func (repo *workspaceStateLibraryRepoStub) Save(_ context.Context, item library.Library) error {
	repo.item = item
	return nil
}

func (repo *workspaceStateLibraryRepoStub) Delete(_ context.Context, _ string) error {
	return nil
}

type workspaceStateRepoStub struct {
	items []library.WorkspaceStateRecord
	head  map[string]library.WorkspaceStateRecord
}

func (repo *workspaceStateRepoStub) ListByLibraryID(_ context.Context, libraryID string) ([]library.WorkspaceStateRecord, error) {
	result := make([]library.WorkspaceStateRecord, 0, len(repo.items))
	for _, item := range repo.items {
		if item.LibraryID == libraryID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (repo *workspaceStateRepoStub) GetHeadByLibraryID(_ context.Context, libraryID string) (library.WorkspaceStateRecord, error) {
	if repo.head == nil {
		return library.WorkspaceStateRecord{}, library.ErrWorkspaceStateNotFound
	}
	item, ok := repo.head[libraryID]
	if !ok {
		return library.WorkspaceStateRecord{}, library.ErrWorkspaceStateNotFound
	}
	return item, nil
}

func (repo *workspaceStateRepoStub) Save(_ context.Context, item library.WorkspaceStateRecord) error {
	repo.items = append(repo.items, item)
	if repo.head == nil {
		repo.head = map[string]library.WorkspaceStateRecord{}
	}
	repo.head[item.LibraryID] = item
	return nil
}

func TestSaveWorkspaceStateTracksLatestHead(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	libraryItem, err := library.NewLibrary(library.LibraryParams{
		ID:   "library-1",
		Name: "Library 1",
		CreatedBy: library.CreateMeta{
			Source: "test",
		},
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new library: %v", err)
	}

	libraries := &workspaceStateLibraryRepoStub{item: libraryItem}
	workspace := &workspaceStateRepoStub{}
	service := &LibraryService{
		libraries: libraries,
		workspace: workspace,
		nowFunc: func() time.Time {
			return now
		},
	}

	first, err := service.SaveWorkspaceState(context.Background(), dto.SaveWorkspaceStateRequest{
		LibraryID: "library-1",
		StateJSON: `{"version":1,"activeEditor":"video"}`,
	})
	if err != nil {
		t.Fatalf("save first workspace state: %v", err)
	}
	second, err := service.SaveWorkspaceState(context.Background(), dto.SaveWorkspaceStateRequest{
		LibraryID: "library-1",
		StateJSON: `{"version":1,"activeEditor":"subtitle"}`,
	})
	if err != nil {
		t.Fatalf("save second workspace state: %v", err)
	}
	if first.StateVersion != 1 {
		t.Fatalf("expected first state version 1, got %d", first.StateVersion)
	}
	if second.StateVersion != 2 {
		t.Fatalf("expected second state version 2, got %d", second.StateVersion)
	}

	head, err := service.GetWorkspaceState(context.Background(), dto.GetWorkspaceStateRequest{LibraryID: "library-1"})
	if err != nil {
		t.Fatalf("get workspace head: %v", err)
	}
	if head.ID != second.ID {
		t.Fatalf("expected head id %q, got %q", second.ID, head.ID)
	}
	if head.StateVersion != 2 {
		t.Fatalf("expected head version 2, got %d", head.StateVersion)
	}
	if head.StateJSON != second.StateJSON {
		t.Fatalf("expected latest state json %q, got %q", second.StateJSON, head.StateJSON)
	}
	if libraries.item.UpdatedAt != now {
		t.Fatalf("expected library updated_at %s, got %s", now, libraries.item.UpdatedAt)
	}
}

func TestGetWorkspaceStateReturnsEmptyWhenHeadMissing(t *testing.T) {
	t.Parallel()

	service := &LibraryService{
		workspace: &workspaceStateRepoStub{},
	}

	result, err := service.GetWorkspaceState(context.Background(), dto.GetWorkspaceStateRequest{
		LibraryID: "library-empty",
	})
	if err != nil {
		t.Fatalf("get workspace state without head: %v", err)
	}
	if result.LibraryID != "library-empty" {
		t.Fatalf("expected library id %q, got %q", "library-empty", result.LibraryID)
	}
	if result.ID != "" {
		t.Fatalf("expected empty id, got %q", result.ID)
	}
	if result.StateJSON != "" {
		t.Fatalf("expected empty state json, got %q", result.StateJSON)
	}
	if result.StateVersion != 0 {
		t.Fatalf("expected state version 0, got %d", result.StateVersion)
	}
}
