package wails

import (
	"context"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/application/library/service"
)

type LibraryHandler struct {
	service *service.LibraryService
}

func NewLibraryHandler(service *service.LibraryService) *LibraryHandler {
	return &LibraryHandler{service: service}
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

func (handler *LibraryHandler) CreateYTDLPJob(ctx context.Context, request dto.CreateYTDLPJobRequest) (dto.LibraryOperationDTO, error) {
	return handler.service.CreateYTDLPJob(ctx, request)
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
