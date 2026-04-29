package service

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/dependencies/dto"
	"xiadown/internal/application/softwareupdate"
	"xiadown/internal/domain/dependencies"
)

const (
	defaultDownloadTimeout = 30 * time.Minute
)

const (
	sourceKindGitHubRelease = "github_release"
	sourceKindRuntime       = "runtime"
)

type dependencySource struct {
	DependencyKind string
	Kind           string
	SourceRef      string
	Manager        string
}

var dependencySources = map[dependencies.DependencyName]dependencySource{
	dependencies.DependencyYTDLP: {
		DependencyKind: string(dependencies.KindBin),
		Kind:           sourceKindGitHubRelease,
		SourceRef:      "yt-dlp/yt-dlp",
	},
	dependencies.DependencyFFmpeg: {
		DependencyKind: string(dependencies.KindBin),
		Kind:           sourceKindGitHubRelease,
		SourceRef:      "jellyfin/jellyfin-ffmpeg",
	},
	dependencies.DependencyBun: {
		DependencyKind: string(dependencies.KindBin),
		Kind:           sourceKindGitHubRelease,
		SourceRef:      "oven-sh/bun",
	},
}

type DependenciesService struct {
	repo         dependencies.Repository
	updates      *softwareupdate.Service
	appVersion   string
	httpClient   dependencyHTTPClientProvider
	now          func() time.Time
	installMu    sync.RWMutex
	installState map[dependencies.DependencyName]dto.DependencyInstallState
}

type dependencyHTTPClientProvider interface {
	HTTPClient() *http.Client
}

type Option func(*DependenciesService)

func WithHTTPClientProvider(provider dependencyHTTPClientProvider) Option {
	return func(service *DependenciesService) {
		service.httpClient = provider
	}
}

