import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { getXiaText } from "@/features/xiadown/shared";
import { ListenLyricsCandidateCard } from "@/app/main/listen/lyrics-match-dialog";
import { resolveListenLyricsErrorPresentation } from "@/app/main/listen/lyrics-errors";
import { ListenLyricsWorkspace } from "@/app/main/listen/lyrics-workspace";

const text = getXiaText("en");

describe("listen lyrics workspace", () => {
  test("renders localized errors with a stable code and retry action", () => {
    const presentation = resolveListenLyricsErrorPresentation(
      text,
      Object.assign(
        new Error(
          'Get "https://lrclib.net/api/get-cached?artist_name=Private": Bad Gateway',
        ),
        { code: "lyrics_provider_unavailable", retryable: true },
      ),
    );
    const markup = renderToStaticMarkup(
      <ListenLyricsWorkspace
        text={text}
        track={{ title: "Unavailable Song", artist: "Artist" }}
        current={{
          error: presentation.message,
          errorCode: presentation.code,
          errorRetryable: presentation.retryable,
          onRetry: () => undefined,
        }}
        currentTimeMs={0}
      />,
    );

    expect(markup).toContain('role="alert"');
    expect(markup).toContain(text.listen.lyricsErrorProviderUnavailable);
    expect(markup).toContain("lyrics_provider_unavailable");
    expect(markup).toContain(text.listen.retry);
    expect(markup).not.toContain("lrclib.net");
    expect(markup).not.toContain("Bad Gateway");
  });

  test("keeps lyrics immersive and mounts page controls in one overlay slot", async () => {
    const [source, controlsSource] = await Promise.all([
      Bun.file(new URL("./lyrics-workspace.tsx", import.meta.url)).text(),
      Bun.file(new URL("./lyrics-controls.tsx", import.meta.url)).text(),
    ]);
    const markup = renderToStaticMarkup(
      <ListenLyricsWorkspace
        text={text}
        track={{
          videoId: "track-one",
          title: "Example Song",
          artist: "Example Artist",
          durationSeconds: 180,
        }}
        current={{
          lyrics: {
            videoId: "track-one",
            kind: "synced",
            source: "LRCLib",
            timingQuality: "line",
            text: "First line",
            lines: [
              {
                startMs: 1000,
                durationMs: 2000,
                text: "First line",
              },
            ],
          },
        }}
        currentTimeMs={1500}
        controls={<div data-testid="lyrics-controls" />}
      />,
    );

    expect(markup).toContain('data-listen-lyrics-workspace="true"');
    expect(markup).toContain('data-renderer-effective="scroll"');
    expect(markup).toContain('data-renderer-preference="scroll"');
    expect(markup).toContain('data-controls-placement="overlay"');
    expect(markup).toContain("listen-lyrics-workspace__controls");
    expect(markup.match(/data-testid="lyrics-controls"/g)).toHaveLength(1);
    expect(markup).not.toContain("app-glass-group");
    expect(markup).not.toContain("listen-lyrics-workspace__toolbar");
    expect(markup).toContain('data-listen-lyrics-renderer="scroll"');
    expect(source).not.toContain("<Sheet");
    expect(controlsSource).toContain('role="radiogroup"');
    expect(controlsSource).toContain("text.listen.lyricsDynamicMode");
    expect(controlsSource).toContain("text.listen.lyricsFocusMode");
    expect(controlsSource).toContain("props.text.listen.lyricsTimingOffset");
    expect(controlsSource).toContain("props.text.listen.lyricsSearchOnline");
    expect(controlsSource).not.toContain('type="file"');
  });

  test("uses plain only as fallback while preserving the preferred mode", async () => {
    const source = await Bun.file(new URL("./lyrics-workspace.tsx", import.meta.url)).text();
    const markup = renderToStaticMarkup(
      <ListenLyricsWorkspace
        text={text}
        track={{ title: "Plain Song", artist: "Artist" }}
        current={{
          lyrics: {
            videoId: "plain",
            kind: "plain",
            source: "LRCLib",
            text: "Plain line",
            lines: [],
          },
        }}
        currentTimeMs={0}
      />,
    );

    expect(markup).toContain('data-renderer-effective="plain"');
    expect(markup).toContain('data-renderer-preference="scroll"');
    expect(markup).toContain('data-listen-lyrics-renderer="plain"');
    expect(markup).not.toContain('type="file"');
    expect(source).toContain('renderer={renderer}');
    expect(source).not.toContain(
      'renderer={effectiveRenderer}',
    );
  });

  test("classifies unavailable lyrics as empty instead of plain", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsWorkspace
        text={text}
        track={{ title: "Unavailable Song", artist: "Artist" }}
        current={{
          lyrics: {
            videoId: "unavailable",
            kind: "unavailable",
            source: "none",
            text: "",
            lines: [],
          },
        }}
        currentTimeMs={0}
      />,
    );

    expect(markup).toContain('data-renderer-effective="empty"');
    expect(markup).not.toContain('data-renderer-effective="plain"');
  });

  test("does not expose a file-import control or service call", async () => {
    const [controlsSource, apiSource, playbackSource] = await Promise.all([
      Bun.file(new URL("./lyrics-controls.tsx", import.meta.url)).text(),
      Bun.file(new URL("./lyrics-api.ts", import.meta.url)).text(),
      Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
    ]);
    expect(controlsSource).not.toContain('type="file"');
    expect(controlsSource).not.toContain("onImportLyrics");
    expect(apiSource).not.toContain("ImportLyrics");
    expect(apiSource).not.toContain("ClearImportedLyrics");
    expect(playbackSource).not.toContain("callListenImportLyrics");
  });

  test("wires all real playback lyric surfaces through the workspace", async () => {
    const [playbackSource, controlsSource, workspaceSource, settingsSource] = await Promise.all([
      Bun.file(new URL("./Playback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./lyrics-controls.tsx", import.meta.url)).text(),
      Bun.file(new URL("./lyrics-workspace.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../settings/SettingsApp.tsx", import.meta.url)).text(),
    ]);

    expect(playbackSource.match(/<ListenLyricsWorkspace/g)).toHaveLength(4);
    expect(playbackSource.match(/surfaceActive=/g)).toHaveLength(4);
    expect(playbackSource.match(/<ListenLyricsControls/g)).toHaveLength(2);
    expect(playbackSource).toContain('props.presentation === "page" ? localLyricsControls');
    expect(playbackSource).toContain('props.presentation === "page" ? onlineLyricsControls');
    expect(playbackSource).toContain("onLyricsChange={handleLocalLyricsChange}");
    expect(playbackSource).toContain("onLyricsChange={handleOnlineLyricsChange}");
    expect(workspaceSource).toContain("useListenLyricsOffsetPreference(");
    expect(workspaceSource).not.toContain("setOffsetMs");
    expect(workspaceSource).not.toContain("readListenLyricsOffset(");
    expect(workspaceSource).toContain('lyrics.kind === "unavailable"');
    expect(controlsSource).toContain("returnFocusRef={triggerRef}");
    expect(controlsSource).toContain("setMatchOpen(false)");
    expect(playbackSource).toContain("const romanizedLyrics = romanizedLyricsSetting");
    expect(playbackSource).toContain("const pinyinLyrics = pinyinLyricsSetting");
    expect(playbackSource).toContain("const syncedLyricsEnabled = true");
    expect(playbackSource).not.toContain(
      "state.settings?.syncedLyricsEnabled",
    );
    expect(settingsSource).not.toContain(
      "saveSettingsPatch({ syncedLyricsEnabled",
    );
    expect(playbackSource).not.toContain("lyricsTranscriptionAvailable &&");
    expect(playbackSource).not.toContain(
      "lyricsAvailable={localLyricsAvailable || Boolean(localLyricsCurrentState.error)}",
    );
    expect(settingsSource).toContain(
      "checked={currentSettings?.romanizedLyrics !== false}",
    );
    expect(settingsSource).toContain(
      "checked={currentSettings?.pinyinLyrics !== false}",
    );
  });

  test("uses the governed dialog, request gates, and deduplicated preview cache", async () => {
    const [source, css] = await Promise.all([
      Bun.file(new URL("./lyrics-match-dialog.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
    ]);

    expect(source).toContain("<Dialog open={props.open}");
    expect(source).toContain("<DialogTitle>");
    expect(source).toContain("<DialogDescription>");
    expect(source).toContain("searchGateRef.current.isCurrent(request)");
    expect(source).toContain("previewGateRef.current.isCurrent(request)");
    expect(source).toContain("previewCacheRef.current");
    expect(source).toContain('role="progressbar"');
    expect(source).toContain("max-w-[min(52rem,calc(100vw-2rem))]");
    expect(source).not.toContain("max-w-[min(64rem");
    expect(css).toMatch(
      /\.listen-lyrics-match-dialog\s*\{[^}]*width:\s*min\(52rem,/s,
    );
    expect(css).toMatch(
      /\.listen-lyrics-match-dialog__workspace\s*\{[^}]*min-height:\s*19rem;/s,
    );
    expect(css).toMatch(
      /@media \(max-width: 760px\)[\s\S]*?\.listen-lyrics-match-dialog\s*\{[^}]*overflow-y:\s*auto;/,
    );
    expect(css).toMatch(
      /@media \(max-width: 540px\)[\s\S]*?\.listen-lyrics-match-dialog__workspace\s*\{[^}]*min-height:\s*16\.5rem;/,
    );
  });

  test("adapts its floating controls to the actual player or companion width", async () => {
    const [source, controlsSource, css] = await Promise.all([
      Bun.file(new URL("./lyrics-workspace.tsx", import.meta.url)).text(),
      Bun.file(new URL("./lyrics-controls.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
    ]);

    expect(css).toContain("container-name: listen-lyrics-workspace");
    expect(css).toContain("container-type: inline-size");
    expect(css).toContain(
      "@container listen-lyrics-workspace (max-width: 540px)",
    );
    expect(css).toContain(".listen-lyrics-workspace__controls");
    expect(css).toContain('.listen-lyrics-controls[data-placement="companion"]');
    expect(css).not.toContain("listen-lyrics-settings-sheet");
    expect(source).toContain('data-controls-placement={props.controls ? "overlay" : "none"}');
    expect(controlsSource).toContain('side="top"');
    expect(controlsSource).toContain('align="center"');
    expect(controlsSource).toContain('collisionPadding={12}');
  });

  test("renders candidate evidence, capabilities, and localized rejection", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsCandidateCard
        text={text}
        selected
        onSelect={() => undefined}
        candidate={{
          providerId: "lrclib",
          providerTrackId: "candidate-42",
          title: "Example Song (Live)",
          artist: "Example Artist",
          album: "Example Album",
          durationSeconds: 185,
          durationDiff: 5,
          hasSynced: true,
          hasPlain: true,
          timingQuality: "word",
          confidence: 62,
          titleScore: 58,
          artistScore: 96,
          albumScore: 88,
          durationScore: 72,
          accepted: false,
          rejection: "incompatible title version",
        }}
      />,
    );

    expect(markup).toContain('aria-pressed="true"');
    expect(markup).toContain('data-accepted="false"');
    expect(markup).toContain(text.listen.lyricsMatchWordSynced);
    expect(markup).toContain(text.listen.lyricsMatchPlain);
    expect(markup).toContain(text.listen.lyricsMatchRejectVersion);
    expect(markup).toContain("Example Album");
    expect(markup.match(/role="progressbar"/g)).toHaveLength(4);
    expect(markup).toContain('aria-valuenow="96"');
  });
});
