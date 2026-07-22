import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { YouTubeSubscriptionIconButton } from "@/app/youtube/YouTubeSubscriptionIconButton";

describe("YouTubeSubscriptionIconButton", () => {
  test("uses the same icon-only ghost contract for both subscription states", () => {
    const subscribe = renderToStaticMarkup(
      <YouTubeSubscriptionIconButton
        subscribed={false}
        label="Subscribe"
        onClick={() => {}}
      />,
    );
    const unsubscribe = renderToStaticMarkup(
      <YouTubeSubscriptionIconButton
        subscribed
        label="Unsubscribe"
        onClick={() => {}}
      />,
    );

    for (const markup of [subscribe, unsubscribe]) {
      expect(markup).toContain('data-variant="ghost"');
      expect(markup).toContain('data-shape="circle"');
      expect(markup).toContain('data-size="compactIcon"');
      expect(markup).toContain("youtube-subscription-icon-button");
      expect(markup).not.toContain("<span>");
    }
    expect(subscribe).toContain('aria-label="Subscribe"');
    expect(subscribe).toContain('title="Subscribe"');
    expect(subscribe).toContain('aria-pressed="false"');
    expect(subscribe).toContain("lucide-user-plus");
    expect(unsubscribe).toContain('aria-label="Unsubscribe"');
    expect(unsubscribe).toContain('title="Unsubscribe"');
    expect(unsubscribe).toContain('aria-pressed="true"');
    expect(unsubscribe).toContain("lucide-user-check");
  });

  test("keeps the busy state accessible without adding text chrome", () => {
    const markup = renderToStaticMarkup(
      <YouTubeSubscriptionIconButton
        subscribed={false}
        busy
        label="Subscribe"
        onClick={() => {}}
      />,
    );

    expect(markup).toContain('aria-busy="true"');
    expect(markup).toContain("disabled");
    expect(markup).toContain("lucide-loader-circle");
    expect(markup).toContain("app-motion-spin");
  });

  test("Dream CSS keeps pressed status tonal and borderless", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/youtube.css", import.meta.url),
    ).text();
    const pressedRule = css.match(
      /\.youtube-subscription-icon-button[^{}]*\[aria-pressed="true"\]\s*\{([^}]*)\}/s,
    )?.[1];

    expect(css).toContain("--app-button-border: 0");
    expect(pressedRule).toContain("border-color: transparent");
    expect(pressedRule).toContain("background: transparent");
    expect(pressedRule).toContain("--app-accent-text");
  });
});