func NewDependenciesService(repo dependencies.Repository, updates *softwareupdate.Service, appVersion string, options ...Option) *DependenciesService {
	service := &DependenciesService{
		repo:         repo,
		updates:      updates,
		appVersion:   strings.TrimSpace(appVersion),
		now:          time.Now,
		installState: make(map[dependencies.DependencyName]dto.DependencyInstallState),
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

const (
	installStageIdle        = "idle"
	installStageDownloading = "downloading"
	installStageExtracting  = "extracting"
	installStageVerifying   = "verifying"
	installStageDone        = "done"
	installStageError       = "error"

	downloadProgressStart = 0
	downloadProgressEnd   = 80
	extractProgressStart  = 80
	extractProgressEnd    = 95
	verifyProgressStart   = 95
	verifyProgressEnd     = 100
)

func (service *DependenciesService) setInstallState(name dependencies.DependencyName, stage string, progress int, message string) {
	if name == "" {
		return
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	service.installMu.Lock()
	defer service.installMu.Unlock()
	service.installState[name] = dto.DependencyInstallState{
		Name:      string(name),
		Stage:     stage,
		Progress:  progress,
		Message:   message,
		UpdatedAt: service.now().Format(time.RFC3339),
	}
}

func (service *DependenciesService) GetInstallState(ctx context.Context, request dto.GetDependencyInstallStateRequest) (dto.DependencyInstallState, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return dto.DependencyInstallState{}, dependencies.ErrInvalidDependency
	}
	dependencyName := dependencies.DependencyName(name)
	service.installMu.RLock()
	defer service.installMu.RUnlock()
	if state, ok := service.installState[dependencyName]; ok {
		return state, nil
	}
	return dto.DependencyInstallState{
		Name:      name,
		Stage:     installStageIdle,
		Progress:  0,
		UpdatedAt: service.now().Format(time.RFC3339),
	}, nil
}

func (service *DependenciesService) EnsureDefaults(ctx context.Context) error {
	defaults := []dependencies.DependencyName{
		dependencies.DependencyYTDLP,
		dependencies.DependencyFFmpeg,
		dependencies.DependencyBun,
	}
	existing, err := service.repo.List(ctx)
	if err != nil {
		return err
	}
	seen := make(map[dependencies.DependencyName]struct{}, len(existing))
	for _, item := range existing {
		seen[item.Name] = struct{}{}
	}
	for _, dependency := range defaults {
		if _, ok := seen[dependency]; ok {
			continue
		}
		now := service.now()
		entry, err := dependencies.NewDependency(dependencies.DependencyParams{
			Name:      string(dependency),
			Status:    string(dependencies.StatusMissing),
			UpdatedAt: &now,
		})
		if err != nil {
			return err
		}
		if err := service.repo.Save(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

func (service *DependenciesService) ListDependencies(ctx context.Context) ([]dto.Dependency, error) {
	items, err := service.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.Dependency, 0, len(items))
	for _, item := range items {
		if _, err := resolveDependencySource(item.Name); err != nil {
			continue
		}
		status := item.Status
		if status == "" {
			status = dependencies.StatusMissing
		}
		if item.ExecPath != "" {
			switch {
			case !pathExists(item.ExecPath):
				status = dependencies.StatusInvalid
			case item.Name == dependencies.DependencyFFmpeg && !pathExists(ffprobePathForFFmpegExec(item.ExecPath)):
				status = dependencies.StatusInvalid
			}
		}
		entry := toDependencyDTO(item)
		entry.Status = string(status)
		result = append(result, entry)
	}
	return result, nil
}

func (service *DependenciesService) ListDependencyUpdates(ctx context.Context) ([]dto.DependencyUpdateInfo, error) {
	items, err := service.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.DependencyUpdateInfo, 0, len(items))
	for _, item := range items {
		source, err := resolveDependencySource(item.Name)
		if err != nil {
			continue
		}
		info := dto.DependencyUpdateInfo{
			Name: string(item.Name),
		}
		release, releaseErr := service.resolveDependencyRelease(ctx, item.Name)
		if releaseErr == nil {
			info.LatestVersion = release.TargetVersion()
			info.RecommendedVersion = release.RecommendedVersion
			info.UpstreamVersion = release.UpstreamVersion
			info.ReleaseNotes = release.Notes
			info.ReleaseNotesURL = release.ReleasePage
			info.AutoUpdate = release.AutoUpdate
			info.Required = release.Required
		}
		// Runtime dependencies don't expose a stable remote "latest" endpoint.
		// Use the installed runtime version as the target display version.
		if source.Kind == sourceKindRuntime && strings.TrimSpace(info.LatestVersion) == "" {
			info.LatestVersion = strings.TrimSpace(item.Version)
		}
		result = append(result, info)
	}
	return result, nil
}

func (service *DependenciesService) InstallDependency(ctx context.Context, request dto.InstallDependencyRequest) (dto.Dependency, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return dto.Dependency{}, dependencies.ErrInvalidDependency
	}
	dependencyName := dependencies.DependencyName(name)
	source, err := resolveDependencySource(dependencyName)
	if err != nil {
		service.setInstallState(dependencyName, installStageError, downloadProgressStart, "invalid dependency")
		return dto.Dependency{}, err
	}
	service.setInstallState(dependencyName, installStageDownloading, 0, "")
	switch source.Kind {
	case sourceKindGitHubRelease:
		if strings.TrimSpace(request.Manager) != "" {
			service.setInstallState(dependencyName, installStageError, downloadProgressStart, "manager is unsupported for this dependency")
			return dto.Dependency{}, fmt.Errorf("manager is unsupported for dependency %s", dependencyName)
		}
		if installed, handled, err := service.installDependencyFromReleaseCatalog(ctx, dependencyName, request.Version); handled {
			if err != nil {
				service.setInstallState(dependencyName, installStageError, downloadProgressStart, err.Error())
			}
			return installed, err
		}
		service.setInstallState(dependencyName, installStageError, downloadProgressStart, softwareupdate.ErrReleaseNotFound.Error())
		return dto.Dependency{}, softwareupdate.ErrReleaseNotFound
	case sourceKindRuntime:
		service.setInstallState(dependencyName, installStageError, downloadProgressStart, "runtime dependencies are not supported")
		return dto.Dependency{}, dependencies.ErrInvalidDependency
	default:
		service.setInstallState(dependencyName, installStageError, downloadProgressStart, "unsupported source")
		return dto.Dependency{}, fmt.Errorf("unsupported source for dependency %s", dependencyName)
	}
}

func (service *DependenciesService) VerifyDependency(ctx context.Context, request dto.VerifyDependencyRequest) (dto.Dependency, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return dto.Dependency{}, dependencies.ErrInvalidDependency
	}
	dependency, err := service.repo.Get(ctx, name)
	if err != nil {
		return dto.Dependency{}, err
	}
	var status dependencies.DependencyStatus
	var version string
	if dependency.ExecPath == "" || !pathExists(dependency.ExecPath) {
		status = dependencies.StatusMissing
		version = ""
	} else {
		ver, verErr := resolveInstalledDependencyVersion(ctx, dependency.Name, dependency.ExecPath)
		if verErr != nil {
			status = dependencies.StatusInvalid
			version = ""
		} else {
			status = dependencies.StatusInstalled
			version = ver
		}
	}
	now := service.now()
	updated, err := dependencies.NewDependency(dependencies.DependencyParams{
		Name:        string(dependency.Name),
		ExecPath:    dependency.ExecPath,
		Version:     version,
		Status:      string(status),
		InstalledAt: dependency.InstalledAt,
		UpdatedAt:   &now,
	})
	if err != nil {
		return dto.Dependency{}, err
	}
	if err := service.repo.Save(ctx, updated); err != nil {
		return dto.Dependency{}, err
	}
	return toDependencyDTO(updated), nil
}

func (service *DependenciesService) SetDependencyPath(ctx context.Context, request dto.SetDependencyPathRequest) (dto.Dependency, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return dto.Dependency{}, dependencies.ErrInvalidDependency
	}
	dependencyName := dependencies.DependencyName(name)
	if source, err := resolveDependencySource(dependencyName); err == nil && source.DependencyKind == string(dependencies.KindRuntime) {
		return dto.Dependency{}, fmt.Errorf("manual path is unsupported for runtime dependency %s", dependencyName)
	}
	execPath := strings.TrimSpace(request.ExecPath)
	if execPath == "" {
		return dto.Dependency{}, dependencies.ErrInvalidDependency
	}
	now := service.now()
	version := ""
	status := dependencies.StatusMissing
	var installedAt *time.Time
	if pathExists(execPath) {
		if resolved, err := resolveInstalledDependencyVersion(ctx, dependencyName, execPath); err == nil {
			version = resolved
			status = dependencies.StatusInstalled
			installedAt = &now
		} else {
			status = dependencies.StatusInvalid
		}
	}
	updated, err := dependencies.NewDependency(dependencies.DependencyParams{
		Name:        name,
		ExecPath:    execPath,
		Version:     version,
		Status:      string(status),
		InstalledAt: installedAt,
		UpdatedAt:   &now,
	})
	if err != nil {
		return dto.Dependency{}, err
	}
	if err := service.repo.Save(ctx, updated); err != nil {
		return dto.Dependency{}, err
	}
	return toDependencyDTO(updated), nil
}

