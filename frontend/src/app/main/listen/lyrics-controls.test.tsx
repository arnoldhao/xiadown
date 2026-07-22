import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { getXiaText } from "@/features/xiadown/shared";
import { ListenLyricsControls } from "@/app/main/listen/lyrics-controls";

const text = getXiaText("en");
const track = {
  videoId: "controls-track",
  title: "Controls Song",
  artist: "Controls Artist",
  durationSeconds: 180,
};

describe("listen lyrics controls", () => {
  test("renders one compact two-mode switch and a real menu trigger", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsControls
        placement="companion"
        text={text}
        track={track}
        lyrics={{
          videoId: track.videoId,
          kind: "synced",
          source: "LRCLIB",
          text: "Current line",
          lines: [{ startMs: 1000, durationMs: 2000, text: "Current line" }],
        }}
        currentTimeMs={1500}
        onLyricsChange={() => undefined}
        onRestoreAutomatic={() => undefined}
      />,
    );

    expect(markup).toContain('data-placement="companion"');
    expect(markup).toContain('role="radiogroup"');
    expect(markup.match(/role="radio"/g)).toHaveLength(2);
    expect(markup).toContain(`aria-label="${text.listen.lyricsDynamicMode}"`);
    expect(markup).toContain(`aria-label="${text.listen.lyricsFocusMode}"`);
    expect(markup).toContain('aria-haspopup="menu"');
    expect(markup).not.toContain('type="file"');
  });

  test("disables only synchronized presentation controls for Plain fallback", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsControls
        placement="overlay"
        text={text}
        track={track}
        lyrics={{
          videoId: track.videoId,
          kind: "plain",
          source: "online",
          text: "Plain fallback",
          lines: [],
        }}
        currentTimeMs={0}
        onLyricsChange={() => undefined}
        onRestoreAutomatic={() => undefined}
      />,
    );

    expect(markup).toContain('data-timing-available="false"');
    expect(markup).toContain('data-surface-role="control"');
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-shape="capsule"');
    expect(markup).toContain('aria-disabled="true"');
    expect(markup.match(/role="radio"[^>]*disabled=""/g)).toHaveLength(2);
    expect(markup).toContain('aria-haspopup="menu"');
  });

  test("keeps dialog state beside the menu without a Focus style picker", async () => {
    const source = await Bun.file(
      new URL("./lyrics-controls.tsx", import.meta.url),
    ).text();

    expect(source).toContain("<DropdownMenu open={menuOpen}");
    expect(source).toContain("open={matchOpen}");
    expect(source.indexOf("<ListenLyricsMatchDialog")).toBeGreaterThan(
      source.indexOf("</DropdownMenu>"),
    );
    expect(source).toContain("returnFocusRef={triggerRef}");
    expect(source).toContain("launchingMatchRef.current");
    expect(source).toContain("onCloseAutoFocus={(event) => {");
    expect(source.indexOf("setMatchOpen(true)")).toBeGreaterThan(
      source.indexOf("onCloseAutoFocus={(event) => {"),
    );
    expect(source).toContain("}, [lyricsVersionKey, trackKey]);");
    expect(source).not.toContain("[lyricsVersionKey, props.track, trackKey]");
    expect(source).toContain(
      "const focusStyle = DEFAULT_LISTEN_LYRICS_FOCUS_STYLE;",
    );
    expect(source).not.toContain("DropdownMenuRadioGroup");
    expect(source).not.toContain("saveListenLyricsFocusStylePreference");
    expect(source).not.toContain("lyricsFocusStyle");
    expect(source).toContain("props.text.listen.lyricsSearchOnline");
  });

  test("matches footer button sizing in companion and fullscreen hosts", async () => {
    const [layoutCss, appearanceCss] = await Promise.all([
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
    ]);

    expect(appearanceCss).toContain('[data-placement="companion"]');
    expect(appearanceCss).toContain('[data-placement="fullscreen"]');
    expect(appearanceCss).toContain("--app-button-inline-size: 2.5rem");
    expect(appearanceCss).toContain("--app-button-block-size: 2.5rem");
    expect(appearanceCss).toContain("--app-button-icon-size: 1rem");
    expect(
      layoutCss.match(
        /\.listen-lyrics-controls__mode-button,\s*\.listen-lyrics-controls__menu-trigger\s*\{([^}]*)\}/s,
      ),
    ).toBeNull();
  });
});
