package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"xiadown/internal/application/softwareupdate"
)

func TestManifestCatalogProviderSelectsCurrentPlatformAssets(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"appId":"cc.dreamapp.xiadown",
			"manifestVersion":"2026.04.06.1",
			"defaultChannel":"stable",
			"updatedAt":"2026-04-06T02:42:11Z",
			"channels":{
				"stable":{
					"app":{
							"source":{"provider":"github-release","owner":"example-owner","repo":"xiadown"},
						"version":"1.3.0",
						"publishedAt":"2026-04-06T00:00:00Z",
						"platforms":{
							"darwin-arm64":{
								"artifactName":"xiadown-macos-arm64-1.3.0.zip",
								"sources":[{"name":"github","kind":"origin","url":"https://example.com/app.zip","priority":20,"enabled":true}],
								"installStrategy":"archive",
								"artifactType":"zip"
							},
							"windows-amd64":{
								"artifactName":"xiadown-windows-x64-1.3.0-installer.exe",
								"sources":[{"name":"github","kind":"origin","url":"https://example.com/app.exe","priority":20,"enabled":true}],
								"installStrategy":"app-installer",
								"artifactType":"exe"
							}
						}
					},
					"listen":{
						"liveChannel":{
							"schemaVersion":1,
							"url":"https://updates.example.com/listen/live/channel.json",
							"version":"2026.04.26.1",
							"updatedAt":"2026-04-26T08:30:00Z",
							"minAppVersion":"0.0.1",
							"ttlSeconds":300,
							"fallback":"embedded",
							"sha256":"abc123"
						}
					},
					"tools":{
						"ffmpeg":{
							"displayName":"FFmpeg",
							"kind":"dependency",
							"source":{"provider":"github-release","owner":"jellyfin","repo":"jellyfin-ffmpeg"},
							"upstreamVersion":"7.1.3-5",
							"recommendedVersion":"7.1.3-5",
							"publishedAt":"2026-04-06T00:00:00Z",
							"platforms":{
								"darwin-arm64":{
									"artifactName":"jellyfin-ffmpeg_7.1.3-5_portable_macarm64-gpl.tar.xz",
									"sources":[{"name":"github","kind":"origin","url":"https://example.com/ffmpeg.tar.xz","priority":20,"enabled":true}],
									"installStrategy":"archive",
									"artifactType":"tar.xz",
									"binaries":["ffmpeg","ffprobe"]
								}
							}
						}
					}
				}
			}
		}`))
	}))
	defer server.Close()

	provider := NewManifestCatalogProvider(server.Client(), server.URL)
	provider.goos = "darwin"
	provider.goarch = "arm64"

	catalog, err := provider.FetchCatalog(context.Background(), softwareupdate.Request{})
	if err != nil {
		t.Fatalf("fetch catalog failed: %v", err)
	}
	if catalog.App == nil {
		t.Fatal("expected app release")
	}
	if catalog.App.Asset.ArtifactName != "xiadown-macos-arm64-1.3.0.zip" {
		t.Fatalf("unexpected app asset: %s", catalog.App.Asset.ArtifactName)
	}
	ffmpeg, ok := catalog.Dependencies["ffmpeg"]
	if !ok {
		t.Fatal("expected ffmpeg release")
	}
	if ffmpeg.Asset.ArtifactType != "tar.xz" {
		t.Fatalf("unexpected ffmpeg artifact type: %s", ffmpeg.Asset.ArtifactType)
	}
	if ffmpeg.Asset.PrimaryExecutableName() != "ffmpeg" {
		t.Fatalf("unexpected primary executable: %s", ffmpeg.Asset.PrimaryExecutableName())
	}
	if catalog.Listen.LiveChannel.URL != "https://updates.example.com/listen/live/channel.json" {
		t.Fatalf("unexpected Listen live catalog URL: %s", catalog.Listen.LiveChannel.URL)
	}
	if catalog.Listen.LiveChannel.SchemaVersion != 1 {
		t.Fatalf("unexpected Listen live catalog schema version: %d", catalog.Listen.LiveChannel.SchemaVersion)
	}
	if catalog.Listen.LiveChannel.Version != "2026.04.26.1" {
		t.Fatalf("unexpected Listen live catalog version: %s", catalog.Listen.LiveChannel.Version)
	}
	if catalog.Listen.LiveChannel.UpdatedAt.IsZero() {
		t.Fatal("expected Listen live catalog updatedAt")
	}
	if catalog.Listen.LiveChannel.SHA256 != "abc123" {
		t.Fatalf("unexpected Listen live catalog sha256: %s", catalog.Listen.LiveChannel.SHA256)
	}
	if catalog.Listen.LiveChannel.MinAppVersion != "0.0.1" {
		t.Fatalf("unexpected Listen live catalog min app version: %s", catalog.Listen.LiveChannel.MinAppVersion)
	}
	if catalog.Listen.LiveChannel.TTLSeconds != 300 {
		t.Fatalf("unexpected Listen live catalog ttl: %d", catalog.Listen.LiveChannel.TTLSeconds)
	}
	if catalog.Listen.LiveChannel.Fallback != "embedded" {
		t.Fatalf("unexpected Listen live catalog fallback: %s", catalog.Listen.LiveChannel.Fallback)
	}
}
