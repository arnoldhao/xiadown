package librarybackup

import (
	"context"
	"strings"

	domainbackup "xiadown/internal/domain/librarybackup"
)

type Engine interface {
	Create(context.Context) (domainbackup.Manifest, error)
	List(context.Context) ([]domainbackup.BackupSummary, error)
	Verify(context.Context, string) (domainbackup.VerificationResult, error)
	PlanRestore(context.Context, string) (domainbackup.RestorePlan, error)
	PendingRestore(context.Context) (*domainbackup.RestorePlan, error)
	CancelPendingRestore(context.Context) error
}

type Service struct {
	engine Engine
}

func NewService(engine Engine) *Service {
	return &Service{engine: engine}
}

func (service *Service) Create(ctx context.Context) (domainbackup.Manifest, error) {
	return service.engine.Create(ctx)
}

func (service *Service) List(ctx context.Context) ([]domainbackup.BackupSummary, error) {
	return service.engine.List(ctx)
}

func (service *Service) Verify(ctx context.Context, backupID string) (domainbackup.VerificationResult, error) {
	return service.engine.Verify(ctx, strings.TrimSpace(backupID))
}

func (service *Service) PlanRestore(ctx context.Context, backupID string) (domainbackup.RestorePlan, error) {
	return service.engine.PlanRestore(ctx, strings.TrimSpace(backupID))
}

func (service *Service) PendingRestore(ctx context.Context) (*domainbackup.RestorePlan, error) {
	return service.engine.PendingRestore(ctx)
}

func (service *Service) CancelPendingRestore(ctx context.Context) error {
	return service.engine.CancelPendingRestore(ctx)
}
