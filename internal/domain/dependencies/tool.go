package dependencies

import (
	"strings"
	"time"
)

type DependencyName string

const (
	DependencyYTDLP  DependencyName = "yt-dlp"
	DependencyFFmpeg DependencyName = "ffmpeg"
	DependencyBun    DependencyName = "bun"
)

type DependencyKind string

const (
	KindBin     DependencyKind = "bin"
	KindRuntime DependencyKind = "runtime"
)

type DependencyStatus string

const (
	StatusMissing   DependencyStatus = "missing"
	StatusInstalled DependencyStatus = "installed"
	StatusInvalid   DependencyStatus = "invalid"
)

type Dependency struct {
	Name        DependencyName
	ExecPath    string
	Version     string
	Status      DependencyStatus
	InstalledAt *time.Time
	UpdatedAt   time.Time
}

type DependencyParams struct {
	Name        string
	ExecPath    string
	Version     string
	Status      string
	InstalledAt *time.Time
	UpdatedAt   *time.Time
}

func NewDependency(params DependencyParams) (Dependency, error) {
	name := DependencyName(strings.TrimSpace(params.Name))
	if name == "" {
		return Dependency{}, ErrInvalidDependency
	}
	status := DependencyStatus(strings.TrimSpace(params.Status))
	if status == "" {
		status = StatusMissing
	}
	updatedAt := time.Now()
	if params.UpdatedAt != nil {
		updatedAt = *params.UpdatedAt
	}
	return Dependency{
		Name:        name,
		ExecPath:    strings.TrimSpace(params.ExecPath),
		Version:     strings.TrimSpace(params.Version),
		Status:      status,
		InstalledAt: params.InstalledAt,
		UpdatedAt:   updatedAt,
	}, nil
}
