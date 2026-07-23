import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  SECONDARY_REVEAL_CLOSE_DELAY,
  SECONDARY_REVEAL_OPEN_DELAY,
  SecondaryReveal,
} from "./secondary-reveal";

describe("SecondaryReveal", () => {
  test("uses a restrained hover delay and a pointer-bridge grace period", () => {
    expect(SECONDARY_REVEAL_OPEN_DELAY).toBe(300);
    expect(SECONDARY_REVEAL_CLOSE_DELAY).toBe(180);
  });

  test("gives the explicit trigger non-modal disclosure semantics", () => {
    const markup = renderToStaticMarkup(
      <SecondaryReveal ariaLabel="Player details" content={<div>panel</div>}>
        {({ anchorProps, triggerProps }) => (
          <div {...anchorProps}>
            <button {...triggerProps} type="button">
              player
            </button>
          </div>
        )}
      </SecondaryReveal>,
    );

    expect(markup).toContain('aria-haspopup="dialog"');
    expect(markup).toContain('aria-expanded="false"');
    expect(markup).toContain("aria-controls=");
    expect(markup).not.toContain("panel");
  });

  test("contracts Escape, outside-pointer dismissal, and WebView-safe hover", async () => {
    const source = await Bun.file(
      new URL("./secondary-reveal.tsx", import.meta.url),
    ).text();

    expect(source).not.toContain("matchMedia");
    expect(source).not.toContain("pointer: fine");
    expect(source).toContain('event.pointerType === "touch"');
    expect(source).toContain("onMouseEnter");
    expect(source).toContain("onMouseOver");
    expect(source).toContain("onMouseOut");
    expect(source).toContain("onMouseMove");
    expect(source).toContain(
      'document.addEventListener("mousemove", handleMouseMove, true)',
    );
    expect(source).toContain(
      "containsEventTarget(event.currentTarget, event.relatedTarget)",
    );
    expect(source).toContain("onActivate");
    expect(source).toContain("pinOnClick = true");
    expect(source).toContain("if (!pinOnClick && event.detail > 0)");
    expect(source).toContain("if (!pinOnClick)");
    expect(source).toContain('event.key === "Escape"');
    expect(source).toContain('document.addEventListener("pointerdown"');
    expect(source).toContain('role="dialog"');
    expect(source).toContain('aria-modal={false}');
    expect(source).toContain('data-positioned={position ? "true" : "false"}');
    expect(source).not.toContain('visibility: position ? "visible" : "hidden"');
    expect(source).toContain("panelHoveredRef.current");
    expect(source).toContain("suppressNextFocusOpenRef.current = true");
    expect(source).toContain("close: () => closeReveal(true)");
  });

  test("keeps shared layout and motion owned by the Dream CSS entry", async () => {
    const [entry, components, motion, workspaceActivity] = await Promise.all([
      Bun.file(new URL("../styles/dream.css", import.meta.url)).text(),
      Bun.file(
        new URL("../styles/dream/components.css", import.meta.url),
      ).text(),
      Bun.file(new URL("../styles/dream/motion.css", import.meta.url)).text(),
      Bun.file(
        new URL("../styles/dream/activity.css", import.meta.url),
      ).text(),
    ]);

    expect(entry).toContain('@import "./dream/components.css";');
    expect(entry).toContain('@import "./dream/motion.css";');
    expect(components).toMatch(
      /\.app-secondary-reveal__positioner\s*\{[^}]*position:\s*fixed;[^}]*z-index:\s*var\(--app-layer-popover\);[^}]*pointer-events:\s*auto;/s,
    );
    expect(components).toMatch(
      /\.app-secondary-reveal__positioner\[data-positioned="true"\]\s*\{[^}]*visibility:\s*visible;/s,
    );
    expect(components).toMatch(
      /\.app-secondary-reveal__panel\s*\{[^}]*width:\s*max-content;[^}]*max-width:\s*calc\(100vw - 16px\);/s,
    );
    expect(motion).toContain("animation-name: app-secondary-reveal-in-left;");
    expect(motion).toContain(
      "animation-name: app-secondary-reveal-in-right;",
    );
    expect(motion).toContain("@keyframes app-secondary-reveal-in-left");
    expect(motion).toContain("@keyframes app-secondary-reveal-in-right");
    expect(motion).toMatch(
      /@media \(prefers-reduced-motion: reduce\)\s*\{\s*\.app-secondary-reveal__panel\s*\{\s*animation:\s*none;/s,
    );
    expect(workspaceActivity).not.toContain(".app-secondary-reveal__");
    expect(workspaceActivity).not.toContain(
      "@keyframes app-secondary-reveal-in-",
    );
  });
});
