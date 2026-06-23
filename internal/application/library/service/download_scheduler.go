package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
	domainsettings "xiadown/internal/domain/settings"
)

func (service *LibraryService) NotifyDownloadScheduler() {
	service.signalDownloadScheduler()
}

func (service *LibraryService) signalDownloadScheduler() {
	if service == nil || service.operations == nil {
		return
	}
	service.downloadSchedulerMu.Lock()
	if service.downloadSchedulerRunning {
		service.downloadSchedulerPending = true
		service.downloadSchedulerMu.Unlock()
		return
	}
	service.downloadSchedulerRunning = true
	service.downloadSchedulerMu.Unlock()
	go service.runDownloadScheduler(context.Background())
}

func (service *LibraryService) runDownloadScheduler(ctx context.Context) {
	for {
		service.drainDownloadScheduler(ctx)
		service.downloadSchedulerMu.Lock()
		if service.downloadSchedulerPending {
			service.downloadSchedulerPending = false
			service.downloadSchedulerMu.Unlock()
			continue
		}
		service.downloadSchedulerRunning = false
		service.downloadSchedulerMu.Unlock()
		return
	}
}

func (service *LibraryService) drainDownloadScheduler(ctx context.Context) {
	for {
		limit := service.resolveYTDLPConcurrentDownloads(ctx)
		if !service.downloadSchedulerHasCapacity(limit) {
			return
		}
		operation, history, request, ok := service.nextQueuedYTDLPDownload(ctx)
		if !ok {
			return
		}
		if !service.reserveDownloadSchedulerSlot(operation.ID, limit) {
			return
		}
		go service.runScheduledDownloadOperation(operation, history, request)
	}
}

func (service *LibraryService) downloadSchedulerHasCapacity(limit int) bool {
	service.downloadSchedulerMu.Lock()
	defer service.downloadSchedulerMu.Unlock()
	if service.downloadSchedulerActive == nil {
		service.downloadSchedulerActive = make(map[string]struct{})
	}
	return len(service.downloadSchedulerActive) < limit
}

func (service *LibraryService) reserveDownloadSchedulerSlot(operationID string, limit int) bool {
	trimmed := strings.TrimSpace(operationID)
	if trimmed == "" {
		return false
	}
	service.downloadSchedulerMu.Lock()
	defer service.downloadSchedulerMu.Unlock()
	if service.downloadSchedulerActive == nil {
		service.downloadSchedulerActive = make(map[string]struct{})
	}
	if len(service.downloadSchedulerActive) >= limit {
		return false
	}
	if _, exists := service.downloadSchedulerActive[trimmed]; exists {
		return false
	}
	service.downloadSchedulerActive[trimmed] = struct{}{}
	return true
}

func (service *LibraryService) releaseDownloadSchedulerSlot(operationID string) {
	trimmed := strings.TrimSpace(operationID)
	if trimmed == "" {
		return
	}
	service.downloadSchedulerMu.Lock()
	delete(service.downloadSchedulerActive, trimmed)
	service.downloadSchedulerMu.Unlock()
}

func (service *LibraryService) runScheduledDownloadOperation(operation library.LibraryOperation, history library.HistoryRecord, request dto.CreateYTDLPJobRequest) {
	defer func() {
		service.releaseDownloadSchedulerSlot(operation.ID)
		service.signalDownloadScheduler()
	}()
	service.runDownloadOperation(context.Background(), operation, history, request)
}

func (service *LibraryService) nextQueuedYTDLPDownload(ctx context.Context) (library.LibraryOperation, library.HistoryRecord, dto.CreateYTDLPJobRequest, bool) {
	items, err := service.operations.List(ctx)
	if err != nil {
		return library.LibraryOperation{}, library.HistoryRecord{}, dto.CreateYTDLPJobRequest{}, false
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	for _, item := range items {
		if item.Kind != "download" || item.Status != library.OperationStatusQueued {
			continue
		}
		if service.isDownloadSchedulerActive(item.ID) {
			continue
		}
		request := dto.CreateYTDLPJobRequest{}
		if err := json.Unmarshal([]byte(item.InputJSON), &request); err != nil {
			continue
		}
		if isResourceDownloadRequest(request) {
			continue
		}
		latest, err := service.operations.Get(ctx, item.ID)
		if err != nil || latest.Status != library.OperationStatusQueued {
			continue
		}
		history, err := service.findOrRebuildOperationHistory(ctx, latest, request)
		if err != nil {
			continue
		}
		return latest, history, withYTDLPOperationLibrary(request, latest), true
	}
	return library.LibraryOperation{}, library.HistoryRecord{}, dto.CreateYTDLPJobRequest{}, false
}

func (service *LibraryService) isDownloadSchedulerActive(operationID string) bool {
	trimmed := strings.TrimSpace(operationID)
	if trimmed == "" {
		return false
	}
	service.downloadSchedulerMu.Lock()
	defer service.downloadSchedulerMu.Unlock()
	_, exists := service.downloadSchedulerActive[trimmed]
	return exists
}

func (service *LibraryService) resolveYTDLPConcurrentDownloads(ctx context.Context) int {
	if service == nil || service.settings == nil {
		return domainsettings.DefaultYTDLPConcurrentDownloads
	}
	current, err := service.settings.GetSettings(ctx)
	if err != nil {
		return domainsettings.DefaultYTDLPConcurrentDownloads
	}
	value := current.YTDLPConcurrentDownloads
	if value <= 0 {
		return domainsettings.DefaultYTDLPConcurrentDownloads
	}
	if value < domainsettings.MinYTDLPConcurrentDownloads {
		return domainsettings.MinYTDLPConcurrentDownloads
	}
	if value > domainsettings.MaxYTDLPConcurrentDownloads {
		return domainsettings.MaxYTDLPConcurrentDownloads
	}
	return value
}