func (service *DependenciesService) RemoveDependency(ctx context.Context, request dto.RemoveDependencyRequest) error {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return dependencies.ErrInvalidDependency
	}
	dependency, err := service.repo.Get(ctx, name)
	if err != nil && err != dependencies.ErrDependencyNotFound {
		return err
	}
	if err == nil && dependency.ExecPath != "" {
		_ = os.RemoveAll(filepath.Dir(dependency.ExecPath))
	}
	if baseDir, baseErr := dependenciesBaseDir(); baseErr == nil {
		_ = os.RemoveAll(filepath.Join(baseDir, name))
	}
	now := service.now()
	updated, err := dependencies.NewDependency(dependencies.DependencyParams{
		Name:      name,
		Status:    string(dependencies.StatusMissing),
		UpdatedAt: &now,
	})
	if err != nil {
		return err
	}
	return service.repo.Save(ctx, updated)
}

func (service *DependenciesService) ResolveExecPath(ctx context.Context, name dependencies.DependencyName) (string, error) {
	dependency, err := service.repo.Get(ctx, string(name))
	if err != nil {
		return "", err
	}
	if dependency.ExecPath == "" || !pathExists(dependency.ExecPath) {
		return "", fmt.Errorf("%s is not installed", name)
	}
	if name == dependencies.DependencyFFmpeg && !pathExists(ffprobePathForFFmpegExec(dependency.ExecPath)) {
		return "", fmt.Errorf("ffprobe is not installed")
	}
	return dependency.ExecPath, nil
}

func (service *DependenciesService) IsDependencyReady(ctx context.Context, name dependencies.DependencyName) (bool, error) {
	ready, _, err := service.DependencyReadiness(ctx, name)
	return ready, err
}

func (service *DependenciesService) DependencyReadiness(ctx context.Context, name dependencies.DependencyName) (bool, string, error) {
	if strings.TrimSpace(string(name)) == "" {
		return false, "invalid_tool", dependencies.ErrInvalidDependency
	}
	dependency, err := service.repo.Get(ctx, string(name))
	if err != nil {
		if errors.Is(err, dependencies.ErrDependencyNotFound) {
			return false, "not_found", nil
		}
		return false, "", err
	}
	if dependency.Status != dependencies.StatusInstalled {
		if dependency.Status == dependencies.StatusInvalid {
			return false, "invalid", nil
		}
		return false, "not_installed", nil
	}
	if strings.TrimSpace(dependency.ExecPath) == "" {
		return false, "missing_exec_path", nil
	}
	if !pathExists(dependency.ExecPath) {
		return false, "exec_not_found", nil
	}
	if name == dependencies.DependencyFFmpeg && !pathExists(ffprobePathForFFmpegExec(dependency.ExecPath)) {
		return false, "ffprobe_not_found", nil
	}
	return true, "", nil
}

