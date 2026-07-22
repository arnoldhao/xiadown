import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  SiteBrandIcon,
  siteBrandColor,
  siteBrandSurfaceStyle,
} from "./site-brand-icon";

describe("SiteBrandIcon appearance contract", () => {
  test("publishes only data-driven color variables from React", () => {
    expect(siteBrandColor("youtube")).toBe("#FF0000");
    expect(siteBrandColor("douyin")).toBe(siteBrandColor("tiktok"));
    expect(siteBrandColor("xiaohongshu")).toBe("#FF2442");
    expect(siteBrandSurfaceStyle("youtube")).toEqual({
      "--app-session-brand-color-default": "#FF0000",
    });
  });

  test("renders Douyin with the shared short-video brand glyph", () => {
    const douyin = renderToStaticMarkup(<SiteBrandIcon siteKey="douyin" />);
    const tiktok = renderToStaticMarkup(<SiteBrandIcon siteKey="tiktok" />);

    expect(douyin).toContain("app-site-brand-icon");
    expect(douyin.match(/<path d="([^"]+)"/i)?.[1]).toBe(
      tiktok.match(/<path d="([^"]+)"/i)?.[1],
    );
  });

  test("renders Xiaohongshu with its official brand glyph", () => {
    const markup = renderToStaticMarkup(<SiteBrandIcon siteKey="xiaohongshu" />);

    expect(markup).toContain("app-site-brand-icon");
    expect(markup).toContain("--app-site-brand-fallback-color:#FF2442");
    expect(markup).toContain("<path d=");
  });

  test("leaves the static color recipe to Dream CSS", async () => {
    const [markup, workflows] = await Promise.all([
      Promise.resolve(
        renderToStaticMarkup(<SiteBrandIcon siteKey="youtube" />),
      ),
      Bun.file(
        new URL("../styles/dream/workflows.css", import.meta.url),
      ).text(),
    ]);

    expect(markup).toContain("app-site-brand-icon");
    expect(markup).toContain("--app-site-brand-fallback-color:#FF0000");
    expect(markup).not.toContain("color:var(");
    expect(workflows).toContain(".app-site-brand-surface");
    expect(workflows).toContain(".app-site-brand-icon");
    expect(workflows).toContain("var(--app-site-brand-resolved-color) 10.2%");
    expect(workflows).toContain("var(--app-site-brand-resolved-color) 20%");
  });
});
