package wails

import (
	"context"
	"testing"
	"time"

	applicationbackup "xiadown/internal/application/librarybackup"
	domainbackup "xiadown/internal/domain/librarybackup"
)

type fakeLibraryBackupEngine struct {
	verifiedID string
	plannedID  string
	cancelled  bool
}

func (engine *fakeLibraryBackupEngine) Create(context.Context) (domainbackup.Manifest, error) {
	return domainbackup.Manifest{BackupID: "created"}, nil
}

func (engine *fakeLibraryBackupEngine) List(context.Context) ([]domainbackup.BackupSummary, error) {
	return []domainbackup.BackupSummary{{BackupID: "listed"}}, nil
}

func (engine *fakeLibraryBackupEngine) Verify(_ context.Context, id string) (domainbackup.VerificationResult, error) {
	engine.verifiedID = id
	return domainbackup.VerificationResult{BackupID: id, Valid: true}, nil
}

func (engine *fakeLibraryBackupEngine) PlanRestore(_ context.Context, id string) (domainbackup.RestorePlan, error) {
	engine.plannedID = id
	return domainbackup.RestorePlan{BackupID: id, AppliesOnLaunch: true}, nil
}

func (engine *fakeLibraryBackupEngine) PendingRestore(context.Context) (*domainbackup.RestorePlan, error) {
	return &domainbackup.RestorePlan{BackupID: "pending", RequestedAt: time.Now(), AppliesOnLaunch: true}, nil
}

func (engine *fakeLibraryBackupEngine) CancelPendingRestore(context.Context) error {
	engine.cancelled = true
	return nil
}

func TestLibraryBackupHandlerExposesDeferredRestoreAdministration(t *testing.T) {
	engine := new(fakeLibraryBackupEngine)
	handler := NewLibraryBackupHandler(applicationbackup.NewService(engine))
	ctx := context.Background()
	if handler.ServiceName() != "LibraryBackupHandler" {
		t.Fatalf("ServiceName = %q", handler.ServiceName())
	}
	created, err := handler.CreateLibraryMetadataBackup(ctx)
	if err != nil || created.BackupID != "created" {
		t.Fatalf("CreateLibraryMetadataBackup = %+v, %v", created, err)
	}
	listed, err := handler.ListLibraryMetadataBackups(ctx)
	if err != nil || len(listed) != 1 || listed[0].BackupID != "listed" {
		t.Fatalf("ListLibraryMetadataBackups = %+v, %v", listed, err)
	}
	if _, err := handler.VerifyLibraryMetadataBackup(ctx, LibraryBackupIDRequest{BackupID: "  verified  "}); err != nil {
		t.Fatal(err)
	}
	if engine.verifiedID != "verified" {
		t.Fatalf("verified ID = %q", engine.verifiedID)
	}
	plan, err := handler.PlanLibraryMetadataRestore(ctx, LibraryBackupIDRequest{BackupID: "  planned  "})
	if err != nil || !plan.AppliesOnLaunch || engine.plannedID != "planned" {
		t.Fatalf("PlanLibraryMetadataRestore = %+v, %v", plan, err)
	}
	pending, err := handler.GetPendingLibraryMetadataRestore(ctx)
	if err != nil || pending == nil || pending.BackupID != "pending" {
		t.Fatalf("GetPendingLibraryMetadataRestore = %+v, %v", pending, err)
	}
	if err := handler.CancelPendingLibraryMetadataRestore(ctx); err != nil || !engine.cancelled {
		t.Fatalf("CancelPendingLibraryMetadataRestore cancelled=%t err=%v", engine.cancelled, err)
	}
}
