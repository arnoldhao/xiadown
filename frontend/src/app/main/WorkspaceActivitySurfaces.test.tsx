import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { siYoutubemusic } from "simple-icons";

import { AppShell } from "@/app/workspace/AppShell";
import { getXiaText } from "@/features/xiadown/shared";
import { projectOperationActivitySnapshot } from "@/shared/activity/operations";
import type { SniffStatusSnapshot } from "@/shared/activity/sniff";
import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import {
  ListenNowPlayingHoverPanel,
  resolveListenArtworkCandidates,
} from "./sidebar";

import {
  OperationsCompanionView,
  PlayerCompanionFooter,
  PlayerCompanionView,
  isWorkspaceActivityContextMenuKey,
  resolveWorkspaceActivityKeyboardMenuPoint,
  resolveWorkspaceActivityPointerMenuPoint,
  SniffCompanionFooter,
  SniffCompanionView,
  SniffWorkspaceSessionActivity,
  WideOperationActivity,
  WidePlaybackActivity,
  WideSniffActivity,
  type WorkspaceActivityLabels,
} from "./WorkspaceActivitySurfaces";

const labels: WorkspaceActivityLabels = {
  sniff: "Sniff",
  stopSniff: "Stop sniff",
  sniffState: {
    idle: "Idle",
    starting: "Starting",
    active: "Active",
    closing: "Closing",
    error: "Error",
    orphan: "Detached",
  },
  resources: "Resources",
  downloadable: "Downloadable",
  session: "Session",
  updated: "Updated",
  clear: "Clear",
  operations: "Operations",
  downloads: "Downloads",
  transcodes: "Transcodes",
  nowPlaying: "Now playing",
  previous: "Previous",
  play: "Play",
  pause: "Pause",
  next: "Next",
  noActivity: "No activity",
};

const panelText = getXiaText("en");

const sniffStatus: SniffStatusSnapshot = {
  runtime: "managed",
  state: "active",
  title: "Example",
  url: "https://example.com",
  favicon: "https://example.com/favicon.ico",
  resourceCount: 3,
  downloadableCount: 2,
  canClear: true,
  canStop: true,
};

const sniffSessionLabels = {
  sniff: "Sniff",
  session: "Session details",
  resources: "Resources",
  downloadable: "Downloadable",
  status: "Active",
  updated: "Updated",
};

