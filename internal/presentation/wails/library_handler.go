package wails

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/application/library/service"
)

type LibraryHandler struct {
	service *service.LibraryService
	windows *WindowManager
}

func NewLibraryHandler(service *service.LibraryService, windows *WindowManager) *LibraryHandler {
	return &LibraryHandler{service: service, windows: windows}
}

func (handler *LibraryHandler) ServiceName() string {
	return "LibraryHandler"
}

func (handler *LibraryHandler) ListLibraries(ctx context.Context) ([]dto.LibraryDTO, error) {
	return handler.service.ListLibraries(ctx)
}

func (handler *LibraryHandler) GetLibrary(ctx context.Context, request dto.GetLibraryRequest) (dto.LibraryDTO, error) {
	return handler.service.GetLibrary(ctx, request)
}

func (handler *LibraryHandler) RenameLibrary(ctx context.Context, request dto.RenameLibraryRequest) (dto.LibraryDTO, error) {
	return handler.service.RenameLibrary(ctx, request)
}

func (handler *LibraryHandler) DeleteLibrary(ctx context.Context, request dto.DeleteLibraryRequest) error {
	return handler.service.DeleteLibrary(ctx, request)
}

func (handler *LibraryHandler) GetModuleConfig(ctx context.Context) (dto.LibraryModuleConfigDTO, error) {
	return handler.service.GetModuleConfig(ctx)
}

func (handler *LibraryHandler) GetDefaultModuleConfig(ctx context.Context) (dto.LibraryModuleConfigDTO, error) {
	return handler.service.GetDefaultModuleConfig(ctx)
}

func (handler *LibraryHandler) UpdateModuleConfig(ctx context.Context, request dto.UpdateLibraryModuleConfigRequest) (dto.LibraryModuleConfigDTO, error) {
	return handler.service.UpdateModuleConfig(ctx, request)
}

func (handler *LibraryHandler) ListOperations(ctx context.Context, request dto.ListOperationsRequest) ([]dto.OperationListItemDTO, error) {
	return handler.service.ListOperations(ctx, request)
}

func (handler *LibraryHandler) GetOperation(ctx context.Context, request dto.GetOperationRequest) (dto.LibraryOperationDTO, error) {
	return handler.service.GetOperation(ctx, request)
}

func (handler *LibraryHandler) RenameOperation(ctx context.Context, request dto.RenameOperationRequest) (dto.LibraryOperationDTO, error) {
	return handler.service.RenameOperation(ctx, request)
}

func (handler *LibraryHandler) RenameFile(ctx context.Context, request dto.RenameFileRequest) (dto.LibraryFileDTO, error) {
	return handler.service.RenameFile(ctx, request)
}

func (handler *LibraryHandler) ListMissingLibraryFiles(ctx context.Context) (dto.ListMissingLibraryFilesResponse, error) {
	return handler.service.ListMissingLibraryFiles(ctx)
}

func (handler *LibraryHandler) ScanMissingLibraryFiles(ctx context.Context, request dto.ScanMissingLibraryFilesRequest) (dto.ScanMissingLibraryFilesResponse, error) {
	return handler.service.ScanMissingLibraryFiles(ctx, request)
}

func (handler *LibraryHandler) ApplyLibraryRelinks(ctx context.Context, request dto.ApplyLibraryRelinksRequest) (dto.ApplyLibraryRelinksResponse, error) {
	return handler.service.ApplyLibraryRelinks(ctx, request)
}

func (handler *LibraryHandler) ListMissingListenLocalFiles(ctx context.Context) (dto.ListMissingLibraryFilesResponse, error) {
	return handler.service.ListMissingListenLocalFiles(ctx)
}

func (handler *LibraryHandler) ScanMissingListenLocalFiles(ctx context.Context, request dto.ScanMissingLibraryFilesRequest) (dto.ScanMissingLibraryFilesResponse, error) {
	return handler.service.ScanMissingListenLocalFiles(ctx, request)
}

func (handler *LibraryHandler) ApplyListenLocalRelinks(ctx context.Context, request dto.ApplyLibraryRelinksRequest) (dto.ApplyLibraryRelinksResponse, error) {
	return handler.service.ApplyListenLocalRelinks(ctx, request)
}

func (handler *LibraryHandler) SelectLibraryDirectory(_ context.Context, title string, initialPath string) (string, error) {
	if handler == nil || handler.windows == nil {
		return "", fmt.Errorf("window manager not available")
	}
	return handler.windows.SelectMainDirectoryDialog(strings.TrimSpace(title), initialDirectoryFromPath(initialPath))
}

func (handler *LibraryHandler) CancelOperation(ctx context.Context, request dto.CancelOperationRequest) (dto.LibraryOperationDTO, error) {
	return handler.service.CancelOperation(ctx, request)
}