func (service *DependenciesService) ResolveDependencyDirectory(ctx context.Context, name dependencies.DependencyName) (string, error) {
	execPath, err := service.ResolveExecPath(ctx, name)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(execPath)
	if !pathExists(dir) {
		return "", fmt.Errorf("%s directory not found", name)
	}
	return dir, nil
}

func (service *DependenciesService) resolveDependencyRelease(ctx context.Context, name dependencies.DependencyName) (softwareupdate.DependencyRelease, error) {
	if service.updates != nil {
		release, err := service.updates.ResolveDependencyRelease(ctx, softwareupdate.DependencyRequest{
			AppVersion: service.appVersion,
			Name:       name,
		})
		if err == nil {
			return release, nil
		}
		return softwareupdate.DependencyRelease{}, err
	}
	return softwareupdate.DependencyRelease{}, softwareupdate.ErrReleaseNotFound
}

func (service *DependenciesService) installDependencyFromReleaseCatalog(ctx context.Context, name dependencies.DependencyName, version string) (dto.Dependency, bool, error) {
	if service.updates == nil {
		return dto.Dependency{}, true, softwareupdate.ErrReleaseNotFound
	}
	if name != dependencies.DependencyYTDLP && name != dependencies.DependencyFFmpeg && name != dependencies.DependencyBun {
		return dto.Dependency{}, false, nil
	}
	release, err := service.updates.ResolveDependencyRelease(ctx, softwareupdate.DependencyRequest{
		AppVersion: service.appVersion,
		Name:       name,
	})
	if err != nil {
		return dto.Dependency{}, true, err
	}
	targetVersion := strings.TrimSpace(release.TargetVersion())
	if targetVersion == "" || len(release.Asset.DownloadURLs()) == 0 {
		return dto.Dependency{}, true, fmt.Errorf("manifest release for %s is incomplete", name)
	}
	requestedVersion := normalizeManagedDependencyVersion(name, version)
	if requestedVersion != "" && requestedVersion != "latest" && requestedVersion != normalizeManagedDependencyVersion(name, targetVersion) {
		return dto.Dependency{}, true, fmt.Errorf("requested %s version %s is not available in manifest", name, version)
	}
	installed, err := service.installCatalogRelease(ctx, release)
	return installed, true, err
}

func normalizeManagedDependencyVersion(name dependencies.DependencyName, version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}
	if strings.EqualFold(trimmed, "latest") {
		return "latest"
	}
	switch name {
	case dependencies.DependencyBun:
		return normalizeBunVersion(trimmed)
	case dependencies.DependencyFFmpeg:
		return normalizeFFmpegVersion(trimmed)
	default:
		return strings.TrimPrefix(trimmed, "v")
	}
}

func managedDependencyVersionFromPath(name dependencies.DependencyName, execPath string) string {
	trimmedExecPath := strings.TrimSpace(execPath)
	if name == "" || trimmedExecPath == "" {
		return ""
	}
	normalizedPath := strings.Trim(filepath.ToSlash(filepath.Clean(trimmedExecPath)), "/")
	if normalizedPath == "" {
		return ""
	}
	parts := strings.Split(normalizedPath, "/")
	for index := 0; index+2 < len(parts); index++ {
		if parts[index] != "dependencies" {
			continue
		}
		if !strings.EqualFold(parts[index+1], string(name)) {
			continue
		}
		version := normalizeManagedDependencyVersion(name, parts[index+2])
		if version == "" || version == "latest" {
			return ""
		}
		return version
	}
	return ""
}

func (service *DependenciesService) installCatalogRelease(ctx context.Context, release softwareupdate.DependencyRelease) (dto.Dependency, error) {
	switch strings.ToLower(strings.TrimSpace(release.Asset.InstallStrategy)) {
	case "binary":
		return service.installCatalogBinaryRelease(ctx, release)
	case "archive":
		return service.installCatalogArchiveRelease(ctx, release)
	default:
		return dto.Dependency{}, fmt.Errorf("unsupported install strategy %s for %s", release.Asset.InstallStrategy, release.Name)
	}
}

