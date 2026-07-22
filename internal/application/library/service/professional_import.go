package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"xiadown/internal/domain/library"
)

// ProfessionalImportProbe is intentionally small so the batch scanner can use
// LibraryService's ffprobe-aware inspection without depending on private media
// probe structures.
type ProfessionalImportProbe struct {
	HasVideo bool
	HasAudio bool
	Format   string
}

type ProfessionalImportRegistration struct {
	BatchID     string
	LibraryID   string
	FileID      string
	HistoryID   string
	FileEventID string
	StoragePath string
}

func (service *LibraryService) NotifyCatalogProjectionCompleted(ctx context.Context, fileID string) {
	if service == nil || service.files == nil {
		return
	}
	item, err := service.files.Get(ctx, strings.TrimSpace(fileID))
	if err != nil {
		return
	}
	service.publishFileUpdate(service.mustBuildFileDTO(ctx, item))
}

type ProfessionalImportRequest struct {
	BatchID      string
	CandidateID  string
	LibraryID    string
	SourcePath   string
	StoragePath  string
	DisplayName  string
	Kind         string
	SessionRunID string
	FileID       string
	HistoryID    string
	FileEventID  string
}

func (service *LibraryService) InspectProfessionalImport(ctx context.Context, path string) (ProfessionalImportProbe, error) {
	resolved, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return ProfessionalImportProbe{}, err
	}
	probe := service.probeLocalMedia(ctx, resolved)
	return ProfessionalImportProbe{
		HasVideo: mediaProbeHasVideo(probe),
		HasAudio: mediaProbeHasAudio(probe),
		Format:   strings.TrimSpace(probe.Format),
	}, nil
}

// EnsureProfessionalImportLibrary gives a batch a deterministic legacy bundle
// boundary. Retries therefore reuse the same Library instead of producing one
// empty bundle per interrupted attempt.
func (service *LibraryService) EnsureProfessionalImportLibrary(ctx context.Context, libraryID, displayName string) (string, error) {
	if service == nil || service.libraries == nil {
		return "", fmt.Errorf("library repository not configured")
	}
	libraryID = strings.TrimSpace(libraryID)
	if libraryID == "" {
		return "", fmt.Errorf("library id is required")
	}
	if _, err := service.libraries.Get(ctx, libraryID); err == nil {
		return libraryID, nil
	} else if !errors.Is(err, library.ErrLibraryNotFound) {
		return "", err
	}
	now := service.now()
	item, err := library.NewLibrary(library.LibraryParams{
		ID:        libraryID,
		Name:      resolveInitialLibraryName(libraryID, strings.TrimSpace(displayName), false),
		CreatedBy: library.CreateMeta{Source: "professional_import"},
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		return "", err
	}
	if err := service.libraries.Save(ctx, item); err != nil {
		return "", err
	}
	return item.ID, nil
}

// RegisterProfessionalImport deliberately delegates to createImportFile. This
// preserves the existing physical file registry, import history, file-event
// stream, local-audio synchronization and UI event publication.
func (service *LibraryService) RegisterProfessionalImport(ctx context.Context, request ProfessionalImportRequest) (ProfessionalImportRegistration, error) {
	if service == nil {
		return ProfessionalImportRegistration{}, fmt.Errorf("library service not configured")
	}
	storagePath, err := filepath.Abs(strings.TrimSpace(request.StoragePath))
	if err != nil {
		return ProfessionalImportRegistration{}, err
	}
	sourcePath, err := filepath.Abs(strings.TrimSpace(request.SourcePath))
	if err != nil {
		return ProfessionalImportRegistration{}, err
	}
	if _, err := service.libraries.Get(ctx, strings.TrimSpace(request.LibraryID)); err != nil {
		return ProfessionalImportRegistration{}, err
	}
	fileItem, history, eventRecord, err := service.createImportFile(ctx, importFileParams{
		LibraryID:      strings.TrimSpace(request.LibraryID),
		Path:           storagePath,
		OriginPath:     sourcePath,
		Name:           strings.TrimSpace(request.DisplayName),
		Kind:           strings.TrimSpace(request.Kind),
		Source:         "desktop_import",
		SessionRunID:   strings.TrimSpace(request.SessionRunID),
		KeepSourceFile: true,
		// Keep the established import_media history action so existing history
		// readers remain compatible; the durable batch ID carries the richer
		// professional-import identity.
		Action:                 "import_media",
		BatchID:                strings.TrimSpace(request.BatchID),
		FileID:                 strings.TrimSpace(request.FileID),
		HistoryID:              strings.TrimSpace(request.HistoryID),
		EventID:                strings.TrimSpace(request.FileEventID),
		OptionalProbe:          true,
		DeferCatalogProjection: true,
	})
	if err != nil {
		return ProfessionalImportRegistration{}, err
	}
	service.publishFileUpdate(service.mustBuildFileDTO(ctx, fileItem))
	service.publishHistoryUpdate(toHistoryDTO(history))
	service.publishFileEventUpdate(toFileEventDTO(eventRecord))
	return ProfessionalImportRegistration{
		BatchID:     request.BatchID,
		LibraryID:   fileItem.LibraryID,
		FileID:      fileItem.ID,
		HistoryID:   history.ID,
		FileEventID: eventRecord.ID,
		StoragePath: fileItem.Storage.LocalPath,
	}, nil
}