func (handler *LibraryHandler) ResumeOperation(ctx context.Context, request dto.ResumeOperationRequest) (dto.LibraryOperationDTO, error) {
	return handler.service.ResumeOperation(ctx, request)
}

func (handler *LibraryHandler) DeleteOperation(ctx context.Context, request dto.DeleteOperationRequest) error {
	return handler.service.DeleteOperation(ctx, request)
}

func (handler *LibraryHandler) DeleteOperations(ctx context.Context, request dto.DeleteOperationsRequest) error {
	return handler.service.DeleteOperations(ctx, request)
}

func (handler *LibraryHandler) DeleteFile(ctx context.Context, request dto.DeleteFileRequest) error {
	return handler.service.DeleteFile(ctx, request)
}

func (handler *LibraryHandler) DeleteFiles(ctx context.Context, request dto.DeleteFilesRequest) error {
	return handler.service.DeleteFiles(ctx, request)
}

func (handler *LibraryHandler) ListLibraryHistory(ctx context.Context, request dto.ListLibraryHistoryRequest) ([]dto.LibraryHistoryRecordDTO, error) {
	return handler.service.ListLibraryHistory(ctx, request)
}

func (handler *LibraryHandler) ListFileEvents(ctx context.Context, request dto.ListFileEventsRequest) ([]dto.FileEventRecordDTO, error) {
	return handler.service.ListFileEvents(ctx, request)
}

func (handler *LibraryHandler) SaveWorkspaceState(ctx context.Context, request dto.SaveWorkspaceStateRequest) (dto.WorkspaceStateRecordDTO, error) {
	return handler.service.SaveWorkspaceState(ctx, request)
}

func (handler *LibraryHandler) GetWorkspaceState(ctx context.Context, request dto.GetWorkspaceStateRequest) (dto.WorkspaceStateRecordDTO, error) {
	return handler.service.GetWorkspaceState(ctx, request)
}

func (handler *LibraryHandler) GetWorkspaceProject(ctx context.Context, request dto.GetWorkspaceProjectRequest) (dto.WorkspaceProjectDTO, error) {
	return handler.service.GetWorkspaceProject(ctx, request)
}

func (handler *LibraryHandler) OpenFileLocation(ctx context.Context, request dto.OpenFileLocationRequest) error {
	return handler.service.OpenFileLocation(ctx, request)
}

func (handler *LibraryHandler) OpenPath(ctx context.Context, request dto.OpenPathRequest) error {
	return handler.service.OpenPath(ctx, request)
}

func (handler *LibraryHandler) PrepareYTDLPDownload(ctx context.Context, request dto.PrepareYTDLPDownloadRequest) (dto.PrepareYTDLPDownloadResponse, error) {
	return handler.service.PrepareYTDLPDownload(ctx, request)
}

func (handler *LibraryHandler) ResolveDomainIcon(ctx context.Context, request dto.ResolveDomainIconRequest) (dto.ResolveDomainIconResponse, error) {
	return handler.service.ResolveDomainIcon(ctx, request)
}

func (handler *LibraryHandler) ParseYTDLPDownload(ctx context.Context, request dto.ParseYTDLPDownloadRequest) (dto.ParseYTDLPDownloadResponse, error) {
	return handler.service.ParseYTDLPDownload(ctx, request)
}

func (handler *LibraryHandler) StartResourceSniff(ctx context.Context, request dto.StartResourceSniffRequest) (dto.StartResourceSniffResult, error) {
	return handler.service.StartResourceSniff(ctx, request)
}

func (handler *LibraryHandler) GetResourceSniffSession(ctx context.Context, request dto.GetResourceSniffSessionRequest) (dto.ResourceSniffSession, error) {
	return handler.service.GetResourceSniffSession(ctx, request)
}

func (handler *LibraryHandler) ListResourceSniffSessions(ctx context.Context) ([]dto.ResourceSniffSession, error) {
	return handler.service.ListResourceSniffSessions(ctx)
}

func (handler *LibraryHandler) ListResourceSniffResources(ctx context.Context, request dto.ListResourceSniffResourcesRequest) (dto.ListResourceSniffResourcesResponse, error) {
	return handler.service.ListResourceSniffResources(ctx, request)
}

func (handler *LibraryHandler) ClearResourceSniffResources(ctx context.Context, request dto.ClearResourceSniffResourcesRequest) error {
	return handler.service.ClearResourceSniffResources(ctx, request)
}

func (handler *LibraryHandler) GetResourceSniffPreview(ctx context.Context, request dto.GetResourceSniffPreviewRequest) (dto.ResourceSniffPreviewResponse, error) {
	return handler.service.GetResourceSniffPreview(ctx, request)
}

