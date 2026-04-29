package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/dependencies/dto"
	"xiadown/internal/domain/dependencies"
)

type memoryRepo struct {
	items map[string]dependencies.Dependency
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{items: make(map[string]dependencies.Dependency)}
}

func (repo *memoryRepo) List(_ context.Context) ([]dependencies.Dependency, error) {
	result := make([]dependencies.Dependency, 0, len(repo.items))
	for _, item := range repo.items {
		result = append(result, item)
	}
	return result, nil
}

func (repo *memoryRepo) Get(_ context.Context, name string) (dependencies.Dependency, error) {
	item, ok := repo.items[name]
	if !ok {
		return dependencies.Dependency{}, dependencies.ErrDependencyNotFound
	}
	return item, nil
}

func (repo *memoryRepo) Save(_ context.Context, dependency dependencies.Dependency) error {
	repo.items[string(dependency.Name)] = dependency
	return nil
}

func (repo *memoryRepo) Delete(_ context.Context, name string) error {
	delete(repo.items, name)
	return nil
}

func TestEnsureDefaultsIncludesCoreDependencies(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	service := NewDependenciesService(repo, nil, "")
	if err := service.EnsureDefaults(context.Background()); err != nil {
		t.Fatalf("ensure defaults failed: %v", err)
	}
	for _, name := range []dependencies.DependencyName{
		dependencies.DependencyYTDLP,
		dependencies.DependencyFFmpeg,
		dependencies.DependencyBun,
	} {
		if _, err := repo.Get(context.Background(), string(name)); err != nil {
			t.Fatalf("expected default dependency %s: %v", name, err)
		}
	}
}

