import { describe, expect, test } from "bun:test";

import { RSSSitePrepareLifecycle } from "./site-player-lifecycle";
import { nextRSSSiteVideoSurfaceSequence } from "./site-video-surface-sequence";

describe("RSS interactive site playback", () => {
  test("settles only the newest Prepare and cancels cleanup once", () => {
    let requestId = 40;
    const lifecycle = new RSSSitePrepareLifecycle(() => ++requestId);
    const older = lifecycle.begin();
    const newer = lifecycle.begin();

    expect(lifecycle.isCurrent(older)).toBeFalse();
    expect(lifecycle.isCurrent(newer)).toBeTrue();
    expect(lifecycle.settle(newer)).toEqual({ pending: true, current: true });
    expect(lifecycle.cancel(older)).toBeTrue();
    expect(lifecycle.cancel(older)).toBeFalse();
  });

  test("keeps geometry sequence monotonic across same-tick updates and rollback", () => {
    const now = 2_000_000;
    const first = nextRSSSiteVideoSurfaceSequence(now);
    const second = nextRSSSiteVideoSurfaceSequence(now);
    const rollback = nextRSSSiteVideoSurfaceSequence(now - 10_000);
    expect(second).toBeGreaterThan(first);
    expect(rollback).toBeGreaterThan(second);
  });

  test("uses an interactive composition aperture and leaves the fake footer inert", async () => {
    const [source, apiSource, css, rssAppearance] = await Promise.all([
      Bun.file(new URL("./RSSSiteVideoPlayback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./api.ts", import.meta.url)).text(),
      Bun.file(new URL("./rss-workspace.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/rss.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("prepareRSSSiteVideo({");
    expect(source).toContain("await acceptRSSSiteVideoPrepare(prepareToken.requestId)");
    expect(source).toContain("cancelRSSSiteVideoPrepare(prepareToken.requestId)");
    expect(source).toContain("closeRSSSiteVideo(sessionID)");
    expect(source).toContain("showRSSSiteVideo(sessionID, {");
    expect(source).toContain("interactive: true");
    expect(source).toContain("active && System.IsWindows()");
    expect(source).toContain("useListenNativeVideoUnderlay(");
    expect(source).toContain("nativeUnderlayActive");
    expect(source).toContain('dataset.rssSiteVideoActive = "true"');
    expect(source).toContain("setHole(rect, radius)");
    expect(source).toContain("resetHole()");
    expect(source).toContain("RSS_NATIVE_OVERLAY_BLOCKER_SELECTOR");
    expect(source).toContain(".app-secondary-reveal__positioner");
    expect(source).toContain("geometrySuspended ||");
    expect(source).toContain("RSS_SITE_NATIVE_SHOW_RETRY_LIMIT");
    expect(source).toContain("retryTimer = window.setTimeout(show, 500)");
    expect(source).toContain("surfaceRectFullyVisible");
    expect(source).toContain('const inertAttribute = { inert: "" }');
    expect(source).not.toContain("<iframe");
    expect(apiSource).toContain("RSSSitePlayerHandler");
    expect(apiSource).toContain("rect: { ...rect, interactive: true }");
    expect(css).toMatch(
      /\.rss-site-video-transport \{[^}]*pointer-events: none;/s,
    );
    expect(rssAppearance).toMatch(
      /\.rss-site-video-transport \{[^}]*opacity: 0\.3;[^}]*filter: blur\(3px\) saturate\(0\.55\);/s,
    );
    expect(css).toContain(".rss-site-video-transport-notice");
    expect(rssAppearance).toContain('[data-rss-site-video-active="true"]');
    expect(rssAppearance).toMatch(/data-rss-site-video-active="true"[\s\S]*?\.app-workspace-primary-pane[\s\S]*?\.rss-site-video-surface[\s\S]*?background:\s*transparent\s*!important/s);
  });
});
