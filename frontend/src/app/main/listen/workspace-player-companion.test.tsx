import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { getXiaText } from "@/features/xiadown/shared";

import { ListenPlayerFooter } from "./playback-ui";
import {
  openListenArtistFromPlayerSurface,
  resolveListenPlayerSurfaceActive,
} from "./workspace-player-shared";

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

describe("workspace Now Playing companion", () => {
  test("keeps visible global player surfaces active across workspaces", () => {
    expect(resolveListenPlayerSurfaceActive(true, undefined)).toBe(true);
    expect(resolveListenPlayerSurfaceActive(true, true)).toBe(true);
    expect(resolveListenPlayerSurfaceActive(true, false)).toBe(false);
    expect(resolveListenPlayerSurfaceActive(false, true)).toBe(true);
    expect(resolveListenPlayerSurfaceActive(false, false)).toBe(false);
    expect(resolveListenPlayerSurfaceActive(false, undefined)).toBe(false);
  });

  test("switches to Music before opening an artist from the global player", () => {
    const calls: string[] = [];
    openListenArtistFromPlayerSurface({
      workspaceActive: false,
      workspaceLayout: true,
      openPlaybackSource: (source) => calls.push(`source:${source}`),
      schedule: (openArtist) => {
        calls.push("scheduled");
        openArtist();
      },
      openArtist: () => calls.push("artist"),
    });

    expect(calls).toEqual(["source:youtube_music", "scheduled", "artist"]);
  });

  test("opens the artist directly when Music already owns the player", () => {
    const calls: string[] = [];
    openListenArtistFromPlayerSurface({
      workspaceActive: true,
      workspaceLayout: true,
      openPlaybackSource: (source) => calls.push(`source:${source}`),
      schedule: () => calls.push("scheduled"),
      openArtist: () => calls.push("artist"),
    });

    expect(calls).toEqual(["artist"]);
  });

  test("places source and standard fullscreen left of Lyrics and Up Next", () => {
    const markup = renderToStaticMarkup(
      <ListenPlayerFooter
        mediaMode="cover"
        presentation="companion"
        reserveWindowControls={false}
        airPlaySupported
        sourceBadge={
          <>
            <svg data-source-icon="youtube-music" />
            <span>{text.workspace.youtubeMusic}</span>
          </>
        }
        sourceLabel={text.workspace.youtubeMusic}
        hasVideo
        videoLoading={false}
        lyricsAvailable
        queueOpen
        text={text}
        onAirPlay={() => undefined}
        onMediaModeChange={() => undefined}
        onToggleQueue={() => undefined}
        onOpenSource={() => undefined}
        onRequestFullscreen={() => undefined}
      />,
    );

    const leadingIndex = markup.indexOf('data-footer-region="leading"');
    const dynamicIndex = markup.indexOf('data-footer-region="dynamic"');
    const trailingIndex = markup.indexOf('data-footer-region="trailing"');
    const leadingMarkup = markup.slice(leadingIndex, trailingIndex);
    const trailingMarkup = markup.slice(trailingIndex);

    expect(leadingIndex).toBeGreaterThan(-1);
    expect(dynamicIndex).toBe(-1);
    expect(trailingIndex).toBeGreaterThan(leadingIndex);
    expect(markup).toContain("listen-player-footer__bar--companion");
    expect(leadingMarkup).toContain("listen-player-footer__source-button");
    expect(markup).toContain("listen-player-footer-icon-button");
    expect(markup).toContain('data-app-button="true"');
    expect(markup).not.toContain("text-sidebar-foreground/");
    expect(leadingMarkup).toContain(
      `aria-label="${text.workspace.youtubeMusic}"`,
    );
    expect(leadingMarkup).toContain(
      `aria-label="${text.completed.previewEnterFullscreen}"`,
    );
    expect(leadingMarkup).toContain("lucide-fullscreen");
    expect(leadingMarkup).not.toContain("lucide-maximize-2");
    expect(leadingMarkup.indexOf(text.workspace.youtubeMusic)).toBeLessThan(
      leadingMarkup.indexOf(text.completed.previewEnterFullscreen),
    );
    expect(trailingMarkup).toContain(`aria-label="${text.listen.lyrics}"`);
    expect(trailingMarkup).toContain(`aria-label="${text.listen.upNext}"`);
    expect(trailingMarkup.indexOf(text.listen.lyrics)).toBeLessThan(
      trailingMarkup.indexOf(text.listen.upNext),
    );
    expect(trailingMarkup).toMatch(
      new RegExp(`data-active="true"[^>]*aria-label="${text.listen.upNext}"`),
    );
    expect(markup).not.toContain(`aria-label="${text.listen.airPlay}"`);
    expect(markup).not.toContain(`aria-label="${text.listen.video}"`);
  });

  test("renders exactly one dynamic control group for Lyrics or Queue", () => {
    const lyricsMarkup = renderToStaticMarkup(
      <ListenPlayerFooter
        mediaMode="lyrics"
        presentation="companion"
        reserveWindowControls={false}
        airPlaySupported={false}
        hasVideo={false}
        lyricsAvailable
        text={text}
        onMediaModeChange={() => undefined}
        lyricsControls={<div data-testid="lyrics-controls" />}
        companionControls={<div data-testid="lyrics-controls" />}
      />,
    );
    const queueMarkup = renderToStaticMarkup(
      <ListenPlayerFooter
        mediaMode="cover"
        presentation="companion"
        reserveWindowControls={false}
        airPlaySupported={false}
        hasVideo={false}
        lyricsAvailable
        queueOpen
        text={text}
        onMediaModeChange={() => undefined}
        onToggleQueue={() => undefined}
        companionControls={<div data-testid="queue-controls" />}
      />,
    );
    const coverMarkup = renderToStaticMarkup(
      <ListenPlayerFooter
        mediaMode="cover"
        presentation="companion"
        reserveWindowControls={false}
        airPlaySupported={false}
        hasVideo={false}
        lyricsAvailable
        text={text}
        onMediaModeChange={() => undefined}
        lyricsControls={<div data-testid="lyrics-controls" />}
      />,
    );

    expect(lyricsMarkup).toContain("listen-player-footer__center-context");
    expect(lyricsMarkup).toMatch(
      /<div data-footer-region="dynamic" class="[^"]*listen-player-footer__center-context[^"]*"><div data-testid="lyrics-controls"/,
    );
    expect(lyricsMarkup).toContain("listen-player-footer__bar--companion");
    expect(lyricsMarkup).not.toContain("app-glass-group");
    expect(lyricsMarkup.match(/data-testid="lyrics-controls"/g)).toHaveLength(1);
    expect(lyricsMarkup).toMatch(
      new RegExp(`data-active="true"[^>]*aria-label="${text.listen.lyrics}"`),
    );
    expect(queueMarkup.match(/data-testid="queue-controls"/g)).toHaveLength(1);
    expect(queueMarkup).toMatch(
      new RegExp(`data-active="true"[^>]*aria-label="${text.listen.upNext}"`),
    );
    expect(queueMarkup.indexOf('data-footer-region="leading"')).toBeLessThan(
      queueMarkup.indexOf('data-footer-region="dynamic"'),
    );
    expect(queueMarkup.indexOf('data-footer-region="dynamic"')).toBeLessThan(
      queueMarkup.indexOf('data-footer-region="trailing"'),
    );
    expect(coverMarkup).not.toContain("listen-player-footer__center-context");
    expect(coverMarkup).not.toContain('data-testid="lyrics-controls"');
  });

  test("resets local and online internal context for every explicit companion mode", async () => {
    const playbackSource = await Bun.file(
      new URL("./Playback.tsx", import.meta.url),
    ).text();
    const resetGuard = "if (!props.companionMode) {";
    const localResetStart = playbackSource.indexOf(resetGuard);
    const localResetEnd = playbackSource.indexOf(
      "}, [props.companionMode]);",
      localResetStart,
    );
    const onlineResetStart = playbackSource.indexOf(
      resetGuard,
      localResetEnd + 1,
    );
    const onlineResetEnd = playbackSource.indexOf(
      "}, [props.companionMode]);",
      onlineResetStart,
    );
    const localResetEffect = playbackSource.slice(
      localResetStart,
      localResetEnd,
    );
    const onlineResetEffect = playbackSource.slice(
      onlineResetStart,
      onlineResetEnd,
    );

    expect(localResetStart).toBeGreaterThan(-1);
    expect(localResetEnd).toBeGreaterThan(localResetStart);
    expect(onlineResetStart).toBeGreaterThan(localResetEnd);
    expect(onlineResetEnd).toBeGreaterThan(onlineResetStart);
    expect(
      playbackSource.indexOf(resetGuard, onlineResetEnd + 1),
    ).toBe(-1);
    expect(localResetEffect).toContain("setLocalQueueOpen(false);");
    expect(localResetEffect).toContain('setLocalMediaMode("cover");');
    expect(onlineResetEffect).toContain("setQueueOpen(false);");
    expect(onlineResetEffect).toContain('setMediaMode("cover");');
  });

  test("revises mounted Now Playing artwork when same-track metadata adds a cover", async () => {
    const playbackSource = await Bun.file(
      new URL("./Playback.tsx", import.meta.url),
    ).text();

    expect(playbackSource).toContain("const artworkRevisionKey = [");
    expect(playbackSource).toContain("props.track.thumbnailUrl?.trim() ?? \"\"");
    expect(playbackSource).toContain("key={artworkRevisionKey}");
    expect(playbackSource.match(/key=\{artworkRevisionKey\}/g)).toHaveLength(2);
  });

  test("uses an explicit presentation contract and reserves native video for fullscreen", async () => {
    const [pageSource, playbackSource, footerSource, css] = await Promise.all([
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
      Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./playback-ui.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
    ]);

    expect(pageSource).toContain("data-player-presentation={playerPresentation}");
    expect(pageSource).toContain("active={playerSurfaceActive}");
    expect(pageSource).not.toContain(
      "active={props.active && props.playerSurfaceVisible !== false}",
    );
    expect(pageSource).toContain('playerPresentation === "page" && !hushFullscreen');
    expect(pageSource).toContain('isWindows && playerPresentation === "page"');
    expect(playbackSource).toContain('props.presentation === "fullscreen" &&');
    expect(playbackSource).toContain(
      'props.presentation === "fullscreen"\n            ? showInlineEmbeddedVideo',
    );
    expect(footerSource).toContain('onClick={() => toggleMediaMode("lyrics")}');
    expect(footerSource).not.toContain('onRequestCompanionMode?.("lyrics")');
    expect(playbackSource).toContain("const renderedMediaMode = props.mediaMode");
    expect(playbackSource).toContain(
      "if (!props.workspaceFullscreen || isLive || props.listOpen || queueOpen)",
    );
    expect(playbackSource).not.toContain(
      'if (props.presentation === "companion") {\n        return;',
    );
    expect(playbackSource).toContain("onOpenPlaybackSource?: (source: ListenPlaybackSource)");
    expect(footerSource).toContain('data-footer-region="leading"');
    expect(footerSource).toContain('data-footer-region="dynamic"');
    expect(footerSource).toContain('data-footer-region="trailing"');
    expect(footerSource).toContain("<Fullscreen");
    expect(playbackSource).toContain("onToggleQueue={props.onToggleQueue}");
    expect(playbackSource).toMatch(
      /const workspaceQueueActive =\s*\(props\.workspaceFullscreen === true \|\| workspaceCompanion\)/,
    );
    expect(playbackSource).toContain(
      'data-player-context={workspaceQueueActive ? "queue" : "lyrics"}',
    );
    expect(playbackSource).toContain(
      "{workspaceQueueActive ? mediaStage : props.lyrics}",
    );
    expect(
      playbackSource.match(
        /showFooter=\{props\.presentation !== "companion"\}/g,
      ),
    ).toHaveLength(2);
    const singlePanelStart = playbackSource.indexOf("{singleColumnContext ? (");
    const singlePanelEnd = playbackSource.indexOf(
      ") : (\n                  <>",
      singlePanelStart,
    );
    expect(singlePanelStart).toBeGreaterThan(-1);
    expect(singlePanelEnd).toBeGreaterThan(singlePanelStart);
    const singlePanelSource = playbackSource.slice(
      singlePanelStart,
      singlePanelEnd,
    );
    expect(singlePanelSource).toContain("listen-single-lyrics-panel");
    expect(singlePanelSource).toContain("onClick={props.onTogglePlayback}");
    expect(singlePanelSource).toContain("label={props.text.listen.next}");
    expect(singlePanelSource).toContain("onClick={props.onNext}");
    expect(singlePanelSource).toContain("<SkipForward");
    expect(playbackSource).toContain('command.command === "open-artist" && !isLive');
    expect(playbackSource).toContain("command.artist");
    expect(playbackSource).toContain(
      "listenArtistBrowseTrack(props.track, command.artist, artistLabelParts)",
    );
    expect(playbackSource).not.toContain(
      "listenArtistBrowseTrack(props.track, command.artist.name",
    );
    expect(css).toContain(".listen-workspace-companion-player__stack");
    expect(css).toContain(".listen-player-footer__bar--companion");
  });

  test("shows Live video in fullscreen without lyrics or Up Next", () => {
    const markup = renderToStaticMarkup(
      <ListenPlayerFooter
        mediaMode="cover"
        presentation="fullscreen"
        reserveWindowControls={false}
        airPlaySupported={false}
        hasVideo
        live
        muted={false}
        lyricsAvailable={false}
        text={text}
        onMediaModeChange={() => undefined}
        onToggleQueue={() => undefined}
        onToggleMute={() => undefined}
      />,
    );

    expect(markup).toContain(`aria-label="${text.listen.video}"`);
    expect(markup).not.toContain(`aria-label="${text.listen.lyrics}"`);
    expect(markup).not.toContain(`aria-label="${text.listen.upNext}"`);
  });

  test("fills the portal host and keeps companion content above a true bottom footer", async () => {
    const [appSource, pageSource, playbackSource, css, workspaceCss] =
      await Promise.all([
        Bun.file(new URL("../MainApp.tsx", import.meta.url)).text(),
        Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
        Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
        Bun.file(new URL("./listen.css", import.meta.url)).text(),
        readWorkspaceCss(),
      ]);

    expect(appSource).toContain('"h-full min-h-0 w-full flex-col"');
    expect(appSource).toContain('? "flex"\n              : "hidden"');
    expect(pageSource).toContain(
      "listen-content-surface app-workspace-primary-subpane relative flex h-full min-h-0 w-full",
    );
    expect(playbackSource).toContain(
      '? "listen-workspace-companion-player__grid"',
    );
    expect(playbackSource).toContain(
      '? "w-full justify-self-stretch"',
    );
    expect(css).toContain(".listen-workspace-companion-player__content {");
    expect(css).toContain("flex: 1 1 auto;");
    expect(css).not.toContain("--listen-workspace-companion-title-safe-inset");
    expect(workspaceCss).toContain("--app-workspace-companion-gutter: 1.25rem;");
    expect(css).toContain(
      "var(--app-workspace-companion-gutter, 1.25rem)",
    );
    expect(css).toContain(
      "padding-inline: var(--app-workspace-companion-gutter, 1.25rem);",
    );
    expect(css).toContain(".listen-workspace-companion-player__grid {");
    expect(css).toContain("grid-template-columns: minmax(0, 1fr);");
    expect(css).toContain("max-width: 100%;");
    expect(css).toContain(".listen-workspace-companion-player__stack {");
    expect(css).toContain("box-sizing: border-box;");
    expect(css).toContain(
      ".listen-workspace-companion-player__stack .listen-artwork-shell,",
    );
    expect(css).toContain("max-width: 100%;");
    expect(css).toContain("margin-top: auto;");
  });
});
