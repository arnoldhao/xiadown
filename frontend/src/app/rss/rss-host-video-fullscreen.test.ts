import { describe, expect, test } from "bun:test";

import {
  resolveRSSHostVideoFullscreenOwnership,
  shouldRestoreRSSHostWindow,
} from "./host-video-fullscreen-state";

describe("RSS host-owned video fullscreen", () => {
  test("restores only a Wails fullscreen presentation owned by the player", () => {
    expect(resolveRSSHostVideoFullscreenOwnership(false)).toBe("owned");
    expect(resolveRSSHostVideoFullscreenOwnership(true)).toBe("preexisting");
    expect(shouldRestoreRSSHostWindow("owned")).toBeTrue();
    expect(shouldRestoreRSSHostWindow("preexisting")).toBeFalse();
    expect(shouldRestoreRSSHostWindow("none")).toBeFalse();
  });

  test("uses YouTube-parity native fullscreen for Bilibili and host fullscreen only for fallback web video", async () => {
    const [
      hookSource,
      bilibiliSource,
      bilibiliContractSource,
      surfaceSource,
      webSource,
      transportSource,
      css,
      rssAppearance,
      nativeHandlerSource,
      bridgeSource,
    ] =
      await Promise.all([
        Bun.file(new URL("./host-video-fullscreen.ts", import.meta.url)).text(),
        Bun.file(new URL("./RSSBilibiliPlayback.tsx", import.meta.url)).text(),
        Bun.file(new URL("./video-transport.ts", import.meta.url)).text(),
        Bun.file(new URL("./RSSBilibiliVideoSurface.tsx", import.meta.url)).text(),
        Bun.file(new URL("./RSSWebVideoPlayback.tsx", import.meta.url)).text(),
        Bun.file(
          new URL("../youtube/YouTubeWorkspaceTransportBar.tsx", import.meta.url),
        ).text(),
        Bun.file(new URL("./rss-workspace.css", import.meta.url)).text(),
        Bun.file(
          new URL("../../shared/styles/dream/rss.css", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../../../internal/presentation/wails/rss_video_player_handler.go", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../../../internal/presentation/wails/rss_video_transport_bridge.go", import.meta.url),
        ).text(),
      ]);

    expect(hookSource).toContain("Window.IsFullscreen()");
    expect(hookSource).toContain("Window.Fullscreen()");
    expect(hookSource).toContain("Window.UnFullscreen()");
    expect(hookSource).toContain("Events.Types.Common.WindowUnFullscreen");
    expect(hookSource).toContain("root.dataset.rssHostVideoFullscreen");

    expect(bilibiliSource).toContain("requestRSSBilibiliVideoFullscreen(sessionID)");
    expect(bilibiliSource).toContain("exitRSSBilibiliVideoFullscreen(sessionID)");
    expect(bilibiliSource).toContain("fullscreenActive={status.fullscreen}");
    expect(bilibiliSource).toContain("onFullscreen={toggleFullscreen}");
    expect(bilibiliSource).toContain("active={active && nativeReady}");
    expect(bilibiliSource).toContain(
      "geometrySuspended={status.fullscreen || fullscreenRequestPending}",
    );
    expect(bilibiliSource).toContain("isRSSBilibiliNativeReady(descriptor, status)");
    expect(bilibiliContractSource).toContain("status.controls?.playPause === true");
    expect(bilibiliContractSource).toContain(
      "isRSSBilibiliVideoStatusForSession(status, sessionID)",
    );
    expect(bilibiliSource).not.toContain("useRSSHostVideoFullscreen");
    expect(surfaceSource).toContain("geometrySuspendedRef.current");
    expect(surfaceSource).toContain("resumeGeometryRef.current");
    expect(nativeHandlerSource).toContain("requestNativeWindowFullscreenLocked");
    expect(nativeHandlerSource).toContain("restoreAfterNativeWindowFullscreenLocked");
    expect(nativeHandlerSource).toContain("handleNativeWindowFullscreenEvent");
    expect(nativeHandlerSource).toContain("fullscreenGeneration");
    expect(bridgeSource).toContain("data-xiadown-rss-bilibili-fullscreen");
    expect(bridgeSource).toContain("fullscreenPresentation(active)");

    expect(webSource).toContain("useRSSHostVideoFullscreen(direct || embed)");
    expect(webSource).toContain("fullscreenAvailable: direct || embed");
    expect(webSource).not.toContain("requestFullscreen()");
    expect(webSource).not.toContain("document.fullscreenElement");
    expect(webSource).not.toContain("allowFullScreen");
    expect(webSource).not.toContain("picture-in-picture; fullscreen");

    expect(transportSource).toContain("fullscreenActive ? <Minimize2 /> : <Maximize2 />");
    expect(css).toContain(':root[data-rss-host-video-fullscreen="true"]');
    expect(css).toContain('.rss-host-video-playback[data-host-fullscreen="true"]');
    expect(css).toContain("z-index: var(--app-layer-fullscreen)");
    expect(css).toContain('.rss-bilibili-video-surface[data-native-visible="true"]');
    expect(rssAppearance).toMatch(
      /\.rss-bilibili-video-surface \{[^}]*border-radius: inherit;/,
    );
  });
});
