package softwareupdate

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"xiadown/internal/domain/dependencies"
)

var ErrReleaseNotFound = errors.New("software update release not found")

type Request struct {
	Channel    string
	AppVersion string
}

type AppRequest struct {
	Channel        string
	CurrentVersion string
}

type DependencyRequest struct {
	Channel    string
	AppVersion string
	Name       dependencies.DependencyName
}

type SourceRef struct {
	Provider string
	Owner    string
	Repo     string
}

type DownloadSource struct {
	Name     string
	Kind     string
	URL      string
	Priority int
	Enabled  bool
}

type Asset struct {
	ArtifactName    string
	ContentType     string
	Size            int64
	SHA256          string
	Signature       string
	Sources         []DownloadSource
	InstallStrategy string
	ArtifactType    string
	Binaries        []string
	ExecutableName  string
}

func (asset Asset) SortedSources() []DownloadSource {
	if len(asset.Sources) == 0 {
		return nil
	}
	sources := make([]DownloadSource, 0, len(asset.Sources))
	for _, source := range asset.Sources {
		if !source.Enabled || strings.TrimSpace(source.URL) == "" {
			continue
		}
		sources = append(sources, source)
	}
	sort.SliceStable(sources, func(i, j int) bool {
		if sources[i].Priority == sources[j].Priority {
			return sources[i].Name < sources[j].Name
		}
		return sources[i].Priority < sources[j].Priority
	})
	return sources
}

func (asset Asset) DownloadURLs() []string {
	sources := asset.SortedSources()
	if len(sources) == 0 {
		return nil
	}
	urls := make([]string, 0, len(sources))
	for _, source := range sources {
		urls = append(urls, source.URL)
	}
	return urls
}

func (asset Asset) PrimaryDownloadURL() string {
	urls := asset.DownloadURLs()
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func (asset Asset) PrimaryExecutableName() string {
	if strings.TrimSpace(asset.ExecutableName) != "" {
		return strings.TrimSpace(asset.ExecutableName)
	}
	if len(asset.Binaries) > 0 {
		return strings.TrimSpace(asset.Binaries[0])
	}
	return ""
}

type Compatibility struct {
	MinAppVersion string
	MaxAppVersion string
}

type AppRelease struct {
	Version     string
	PublishedAt time.Time
	Notes       string
	ReleasePage string
	Source      SourceRef
	Asset       Asset
	ResolvedBy  string
}

type DependencyRelease struct {
	Name               dependencies.DependencyName
	DisplayName        string
	Kind               string
	Source             SourceRef
	UpstreamVersion    string
	RecommendedVersion string
	PublishedAt        time.Time
	AutoUpdate         bool
	Required           bool
	Notes              string
	ReleasePage        string
	Compatibility      Compatibility
	Asset              Asset
	ResolvedBy         string
}

func (release DependencyRelease) TargetVersion() string {
	if strings.TrimSpace(release.RecommendedVersion) != "" {
		return strings.TrimSpace(release.RecommendedVersion)
	}
	return strings.TrimSpace(release.UpstreamVersion)
}

type RemoteContentRef struct {
	SchemaVersion int
	URL           string
	Version       string
	UpdatedAt     time.Time
	MinAppVersion string
	TTLSeconds    int
	Fallback      string
	SHA256        string
	Hash          string
}

func (ref RemoteContentRef) Configured() bool {
	return strings.TrimSpace(ref.URL) != ""
}

func (ref RemoteContentRef) Fingerprint() string {
	parts := make([]string, 0, 9)
	if ref.SchemaVersion > 0 {
		parts = append(parts, fmt.Sprintf("schema:%d", ref.SchemaVersion))
	}
	if value := strings.TrimSpace(ref.Version); value != "" {
		parts = append(parts, "version:"+value)
	}
	if value := strings.TrimSpace(ref.SHA256); value != "" {
		parts = append(parts, "sha256:"+value)
	}
	if value := strings.TrimSpace(ref.Hash); value != "" {
		parts = append(parts, "hash:"+value)
	}
	if !ref.UpdatedAt.IsZero() {
		parts = append(parts, "updatedAt:"+ref.UpdatedAt.UTC().Format(time.RFC3339Nano))
	}
	if value := strings.TrimSpace(ref.MinAppVersion); value != "" {
		parts = append(parts, "minAppVersion:"+value)
	}
	if ref.TTLSeconds > 0 {
		parts = append(parts, fmt.Sprintf("ttlSeconds:%d", ref.TTLSeconds))
	}
	if value := strings.TrimSpace(ref.Fallback); value != "" {
		parts = append(parts, "fallback:"+value)
	}
	if value := strings.TrimSpace(ref.URL); value != "" {
		parts = append(parts, "url:"+value)
	}
	return strings.Join(parts, "|")
}

type DreamFMConfig struct {
	LiveChannel RemoteContentRef
}

type Catalog struct {
	AppID           string
	ManifestVersion string
	Channel         string
	UpdatedAt       time.Time
	App             *AppRelease
	Dependencies    map[dependencies.DependencyName]DependencyRelease
	DreamFM         DreamFMConfig
}

func (catalog Catalog) Dependency(name dependencies.DependencyName) (DependencyRelease, bool) {
	if catalog.Dependencies == nil {
		return DependencyRelease{}, false
	}
	release, ok := catalog.Dependencies[name]
	return release, ok
}

type Snapshot struct {
	Catalog    Catalog
	CheckedAt  time.Time
	LastError  string
	LastSource string
}
