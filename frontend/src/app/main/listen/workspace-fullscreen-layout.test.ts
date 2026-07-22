import { describe, expect, test } from "bun:test";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { getXiaText } from "@/features/xiadown/shared";

import { ListenPlayerFooter } from "./playback-ui";

const text = getXiaText("en");

async function readWorkspaceCss() {
  const [layout, appearance] = await Promise.all([
    Bun.file(new URL("../../workspace/workspace.css", import.meta.url)).text(),
    Bun.file(
      new URL("../../../shared/styles/dream/workspace.css", import.meta.url),
    ).text(),
  ]);
  return `${layout}\n${appearance}`;
}

describe("music workspace fullscreen player", () => {
  test("keeps the video-fit control visibly focused for keyboard and forced-colors users", async () => {
    const [source, appearance] = await Promise.all([
      Bun.file(
        new URL("./native-video-surfaces.tsx", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/components.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain('className="listen-video-fit-group-button"');
    expect(appearance).not.toContain(".listen-video-fit-badge");
    expect(appearance).toMatch(
      /\.listen-video-fit-group-button:focus-visible,[\s\S]*?outline:\s*2px solid hsl\(var\(--app-focus-ring, var\(--ring\)\) \/ 0\.68\)/,
    );
    expect(appearance).toMatch(
      /@media \(forced-colors: active\)[\s\S]*?\.listen-video-fit-group-button:focus-visible,[\s\S]*?outline-color:\s*Highlight/,
    );
  });

  test("lets the presentation host own modal isolation and focus restoration", async () => {
    const source = await Bun.file(
      new URL("../MainApp.tsx", import.meta.url),
    ).text();

    expect(source).toContain('role={playerFullscreen ? "dialog" : "complementary"}');
    expect(source).toContain("aria-modal={playerFullscreen || undefined}");
    expect(source).toContain("element.inert = true");
    expect(source).toContain("element.inert = false");
    expect(source).toContain("fullscreenCloseButtonRef.current?.focus()");
    expect(source).toContain("previousFocus?.focus()");
    expect(source).toContain('event.key !== "Escape"');
  });

  test("uses an artwork-driven fullscreen shell with a fixed now-playing column", async () => {
    const [source, pageSource, css, motion] = await Promise.all([
      Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/motion.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("ListenWorkspaceFullscreenBackdrop");
    expect(source).toContain('data-workspace-fullscreen={props.workspaceFullscreen ? "true" : undefined}');
    expect(source).toContain("backdropCandidates={buildListenPosterCandidates(");
    expect(source).toContain("track.coverURL || LISTEN_DEFAULT_COVER_IMAGE_URL");
    expect(source).toContain(
      "props.active &&\n    props.presentation === \"page\" &&\n    props.mode === \"hush\"",
    );
    expect(pageSource).toContain(
      "props.playerFullscreen === true\n      ? \"fullscreen\"",
    );
    expect(css).toContain(".listen-workspace-fullscreen-backdrop__artwork");
    expect(motion).toContain("listen-workspace-fullscreen-artwork-drift");
    expect(motion).toMatch(
      /@media \(prefers-reduced-motion: reduce\)[\s\S]*?\.listen-workspace-fullscreen-backdrop__artwork\s*\{[^}]*animation:\s*none;/,
    );
    const fullscreenRule = css.match(
      /\.listen-workspace-fullscreen-player\s*\{([^}]*)\}/s,
    )?.[1];
    const gridRule = css.match(
      /\.listen-workspace-fullscreen-player__grid\s*\{([^}]*)\}/s,
    )?.[1];
    const stackRule = css.match(
      /\.listen-workspace-fullscreen-player__stack\s*\{([^}]*)\}/s,
    )?.[1];
    expect(fullscreenRule).toContain(
      "--listen-workspace-fullscreen-now-playing-width:",
    );
    expect(fullscreenRule).toContain(
      "var(--app-workspace-companion-width, 390px)",
    );
    expect(gridRule).toMatch(
      /grid-template-columns:\s*var\(--listen-workspace-fullscreen-now-playing-width\)\s*minmax\(0, 1fr\)/s,
    );
    expect(stackRule).toMatch(
      /width:\s*calc\(\s*100%\s*-\s*var\(--app-workspace-companion-gutter,\s*1\.25rem\)\s*-\s*var\(--app-workspace-companion-gutter,\s*1\.25rem\)\s*\)/s,
    );
    expect(390 - 20 * 2).toBe(350);
    expect(source).toContain("listen-workspace-fullscreen-player__now-playing");
    expect(source).toMatch(
      /fullscreenTransport=\{\s*inlineVideoFullscreen\s*\?\s*\(/,
    );
    expect(source).toContain('variant="footer"');
    expect(css).not.toContain(
      '.listen-workspace-fullscreen-player__grid[data-split="false"]',
    );
    expect(css).not.toContain(
      '.listen-workspace-fullscreen-player__grid[data-split="true"]',
    );
    expect(source).toContain(
      "props.workspaceFullscreen || inlineVideoActive",
    );
    expect(source).toContain(
      "props.workspaceFullscreen || splitEnabled",
    );
    expect(css).toContain(".listen-workspace-fullscreen-player__content {\n  display: flex;");
    expect(pageSource).toContain(
      "listen-content-surface app-workspace-primary-subpane relative flex h-full min-h-0 w-full",
    );
  });

  test("reveals native Glass beneath every fullscreen media mode", async () => {
    const [source, css, workflows] = await Promise.all([
      Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
    ]);
    const nativeGlassRule = workflows.match(
      /:root\[data-window-material="native"\][\s\S]*?\.app-main-shell\[data-surface-style="glass"\]\[data-window-material="native"\][\s\S]*?:is\(([\s\S]*?)\)\s*\{([^}]*)\}/,
    );
    const nativeBackdropRule = workflows.match(
      /:root\[data-window-material="native"\]\s*\.app-main-shell\[data-surface-style="glass"\]\[data-window-material="native"\]\s*\.listen-workspace-fullscreen-backdrop\s*\{([^}]*)\}/s,
    );
    const canvasRule = workflows.match(
      /\.listen-workspace-fullscreen-player,\s*\.listen-workspace-fullscreen-backdrop\s*\{([^}]*)\}/s,
    )?.[1];

    expect(nativeGlassRule?.[1]).toContain(
      ".listen-workspace-fullscreen-player",
    );
    expect(nativeGlassRule?.[1]).toContain(
      ".listen-workspace-fullscreen-backdrop",
    );
    expect(nativeGlassRule?.[2]).toContain("background: transparent");
    expect(nativeBackdropRule?.[1]).toContain("opacity: 0.48");
    expect(canvasRule).toContain("background: var(--app-surface-canvas)");
    expect(workflows).toContain("opacity: 0.72");
    expect(workflows).toContain("hsl(var(--background) / 0.66)");
    expect(workflows).toContain("backdrop-filter: saturate(0.92)");
    expect(source).toContain(
      "props.workspaceFullscreen && !props.fullscreenLive && !inlineVideoActive",
    );
    expect(source).toContain("data-fullscreen-media-mode={");
    expect(source).toContain("props.workspaceFullscreen ? renderedMediaMode : undefined");
    expect(workflows).not.toContain(
      ':root[data-platform="windows"][data-window-material="native"]',
    );
  });

  test("shows only observed quality as subtle metadata between the times", async () => {
    const [source, progressSource, listenAppearance] = await Promise.all([
      Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./player-progress.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("resolveListenFullscreenQualityLabel(");
    expect(source).not.toContain("configuredPlaybackAudioQuality");
    expect(source).not.toContain("text.settings.playbackAudioQualityOptions");
    expect(progressSource).toContain("grid-cols-[1fr_auto_1fr]");
    expect(progressSource).toContain("listen-player-progress__center-label");
    expect(listenAppearance).toMatch(
      /\.listen-player-progress__center-label\s*\{[^}]*font-size:\s*0\.5rem;[^}]*font-weight:\s*400;[^}]*letter-spacing:\s*0\.01em;/s,
    );
    expect(source).toContain(
      "sourceBadge={props.workspaceFullscreen ? undefined : props.sourceBadge}",
    );
  });

  test("blocks each fullscreen lyrics default while its own queue is open", async () => {
    const source = await Bun.file(
      new URL("./Playback.tsx", import.meta.url),
    ).text();
    const localEffectStart = source.indexOf(
      'props.presentation !== "fullscreen" ||\n      props.mode !== "linger"',
    );
    const localEffectEnd = source.indexOf(
      "\n\n  if (props.mode !== \"linger\")",
      localEffectStart,
    );
    const onlineEffectStart = source.indexOf(
      "if (!props.workspaceFullscreen || isLive || props.listOpen || queueOpen)",
    );
    const onlineEffectEnd = source.indexOf(
      "\n\n  const showInlineEmbeddedVideo",
      onlineEffectStart,
    );
    const localEffect = source.slice(localEffectStart, localEffectEnd);
    const onlineEffect = source.slice(onlineEffectStart, onlineEffectEnd);

    expect(localEffectStart).toBeGreaterThan(-1);
    expect(localEffectEnd).toBeGreaterThan(localEffectStart);
    expect(onlineEffectStart).toBeGreaterThan(-1);
    expect(onlineEffectEnd).toBeGreaterThan(onlineEffectStart);
    expect(localEffect).toContain("props.listOpen ||\n      localQueueOpen ||");
    expect(localEffect).toContain("\n    localQueueOpen,");
    expect(onlineEffect).toContain(
      "if (!props.workspaceFullscreen || isLive || props.listOpen || queueOpen)",
    );
    expect(onlineEffect).toContain("\n    queueOpen,");
  });

  test("keeps a shrinkable volume control before the footer action group", async () => {
    const [source, controlsSource, css, footerSource] = await Promise.all([
      Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./playback-controls.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(new URL("./playback-ui.tsx", import.meta.url)).text(),
    ]);

    expect(source).toContain('className="listen-workspace-fullscreen-player__volume wails-no-drag"');
    expect(controlsSource).toContain("listen-player-volume__maximum");
    const fullscreenLeadingIndex = footerSource.indexOf(
      "{!workspaceCompanion && props.leading ? (",
    );
    const fullscreenActionsIndex = footerSource.indexOf(
      "{!workspaceCompanion ? (",
      fullscreenLeadingIndex,
    );
    expect(fullscreenLeadingIndex).toBeGreaterThan(-1);
    expect(fullscreenActionsIndex).toBeGreaterThan(fullscreenLeadingIndex);
    expect(
      footerSource.slice(fullscreenLeadingIndex, fullscreenActionsIndex),
    ).toContain("listen-player-footer__leading");
    expect(source).toContain("workspaceQueueActive");
    expect(source).toContain("<ListenWorkspaceOnlineQueueCompanion");
    expect(source).toContain("<ListenWorkspaceLocalQueueCompanion");
    expect(source).toContain(
      'props.presentation === "page" ? props.queueOverlay : null',
    );
    const queueStageStart = source.indexOf(
      "const mediaStage =\n    workspaceQueueActive ? (",
    );
    const queueStageEnd = source.indexOf(
      ") : inlineVideoSurface ? (",
      queueStageStart,
    );
    const queueStageSource = source.slice(queueStageStart, queueStageEnd);
    expect(queueStageStart).toBeGreaterThan(-1);
    expect(queueStageEnd).toBeGreaterThan(queueStageStart);
    expect(queueStageSource).toContain(
      'props.workspaceFullscreen &&\n            "listen-workspace-fullscreen-player__queue"',
    );
    expect(
      queueStageSource.match(/listen-workspace-fullscreen-player__queue/g),
    ).toHaveLength(2);
    expect(queueStageSource).not.toContain("workspaceCompanion &&");
    expect(queueStageSource).toContain("<GlassSurface");
    expect(queueStageSource).toContain('surfaceRole="card"');
    expect(queueStageSource).toContain('shape="card"');
    expect(queueStageSource).toContain(
      "listen-workspace-fullscreen-player__queue-surface",
    );
    expect(css).toContain(".listen-workspace-fullscreen-player__volume");
    expect(css).toContain("width: fit-content");
    expect(css).toContain("flex: 0 1 auto");
    expect(css).toContain("@media (max-width: 520px)");
    expect(css).toContain(".listen-player-volume__maximum");
    expect(css).toContain(".listen-workspace-fullscreen-player__queue");
    expect(css).not.toMatch(/(?:-webkit-)?backdrop-filter\s*:/);
  });

  test("places fullscreen transport and contextual groups before playback actions", () => {
    const fullscreenTransport = React.createElement("div", {
      "data-testid": "fullscreen-transport",
    });
    const lyricsMarkup = renderToStaticMarkup(
      React.createElement(ListenPlayerFooter, {
        mediaMode: "lyrics",
        presentation: "fullscreen",
        reserveWindowControls: false,
        airPlaySupported: false,
        hasVideo: true,
        lyricsAvailable: true,
        text,
        onMediaModeChange: () => undefined,
        fullscreenTransport,
        lyricsControls: React.createElement("div", {
          "data-testid": "lyrics-controls",
        }),
      }),
    );
    const queuedLyricsMarkup = renderToStaticMarkup(
      React.createElement(ListenPlayerFooter, {
        mediaMode: "lyrics",
        presentation: "fullscreen",
        reserveWindowControls: false,
        airPlaySupported: false,
        hasVideo: true,
        lyricsAvailable: true,
        queueOpen: true,
        text,
        onMediaModeChange: () => undefined,
        fullscreenTransport,
        lyricsControls: React.createElement("div", {
          "data-testid": "queued-lyrics-controls",
        }),
      }),
    );
    const carvedVideoMarkup = renderToStaticMarkup(
      React.createElement(ListenPlayerFooter, {
        mediaMode: "video",
        presentation: "fullscreen",
        reserveWindowControls: false,
        airPlaySupported: false,
        hasVideo: true,
        lyricsAvailable: true,
        text,
        onMediaModeChange: () => undefined,
        onToggleVideoAppFullscreen: () => undefined,
        onRequestVideoFullscreen: () => undefined,
      }),
    );
    const appFullscreenVideoMarkup = renderToStaticMarkup(
      React.createElement(ListenPlayerFooter, {
        mediaMode: "video",
        presentation: "fullscreen",
        reserveWindowControls: false,
        airPlaySupported: false,
        hasVideo: true,
        lyricsAvailable: true,
        text,
        onMediaModeChange: () => undefined,
        videoAppFullscreen: true,
        fullscreenTransport,
        onToggleVideoAppFullscreen: () => undefined,
        onRequestVideoFullscreen: () => undefined,
      }),
    );

    const lyricsFooter = lyricsMarkup.match(/^<footer[^>]*>/)?.[0];
    const carvedVideoFooter = carvedVideoMarkup.match(/^<footer[^>]*>/)?.[0];
    const appFullscreenVideoFooter = appFullscreenVideoMarkup.match(
      /^<footer[^>]*>/,
    )?.[0];
    expect(lyricsFooter).toContain('data-video-transport="true"');
    expect(lyricsFooter).not.toContain("data-media-chrome");
    expect(carvedVideoFooter).not.toContain("data-media-chrome");
    expect(appFullscreenVideoFooter).toContain('data-media-chrome="dark"');
    expect(appFullscreenVideoFooter).toContain('data-video-transport="true"');
    expect(lyricsMarkup).toContain("listen-player-footer__transport-group");
    expect(lyricsMarkup).toContain("listen-player-footer__context-group");
    expect(lyricsMarkup).toContain(
      "listen-player-footer__context-group shrink-0",
    );
    expect(lyricsMarkup.match(/listen-player-footer__bar/g)).toHaveLength(3);
    expect(lyricsMarkup.match(/app-glass-surface app-glass-group/g)).toHaveLength(3);
    expect(lyricsMarkup.match(/data-elevation="floating"/g)).toHaveLength(3);
    expect(lyricsMarkup.match(/data-shape="capsule"/g)).toHaveLength(3);
    expect(lyricsMarkup.match(/data-surface-role="control"/g)).toHaveLength(3);
    expect(lyricsMarkup.match(/data-material="regular"/g)).toHaveLength(3);
    expect(lyricsMarkup.indexOf("listen-player-footer__transport-group")).toBeLessThan(
      lyricsMarkup.indexOf("listen-player-footer__context-group"),
    );
    expect(lyricsMarkup.indexOf("listen-player-footer__context-group")).toBeLessThan(
      lyricsMarkup.lastIndexOf("listen-player-footer__bar"),
    );
    expect(lyricsMarkup.match(/data-testid="fullscreen-transport"/g)).toHaveLength(1);
    expect(lyricsMarkup.match(/data-testid="lyrics-controls"/g)).toHaveLength(1);
    expect(queuedLyricsMarkup).toContain(
      "listen-player-footer__transport-group",
    );
    expect(queuedLyricsMarkup).not.toContain(
      "listen-player-footer__context-group",
    );
    expect(queuedLyricsMarkup).not.toContain(
      'data-testid="queued-lyrics-controls"',
    );
    expect(carvedVideoMarkup).toContain("lucide-maximize-2");
    expect(carvedVideoMarkup).toContain("lucide-fullscreen");
    expect(carvedVideoMarkup).not.toContain("lucide-shrink");
    expect(carvedVideoMarkup).toContain(text.listen.windowFullscreenEnter);
    expect(carvedVideoMarkup).toContain(text.completed.previewEnterFullscreen);
    expect(appFullscreenVideoMarkup).toContain("lucide-shrink");
    expect(appFullscreenVideoMarkup).toContain("lucide-fullscreen");
    expect(appFullscreenVideoMarkup).toContain(text.listen.windowFullscreenExit);
    expect(appFullscreenVideoMarkup).toContain(
      text.completed.previewEnterFullscreen,
    );
  });

  test("keeps app-fullscreen video controls legible on shared floating glass", async () => {
    const [layoutCss, appearanceCss, footerSource, glassCss] = await Promise.all([
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
      Bun.file(new URL("./playback-ui.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/glass.css", import.meta.url),
      ).text(),
    ]);
    const css = `${layoutCss}\n${appearanceCss}`;

    const cssRules = Array.from(appearanceCss.matchAll(/([^{}]+)\{([^{}]*)\}/g));
    const darkChromeRule = cssRules.find(([, selectors]) =>
      selectors.includes('[data-video-app-fullscreen="true"]') &&
      selectors.includes('[data-media-chrome="dark"]') &&
      selectors.includes(".listen-player-footer__bar")
    )?.[2];
    const darkHoverRule = cssRules.find(([, selectors]) =>
      selectors.includes('[data-media-chrome="dark"]') &&
      selectors.includes('[data-active="true"]')
    )?.[2];
    expect(darkChromeRule).toContain("color: var(--app-media-chrome-foreground)");
    expect(darkChromeRule).not.toContain("background:");
    expect(darkChromeRule).not.toContain("backdrop-filter:");
    expect(darkHoverRule).toContain("var(--app-media-chrome-foreground) 12%");
    expect(darkHoverRule).toContain(
      "color: var(--app-media-chrome-foreground) !important",
    );
    const darkControlGlassRule = glassCss.match(
      /\[data-media-chrome="dark"\]\s*> \.app-glass-surface\[data-surface-role="control"\]\s*\{([^}]*)\}/s,
    )?.[1];
    expect(darkControlGlassRule).toContain(
      "--app-glass-fill: rgb(9 9 11 / 48%)",
    );
    expect(darkControlGlassRule).toContain(
      "--app-glass-line: rgb(255 255 255 / 14%)",
    );
    expect(darkControlGlassRule).not.toContain("backdrop-filter");
    expect(footerSource).toContain("<GlassGroup");
    expect(footerSource).toContain('elevation="floating"');
    expect(footerSource).toContain('shape="capsule"');
    expect(footerSource).toContain('surfaceRole="control"');
    expect(footerSource).not.toContain("bg-black/10");
    expect(footerSource).not.toContain("backdrop-blur-xl");
    expect(css).toMatch(
      /\.listen-workspace-fullscreen-player__footer\[data-video-transport="true"\][\s\S]*?pointer-events:\s*none;[\s\S]*?\.listen-player-footer__transport-group[\s\S]*?width:\s*clamp\(19rem, 38vw, 32rem\);[\s\S]*?flex:\s*0 1 clamp\(19rem, 38vw, 32rem\);/,
    );
    expect(css).not.toContain('[data-media-chrome="true"]');
  });

  test("keeps Video carved until App fullscreen is explicitly toggled", async () => {
    const [source, dreamCss] = await Promise.all([
      Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/components.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain(
      "const [videoAppFullscreen, setVideoAppFullscreen] = React.useState(false);",
    );
    expect(source).toContain("videoAppFullscreen;");
    expect(source).toContain('data-video-app-fullscreen={inlineVideoFullscreen ? "true" : "false"}');
    expect(source).toContain("inlineVideoActive ? handleToggleVideoAppFullscreen : undefined");
    expect(source).toContain(
      "inlineVideoActive && props.inlineVideoVisible === true",
    );
    expect(source).not.toContain(
      "props.workspaceFullscreen === true && inlineVideoActive;",
    );
    expect(
      dreamCss.match(
        /\.listen-inline-video-frame\[data-native-video="underlay"\]::before\s*\{([^}]*)\}/s,
      )?.[1],
    ).toContain("display: none");
    const nativeVideoStageRule = dreamCss.match(
      /\.listen-inline-video-stage\[data-native-video="underlay"\]\s*\{([^}]*)\}/s,
    )?.[1];
    expect(nativeVideoStageRule).toContain("box-shadow: none");
    expect(nativeVideoStageRule).toContain("clip-path: none");
    expect(nativeVideoStageRule).toContain("-webkit-mask-image: none");
    expect(nativeVideoStageRule).toContain("contain: none");
    expect(nativeVideoStageRule).toContain("isolation: auto");
    expect(
      dreamCss.match(
        /\.listen-inline-video-stage\[data-native-video="underlay"\]::after\s*\{([^}]*)\}/s,
      )?.[1],
    ).toContain("box-shadow: none");
  });

  test("keeps Live fullscreen media controls video-only and uses the native underlay", async () => {
    const [
      source,
      nativeVideoSource,
      footerSource,
      underlaySource,
      layoutCss,
      appearanceCss,
    ] = await Promise.all([
      Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("./native-video-surfaces.tsx", import.meta.url),
      ).text(),
      Bun.file(new URL("./playback-ui.tsx", import.meta.url)).text(),
      Bun.file(new URL("./native-video-underlay.ts", import.meta.url)).text(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
    ]);
    const css = `${layoutCss}\n${appearanceCss}`;

    expect(source).toContain(
      'props.presentation === "fullscreen" &&\n    mediaMode === "video"',
    );
    expect(source).not.toContain(
      'props.presentation === "fullscreen" &&\n    !isLive &&\n    mediaMode === "video"',
    );
    expect(source).toContain("videoHidden={props.videoHidden}");
    expect(footerSource).toContain(
      "const showVideoAction =\n    !props.videoHidden && workspaceFullscreen",
    );
    expect(footerSource).toContain(
      "const showLyricsAction = props.showMediaActions !== false && !props.live",
    );
    expect(footerSource).toContain(
      "(!workspaceCompanion || Boolean(props.onToggleQueue))",
    );
    expect(css).toContain(".app-workspace-shell,");
    expect(css).toContain(".app-workspace-stage,");
    expect(css).toContain(".app-workspace-companion__content,");
    expect(css).toContain(".listen-workspace-fullscreen-player__video\n  )");
    expect(nativeVideoSource).toMatch(
      /useListenNativeVideoUnderlay\(\s*props\.visible\s*&&\s*!props\.geometrySuspended,?\s*\)/,
    );
    expect(nativeVideoSource).toMatch(
      /useListenNativeVideoUnderlay\(\s*props\.liveVideoVisible\s*&&\s*!props\.geometrySuspended,?\s*\)/,
    );
    expect(underlaySource).toContain("if (!activeRef.current)");
    expect(underlaySource).toContain("holeRef.current = hole");
    expect(underlaySource).not.toContain(
      "activateNativeVideoUnderlay(owner);\n    return () =>",
    );
  });

  test("keeps modals, menus, and tooltips above the fullscreen host", async () => {
    const [tooltipSource, dropdownSource, dialogSource, anatomyCss] = await Promise.all([
      Bun.file(new URL("../../../shared/ui/tooltip.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../../components/ui/dropdown-menu.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../../components/ui/dialog.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/anatomy.css", import.meta.url),
      ).text(),
    ]);

    expect(dialogSource).toContain("app-dialog-overlay-base");
    expect(dialogSource).toContain("app-dialog-content-base");
    expect(dropdownSource).toContain("app-menu-content-base");
    expect(tooltipSource).toContain("app-dream-tooltip");
    expect(anatomyCss).toMatch(
      /\.app-dialog-overlay-base\s*\{[^}]*z-index:\s*var\(--app-layer-modal-overlay\);/s,
    );
    expect(anatomyCss).toMatch(
      /\.app-dialog-content-base\s*\{[^}]*z-index:\s*var\(--app-layer-modal\);/s,
    );
    expect(anatomyCss).toMatch(
      /\.app-menu-content-base\s*\{[^}]*z-index:\s*var\(--app-layer-popover\);/s,
    );
    expect(anatomyCss).toMatch(
      /\.app-dream-tooltip\s*\{[^}]*position:\s*fixed;[^}]*z-index:\s*var\(--app-layer-tooltip\);/s,
    );
  });

  test("uses native player-window fullscreen without fullscreening the React app", async () => {
    const [source, fullscreenSource, playerSource, windowsSource] = await Promise.all([
      Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
      Bun.file(
        new URL(
          "../../../../../internal/presentation/wails/listen_embedded_video_fullscreen.go",
          import.meta.url,
        ),
      ).text(),
	  Bun.file(
		new URL(
		  "../../../../../internal/presentation/wails/listen_player_handler.go",
		  import.meta.url,
		),
	  ).text(),
	  Bun.file(
		new URL(
		  "../../../../../internal/presentation/wails/listen_player_webview_windows.go",
		  import.meta.url,
		),
	  ).text(),
    ]);

	expect(source).toContain('"RequestEmbeddedVideoFullscreen"');
	expect(source).toContain('"ExitEmbeddedVideoFullscreen"');
	expect(source).toContain('data.type === "embedded-video-fullscreen-change"');
	expect(source).toContain("nativeVideoGeometrySuspended={embeddedVideoGeometrySuspended}");
    expect(source).toContain('status.provider === "stream"');
    expect(source).not.toContain("Window.Fullscreen()");
    expect(source).not.toContain("Window.UnFullscreen()");
	expect(playerSource).toContain("window.Fullscreen()")
	expect(playerSource).toContain("window.UnFullscreen()")
	expect(windowsSource).toContain("func listenEmbeddedVideoUsesNativeWindowFullscreen() bool")
	expect(windowsSource).toContain('RegisterKeyBinding("escape"')
    expect(fullscreenSource).toContain("fullscreenButton.click()");
    expect(fullscreenSource).toContain("video.webkitEnterFullscreen()");
    expect(fullscreenSource).toContain("target.requestFullscreen");
  });

  test("uses a close affordance aligned after macOS traffic controls", async () => {
    const [source, workspaceCss, windowControlsSource, tokensSource] = await Promise.all([
      Bun.file(new URL("../MainApp.tsx", import.meta.url)).text(),
      readWorkspaceCss(),
      Bun.file(
        new URL(
          "../../../components/layout/WindowControls.tsx",
          import.meta.url,
        ),
      ).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/tokens.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain(
      "ref={playerFullscreen ? fullscreenCloseButtonRef : undefined}",
    );
    expect(source).toContain(
      'fullscreenPlayer === "music"\n      ? activityLabels.nowPlaying',
    );
    expect(source).toContain(
      'owner={playerFullscreen ? "fullscreen" : "companion"}',
    );
    expect(source).toContain(
      'event.key !== "Escape" || event.defaultPrevented',
    );
    expect(source).toContain(
      `'[role="dialog"][data-state="open"]'`,
    );
    expect(source).toContain(
      "app-workspace-companion__titlebar--controls-only",
    );
    expect(source).toContain(
      'className="fixed right-0 top-0 z-[var(--app-layer-window-controls)]"',
    );
    expect(source).not.toContain("fixed top-[9px] z-[86]");
    expect(source).toContain('<X className="h-4 w-4" />');
    expect(workspaceCss).toContain(
      "calc(var(--app-macos-traffic-lights-gap) + 6px)",
    );
    expect(workspaceCss).toContain(
      '.app-workspace-companion--player-fullscreen[data-platform="windows"]',
    );
    expect(workspaceCss).toContain(
      "height: var(--app-player-fullscreen-titlebar-height, 48px)",
    );
    expect(workspaceCss).toContain(
      '.listen-workspace-fullscreen-player[data-video-app-fullscreen="true"]',
    );
    expect(workspaceCss).toMatch(
      /> \.app-workspace-companion__header\s*\{[^}]*background:\s*transparent;[^}]*color:\s*var\(--app-media-chrome-foreground\);[^}]*box-shadow:\s*none/s,
    );
    expect(workspaceCss).toMatch(
      /\[data-window-controls-owner="fullscreen"\]\s*> button\s*\{[^}]*color:\s*var\(--app-media-chrome-foreground-muted\) !important/s,
    );
    expect(tokensSource).toContain(
      "--app-player-fullscreen-titlebar-height: 48px",
    );
    expect(windowControlsSource).toContain("className?: string");
  });

  test("layers title controls over a video that fills title and content", async () => {
    const [workspaceCss, listenCss, dreamCss] = await Promise.all([
      readWorkspaceCss(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/components.css", import.meta.url),
      ).text(),
    ]);

    expect(workspaceCss).toMatch(
      /\.app-workspace-companion\.app-workspace-companion--player-fullscreen\s*\{[^}]*position:\s*fixed;[^}]*inset:\s*0;/s,
    );
    expect(workspaceCss).toMatch(
      /\.app-workspace-companion--player-fullscreen\s*>\s*\.app-workspace-companion__header\s*\{[^}]*position:\s*absolute;[^}]*z-index:\s*var\(--app-layer-window-controls\);[^}]*inset:\s*0 0 auto;/s,
    );
    expect(workspaceCss).toMatch(
      /\[data-video-app-fullscreen="true"\][\s\S]*?> \.app-workspace-companion__header\s*\{[^}]*background:\s*transparent;[^}]*box-shadow:\s*none;/s,
    );
    expect(listenCss).toMatch(
      /\.listen-workspace-fullscreen-player__video\s*\{[^}]*position:\s*absolute;[^}]*z-index:\s*1;[^}]*inset:\s*0;/s,
    );
    expect(dreamCss).toMatch(
      /\.listen-inline-video-frame-fullscreen\s*\{[^}]*width:\s*100%;[^}]*height:\s*100%;/s,
    );
    expect(dreamCss).toMatch(
      /\.listen-inline-video-frame-fullscreen \.listen-inline-video-stage\s*\{[^}]*width:\s*100%;[^}]*height:\s*100%;[^}]*border-radius:\s*0;/s,
    );
  });

  test("keeps fullscreen volume open until outside pointer or Escape", async () => {
    const source = await Bun.file(
      new URL("./fullscreen-volume-control.tsx", import.meta.url),
    ).text();

    expect(source).toContain('document.addEventListener("pointerdown", closeOnOutsidePointer, true)');
    expect(source).toContain('document.addEventListener("keydown", closeOnEscape, true)');
    expect(source).not.toContain("onPointerLeave={scheduleClose}");
  });

  test("renders fullscreen players exclusively from stale companion content", async () => {
    const source = await Bun.file(
      new URL("../MainApp.tsx", import.meta.url),
    ).text();

    expect(source).toContain(
      'fullscreenPlayer === "music" ||\n              (!playerFullscreen && playbackCompanionOpen)',
    );
    expect(source).toContain(
      '!playerFullscreen && companion.destination?.id === "operations"',
    );
    expect(source).toContain(
      '!playerFullscreen && companion.destination?.id === "sniff"',
    );
    expect(source).toMatch(
      /!playerFullscreen\s*&&\s*companion\.destination\?\.id === "playback-summary"/,
    );
    expect(source).toContain("requestMusicPlayerFullscreen();");
    expect(source).toContain(
      "workspaceCompanionAffectsLayout(\n    companion.open,\n    playerFullscreen,",
    );
    expect(source).toContain(
      "companionOpen={companionAffectsLayout}",
    );
    expect(source).not.toContain(
      "activeWorkspaceId !== fullscreenPlayer",
    );
    const fullscreenRequest = source.slice(
      source.indexOf("const requestMusicPlayerFullscreen"),
      source.indexOf("const exitMusicPlayerFullscreen"),
    );
    expect(fullscreenRequest).toContain('setFullscreenPlayer("music")');
    expect(fullscreenRequest).not.toContain("openGlobalCompanion");
  });
});