func (handler *LibraryHandler) PrepareResourceSniffRawPreview(ctx context.Context, request dto.PrepareResourceSniffRawPreviewRequest) (dto.PrepareResourceSniffRawPreviewResponse, error) {
	return handler.service.PrepareResourceSniffRawPreview(ctx, request)
}

func (handler *LibraryHandler) PrepareResourceSniffRawDownload(ctx context.Context, request dto.PrepareResourceSniffRawDownloadRequest) (dto.ParseYTDLPDownloadResponse, error) {
	return handler.service.PrepareResourceSniffRawDownload(ctx, request)
}

func (handler *LibraryHandler) ParseResourceSniff(ctx context.Context, request dto.ParseResourceSniffRequest) (dto.ParseResourceSniffResponse, error) {
	return handler.service.ParseResourceSniff(ctx, request)
}

func (handler *LibraryHandler) CancelResourceSniff(ctx context.Context, request dto.CancelResourceSniffRequest) error {
	return handler.service.CancelResourceSniff(ctx, request)
}

func (handler *LibraryHandler) GetCDPBrowserStatus(ctx context.Context) (dto.CDPBrowserStatus, error) {
	return handler.service.GetCDPBrowserStatus(ctx)
}

func (handler *LibraryHandler) StopCDPBrowserRuntime(ctx context.Context, request dto.StopCDPBrowserRuntimeRequest) error {
	return handler.service.StopCDPBrowserRuntime(ctx, request)
}

func (handler *LibraryHandler) CreateYTDLPJob(ctx context.Context, request dto.CreateYTDLPJobRequest) (dto.LibraryOperationDTO, error) {
	return handler.service.CreateYTDLPJob(ctx, request)
}

func (handler *LibraryHandler) CreateYTDLPBatchJobs(ctx context.Context, request dto.CreateYTDLPBatchJobsRequest) (dto.CreateYTDLPBatchJobsResponse, error) {
	return handler.service.CreateYTDLPBatchJobs(ctx, request)
}

func (handler *LibraryHandler) CheckYTDLPOperationFailure(ctx context.Context, request dto.CheckYTDLPOperationFailureRequest) (dto.CheckYTDLPOperationFailureResponse, error) {
	return handler.service.CheckYTDLPOperationFailure(ctx, request)
}

func (handler *LibraryHandler) RetryYTDLPOperation(ctx context.Context, request dto.RetryYTDLPOperationRequest) (dto.LibraryOperationDTO, error) {
	return handler.service.RetryYTDLPOperation(ctx, request)
}

func (handler *LibraryHandler) GetYTDLPOperationLog(ctx context.Context, request dto.GetYTDLPOperationLogRequest) (dto.GetYTDLPOperationLogResponse, error) {
	return handler.service.GetYTDLPOperationLog(ctx, request)
}

func (handler *LibraryHandler) CreateVideoImport(ctx context.Context, request dto.CreateVideoImportRequest) (dto.LibraryFileDTO, error) {
	return handler.service.CreateVideoImport(ctx, request)
}

func (handler *LibraryHandler) CreateTranscodeJob(ctx context.Context, request dto.CreateTranscodeJobRequest) (dto.LibraryOperationDTO, error) {
	return handler.service.CreateTranscodeJob(ctx, request)
}

func (handler *LibraryHandler) ProbeTranscodeInput(ctx context.Context, request dto.ProbeTranscodeInputRequest) (dto.ProbeTranscodeInputResponse, error) {
	return handler.service.ProbeTranscodeInput(ctx, request)
}

func (handler *LibraryHandler) ListTranscodePresets(ctx context.Context) ([]dto.TranscodePreset, error) {
	return handler.service.ListTranscodePresets(ctx)
}

func (handler *LibraryHandler) ListTranscodePresetsForDownload(ctx context.Context, request dto.ListTranscodePresetsForDownloadRequest) ([]dto.TranscodePreset, error) {
	return handler.service.ListTranscodePresetsForDownload(ctx, request)
}

func (handler *LibraryHandler) SaveTranscodePreset(ctx context.Context, preset dto.TranscodePreset) (dto.TranscodePreset, error) {
	return handler.service.SaveTranscodePreset(ctx, preset)
}

func (handler *LibraryHandler) DeleteTranscodePreset(ctx context.Context, request dto.DeleteTranscodePresetRequest) error {
	return handler.service.DeleteTranscodePreset(ctx, request)
}

func initialDirectoryFromPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	if strings.TrimSpace(filepath.Ext(cleaned)) == "" {
		return cleaned
	}
	return filepath.Dir(cleaned)
}
