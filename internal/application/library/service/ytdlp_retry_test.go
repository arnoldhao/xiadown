package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type retryLibraryRepo struct {
	mu    sync.RWMutex
	items map[string]library.Library
}

func (repo *retryLibraryRepo) List(_ context.Context) ([]library.Library, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	result := make([]library.Library, 0, len(repo.items))
	for _, item := range repo.items {
		result = append(result, item)
	}
	return result, nil
}

func (repo *retryLibraryRepo) Get(_ context.Context, id string) (library.Library, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	item, ok := repo.items[id]
	if !ok {
		return library.Library{}, library.ErrLibraryNotFound
	}
	return item, nil
}

func (repo *retryLibraryRepo) Save(_ context.Context, item library.Library) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.items == nil {
		repo.items = map[string]library.Library{}
	}
	repo.items[item.ID] = item
	return nil
}

func (repo *retryLibraryRepo) Delete(_ context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	delete(repo.items, id)
	return nil
}

type retryOperationRepo struct {
	mu    sync.RWMutex
	items map[string]library.LibraryOperation
}

func (repo *retryOperationRepo) List(_ context.Context) ([]library.LibraryOperation, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	result := make([]library.LibraryOperation, 0, len(repo.items))
	for _, item := range repo.items {
		result = append(result, item)
	}
	return result, nil
}

func (repo *retryOperationRepo) ListByLibraryID(_ context.Context, libraryID string) ([]library.LibraryOperation, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	result := make([]library.LibraryOperation, 0, len(repo.items))
	for _, item := range repo.items {
		if item.LibraryID == libraryID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (repo *retryOperationRepo) Get(_ context.Context, id string) (library.LibraryOperation, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	item, ok := repo.items[id]
	if !ok {
		return library.LibraryOperation{}, library.ErrOperationNotFound
	}
	return item, nil
}

func (repo *retryOperationRepo) Save(_ context.Context, item library.LibraryOperation) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.items == nil {
		repo.items = map[string]library.LibraryOperation{}
	}
	repo.items[item.ID] = item
	return nil
}

func (repo *retryOperationRepo) Delete(_ context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	delete(repo.items, id)
	return nil
}

type retryHistoryRepo struct {
	mu    sync.RWMutex
	items map[string]library.HistoryRecord
}

func (repo *retryHistoryRepo) ListByLibraryID(_ context.Context, libraryID string) ([]library.HistoryRecord, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	result := make([]library.HistoryRecord, 0, len(repo.items))
	for _, item := range repo.items {
		if item.LibraryID == libraryID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (repo *retryHistoryRepo) Get(_ context.Context, id string) (library.HistoryRecord, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	item, ok := repo.items[id]
	if !ok {
		return library.HistoryRecord{}, library.ErrHistoryRecordNotFound
	}
	return item, nil
}

func (repo *retryHistoryRepo) Save(_ context.Context, item library.HistoryRecord) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.items == nil {
		repo.items = map[string]library.HistoryRecord{}
	}
	repo.items[item.ID] = item
	return nil
}

func (repo *retryHistoryRepo) Delete(_ context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	delete(repo.items, id)
	return nil
}

func (repo *retryHistoryRepo) DeleteByOperationID(_ context.Context, operationID string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for id, item := range repo.items {
		if item.Refs.OperationID == operationID {
			delete(repo.items, id)
		}
	}
	return nil
}

func TestRetryYTDLPOperationReusesExistingLibraryWhenStoredRequestMissesLibraryID(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 25, 9, 0, 0, 0, time.UTC)
	libraryItem := mustNewLibrary(t, "lib-1", now)

	inputJSON, err := json.Marshal(dto.CreateYTDLPJobRequest{
		URL:   "https://example.com/watch?v=1",
		Title: "Example",
	})
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	failedOperation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID:          "op-failed",
		LibraryID:   libraryItem.ID,
		Kind:        "download",
		Status:      string(library.OperationStatusFailed),
		DisplayName: "Example",
		InputJSON:   string(inputJSON),
		OutputJSON:  "{}",
		CreatedAt:   &now,
	})
	if err != nil {
		t.Fatalf("new operation: %v", err)
	}

	libraries := &retryLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}}
	operations := &retryOperationRepo{items: map[string]library.LibraryOperation{failedOperation.ID: failedOperation}}
	histories := &retryHistoryRepo{}
	service := &LibraryService{
		libraries:  libraries,
		operations: operations,
		histories:  histories,
		nowFunc:    func() time.Time { return now },
	}

	retried, err := service.RetryYTDLPOperation(ctx, dto.RetryYTDLPOperationRequest{OperationID: failedOperation.ID})
	if err != nil {
		t.Fatalf("retry operation: %v", err)
	}
	if retried.LibraryID != libraryItem.ID {
		t.Fatalf("expected retried operation to reuse library %q, got %q", libraryItem.ID, retried.LibraryID)
	}

	storedOperation, err := operations.Get(ctx, retried.ID)
	if err != nil {
		t.Fatalf("get retried operation: %v", err)
	}
	storedRequest := dto.CreateYTDLPJobRequest{}
	if err := json.Unmarshal([]byte(storedOperation.InputJSON), &storedRequest); err != nil {
		t.Fatalf("unmarshal retried input: %v", err)
	}
	if storedRequest.LibraryID != libraryItem.ID {
		t.Fatalf("expected retried input library %q, got %q", libraryItem.ID, storedRequest.LibraryID)
	}
}

func TestScheduleAutoRetryYTDLPReusesExistingLibrary(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 25, 9, 30, 0, 0, time.UTC)
	libraryItem := mustNewLibrary(t, "lib-1", now)
	failedOperation, err := library.NewLibraryOperation(library.LibraryOperationParams{
		ID:          "op-failed",
		LibraryID:   libraryItem.ID,
		Kind:        "download",
		Status:      string(library.OperationStatusFailed),
		DisplayName: "Example",
		InputJSON:   "{}",
		OutputJSON:  "{}",
		CreatedAt:   &now,
	})
	if err != nil {
		t.Fatalf("new operation: %v", err)
	}

	libraries := &retryLibraryRepo{items: map[string]library.Library{libraryItem.ID: libraryItem}}
	operations := &retryOperationRepo{items: map[string]library.LibraryOperation{failedOperation.ID: failedOperation}}
	histories := &retryHistoryRepo{}
	service := &LibraryService{
		libraries:  libraries,
		operations: operations,
		histories:  histories,
		nowFunc:    func() time.Time { return now },
	}

	newOperationID, ok := service.scheduleAutoRetryYTDLP(ctx, failedOperation, dto.CreateYTDLPJobRequest{
		URL:   "https://example.com/watch?v=1",
		Title: "Example",
	}, "timeout")
	if !ok {
		t.Fatalf("expected auto retry to be scheduled")
	}

	storedOperation, err := operations.Get(ctx, newOperationID)
	if err != nil {
		t.Fatalf("get auto retried operation: %v", err)
	}
	if storedOperation.LibraryID != libraryItem.ID {
		t.Fatalf("expected auto retried operation to reuse library %q, got %q", libraryItem.ID, storedOperation.LibraryID)
	}

	storedRequest := dto.CreateYTDLPJobRequest{}
	if err := json.Unmarshal([]byte(storedOperation.InputJSON), &storedRequest); err != nil {
		t.Fatalf("unmarshal auto retried input: %v", err)
	}
	if storedRequest.LibraryID != libraryItem.ID {
		t.Fatalf("expected auto retried input library %q, got %q", libraryItem.ID, storedRequest.LibraryID)
	}
}