func (service *DependenciesService) installCatalogBinaryRelease(ctx context.Context, release softwareupdate.DependencyRelease) (dto.Dependency, error) {
	baseDir, err := dependenciesBaseDir()
	if err != nil {
		return dto.Dependency{}, err
	}
	version := release.TargetVersion()
	dependencyDir := filepath.Join(baseDir, string(release.Name), version)
	execName := strings.TrimSpace(release.Asset.PrimaryExecutableName())
	if execName == "" {
		execName = executableName(release.Name)
	}
	execPath := filepath.Join(dependencyDir, execName)
	if err := downloadFromSourcesWithProgress(ctx, release.Asset.DownloadURLs(), execPath, func(progress int) {
		mapped := mapProgress(progress, downloadProgressStart, downloadProgressEnd)
		service.setInstallState(release.Name, installStageDownloading, mapped, "")
	}, service.httpClient); err != nil {
		return dto.Dependency{}, err
	}
	if err := validateDownloadedExecutable(execPath); err != nil {
		return dto.Dependency{}, err
	}
	if err := markExecutable(execPath); err != nil {
		return dto.Dependency{}, err
	}
	service.setInstallState(release.Name, installStageVerifying, verifyProgressStart, "")
	resolvedVersion, err := resolveInstalledDependencyVersion(ctx, release.Name, execPath)
	if err != nil {
		return dto.Dependency{}, err
	}
	return service.saveInstalledDependency(ctx, release.Name, execPath, resolvedVersion, baseDir, version)
}

func (service *DependenciesService) installCatalogArchiveRelease(ctx context.Context, release softwareupdate.DependencyRelease) (dto.Dependency, error) {
	baseDir, err := dependenciesBaseDir()
	if err != nil {
		return dto.Dependency{}, err
	}
	version := release.TargetVersion()
	dependencyDir := filepath.Join(baseDir, string(release.Name), version)
	if err := os.MkdirAll(dependencyDir, 0o755); err != nil {
		return dto.Dependency{}, err
	}
	archivePath := filepath.Join(dependencyDir, fmt.Sprintf("download-%d%s", time.Now().UnixNano(), archiveSuffixForAsset(release.Asset)))
	defer os.Remove(archivePath)

	if err := downloadFromSourcesWithProgress(ctx, release.Asset.DownloadURLs(), archivePath, func(progress int) {
		mapped := mapProgress(progress, downloadProgressStart, downloadProgressEnd)
		service.setInstallState(release.Name, installStageDownloading, mapped, "")
	}, service.httpClient); err != nil {
		return dto.Dependency{}, err
	}

	binaries := append([]string(nil), release.Asset.Binaries...)
	if len(binaries) == 0 {
		primary := strings.TrimSpace(release.Asset.PrimaryExecutableName())
		if primary != "" {
			binaries = []string{primary}
		}
	}
	if len(binaries) == 0 {
		return dto.Dependency{}, fmt.Errorf("missing binaries for %s release", release.Name)
	}

	var extracted map[string]string
	switch strings.ToLower(strings.TrimSpace(release.Asset.ArtifactType)) {
	case "zip":
		extracted, err = extractZipExecutables(archivePath, dependencyDir, binaries, func(progress int) {
			mapped := mapProgress(progress, extractProgressStart, extractProgressEnd)
			service.setInstallState(release.Name, installStageExtracting, mapped, "")
		})
	case "tar.xz":
		extracted, err = extractTarXZExecutables(archivePath, dependencyDir, binaries, func(progress int) {
			mapped := mapProgress(progress, extractProgressStart, extractProgressEnd)
			service.setInstallState(release.Name, installStageExtracting, mapped, "")
		})
	default:
		err = fmt.Errorf("unsupported artifact type %s for %s", release.Asset.ArtifactType, release.Name)
	}
	if err != nil {
		return dto.Dependency{}, err
	}

	for _, binary := range binaries {
		if path := extracted[binary]; strings.TrimSpace(path) != "" {
			if err := markExecutable(path); err != nil {
				return dto.Dependency{}, err
			}
		}
	}

	service.setInstallState(release.Name, installStageVerifying, verifyProgressStart, "")
	execPath := extracted[strings.TrimSpace(release.Asset.PrimaryExecutableName())]
	if strings.TrimSpace(execPath) == "" {
		execPath = extracted[binaries[0]]
	}
	if strings.TrimSpace(execPath) == "" {
		return dto.Dependency{}, fmt.Errorf("primary executable not found for %s", release.Name)
	}

	resolvedVersion, err := resolveInstalledCatalogVersion(ctx, release.Name, execPath)
	if err != nil {
		return dto.Dependency{}, err
	}
	return service.saveInstalledDependency(ctx, release.Name, execPath, resolvedVersion, baseDir, version)
}

func resolveInstalledCatalogVersion(ctx context.Context, name dependencies.DependencyName, execPath string) (string, error) {
	if name == dependencies.DependencyFFmpeg {
		return validateFFmpegInstallation(ctx, execPath)
	}
	return resolveVersion(ctx, name, execPath)
}

