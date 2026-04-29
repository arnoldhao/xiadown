package softwareupdate

import (
	"context"
	"errors"
	"testing"
	"time"

	"xiadown/internal/domain/dependencies"
)

type catalogProviderStub struct {
	catalog Catalog
	err     error
	calls   int
}

func (stub *catalogProviderStub) FetchCatalog(_ context.Context, _ Request) (Catalog, error) {
	stub.calls++
	if stub.err != nil {
		return Catalog{}, stub.err
	}
	return stub.catalog, nil
}

func TestEnsureCatalogRetriesAfterFailedRefresh(t *testing.T) {
	t.Parallel()

	provider := &catalogProviderStub{err: errors.New("network unavailable")}
	service := NewService(ServiceParams{CatalogProvider: provider})

	if _, err := service.EnsureCatalog(context.Background(), 10*time.Minute, Request{}); err == nil {
		t.Fatal("expected first catalog refresh to fail")
	}
	provider.err = nil
	provider.catalog = Catalog{
		Dependencies: map[dependencies.DependencyName]DependencyRelease{
			dependencies.DependencyYTDLP: {
				Name:               dependencies.DependencyYTDLP,
				RecommendedVersion: "2026.03.17",
			},
		},
	}

	snapshot, err := service.EnsureCatalog(context.Background(), 10*time.Minute, Request{})
	if err != nil {
		t.Fatalf("expected second catalog refresh to recover: %v", err)
	}
	if provider.calls != 2 {
		t.Fatalf("expected failed catalog cache to be bypassed, got %d calls", provider.calls)
	}
	if _, ok := snapshot.Catalog.Dependency(dependencies.DependencyYTDLP); !ok {
		t.Fatal("expected recovered catalog")
	}
}

func TestResolveDependencyReleaseUsesManifestCatalogFirst(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceParams{
		CatalogProvider: &catalogProviderStub{
			catalog: Catalog{
				Dependencies: map[dependencies.DependencyName]DependencyRelease{
					dependencies.DependencyYTDLP: {
						Name:               dependencies.DependencyYTDLP,
						RecommendedVersion: "2026.03.17",
					},
				},
			},
		},
	})

	release, err := service.ResolveDependencyRelease(context.Background(), DependencyRequest{Name: dependencies.DependencyYTDLP})
	if err != nil {
		t.Fatalf("resolve dependency release failed: %v", err)
	}
	if release.ResolvedBy != SourceManifest {
		t.Fatalf("expected manifest source, got %q", release.ResolvedBy)
	}
	if release.TargetVersion() != "2026.03.17" {
		t.Fatalf("unexpected target version: %s", release.TargetVersion())
	}
}

func TestResolveDependencyReleaseReturnsNotFoundWithoutManifestEntry(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceParams{
		CatalogProvider: &catalogProviderStub{catalog: Catalog{}},
	})

	if _, err := service.ResolveDependencyRelease(context.Background(), DependencyRequest{Name: dependencies.DependencyBun}); err != ErrReleaseNotFound {
		t.Fatalf("expected ErrReleaseNotFound, got %v", err)
	}
}
