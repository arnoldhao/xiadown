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
					"dreamFm":{
						"liveChannel":{
							"schemaVersion":1,
							"url":"https://updates.example.com/dream.fm/live/channel.json",
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
	if catalog.DreamFM.LiveChannel.URL != "https://updates.example.com/dream.fm/live/channel.json" {
		t.Fatalf("unexpected DreamFM live catalog URL: %s", catalog.DreamFM.LiveChannel.URL)
	}
	if catalog.DreamFM.LiveChannel.SchemaVersion != 1 {
		t.Fatalf("unexpected DreamFM live catalog schema version: %d", catalog.DreamFM.LiveChannel.SchemaVersion)
	}
	if catalog.DreamFM.LiveChannel.Version != "2026.04.26.1" {
		t.Fatalf("unexpected DreamFM live catalog version: %s", catalog.DreamFM.LiveChannel.Version)
	}
	if catalog.DreamFM.LiveChannel.UpdatedAt.IsZero() {
		t.Fatal("expected DreamFM live catalog updatedAt")
	}
	if catalog.DreamFM.LiveChannel.SHA256 != "abc123" {
		t.Fatalf("unexpected DreamFM live catalog sha256: %s", catalog.DreamFM.LiveChannel.SHA256)
	}
	if catalog.DreamFM.LiveChannel.MinAppVersion != "0.0.1" {
		t.Fatalf("unexpected DreamFM live catalog min app version: %s", catalog.DreamFM.LiveChannel.MinAppVersion)
	}
	if catalog.DreamFM.LiveChannel.TTLSeconds != 300 {
		t.Fatalf("unexpected DreamFM live catalog ttl: %d", catalog.DreamFM.LiveChannel.TTLSeconds)
	}
	if catalog.DreamFM.LiveChannel.Fallback != "embedded" {
		t.Fatalf("unexpected DreamFM live catalog fallback: %s", catalog.DreamFM.LiveChannel.Fallback)
	}
}