func TestDependencyReadinessReasons(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newMemoryRepo()
	service := NewDependenciesService(repo, nil, "")
	if err := service.EnsureDefaults(ctx); err != nil {
		t.Fatalf("ensure defaults failed: %v", err)
	}

	ready, reason, err := service.DependencyReadiness(ctx, dependencies.DependencyBun)
	if err != nil {
		t.Fatalf("dependency readiness failed: %v", err)
	}
	if ready {
		t.Fatalf("expected bun to be not ready by default")
	}
	if reason != "not_installed" {
		t.Fatalf("unexpected reason: %s", reason)
	}

	now := time.Now()
	missingPathDependency, err := dependencies.NewDependency(dependencies.DependencyParams{
		Name:      string(dependencies.DependencyBun),
		ExecPath:  filepath.Join(t.TempDir(), "missing-bun"),
		Version:   "1.2.3",
		Status:    string(dependencies.StatusInstalled),
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new dependency failed: %v", err)
	}
	if err := repo.Save(ctx, missingPathDependency); err != nil {
		t.Fatalf("save dependency failed: %v", err)
	}

	ready, reason, err = service.DependencyReadiness(ctx, dependencies.DependencyBun)
	if err != nil {
		t.Fatalf("dependency readiness failed: %v", err)
	}
	if ready {
		t.Fatalf("expected bun to be not ready when path is missing")
	}
	if reason != "exec_not_found" {
		t.Fatalf("unexpected reason: %s", reason)
	}

	execDir := t.TempDir()
	execPath := filepath.Join(execDir, "bun")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho bun\n"), 0o755); err != nil {
		t.Fatalf("write exec file failed: %v", err)
	}
	readyDependency, err := dependencies.NewDependency(dependencies.DependencyParams{
		Name:      string(dependencies.DependencyBun),
		ExecPath:  execPath,
		Version:   "1.2.3",
		Status:    string(dependencies.StatusInstalled),
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new dependency failed: %v", err)
	}
	if err := repo.Save(ctx, readyDependency); err != nil {
		t.Fatalf("save dependency failed: %v", err)
	}
	ready, reason, err = service.DependencyReadiness(ctx, dependencies.DependencyBun)
	if err != nil {
		t.Fatalf("dependency readiness failed: %v", err)
	}
	if !ready {
		t.Fatalf("expected bun to be ready")
	}
	if reason != "" {
		t.Fatalf("expected empty reason, got: %s", reason)
	}
}

func TestListDependenciesMarksFFmpegInvalidWhenFFprobeMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newMemoryRepo()
	service := NewDependenciesService(repo, nil, "")
	now := time.Now()

	execDir := t.TempDir()
	ffmpegPath := filepath.Join(execDir, executableNameForBinary("ffmpeg"))
	if err := os.WriteFile(ffmpegPath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write ffmpeg stub failed: %v", err)
	}
	dependency, err := dependencies.NewDependency(dependencies.DependencyParams{
		Name:      string(dependencies.DependencyFFmpeg),
		ExecPath:  ffmpegPath,
		Version:   "7.1.1",
		Status:    string(dependencies.StatusInstalled),
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new dependency failed: %v", err)
	}
	if err := repo.Save(ctx, dependency); err != nil {
		t.Fatalf("save dependency failed: %v", err)
	}

	items, err := service.ListDependencies(ctx)
	if err != nil {
		t.Fatalf("list dependencies failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(items))
	}
	if items[0].Status != string(dependencies.StatusInvalid) {
		t.Fatalf("expected ffmpeg to be invalid when ffprobe is missing, got %s", items[0].Status)
	}
}

func TestSetDependencyPathFFmpegRequiresFFprobe(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newMemoryRepo()
	service := NewDependenciesService(repo, nil, "")

	execDir := t.TempDir()
	ffmpegPath := filepath.Join(execDir, executableNameForBinary("ffmpeg"))
	ffprobePath := filepath.Join(execDir, executableNameForBinary("ffprobe"))
	ffmpegScript := "#!/bin/sh\nprintf 'ffmpeg version 7.1.1\\n'"
	ffprobeScript := "#!/bin/sh\nprintf 'ffprobe version 7.1.1\\n'"

	if err := os.WriteFile(ffmpegPath, []byte(ffmpegScript), 0o755); err != nil {
		t.Fatalf("write ffmpeg stub failed: %v", err)
	}

	result, err := service.SetDependencyPath(ctx, dto.SetDependencyPathRequest{
		Name:     string(dependencies.DependencyFFmpeg),
		ExecPath: ffmpegPath,
	})
	if err != nil {
		t.Fatalf("set dependency path failed: %v", err)
	}
	if result.Status != string(dependencies.StatusInvalid) {
		t.Fatalf("expected invalid status without ffprobe, got %s", result.Status)
	}

	if err := os.WriteFile(ffprobePath, []byte(ffprobeScript), 0o755); err != nil {
		t.Fatalf("write ffprobe stub failed: %v", err)
	}

	result, err = service.SetDependencyPath(ctx, dto.SetDependencyPathRequest{
		Name:     string(dependencies.DependencyFFmpeg),
		ExecPath: ffmpegPath,
	})
	if err != nil {
		t.Fatalf("set dependency path failed: %v", err)
	}
	if result.Status != string(dependencies.StatusInstalled) {
		t.Fatalf("expected installed status with ffprobe present, got %s", result.Status)
	}
	if strings.TrimSpace(result.Version) != "7.1.1" {
		t.Fatalf("unexpected ffmpeg version: %s", result.Version)
	}
}

func TestManagedDependencyVersionFromPathUsesManagedVersionDirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join("/Users/test/Library/Application Support/xiadown/dependencies", "ffmpeg", "7.1.3-5", "bin", executableNameForBinary("ffmpeg"))
	version := managedDependencyVersionFromPath(dependencies.DependencyFFmpeg, path)
	if version != "7.1.3-5" {
		t.Fatalf("unexpected managed version: %s", version)
	}
}

func TestToDependencyDTOUsesManagedVersionFromPath(t *testing.T) {
	t.Parallel()

	now := time.Now()
	dependency, err := dependencies.NewDependency(dependencies.DependencyParams{
		Name:      string(dependencies.DependencyFFmpeg),
		ExecPath:  filepath.Join("/tmp/xiadown/dependencies", "ffmpeg", "7.1.3-5", "bin", executableNameForBinary("ffmpeg")),
		Version:   "7.1.3-jellyfin",
		Status:    string(dependencies.StatusInstalled),
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new dependency failed: %v", err)
	}

	dto := toDependencyDTO(dependency)
	if dto.Version != "7.1.3-5" {
		t.Fatalf("unexpected dto version: %s", dto.Version)
	}
}

func TestToDependencyDTOIncludesSourceMetadata(t *testing.T) {
	t.Parallel()

	now := time.Now()
	item, err := dependencies.NewDependency(dependencies.DependencyParams{
		Name:      string(dependencies.DependencyBun),
		ExecPath:  "/tmp/bun",
		Version:   "1.2.3",
		Status:    string(dependencies.StatusInstalled),
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("new dependency failed: %v", err)
	}
	result := toDependencyDTO(item)
	if result.Kind != string(dependencies.KindBin) {
		t.Fatalf("expected bin kind, got %q", result.Kind)
	}
	if result.SourceKind != sourceKindGitHubRelease {
		t.Fatalf("expected GitHub release source kind, got %q", result.SourceKind)
	}
	if result.SourceRef != "oven-sh/bun" {
		t.Fatalf("expected bun source ref, got %q", result.SourceRef)
	}
	if result.Manager != "" {
		t.Fatalf("expected empty manager, got %q", result.Manager)
	}
}
