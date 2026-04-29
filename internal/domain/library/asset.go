package library

import (
	"strings"
	"time"
)

type FileKind string

const (
	FileKindVideo     FileKind = "video"
	FileKindAudio     FileKind = "audio"
	FileKindSubtitle  FileKind = "subtitle"
	FileKindThumbnail FileKind = "thumbnail"
	FileKindTranscode FileKind = "transcode"
)

type FileStorage struct {
	Mode       string
	LocalPath  string
	DocumentID string
}

type ImportOrigin struct {
	BatchID        string
	ImportPath     string
	ImportedAt     time.Time
	KeepSourceFile bool
}

type FileOrigin struct {
	Kind        string
	OperationID string
	Import      *ImportOrigin
}

type FileLineage struct {
	RootFileID string
}

type FileMetadata struct {
	Title     string
	Author    string
	Extractor string
}

type MediaInfo struct {
	Format           string
	Codec            string
	VideoCodec       string
	AudioCodec       string
	DurationMs       *int64
	Width            *int
	Height           *int
	FrameRate        *float64
	BitrateKbps      *int
	VideoBitrateKbps *int
	AudioBitrateKbps *int
	Channels         *int
	SizeBytes        *int64
	DPI              *int
}

type FileState struct {
	Status      string
	Deleted     bool
	Archived    bool
	LastError   string
	LastChecked string
}

type LibraryFile struct {
	ID                string
	LibraryID         string
	Kind              FileKind
	Name              string
	DisplayName       string
	Storage           FileStorage
	Origin            FileOrigin
	Lineage           FileLineage
	Metadata          FileMetadata
	LatestOperationID string
	Media             *MediaInfo
	State             FileState
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type LibraryFileParams struct {
	ID                string
	LibraryID         string
	Kind              string
	Name              string
	DisplayName       string
	Storage           FileStorage
	Origin            FileOrigin
	Lineage           FileLineage
	Metadata          FileMetadata
	LatestOperationID string
	Media             *MediaInfo
	State             FileState
	CreatedAt         *time.Time
	UpdatedAt         *time.Time
}

func NewLibraryFile(params LibraryFileParams) (LibraryFile, error) {
	id := strings.TrimSpace(params.ID)
	libraryID := strings.TrimSpace(params.LibraryID)
	kind := FileKind(strings.TrimSpace(params.Kind))
	name := strings.TrimSpace(params.Name)
	displayName := strings.TrimSpace(params.DisplayName)
	if id == "" || libraryID == "" || kind == "" || name == "" {
		return LibraryFile{}, ErrInvalidLibraryFile
	}
	if displayName == "" {
		displayName = name
	}
	storage := params.Storage
	storage.Mode = strings.TrimSpace(storage.Mode)
	storage.LocalPath = strings.TrimSpace(storage.LocalPath)
	storage.DocumentID = strings.TrimSpace(storage.DocumentID)
	switch kind {
	case FileKindVideo, FileKindAudio, FileKindThumbnail, FileKindTranscode:
		if storage.Mode == "" {
			storage.Mode = "local_path"
		}
		if storage.Mode != "local_path" && storage.Mode != "hybrid" {
			return LibraryFile{}, ErrInvalidLibraryFile
		}
		if storage.LocalPath == "" {
			return LibraryFile{}, ErrInvalidLibraryFile
		}
	case FileKindSubtitle:
		if storage.Mode == "" {
			storage.Mode = "db_document"
		}
		if storage.Mode != "local_path" && storage.Mode != "db_document" && storage.Mode != "hybrid" {
			return LibraryFile{}, ErrInvalidLibraryFile
		}
		if (storage.Mode == "local_path" || storage.Mode == "hybrid") && storage.LocalPath == "" {
			return LibraryFile{}, ErrInvalidLibraryFile
		}
		if (storage.Mode == "db_document" || storage.Mode == "hybrid") && storage.DocumentID == "" {
			return LibraryFile{}, ErrInvalidLibraryFile
		}
	default:
		return LibraryFile{}, ErrInvalidLibraryFile
	}
	origin := params.Origin
	origin.Kind = strings.TrimSpace(origin.Kind)
	origin.OperationID = strings.TrimSpace(origin.OperationID)
	if origin.Kind == "import" {
		if origin.Import == nil || strings.TrimSpace(origin.Import.ImportPath) == "" {
			return LibraryFile{}, ErrInvalidLibraryFile
		}
		origin.OperationID = ""
	} else {
		switch origin.Kind {
		case "download", "transcode":
			if origin.OperationID == "" || origin.Import != nil {
				return LibraryFile{}, ErrInvalidLibraryFile
			}
		default:
			return LibraryFile{}, ErrInvalidLibraryFile
		}
	}
	state := params.State
	state.Status = strings.TrimSpace(state.Status)
	if state.Status == "" {
		state.Status = "active"
	}
	createdAt := time.Now().UTC()
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	updatedAt := createdAt
	if params.UpdatedAt != nil && !params.UpdatedAt.IsZero() {
		updatedAt = params.UpdatedAt.UTC()
	}
	return LibraryFile{
		ID:          id,
		LibraryID:   libraryID,
		Kind:        kind,
		Name:        name,
		DisplayName: displayName,
		Storage:     storage,
		Origin:      origin,
		Lineage:     FileLineage{RootFileID: strings.TrimSpace(params.Lineage.RootFileID)},
		Metadata: FileMetadata{
			Title:     strings.TrimSpace(params.Metadata.Title),
			Author:    strings.TrimSpace(params.Metadata.Author),
			Extractor: strings.TrimSpace(params.Metadata.Extractor),
		},
		LatestOperationID: strings.TrimSpace(params.LatestOperationID),
		Media:             params.Media,
		State:             state,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}
