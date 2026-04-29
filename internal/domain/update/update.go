package update

import (
	"fmt"
	"strings"
	"time"
)

type Kind string

const (
	KindApp        Kind = "app"
	KindDependency Kind = "dependency"
	KindPlugin     Kind = "plugin"
)

type Status string

const (
	StatusIdle           Status = "idle"
	StatusChecking       Status = "checking"
	StatusNoUpdate       Status = "no_update"
	StatusAvailable      Status = "available"
	StatusDownloading    Status = "downloading"
	StatusInstalling     Status = "installing"
	StatusReadyToRestart Status = "ready_to_restart"
	StatusError          Status = "error"
)

type Info struct {
	Kind              Kind
	CurrentVersion    string
	LatestVersion     string
	Changelog         string
	DownloadURL       string
	CheckedAt         time.Time
	Status            Status
	Progress          int // 0-100
	Message           string
	PreparedVersion   string
	PreparedChangelog string
}

type WhatsNew struct {
	Version        string
	CurrentVersion string
	Changelog      string
}

func (info Info) IsUpdateAvailable() bool {
	return info.Status == StatusAvailable || info.Status == StatusDownloading || info.Status == StatusInstalling || info.Status == StatusReadyToRestart
}

func (info Info) IsChecking() bool {
	return info.Status == StatusChecking
}

func (info Info) IsError() bool {
	return info.Status == StatusError
}

func (info Info) HasPreparedUpdate() bool {
	current := NormalizeVersion(info.CurrentVersion)
	prepared := NormalizeVersion(info.PreparedVersion)
	return prepared != "" && CompareVersion(prepared, current) > 0
}

func (info Info) HasRemoteUpdate() bool {
	current := NormalizeVersion(info.CurrentVersion)
	latest := NormalizeVersion(info.LatestVersion)
	return latest != "" && CompareVersion(latest, current) > 0
}

func NormalizeVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}
	return strings.TrimPrefix(trimmed, "v")
}

func CompareVersion(a, b string) int {
	ap := strings.Split(NormalizeVersion(a), ".")
	bp := strings.Split(NormalizeVersion(b), ".")
	maxLen := len(ap)
	if len(bp) > maxLen {
		maxLen = len(bp)
	}
	for i := 0; i < maxLen; i++ {
		ai, bi := 0, 0
		if i < len(ap) {
			_, _ = fmt.Sscanf(ap[i], "%d", &ai)
		}
		if i < len(bp) {
			_, _ = fmt.Sscanf(bp[i], "%d", &bi)
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}