func (service *DependenciesService) saveInstalledDependency(ctx context.Context, name dependencies.DependencyName, execPath string, version string, baseDir string, cleanupVersion string) (dto.Dependency, error) {
	now := service.now()
	dependency, err := dependencies.NewDependency(dependencies.DependencyParams{
		Name:        string(name),
		ExecPath:    execPath,
		Version:     strings.TrimSpace(version),
		Status:      string(dependencies.StatusInstalled),
		InstalledAt: &now,
		UpdatedAt:   &now,
	})
	if err != nil {
		return dto.Dependency{}, err
	}
	if err := service.repo.Save(ctx, dependency); err != nil {
		service.setInstallState(name, installStageError, verifyProgressEnd, err.Error())
		return dto.Dependency{}, err
	}
	service.setInstallState(name, installStageDone, verifyProgressEnd, "")
	_ = cleanupOldDependencyVersions(baseDir, name, cleanupVersion)
	return toDependencyDTO(dependency), nil
}

func archiveSuffixForAsset(asset softwareupdate.Asset) string {
	name := strings.ToLower(strings.TrimSpace(asset.ArtifactName))
	switch {
	case strings.HasSuffix(name, ".tar.xz"):
		return ".tar.xz"
	case strings.HasSuffix(name, ".zip"):
		return ".zip"
	}
	switch strings.ToLower(strings.TrimSpace(asset.ArtifactType)) {
	case "tar.xz":
		return ".tar.xz"
	default:
		return ".zip"
	}
}

func executableNameForBinary(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		return trimmed + ".exe"
	}
	return trimmed
}

func ffprobePathForFFmpegExec(execPath string) string {
	trimmed := strings.TrimSpace(execPath)
	if trimmed == "" {
		return executableNameForBinary("ffprobe")
	}
	return filepath.Join(filepath.Dir(trimmed), executableNameForBinary("ffprobe"))
}

func resolveInstalledDependencyVersion(ctx context.Context, name dependencies.DependencyName, execPath string) (string, error) {
	if name == dependencies.DependencyFFmpeg {
		version, err := validateFFmpegInstallation(ctx, execPath)
		if err != nil {
			return "", err
		}
		if managedVersion := managedDependencyVersionFromPath(name, execPath); managedVersion != "" {
			return managedVersion, nil
		}
		return version, nil
	}
	if managedVersion := managedDependencyVersionFromPath(name, execPath); managedVersion != "" {
		if _, err := resolveVersion(ctx, name, execPath); err != nil {
			return "", err
		}
		return managedVersion, nil
	}
	return resolveVersion(ctx, name, execPath)
}

func validateFFmpegInstallation(ctx context.Context, execPath string) (string, error) {
	trimmedExecPath := strings.TrimSpace(execPath)
	if trimmedExecPath == "" || !pathExists(trimmedExecPath) {
		return "", fmt.Errorf("ffmpeg is not installed")
	}
	ffprobePath := ffprobePathForFFmpegExec(trimmedExecPath)
	if !pathExists(ffprobePath) {
		return "", fmt.Errorf("ffprobe is not installed")
	}
	version, err := resolveVersion(ctx, dependencies.DependencyFFmpeg, trimmedExecPath)
	if err != nil {
		return "", err
	}
	if _, err := resolveVersion(ctx, dependencies.DependencyFFmpeg, ffprobePath); err != nil {
		return "", err
	}
	return version, nil
}

func normalizeFFmpegVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")
	return strings.TrimSpace(trimmed)
}

func toDependencyDTO(dependency dependencies.Dependency) dto.Dependency {
	installedAt := ""
	if dependency.InstalledAt != nil {
		installedAt = dependency.InstalledAt.Format(time.RFC3339)
	}
	version := strings.TrimSpace(dependency.Version)
	if managedVersion := managedDependencyVersionFromPath(dependency.Name, dependency.ExecPath); managedVersion != "" {
		version = managedVersion
	}
	toolKind, sourceKind, sourceRef, manager := dependencySourceMetadata(dependency.Name)
	return dto.Dependency{
		Name:        string(dependency.Name),
		Kind:        toolKind,
		ExecPath:    dependency.ExecPath,
		Version:     version,
		Status:      string(dependency.Status),
		SourceKind:  sourceKind,
		SourceRef:   sourceRef,
		Manager:     manager,
		InstalledAt: installedAt,
		UpdatedAt:   dependency.UpdatedAt.Format(time.RFC3339),
	}
}

func resolveDependencySource(name dependencies.DependencyName) (dependencySource, error) {
	source, ok := dependencySources[name]
	if !ok {
		return dependencySource{}, dependencies.ErrInvalidDependency
	}
	return source, nil
}

