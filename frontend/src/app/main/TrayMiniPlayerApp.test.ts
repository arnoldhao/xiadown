import { describe, expect, test } from "bun:test";

describe("tray mini-player appearance contract", () => {
  test("uses the same artwork-driven player surface as the sidebar preview", async () => {
    const source = await Bun.file(
      new URL("./TrayMiniPlayerApp.tsx", import.meta.url),
    ).text();

    expect(source).toContain("<ListenNowPlayingHoverPanel");
    expect(source).toContain('surface="tray"');
    expect(source).toContain("onControlCommand={sendTrayCommand}");
    expect(source).not.toContain("NowPlayingStatusRevealPanel");
    expect(source).not.toContain("<GlassSurface");
  });
});
