import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { getXiaText } from "@/features/xiadown/shared";

import { ListenPlayerProgress } from "./player-progress";

const text = getXiaText("en");

describe("ListenPlayerProgress", () => {
  test("renders the footer variant with a compact seekable remaining-time timeline", () => {
    const markup = renderToStaticMarkup(
      <ListenPlayerProgress
        variant="footer"
        playing
        progress={{ currentTime: 30, duration: 120, bufferedTime: 60 }}
        text={text}
        onSeek={() => undefined}
      />,
    );

    expect(markup).toContain("listen-player-progress--footer");
    expect(markup).toContain('data-variant="footer"');
    expect(markup).toContain('data-progress-state="ready"');
    expect(markup).toContain('data-playing="true"');
    expect(markup).toContain('type="range"');
    expect(markup).toContain('max="120"');
    expect(markup).toContain('value="30"');
    expect(markup).toContain(`aria-label="${text.listen.seek}"`);
    expect(markup).toContain('aria-valuetext="0:30 / 2:00"');
    expect(markup).toContain(">0:30</span>");
    expect(markup).toContain(">-1:30</span>");
    expect(markup).not.toContain(">2:00</span>");
  });

  test("keeps unknown footer progress finite and non-seekable", () => {
    const markup = renderToStaticMarkup(
      <ListenPlayerProgress
        variant="footer"
        progress={{
          currentTime: Number.NaN,
          duration: Number.POSITIVE_INFINITY,
          bufferedTime: Number.NaN,
        }}
        text={text}
        onSeek={() => undefined}
      />,
    );

    expect(markup).toContain('data-progress-state="unknown"');
    expect(markup).toContain('role="progressbar"');
    expect(markup).toContain(">0:00</span>");
    expect(markup).toContain(">—:—</span>");
    expect(markup).not.toContain('type="range"');
    expect(markup).not.toContain("NaN");
    expect(markup).not.toContain("Infinity");
  });

  test("represents footer Live playback without exposing a fake seek slider", () => {
    const markup = renderToStaticMarkup(
      <ListenPlayerProgress
        variant="footer"
        live
        playing
        progress={{ currentTime: 118, duration: 0, bufferedTime: 0 }}
        text={text}
        onSeek={() => undefined}
      />,
    );

    expect(markup).toContain('data-progress-state="live"');
    expect(markup).toContain('role="progressbar"');
    expect(markup).toContain('aria-valuenow="100"');
    expect(markup).toContain(`>${text.listen.liveBadge}</span>`);
    expect(markup).not.toContain('type="range"');
  });

  test("renders Live through the standard full timeline with zero remaining", () => {
    const markup = renderToStaticMarkup(
      <ListenPlayerProgress
        live
        playing
        progress={{ currentTime: 118, duration: 120, bufferedTime: 120 }}
        text={text}
        onSeek={() => undefined}
      />,
    );

    expect(markup).toContain('role="progressbar"');
    expect(markup).toContain('aria-valuemax="100"');
    expect(markup).toContain('aria-valuenow="100"');
    expect(markup).toContain('aria-valuetext="Live · -0:00"');
    expect(markup.match(/style="width:100%"/g)).toHaveLength(2);
    expect(markup).toContain(">1:58</span>");
    expect(markup).toContain(">-0:00</span>");
    expect(markup).not.toContain('type="range"');
    expect(markup).not.toContain("animate-ping");
    expect(markup).not.toContain(">Live</span>");
    expect(markup).not.toContain("text-red-600");
  });

  test("keeps loading status ahead of Live timeline presentation", async () => {
    const markup = renderToStaticMarkup(
      <ListenPlayerProgress
        live
        loading
        progress={{ currentTime: 0, duration: 0, bufferedTime: 0 }}
        text={text}
      />,
    );
    const appearanceCss = await Bun.file(
      new URL("../../../shared/styles/dream/listen.css", import.meta.url),
    ).text();

    expect(markup).toContain("listen-player-progress__loading-fill");
    expect(appearanceCss).toMatch(
      /\.listen-player-progress__loading-fill\s*\{[^}]*animation:\s*listen-progress-pulse/s,
    );
    expect(markup).toContain(`>${text.listen.loading}</span>`);
    expect(markup).not.toContain('role="progressbar"');
  });
});