func dependencySourceMetadata(name dependencies.DependencyName) (string, string, string, string) {
	source, err := resolveDependencySource(name)
	if err != nil {
		return "", "", "", ""
	}
	return source.DependencyKind, source.Kind, source.SourceRef, source.Manager
}

func dependenciesBaseDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(configDir, "xiadown", "dependencies")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func executableName(name dependencies.DependencyName) string {
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("%s.exe", name)
	default:
		return string(name)
	}
}

func normalizeBunVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	trimmed = strings.TrimPrefix(trimmed, "bun-v")
	trimmed = strings.TrimPrefix(trimmed, "bun-V")
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")
	return strings.TrimSpace(trimmed)
}

func downloadFromSourcesWithProgress(ctx context.Context, urls []string, destPath string, progress func(int), clientProvider dependencyHTTPClientProvider) error {
	if len(urls) == 0 {
		return errors.New("download url is empty")
	}
	var lastErr error
	for _, rawURL := range urls {
		url := strings.TrimSpace(rawURL)
		if url == "" {
			continue
		}
		if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		lastErr = downloadFileWithProgressInternal(ctx, url, destPath, progress, clientProvider)
		if lastErr == nil {
			return nil
		}
	}
	if lastErr == nil {
		lastErr = errors.New("download url is empty")
	}
	return lastErr
}

func downloadFileWithProgressInternal(ctx context.Context, url string, destPath string, progress func(int), clientProvider dependencyHTTPClientProvider) error {
	if url == "" {
		return errors.New("download url is empty")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	client := dependencyDownloadHTTPClient(clientProvider)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	tmpPath := destPath + "." + uuid.NewString() + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 32*1024)
	lastReport := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				_ = os.Remove(tmpPath)
				return err
			}
			written += int64(n)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			_ = os.Remove(tmpPath)
			return readErr
		}
		if progress != nil && (time.Since(lastReport) > 200*time.Millisecond || written == total) {
			progress(percent(written, total))
			lastReport = time.Now()
		}
	}

	if total > 0 && written != total {
		_ = out.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("download incomplete: expected %d bytes, got %d", total, written)
	}
	if progress != nil {
		progress(100)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func dependencyDownloadHTTPClient(provider dependencyHTTPClientProvider) *http.Client {
	if provider == nil {
		return &http.Client{Timeout: defaultDownloadTimeout}
	}
	provided := provider.HTTPClient()
	if provided == nil {
		return &http.Client{Timeout: defaultDownloadTimeout}
	}
	cloned := *provided
	cloned.Timeout = defaultDownloadTimeout
	return &cloned
}

func cleanupOldDependencyVersions(baseDir string, name dependencies.DependencyName, keepVersion string) error {
	if strings.TrimSpace(baseDir) == "" || strings.TrimSpace(keepVersion) == "" {
		return nil
	}
	root := filepath.Join(baseDir, string(name))
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == keepVersion {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, entry.Name()))
	}
	return nil
}

func validateDownloadedExecutable(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, 512)
	n, readErr := io.ReadFull(file, header)
	if readErr != nil && readErr != io.ErrUnexpectedEOF {
		return readErr
	}
	header = header[:n]
	if len(header) < 4 {
		return fmt.Errorf("downloaded file is too small")
	}
	if looksLikeHTML(header) {
		return fmt.Errorf("downloaded file is HTML, likely blocked or redirected")
	}
	if looksLikeZip(header) {
		return fmt.Errorf("downloaded file is archive, expected executable")
	}
	switch runtime.GOOS {
	case "windows":
		if header[0] == 'M' && header[1] == 'Z' {
			return nil
		}
		return fmt.Errorf("downloaded file is not a Windows executable")
	case "darwin":
		if looksLikeMachO(header) {
			return nil
		}
		return fmt.Errorf("downloaded file is not a macOS executable")
	default:
		if looksLikeELF(header) {
			return nil
		}
		return fmt.Errorf("downloaded file is not a Linux executable")
	}
}

func looksLikeHTML(header []byte) bool {
	lower := strings.ToLower(string(header))
	return strings.Contains(lower, "<!doctype") ||
		strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<head") ||
		strings.Contains(lower, "<body")
}

func looksLikeZip(header []byte) bool {
	if len(header) < 4 {
		return false
	}
	return header[0] == 'P' && header[1] == 'K' &&
		((header[2] == 3 && header[3] == 4) ||
			(header[2] == 5 && header[3] == 6) ||
			(header[2] == 7 && header[3] == 8))
}

func looksLikeELF(header []byte) bool {
	return len(header) >= 4 && header[0] == 0x7f && header[1] == 'E' && header[2] == 'L' && header[3] == 'F'
}