describe("workspace activity surfaces", () => {
  test("opens status dropdowns from pointer and platform context-menu keys", async () => {
    expect(resolveWorkspaceActivityPointerMenuPoint(128, 216)).toEqual({
      x: 128,
      y: 216,
    });
    expect(
      resolveWorkspaceActivityKeyboardMenuPoint({
        left: 40,
        bottom: 96,
        width: 120,
      }),
    ).toEqual({ x: 100, y: 96 });
    expect(isWorkspaceActivityContextMenuKey("ContextMenu")).toBe(true);
    expect(isWorkspaceActivityContextMenuKey("F10", true)).toBe(true);
    expect(isWorkspaceActivityContextMenuKey("F10", false)).toBe(false);
    expect(isWorkspaceActivityContextMenuKey("Enter", true)).toBe(false);

    const source = await Bun.file(
      new URL("./WorkspaceActivitySurfaces.tsx", import.meta.url),
    ).text();
    expect(source).toContain("onContextMenu: (event) =>");
    expect(source).toContain("event.preventDefault()");
    expect(source).toContain("event.stopPropagation()");
    expect(source).toContain("onCloseAutoFocus");
    expect(source).toContain("returnFocus?.isConnected");
    expect(source).toContain("WORKSPACE_ACTIVITY_FALLBACK_FOCUS_SELECTOR");
    expect(source).toContain(
      '.app-workspace-nav-button[data-active="true"]',
    );
  });

  test("uses adaptive icon-and-label dropdown anatomy for every status card", async () => {
    const [source, css, sharedMenuStyles] = await Promise.all([
      Bun.file(
        new URL("./WorkspaceActivitySurfaces.tsx", import.meta.url),
      ).text(),
      Bun.file(new URL("../../shared/styles/dream/activity.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/controls.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("app-workspace-status-context-menu__icon");
    expect(source).toContain("app-workspace-status-context-menu__label");
    expect(source.match(/actions=\{props\.menuActions\}/g)).toHaveLength(3);
    expect(css).toMatch(
      /\.app-menu-content\.app-workspace-status-context-menu\s*\{[^}]*width:\s*max-content[^}]*min-width:\s*0[^}]*max-width:\s*calc\(100vw - 16px\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-status-context-menu__item\s*\{[^}]*display:\s*grid[^}]*grid-template-columns:\s*18px minmax\(0, 1fr\)/s,
    );
    expect(sharedMenuStyles).toMatch(
      /:is\(\.app-dream-menu-item, \.app-menu-item\)\[data-tone="destructive"\]:is\([\s\S]*?\)\s*\{[^}]*color:\s*hsl\(var\(--destructive\)\)[^}]*background:\s*hsl\(var\(--destructive\)\s*\/\s*0\.11\)/s,
    );
    expect(sharedMenuStyles).toMatch(
      /@media \(forced-colors:\s*active\)[\s\S]*?:is\(\.app-dream-menu-item, \.app-menu-item\):is\([\s\S]*?\)\s*\{[^}]*color:\s*HighlightText[^}]*background:\s*Highlight[^}]*forced-color-adjust:\s*none/s,
    );
    expect(css).not.toContain(
      '.app-workspace-status-context-menu__item[data-tone="destructive"]',
    );
  });

  test("routes Stop through a backend before clearing the player card", async () => {
    const [
      mainSource,
      listenSource,
      typesSource,
      youtubeSource,
      youtubeTypesSource,
    ] = await Promise.all([
      Bun.file(new URL("./MainApp.tsx", import.meta.url)).text(),
      Bun.file(new URL("./Listen.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen/types.ts", import.meta.url)).text(),
      Bun.file(
        new URL("../youtube/YouTubeWorkspacePage.tsx", import.meta.url),
      ).text(),
      Bun.file(new URL("../youtube/types.ts", import.meta.url)).text(),
    ]);

    expect(mainSource).toContain(
      "await playbackCoordinator.commands.closeSession(activeSession.id)",
    );
    expect(mainSource).toContain(
      'sendListenCommand("stop", undefined, undefined, true)',
    );
    expect(mainSource).toContain('if (source !== "youtube")');
    expect(mainSource).toContain(
      'onSelect: () => openMusicPlaybackSource(source)',
    );
    expect(mainSource).toContain('onSelect: () => openLibraryRoute("running")');
    expect(mainSource).toContain(
      'onSelect: () => void stopSniffActivity()',
    );
    expect(mainSource).toContain(
      "stopSniff: text.sniffDesk.stopSniff",
    );
    expect(mainSource).not.toContain(
      "stopSniff: text.sniffDesk.cdpClose",
    );
    expect(mainSource).not.toContain(
      "close: text.sniffDesk.stopSniff",
    );
    expect(listenSource).toContain("if (command.backendStopped)");
    expect(listenSource).toContain("Call.ByName(`${service}.Reset`)");
    expect(listenSource.indexOf("Call.ByName(`${service}.Reset`)")).toBeLessThan(
      listenSource.indexOf(".then(clearStoppedPlayback)"),
    );
    expect(typesSource).toContain('    | "stop"');
    expect(typesSource).toContain("backendStopped?: boolean");
    expect(mainSource).toContain('if (source === "youtube") {');
    expect(mainSource).toContain("setYouTubePlayback(null)");
    expect(mainSource).toContain('command: "stop",\n            revealWatch: false');
    expect(mainSource.indexOf("closeSession(activeSession.id)")).toBeLessThan(
      mainSource.indexOf("setYouTubePlayback(null)"),
    );
    expect(youtubeTypesSource).toContain('"previous" | "next" | "stop"');
    expect(youtubeSource).toContain("if (result.clearPlayback)");
    expect(youtubeSource).toContain("setPlayback(null)");
    expect(youtubeSource).toContain("setPlayerStatus({})");
    expect(youtubeSource).toContain("setQueue([])");
    expect(youtubeSource).toContain("setCurrentIndex(-1)");
    expect(youtubeSource).toContain(
      "restoredSessionID === stoppedPlaybackSessionIDRef.current",
    );
  });

  test("routes every sidebar status material through one semantic wrapper", async () => {
    const source = await Bun.file(
      new URL("./WorkspaceActivitySurfaces.tsx", import.meta.url),
    ).text();

    expect(source.match(/<WorkspaceStatusSurface\b/g)).toHaveLength(5);
    expect(source.match(/<GlassSurface\b/g)).toHaveLength(1);
    expect(source).not.toContain("<GlassGroup");
    expect(source.match(/surfaceRole="status"/g)).toHaveLength(1);
    expect(source).not.toContain("CompactActivityGroup");
    expect(source).not.toContain("CompactSniffActivity");
    expect(source).not.toContain("CompactPlaybackActivity");
    expect(source).not.toContain("SniffStatusRevealPanel");
    expect(source).not.toContain("useWorkspaceSurfaceStyle");
    expect(source).not.toMatch(/material(?:=|:)/);
  });

  test("keeps the semantic status contract unchanged in Contrast", () => {
    const markup = renderToStaticMarkup(
      <AppShell
        navigation={<span>navigation</span>}
        surfaceStyle="contrast"
      >
        <WideSniffActivity
          labels={labels}
          onOpen={() => undefined}
          onStop={() => undefined}
          status={sniffStatus}
        />
      </AppShell>,
    );

    expect(markup).toContain('data-surface-style="contrast"');
    expect(markup).toContain('data-surface-role="status"');
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-tint="neutral"');
  });

  test("reuses the artwork-driven player as content inside one portal material", async () => {
    const markup = renderToStaticMarkup(
      <ListenNowPlayingHoverPanel
        embedded
        onControlCommand={() => undefined}
        status={{
          artworkURL: "https://catalog.example/cover.jpg",
          canControl: true,
          canNext: true,
          canPrevious: true,
          mode: "muse",
          progress: { bufferedTime: 12, currentTime: 8, duration: 24 },
          state: "playing",
          subtitle: "artist.value",
          title: "track.value",
        }}
        surface="dark"
        text={panelText}
      />,
    );

    expect(markup).toContain('data-embedded="true"');
    expect(markup).toContain('data-surface="dark"');
    expect(markup).toContain("listen-panel-artwork-glow");
    expect(markup).toContain("listen-panel-artwork-main");
    expect(markup).toContain("listen-panel-color-wash");
    expect(markup).toContain("listen-mini-primary-control");
    expect(markup).toContain("listen-mini-progress-track");
    expect(markup).not.toContain("app-glass-surface");

    const css = await Bun.file(
      new URL("../../shared/styles/dream/components.css", import.meta.url),
    ).text();
    expect(css).toMatch(
      /\.listen-now-playing-panel\s*\{[^}]*border-radius:\s*22px[^}]*clip-path:\s*inset\(0 round 22px\)/s,
    );
    expect(css).toMatch(
      /\.listen-now-playing-panel\[data-embedded="true"\]\s*\{[^}]*background:\s*transparent[^}]*border-radius:\s*0[^}]*box-shadow:\s*none[^}]*clip-path:\s*none[^}]*backdrop-filter:\s*none/s,
    );
    expect(css).toMatch(
      /\.listen-panel-layout,\s*\.listen-panel-ring\s*\{[^}]*border-radius:\s*21px/s,
    );
    expect(css).toMatch(
      /\.listen-now-playing-panel\[data-embedded="true"\] \.listen-panel-layout,\s*\.listen-now-playing-panel\[data-embedded="true"\] \.listen-panel-ring\s*\{[^}]*border-radius:\s*0/s,
    );
    expect(markup).not.toContain("rounded-[22px]");
    expect(markup).not.toContain("rounded-[21px]");
  });

  test("maps idle and error states onto the shared status tone vocabulary", () => {
    const idleSniff = renderToStaticMarkup(
      <WideSniffActivity
        labels={labels}
        onOpen={() => undefined}
        onStop={() => undefined}
        status={{ ...sniffStatus, state: "idle" }}
      />,
    );
    const failedPlayback = renderToStaticMarkup(
      <WidePlaybackActivity
        labels={labels}
        onCommand={() => undefined}
        onOpen={() => undefined}
        status={{
          artworkURL: "",
          canControl: true,
          mode: "muse",
          progress: { bufferedTime: 0, currentTime: 0, duration: 0 },
          state: "error",
          subtitle: "artist.value",
          title: "track.value",
        }}
      />,
    );

    expect(idleSniff).toContain('data-tone="idle"');
    expect(failedPlayback).toContain('data-tone="error"');
  });

  test("renders the wide sniff stop affordance as a labelled icon only", () => {
    const markup = renderToStaticMarkup(
      <WideSniffActivity
        labels={labels}
        onOpen={() => undefined}
        onStop={() => undefined}
        status={sniffStatus}
      />,
    );

    const openIndex = markup.indexOf("app-workspace-sniff-wide__open");
    const stopIndex = markup.indexOf("app-workspace-sniff-wide__stop");
    expect(markup).toContain("app-workspace-status-card");
    expect(markup).toContain('data-surface-role="status"');
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-tint="neutral"');
    expect(markup).toContain('data-tone="success"');
    expect(markup).toContain('data-artwork="true"');
    expect(markup).toContain("app-workspace-status-card__artwork-backdrop");
    expect(markup).toContain("app-workspace-sniff-wide__backdrop");
    expect(markup).toContain("https://example.com/favicon.ico");
    expect(markup).toContain("app-workspace-sniff-wide__stop");
    expect(markup).toContain('aria-label="Stop sniff"');
    expect(markup).toContain('title="Stop sniff"');
    expect(markup).not.toContain("<span>Stop sniff</span>");
    expect(openIndex).toBeGreaterThan(-1);
    expect(stopIndex).toBeGreaterThan(openIndex);
  });

  test("renders the sniff workspace control panel with one metric row and no actions", () => {
    const markup = renderToStaticMarkup(
      <SniffWorkspaceSessionActivity
        labels={sniffSessionLabels}
        status={sniffStatus}
      />,
    );

    expect(markup).toContain("app-workspace-status-card");
    expect(markup).toContain("app-glass-surface");
    expect(markup).toContain('data-surface-role="status"');
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-tint="neutral"');
    expect(markup).toContain('data-tone="success"');
    expect(markup).toContain("app-workspace-sniff-session__control");
    expect(markup).toContain("app-workspace-sniff-session__favicon");
    expect(markup).toContain("example.com");
    expect(markup).toContain('data-details-open="true"');
    expect(markup).toContain('data-state="active"');
    expect(markup).not.toContain('aria-expanded=');
    expect(markup).toContain("app-workspace-sniff-session__metrics");
    expect(
      markup.match(/class="app-workspace-sniff-session__metric"/g),
    ).toHaveLength(4);
    expect(markup).not.toContain("app-workspace-sniff-session__detail-actions");
    expect(markup).not.toContain("<button");
  });

  test("mirrors the expanded sniff details and icon actions in the companion", () => {
    const contentMarkup = renderToStaticMarkup(
      <SniffCompanionView
        labels={labels}
        status={sniffStatus}
      />,
    );
    const footerMarkup = renderToStaticMarkup(
      <SniffCompanionFooter
        labels={labels}
        onClear={() => undefined}
        onStop={() => undefined}
        status={sniffStatus}
      />,
    );

    expect(contentMarkup).toContain("app-workspace-sniff-companion__identity");
    expect(contentMarkup).toContain(
      'data-companion-scroll-owner="sniff"',
    );
    expect(contentMarkup).toContain("https://example.com/favicon.ico");
    expect(contentMarkup).toContain(">Active<");
    expect(contentMarkup).toContain(">Updated<");
    expect(contentMarkup).not.toContain('aria-label="Clear"');
    expect(contentMarkup).not.toContain('aria-label="Stop sniff"');
    expect(footerMarkup).toContain("app-workspace-sniff-companion__actions");
    expect(footerMarkup).toContain('aria-label="Clear"');
    expect(footerMarkup).toContain('aria-label="Stop sniff"');
    expect(footerMarkup).not.toContain(">Stop sniff</button>");
  });

  test("separates playback summary content from its transport footer", () => {
    const status = {
      artworkURL: "https://catalog.example/cover.jpg",
      canControl: true,
      canNext: true,
      canPrevious: false,
      mode: "muse" as const,
      progress: { bufferedTime: 8, currentTime: 4, duration: 20 },
      state: "playing" as const,
      subtitle: "artist.value",
      title: "track.value",
    };
    const contentMarkup = renderToStaticMarkup(
      <PlayerCompanionView labels={labels} status={status} />,
    );
    const footerMarkup = renderToStaticMarkup(
      <PlayerCompanionFooter
        labels={labels}
        onCommand={() => undefined}
        status={status}
      />,
    );

    expect(contentMarkup).toContain("app-workspace-player-companion__artwork");
    expect(contentMarkup).toContain("app-workspace-player-companion__timeline");
    expect(contentMarkup).toContain(
      'data-companion-scroll-owner="playback-summary"',
    );
    expect(contentMarkup).not.toContain('aria-label="Previous"');
    expect(footerMarkup).toContain("app-workspace-player-companion__controls");
    expect(footerMarkup).toMatch(/aria-label="Previous"[^>]*disabled/);
    expect(footerMarkup).toContain('aria-label="Pause"');
    expect(footerMarkup).toContain('aria-label="Next"');
    expect(footerMarkup).not.toContain("track.value");
  });

  test("uses the shared 20px companion gutter without shrinking playback artwork", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/activity.css", import.meta.url),
    ).text();

    expect(css).toMatch(
      /\.app-workspace-player-companion,\s*\.app-workspace-sniff-companion,\s*\.app-workspace-operations-companion\s*\{[^}]*padding-inline:\s*var\(--app-workspace-companion-gutter, 1\.25rem\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-player-companion__artwork\s*\{[^}]*width:\s*min\(280px, 82%\)/s,
    );
  });

  test("does not render a sniff workspace session without a runtime", () => {
    const markup = renderToStaticMarkup(
      <SniffWorkspaceSessionActivity
        labels={sniffSessionLabels}
        status={{ ...sniffStatus, runtime: "none", state: "idle" }}
      />,
    );

    expect(markup).toBe("");
  });

  test("keeps the session state visible while closing without sidebar actions", () => {
    const markup = renderToStaticMarkup(
      <SniffWorkspaceSessionActivity
        labels={{ ...sniffSessionLabels, status: "Closing" }}
        status={{ ...sniffStatus, state: "closing" }}
      />,
    );

    expect(markup).not.toContain("<button");
    expect(markup).not.toContain("app-motion-spin");
    expect(markup).toContain(">Closing<");
  });

  test("places operation progress in the left body and speed at the right", () => {
    const snapshot = projectOperationActivitySnapshot([
      {
        operationId: "download-1",
        libraryId: "library-1",
        name: "Track",
        kind: "download",
        status: "running",
        correlation: {},
        progress: {
          percent: 50,
          speedMetric: { bytesPerSecond: 2 * 1024 ** 2 },
        },
        metrics: { fileCount: 0 },
        thumbnailPreviewPath: "/tmp/track-cover.jpg",
        createdAt: "2026-07-10T00:00:00Z",
      },
    ]);
    const markup = renderToStaticMarkup(
      <WideOperationActivity
        httpBaseURL="http://127.0.0.1:34115"
        labels={labels}
        onOpen={() => undefined}
        snapshot={snapshot}
      />,
    );

    const mainIndex = markup.indexOf("app-workspace-operation-row__main");
    const progressIndex = markup.indexOf("app-workspace-operation-row__progress");
    const speedIndex = markup.indexOf("app-workspace-operation-row__speed");
    expect(mainIndex).toBeGreaterThan(-1);
    expect(progressIndex).toBeGreaterThan(mainIndex);
    expect(speedIndex).toBeGreaterThan(progressIndex);
    expect(markup).toContain("2.0 MB/s");
    expect(markup).toContain("app-glass-surface");
    expect(markup).toContain("app-workspace-status-card");
    expect(markup).toContain('data-surface-role="status"');
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-shape="card"');
    expect(markup).toContain('data-tint="neutral"');
    expect(markup).toContain('data-tone="busy"');
    expect(markup).toContain('data-artwork="true"');
    expect(markup).toContain("app-workspace-status-card__artwork-backdrop");
    expect(markup).toContain("app-workspace-operation-wide__backdrop");
    expect(markup).toContain(
      "http://127.0.0.1:34115/api/library/asset/track-cover.jpg?path=%2Ftmp%2Ftrack-cover.jpg",
    );
  });

  test("renders every operations companion card with its thumbnail or kind fallback", () => {
    const snapshot = projectOperationActivitySnapshot([
      {
        operationId: "download-with-cover",
        libraryId: "library-1",
        name: "Covered download",
        kind: "download",
        status: "running",
        correlation: {},
        progress: { percent: 52 },
        thumbnailPreviewPath: "C:\\cache\\covered.jpg",
        metrics: { fileCount: 0 },
        createdAt: "2026-07-10T00:00:00Z",
      },
      {
        operationId: "transcode-without-cover",
        libraryId: "library-1",
        name: "Fallback transcode",
        kind: "transcode",
        status: "queued",
        correlation: {},
        metrics: { fileCount: 0 },
        createdAt: "2026-07-10T00:00:01Z",
      },
    ]);
    const markup = renderToStaticMarkup(
      <OperationsCompanionView
        httpBaseURL="http://127.0.0.1:34115"
        labels={labels}
        snapshot={snapshot}
      />,
    );

    expect(markup.match(/app-workspace-operation-item app-workspace-status-card/g)).toHaveLength(2);
    expect(markup.match(/data-surface-role="status"/g)).toHaveLength(2);
    expect(markup.match(/data-artwork="true"/g)).toHaveLength(2);
    expect(markup).toContain("app-workspace-operation-item__backdrop");
    expect(markup).toContain("covered.jpg?path=C%3A%5Ccache%5Ccovered.jpg");
    expect(markup).toContain('data-fallback="true"');
    expect(markup).toContain('data-tone="idle"');
  });

  test("keeps a recognizable kind fallback on the wide running card", () => {
    const markup = renderToStaticMarkup(
      <WideOperationActivity
        httpBaseURL="http://127.0.0.1:34115"
        labels={labels}
        onOpen={() => undefined}
        snapshot={projectOperationActivitySnapshot([
          {
            operationId: "transcode-without-cover",
            libraryId: "library-1",
            name: "Fallback transcode",
            kind: "transcode",
            status: "running",
            correlation: {},
            metrics: { fileCount: 0 },
            createdAt: "2026-07-10T00:00:00Z",
          },
        ])}
      />,
    );

    expect(markup).toContain("app-workspace-operation-wide__backdrop");
    expect(markup).toContain('data-fallback="true"');
    expect(markup).toContain("lucide-gauge");
    expect(markup).not.toContain("<img");
  });

  test("keeps the operations companion empty state padded and centered", async () => {
    const markup = renderToStaticMarkup(
      <OperationsCompanionView
        httpBaseURL=""
        labels={labels}
        snapshot={projectOperationActivitySnapshot([])}
      />,
    );
    const css = await Bun.file(
      new URL("../../shared/styles/dream/activity.css", import.meta.url),
    ).text();
    const emptyRule = css.match(
      /\.app-workspace-companion-empty\s*\{([^}]*)\}/s,
    )?.[1];
    const labelRule = css.match(
      /\.app-workspace-companion-empty > span\s*\{([^}]*)\}/s,
    )?.[1];

    expect(markup).toContain("app-workspace-companion-empty");
    expect(markup).toContain("No activity");
    expect(emptyRule).toContain("width: 100%");
    expect(emptyRule).toContain("padding: 24px");
    expect(emptyRule).toContain("text-align: center");
    expect(labelRule).toContain("max-width: 100%");
    expect(labelRule).toContain("overflow-wrap: anywhere");
  });

  test("disables only unavailable playback navigation commands", () => {
    const markup = renderToStaticMarkup(
      <WidePlaybackActivity
        labels={labels}
        onCommand={() => undefined}
        onOpen={() => undefined}
        status={{
          artworkURL: "",
          canControl: true,
          canNext: true,
          canPrevious: false,
          mode: "muse",
          playbackSource: "youtube_music",
          playbackSourceLabel: "YouTube Music",
          progress: { bufferedTime: 0, currentTime: 4, duration: 20 },
          state: "playing",
          subtitle: "artist.value",
          title: "track.value",
        }}
      />,
    );

    expect(markup).toMatch(/aria-label="Previous"[^>]*disabled/);
    expect(markup).not.toMatch(/aria-label="Next"[^>]*disabled/);
    expect(markup).toContain('data-controllable="true"');
    expect(markup).toContain('data-surface-role="status"');
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-tint="neutral"');
    expect(markup).toContain('data-tone="success"');
    expect(markup).toContain(
      "app-workspace-status-card__artwork-backdrop app-workspace-player-wide__backdrop",
    );
    expect(markup).toContain('aria-label="Now playing: track.value"');
    expect(markup).toContain('role="group" aria-label="Now playing"');
    expect(markup).toContain('data-playback="timeline"');
    expect(markup).toContain("app-workspace-player-wide__title-marquee");
    expect(markup).toContain('aria-label="YouTube Music"');
    expect(markup).toContain("app-workspace-player-wide__source");
    expect(markup).toContain(`d="${siYoutubemusic.path}"`);
  });

  test("renders live playback as a full standard timeline with its source", () => {
    const markup = renderToStaticMarkup(
      <WidePlaybackActivity
        labels={labels}
        onCommand={() => undefined}
        onOpen={() => undefined}
        status={{
          artworkURL: "https://catalog.example/live.jpg",
          artworkCandidates: [
            "https://catalog.example/live.jpg",
            "https://i.ytimg.com/vi/live-id/hqdefault.jpg",
          ],
          canControl: true,
          live: true,
          mode: "muse",
          playbackSource: "radio",
          playbackSourceLabel: "Radio",
          progress: { bufferedTime: 119, currentTime: 118, duration: 120 },
          state: "playing",
          subtitle: "Lofi Girl",
          title: "lofi hip hop radio",
        }}
      />,
    );

    expect(markup).toContain('data-playback="timeline"');
    expect(markup).toContain('data-live="true"');
    expect(markup).toContain('class="app-workspace-player-wide__progress"');
    expect(markup).toContain('style="width:100%"');
    expect(markup).toContain('aria-label="Radio"');
    expect(markup).not.toContain("app-workspace-player-wide__live");
  });

  test("keeps catalog, YouTube poster, then default artwork in fallback order", () => {
    const candidates = resolveListenArtworkCandidates({
      artworkURL: LISTEN_DEFAULT_COVER_IMAGE_URL,
      artworkCandidates: [
        "https://catalog.example/lofi-girl.jpg",
        "https://catalog.example/lofi-girl.jpg",
        "https://i.ytimg.com/vi/liveVideo01/hqdefault.jpg",
        LISTEN_DEFAULT_COVER_IMAGE_URL,
      ],
      canControl: true,
      mode: "hush",
      progress: { bufferedTime: 0, currentTime: 0, duration: 0 },
      state: "playing",
      subtitle: "Lofi Girl",
      title: "lofi hip hop radio",
    });

    expect(candidates).toEqual([
      "https://catalog.example/lofi-girl.jpg",
      "https://i.ytimg.com/vi/liveVideo01/hqdefault.jpg",
      LISTEN_DEFAULT_COVER_IMAGE_URL,
    ]);
  });

  test("contracts marquee bounce, pause, and reduced-motion truncation", async () => {
    const [css, tokens] = await Promise.all([
      Bun.file(new URL("../../shared/styles/dream/activity.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/tokens.css", import.meta.url),
      ).text(),
    ]);

    expect(css).toContain("@keyframes app-workspace-title-bounce");
    expect(css).toContain('title-marquee[data-overflow="true"] > span');
    expect(css).toContain("animation-play-state: paused");
    expect(css).toContain("@media (prefers-reduced-motion: reduce)");
    expect(css).toContain("text-overflow: ellipsis");
    for (const tone of ["idle", "busy", "success", "error", "orphan"]) {
      expect(css).toContain(`.app-workspace-status-card[data-tone="${tone}"]`);
      expect(css).toContain(`--app-status-tone-${tone}`);
    }
    const statusRule = css.match(
      /\.app-workspace-status-card\s*\{([^}]*)\}/s,
    )?.[1];
    expect(statusRule).toBeDefined();
    expect(statusRule).not.toMatch(
      /^\s*(?:background|border|box-shadow|-webkit-backdrop-filter|backdrop-filter):/m,
    );
    expect(css).not.toContain(
      '.app-workspace-status-card[data-active="true"]',
    );
    expect(css).not.toContain('[data-surface-style="contrast"]');
    expect(css).not.toContain('[data-surface-role="status"]');
    expect(css).not.toContain(".app-workspace-compact-activity-group");
    expect(css).not.toContain(".app-workspace-status-card--compact");
    expect(css).not.toContain(".app-workspace-status-card__compact-open");
    expect(css).not.toContain(".app-workspace-status-card__compact-artwork");
    expect(css).not.toContain(".app-workspace-status-card__compact-backdrop");
    expect(css).not.toContain(".app-workspace-sniff-compact");
    expect(css).not.toContain(".app-workspace-player-compact");
    expect(css).not.toContain(".app-sniff-status-reveal");
    expect(css).toMatch(
      /\.app-workspace-operation-wide\s*\{[^}]*min-height:\s*56px[^}]*justify-content:\s*center/s,
    );
    expect(css).toContain(
      '.app-workspace-status-card[data-artwork="true"]::after',
    );
    expect(css).not.toMatch(
      /\.app-workspace-sniff-wide__stop\s*\{[^}]*inset:\s*0/s,
    );
    expect(css).toMatch(
      /\.app-workspace-player-wide__source\s*\{[^}]*color:\s*var\(--app-text-tertiary\)[^}]*background:\s*transparent[^}]*border:\s*0[^}]*box-shadow:\s*none/s,
    );
    expect(tokens).toContain("--app-status-tone-idle: var(--app-text-tertiary)");
    expect(tokens).toContain(
      "--app-surface-status-fill: var(--app-glass-regular-surface)",
    );
    expect(tokens).toContain(
      "--app-surface-status-filter: var(--app-glass-regular-filter)",
    );
    expect(tokens).toContain(
      "--app-surface-status-shadow: var(--app-glass-regular-shadow)",
    );
    expect(tokens).toContain("--app-surface-status-artwork-opacity: 0.12");
    expect(tokens).toContain(
      "--app-surface-status-artwork-filter:",
    );
    expect(css).toContain(
      "opacity: var(--app-surface-status-artwork-opacity)",
    );
    expect(css).toContain(
      "filter: var(--app-surface-status-artwork-filter)",
    );
    expect(css).toMatch(
      /\.app-workspace-status-card\[data-artwork="true"\]::after\s*\{[^}]*background:\s*var\(--app-surface-status-artwork-veil\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-status-card\s*>\s*\.app-workspace-status-card__artwork-backdrop\s*\{[^}]*opacity:\s*var\(--app-surface-status-artwork-opacity\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-status-card__artwork-backdrop\s*>\s*:is\(img, svg\)\s*\{[^}]*object-fit:\s*cover[^}]*filter:\s*var\(--app-surface-status-artwork-filter\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-status-card__artwork-backdrop\[data-placement="end"\]\s*>\s*img\s*\{[^}]*object-position:\s*right center/s,
    );
    expect(css).toMatch(
      /\.app-workspace-operation-item\s*\{[^}]*min-height:\s*74px[^}]*padding:\s*12px/s,
    );
    expect(css).toMatch(
      /\.app-workspace-operation-wide\s*>\s*\.app-workspace-operation-row,[^}]*\.app-workspace-operation-item\s*>\s*:is\([^}]*\)\s*\{[^}]*position:\s*relative[^}]*z-index:\s*var\(--app-workspace-status-layer-content\)/s,
    );
    expect(css).not.toMatch(/:root\s*\{[^}]*--app-status-tone-/s);
    expect(css).toContain("color: var(--app-text-secondary)");
    expect(css).toMatch(
      /\.app-workspace-sniff-session__control\s*\{[^}]*display:\s*block[^}]*min-height:\s*56px/s,
    );
    expect(css).toMatch(
      /\.app-workspace-sniff-session__metrics\s*\{[^}]*grid-template-columns:\s*repeat\(4, minmax\(0, 1fr\)\)/s,
    );
    expect(css).not.toContain(".app-workspace-sniff-session__detail-actions");
    expect(css).not.toMatch(/z-index:\s*[0-9]+/);
    expect(css).not.toContain("--app-glass-");
    expect(css).not.toContain("backdrop-filter");
    expect(css).not.toMatch(/filter:\s*blur/);
    expect(css).not.toMatch(/border-radius:\s*(?:\d|\.\d)/);
    expect(css).toContain("border-radius: var(--app-radius-capsule)");
    expect(css).toContain("border-radius: var(--app-radius-media)");
    expect(css).toContain("border-radius: var(--app-radius-card)");

    const workflows = await Bun.file(
      new URL("../../shared/styles/dream/workflows.css", import.meta.url),
    ).text();
    expect(workflows).toContain(
      "--app-cdp-status-tone: var(--app-status-tone-success",
    );
    expect(workflows).toContain(
      "--app-cdp-status-tone: var(--app-status-tone-orphan",
    );
    expect(workflows).toContain("background: var(--app-cdp-status-tone)");
  });
});
