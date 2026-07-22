package wails

import (
	"context"

	applicationbackup "xiadown/internal/application/librarybackup"
	domainbackup "xiadown/internal/domain/librarybackup"
)

// LibraryBackupHandler exposes metadata backup administration to the trusted
// desktop UI. Restore requests only create a next-launch marker; no Wails call
// can replace the database while it is open.
type LibraryBackupHandler struct {
	service *applicationbackup.Service
}

type LibraryBackupIDRequest struct {
	BackupID string `json:"backupId"`
}

func NewLibraryBackupHandler(service *applicationbackup.Service) *LibraryBackupHandler {
	return &LibraryBackupHandler{service: service}
}

func (*LibraryBackupHandler) ServiceName() string { return "LibraryBackupHandler" }

func (handler *LibraryBackupHandler) CreateLibraryMetadataBackup(ctx context.Context) (domainbackup.Manifest, error) {
	return handler.service.Create(ctx)
}

func (handler *LibraryBackupHandler) ListLibraryMetadataBackups(ctx context.Context) ([]domainbackup.BackupSummary, error) {
	return handler.service.List(ctx)
}

func (handler *LibraryBackupHandler) VerifyLibraryMetadataBackup(
	ctx context.Context,
	request LibraryBackupIDRequest,
) (domainbackup.VerificationResult, error) {
	return handler.service.Verify(ctx, request.BackupID)
}

func (handler *LibraryBackupHandler) PlanLibraryMetadataRestore(
	ctx context.Context,
	request LibraryBackupIDRequest,
) (domainbackup.RestorePlan, error) {
	return handler.service.PlanRestore(ctx, request.BackupID)
}

func (handler *LibraryBackupHandler) GetPendingLibraryMetadataRestore(ctx context.Context) (*domainbackup.RestorePlan, error) {
	return handler.service.PendingRestore(ctx)
}

func (handler *LibraryBackupHandler) CancelPendingLibraryMetadataRestore(ctx context.Context) error {
	return handler.service.CancelPendingRestore(ctx)
}