func looksLikeMachO(header []byte) bool {
	if len(header) < 4 {
		return false
	}
	magicBE := binary.BigEndian.Uint32(header[:4])
	magicLE := binary.LittleEndian.Uint32(header[:4])
	switch magicBE {
	case 0xFEEDFACE, 0xFEEDFACF, 0xCAFEBABE:
		return true
	}
	switch magicLE {
	case 0xCEFAEDFE, 0xCFFAEDFE, 0xBEBAFECA:
		return true
	}
	return false
}

func extractZipExecutables(archivePath, destDir string, execNames []string, progress func(int)) (map[string]string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	targets := make(map[string]string, len(execNames))
	for _, execName := range execNames {
		trimmed := strings.TrimSpace(execName)
		if trimmed == "" {
			continue
		}
		targets[strings.ToLower(trimmed)] = trimmed
	}
	found := make(map[string]string, len(targets))
	total := len(reader.File)

	for i, file := range reader.File {
		path := filepath.Join(destDir, file.Name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		src, err := file.Open()
		if err != nil {
			return nil, err
		}
		dst, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			_ = src.Close()
			return nil, err
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = dst.Close()
			_ = src.Close()
			return nil, err
		}
		_ = dst.Close()
		_ = src.Close()
		if originalName, ok := targets[strings.ToLower(filepath.Base(path))]; ok {
			found[originalName] = path
		}
		if progress != nil {
			progress(percent(int64(i+1), int64(total)))
		}
	}

	for _, execName := range execNames {
		trimmed := strings.TrimSpace(execName)
		if trimmed == "" {
			continue
		}
		if found[trimmed] == "" {
			return nil, fmt.Errorf("executable %s not found in archive", trimmed)
		}
	}

	return found, nil
}

func extractTarXZExecutables(archivePath, destDir string, execNames []string, progress func(int)) (map[string]string, error) {
	tarPath, err := exec.LookPath("tar")
	if err != nil {
		return nil, fmt.Errorf("tar.xz archives are not supported on this system")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}

	command := exec.Command(tarPath, "-xJf", archivePath, "-C", destDir)
	configureCommand(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("extract tar.xz archive: %w %s", err, strings.TrimSpace(string(output)))
	}
	if progress != nil {
		progress(100)
	}

	targets := make(map[string]string, len(execNames))
	for _, execName := range execNames {
		trimmed := strings.TrimSpace(execName)
		if trimmed == "" {
			continue
		}
		targets[strings.ToLower(trimmed)] = trimmed
	}
	found := make(map[string]string, len(targets))
	walkErr := filepath.WalkDir(destDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if originalName, ok := targets[strings.ToLower(entry.Name())]; ok && found[originalName] == "" {
			found[originalName] = path
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	for _, execName := range execNames {
		trimmed := strings.TrimSpace(execName)
		if trimmed == "" {
			continue
		}
		if found[trimmed] == "" {
			return nil, fmt.Errorf("executable %s not found in archive", trimmed)
		}
	}
	return found, nil
}

func markExecutable(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if err := os.Chmod(path, 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		_ = exec.Command("xattr", "-dr", "com.apple.quarantine", path).Run()
	}
	return nil
}

func resolveVersion(ctx context.Context, name dependencies.DependencyName, execPath string) (string, error) {
	var args []string
	switch name {
	case dependencies.DependencyFFmpeg:
		args = []string{"-version"}
	default:
		args = []string{"--version"}
	}
	command := exec.CommandContext(ctx, execPath, args...)
	if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(execPath), ".cmd") {
		command = exec.CommandContext(ctx, "cmd", append([]string{"/c", execPath}, args...)...)
	}
	configureCommand(command)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, message)
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return "", fmt.Errorf("empty version output")
	}
	switch name {
	case dependencies.DependencyFFmpeg:
		return parseFFmpegVersion(text)
	default:
		return strings.Fields(text)[0], nil
	}
}

func parseFFmpegVersion(output string) (string, error) {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("ffmpeg version output empty")
	}
	fields := strings.Fields(lines[0])
	for i, field := range fields {
		if strings.EqualFold(field, "version") && i+1 < len(fields) {
			return strings.TrimSpace(fields[i+1]), nil
		}
	}
	return "", fmt.Errorf("ffmpeg version not found")
}

func percent(written int64, total int64) int {
	if total <= 0 {
		return 0
	}
	p := int(float64(written) / float64(total) * 100)
	if p > 100 {
		return 100
	}
	if p < 0 {
		return 0
	}
	return p
}

func mapProgress(progress int, start, end int) int {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	if start >= end {
		return start
	}
	return start + int(float64(progress)*(float64(end-start))/100.0)
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
